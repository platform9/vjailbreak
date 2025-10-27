package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	retryablehttp "github.com/hashicorp/go-retryablehttp"
	"github.com/pkg/errors"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/constants"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/k8sutils"
	"github.com/platform9/vjailbreak/v2v-helper/vm"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/extensions/volumeactions"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumes"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/bootfromvolume"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/volumeattach"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/projects"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/groups"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"
)

type OpenStackClients struct {
	BlockStorageClient *gophercloud.ServiceClient
	ComputeClient      *gophercloud.ServiceClient
	NetworkingClient   *gophercloud.ServiceClient
	K8sClient          client.Client
	AuthURL, Tenant    string
}

type OpenStackMetadata struct {
	UUID string `json:"uuid"`
}

var (
	cachedMetadata *OpenStackMetadata
	metadataMutex  sync.RWMutex
)

func GetCurrentInstanceUUID() (string, error) {

	// Step 1. Path with a read lock
	// First Check if the data is already cached. This read lock allows multiple
	// Goroutines to read the cached data concurrently.

	metadataMutex.RLock()
	if cachedMetadata != nil {
		// If cached, return it immediately.
		defer metadataMutex.RUnlock()
		return cachedMetadata.UUID, nil
	}

	// Release the read lock before we get the Write lock.
	metadataMutex.RUnlock()

	// Step 2. Path with write lock
	// Acquire a write lock to fetch and cache the metadata.
	metadataMutex.Lock()
	defer metadataMutex.Unlock()

	// Check again if another goroutine has already fetched the metadata.

	if cachedMetadata != nil {
		return cachedMetadata.UUID, nil
	}

	// Step 4: Fetch metadata from the OpenStack metadata service

	client := retryablehttp.NewClient()
	client.RetryMax = 5
	client.Logger = nil
	req, err := retryablehttp.NewRequest("GET", "http://169.254.169.254/openstack/latest/meta_data.json", http.NoBody)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %s", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get response: %s", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %s", err)
	}

	var metadata OpenStackMetadata
	if err := json.Unmarshal(body, &metadata); err != nil {
		return "", fmt.Errorf("failed to unmarshal metadata: %s", err)
	}
	cachedMetadata = &metadata
	return metadata.UUID, nil
}

// create a new volume
func (osclient *OpenStackClients) CreateVolume(name string, size int64, ostype string, uefi bool, volumetype string, setRDMLabel bool) (*volumes.Volume, error) {
	blockStorageClient := osclient.BlockStorageClient

	PrintLog(fmt.Sprintf("OPENSTACK API: Creating volume with name %s with size %d, for OS type %s, UEFI %v, volume type %s, authurl %s, tenant %s", name, size, ostype, uefi, volumetype, osclient.AuthURL, osclient.Tenant))
	opts := volumes.CreateOpts{
		VolumeType: volumetype,
		Size:       int(math.Ceil(float64(size) / (1024 * 1024 * 1024))),
		Name:       name,
	}

	// Add 1GB to the size to account for the extra space
	opts.Size += 1

	volume, err := volumes.Create(blockStorageClient, opts).Extract()
	if err != nil {
		return nil, fmt.Errorf("failed to create volume: %s", err)
	}

	err = osclient.WaitForVolume(volume.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for volume: %s", err)
	}
	volume, err = volumes.Get(osclient.BlockStorageClient, volume.ID).Extract()
	if err != nil {
		return nil, fmt.Errorf("failed to get volume: %s", err)
	}
	PrintLog(fmt.Sprintf("Volume created successfully. current status %s", volume.Status))

	if uefi {
		err = osclient.SetVolumeUEFI(volume)
		if err != nil {
			return nil, fmt.Errorf("failed to set volume uefi: %s", err)
		}
	}

	if strings.ToLower(ostype) == constants.OSFamilyWindows {
		err = osclient.SetVolumeImageMetadata(volume, setRDMLabel)
		if err != nil {
			return nil, fmt.Errorf("failed to set volume image metadata: %s", err)
		}
	}

	err = osclient.EnableQGA(volume)
	if err != nil {
		return nil, err
	}

	return volume, nil
}

func (osclient *OpenStackClients) DeleteVolume(volumeID string) error {
	PrintLog(fmt.Sprintf("OPENSTACK API: Deleting volume with ID %s, authurl %s, tenant %s", volumeID, osclient.AuthURL, osclient.Tenant))
	err := volumes.Delete(osclient.BlockStorageClient, volumeID, volumes.DeleteOpts{}).ExtractErr()
	if err != nil {
		return fmt.Errorf("failed to delete volume: %s", err)
	}
	return nil
}

func (osclient *OpenStackClients) WaitForVolume(volumeID string) error {
	// Get vjailbreak settings
	vjailbreakSettings, err := k8sutils.GetVjailbreakSettings(context.Background(), osclient.K8sClient)
	if err != nil {
		return errors.Wrap(err, "failed to get vjailbreak settings")
	}
	PrintLog(fmt.Sprintf("OPENSTACK API: Waiting for volume %s to become available, authurl %s, tenant %s", volumeID, osclient.AuthURL, osclient.Tenant))
	for i := 0; i < vjailbreakSettings.VolumeAvailableWaitRetryLimit; i++ {
		volume, err := volumes.Get(osclient.BlockStorageClient, volumeID).Extract()
		if err != nil {
			return fmt.Errorf("failed to get volume: %s", err)
		}
		if volume.Status == "error" {
			return fmt.Errorf("volume %s is in error state", volumeID)
		}
		instanceID, err := GetCurrentInstanceUUID()
		if err != nil {
			return fmt.Errorf("failed to get instance ID: %s", err)
		}

		// Check if the volume is available from nova side as well
		server, err := servers.Get(osclient.ComputeClient, instanceID).Extract()
		if err != nil {
			return fmt.Errorf("failed to get server: %s", err)
		}

		// get the attachments from the server
		found := false
		attachments := server.AttachedVolumes
		for _, attachment := range attachments {
			if attachment.ID == volumeID {
				found = true
				break
			}
		}
		// Check if volume is available and there are no attachments to the volume
		if volume.Status == "available" && len(volume.Attachments) == 0 && !found {
			return nil
		}
		fmt.Printf("Volume %s is still attached to server retrying %d times\n", volumeID, i)
		time.Sleep(time.Duration(vjailbreakSettings.VolumeAvailableWaitIntervalSeconds) * time.Second) // Wait for 5 seconds before checking again
	}
	return fmt.Errorf("volume did not become available within %d seconds", vjailbreakSettings.VolumeAvailableWaitRetryLimit*vjailbreakSettings.VolumeAvailableWaitIntervalSeconds)
}

func (osclient *OpenStackClients) AttachVolumeToVM(volumeID string) error {
	instanceID, err := GetCurrentInstanceUUID()
	if err != nil {
		return fmt.Errorf("failed to get instance ID: %s", err)
	}
	PrintLog(fmt.Sprintf("OPENSTACK API: Attaching volume %s to VM %s, authurl %s, tenant %s", volumeID, instanceID, osclient.AuthURL, osclient.Tenant))

	vjailbreakSettings, err := k8sutils.GetVjailbreakSettings(context.Background(), osclient.K8sClient)
	if err != nil {
		return errors.Wrap(err, "failed to get vjailbreak settings")
	}
	for i := 0; i < vjailbreakSettings.VolumeAvailableWaitRetryLimit; i++ {
		_, err = volumeattach.Create(osclient.ComputeClient, instanceID, volumeattach.CreateOpts{
			VolumeID:            volumeID,
			DeleteOnTermination: false,
		}).Extract()
		if err == nil || strings.Contains(err.Error(), "already attached") {
			err = nil
			break
		}
		time.Sleep(time.Duration(vjailbreakSettings.VolumeAvailableWaitIntervalSeconds) * time.Second) // Wait for 5 seconds before checking again
	}
	if err != nil {
		return fmt.Errorf("failed to attach volume to VM: %s", err)
	}

	PrintLog(fmt.Sprintf("OPENSTACK API: Waiting for volume attachment for volume %s to VM %s, authurl %s, tenant %s", volumeID, instanceID, osclient.AuthURL, osclient.Tenant))
	err = osclient.WaitForVolumeAttachment(volumeID)
	if err != nil {
		return fmt.Errorf("failed to wait for volume attachment: %s", err)
	}

	return nil
}

func (osclient *OpenStackClients) FindDevice(volumeID string) (string, error) {
	files, err := os.ReadDir("/dev/disk/by-id/")
	if err != nil {
		return "", fmt.Errorf("failed to read directory: %s", err)
	}

	for _, file := range files {
		if strings.Contains(file.Name(), volumeID[:18]) {
			devicePath, err := filepath.EvalSymlinks(filepath.Join("/dev/disk/by-id/", file.Name()))
			if err != nil {
				return "", fmt.Errorf("failed to evaluate symlink: %s", err)
			}

			return devicePath, nil
		}
	}

	return "", nil
}

func (osclient *OpenStackClients) WaitForVolumeAttachment(volumeID string) error {
	instanceID, err := GetCurrentInstanceUUID()
	if err != nil {
		return fmt.Errorf("failed to get instance ID: %s", err)
	}
	PrintLog(fmt.Sprintf("OPENSTACK API: Waiting for volume attachment for volume %s to VM %s, authurl %s, tenant %s", volumeID, instanceID, osclient.AuthURL, osclient.Tenant))
	for i := 0; i < constants.MaxIntervalCount; i++ {
		devicePath, _ := osclient.FindDevice(volumeID)
		if devicePath != "" {
			return nil
		}
		time.Sleep(5 * time.Second) // Wait for 5 seconds before checking again
	}
	return fmt.Errorf("volume attachment not found within %d seconds", constants.MaxIntervalCount*5)
}

func (osclient *OpenStackClients) DetachVolumeFromVM(volumeID string) error {
	instanceID, err := GetCurrentInstanceUUID()
	if err != nil {
		return fmt.Errorf("failed to get instance ID: %s", err)
	}
	PrintLog(fmt.Sprintf("OPENSTACK API: Detaching volume %s from VM %s, authurl %s, tenant %s", volumeID, instanceID, osclient.AuthURL, osclient.Tenant))

	for i := 0; i < constants.MaxIntervalCount; i++ {
		err = volumeattach.Delete(osclient.ComputeClient, instanceID, volumeID).ExtractErr()
		if err == nil {
			break
		}
		time.Sleep(5 * time.Second) // Wait for 5 seconds before checking again
	}
	if err != nil && !strings.Contains(err.Error(), "is not attached") {
		return fmt.Errorf("failed to detach volume from VM: %s", err)
	}

	return nil
}

func (osclient *OpenStackClients) EnableQGA(volume *volumes.Volume) error {
	PrintLog(fmt.Sprintf("OPENSTACK API: Enabling QGA for volume %s, authurl %s, tenant %s", volume.ID, osclient.AuthURL, osclient.Tenant))
	options := volumeactions.ImageMetadataOpts{
		Metadata: map[string]string{
			"hw_qemu_guest_agent": "yes",
			"hw_video_model":      "virtio",
			"hw_pointer_model":    "usbtablet",
		},
	}
	err := volumeactions.SetImageMetadata(osclient.BlockStorageClient, volume.ID, options).ExtractErr()
	if err != nil {
		return fmt.Errorf("failed to detach volume from VM: %s", err)
	}
	return nil
}

func (osclient *OpenStackClients) SetVolumeUEFI(volume *volumes.Volume) error {
	PrintLog(fmt.Sprintf("OPENSTACK API: Setting UEFI for volume %s, authurl %s, tenant %s", volume.ID, osclient.AuthURL, osclient.Tenant))
	options := volumeactions.ImageMetadataOpts{
		Metadata: map[string]string{
			"hw_firmware_type": "uefi",
		},
	}
	err := volumeactions.SetImageMetadata(osclient.BlockStorageClient, volume.ID, options).ExtractErr()
	if err != nil {
		return fmt.Errorf("failed to set volume image metadata hw_firmware_type to uefi: %s", err)
	}
	return nil
}

func (osclient *OpenStackClients) SetVolumeImageMetadata(volume *volumes.Volume, setRDMLabel bool) error {
	options := volumeactions.ImageMetadataOpts{
		Metadata: map[string]string{
			"hw_disk_bus": "virtio",
			"os_type":     "windows",
		},
	}
	if setRDMLabel {
		options.Metadata["hw_scsi_model"] = "virtio-scsi"
	}
	err := volumeactions.SetImageMetadata(osclient.BlockStorageClient, volume.ID, options).ExtractErr()
	if err != nil {
		return fmt.Errorf("failed to set volume image metadata for windows: %s", err)
	}
	return nil
}

func (osclient *OpenStackClients) SetVolumeBootable(volume *volumes.Volume) error {
	PrintLog(fmt.Sprintf("OPENSTACK API: Setting volume %s as bootable, authurl %s, tenant %s", volume.ID, osclient.AuthURL, osclient.Tenant))
	options := volumeactions.BootableOpts{
		Bootable: true,
	}
	err := volumeactions.SetBootable(osclient.BlockStorageClient, volume.ID, options).ExtractErr()
	if err != nil {
		return fmt.Errorf("failed to set volume as bootable: %s", err)
	}
	return nil
}

func (osclient *OpenStackClients) GetClosestFlavour(cpu int32, memory int32) (*flavors.Flavor, error) {
	PrintLog(fmt.Sprintf("OPENSTACK API: Getting closest flavor for %d vCPUs and %d MB RAM, authurl %s, tenant %s", cpu, memory, osclient.AuthURL, osclient.Tenant))
	allPages, err := flavors.ListDetail(osclient.ComputeClient, nil).AllPages()
	if err != nil {
		return nil, fmt.Errorf("failed to list flavors: %s", err)
	}

	allFlavors, err := flavors.ExtractFlavors(allPages)
	if err != nil {
		return nil, fmt.Errorf("failed to extract all flavors: %s", err)
	}

	PrintLog(fmt.Sprintf("Current requirements: %d CPUs and %d MB of RAM", cpu, memory))

	bestFlavor := new(flavors.Flavor)
	bestFlavor.VCPUs = constants.MaxCPU
	bestFlavor.RAM = constants.MaxRAM
	// Find the smallest flavor that meets the requirements
	for _, flavor := range allFlavors {
		if flavor.VCPUs >= int(cpu) && flavor.RAM >= int(memory) {
			if flavor.VCPUs < bestFlavor.VCPUs || (flavor.VCPUs == bestFlavor.VCPUs && flavor.RAM < bestFlavor.RAM) {
				bestFlavor = &flavor
			}
		}
	}

	if bestFlavor.VCPUs != constants.MaxCPU {
		PrintLog(fmt.Sprintf("The best flavor is:\nName: %s, ID: %s, RAM: %dMB, VCPUs: %d, Disk: %dGB\n",
			bestFlavor.Name, bestFlavor.ID, bestFlavor.RAM, bestFlavor.VCPUs, bestFlavor.Disk))
	} else {
		PrintLog("No suitable flavor found.")
		return nil, fmt.Errorf("no suitable flavor found for %d vCPUs and %d MB RAM", cpu, memory)
	}

	return bestFlavor, nil
}

func (osclient *OpenStackClients) GetFlavor(flavorId string) (*flavors.Flavor, error) {
	PrintLog(fmt.Sprintf("OPENSTACK API: Getting flavor %s, authurl %s, tenant %s", flavorId, osclient.AuthURL, osclient.Tenant))
	flavor, err := flavors.Get(osclient.ComputeClient, flavorId).Extract()
	if err != nil {
		return nil, fmt.Errorf("failed to get flavor: %s", err)
	}
	return flavor, nil
}

func (osclient *OpenStackClients) GetNetwork(networkname string) (*networks.Network, error) {
	PrintLog(fmt.Sprintf("OPENSTACK API: Fetching network %s, authurl %s, tenant %s", networkname, osclient.AuthURL, osclient.Tenant))
	allPages, err := networks.List(osclient.NetworkingClient, nil).AllPages()
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %s", err)
	}

	allNetworks, err := networks.ExtractNetworks(allPages)
	if err != nil {
		return nil, fmt.Errorf("failed to extract all networks: %s", err)
	}

	for _, network := range allNetworks {
		if network.Name == networkname {
			return &network, nil
		}
	}
	return nil, fmt.Errorf("network not found")
}

func (osclient *OpenStackClients) GetPort(portID string) (*ports.Port, error) {
	PrintLog(fmt.Sprintf("OPENSTACK API: Fetching port %s, authurl %s, tenant %s", portID, osclient.AuthURL, osclient.Tenant))
	port, err := ports.Get(osclient.NetworkingClient, portID).Extract()
	if err != nil {
		return nil, fmt.Errorf("failed to get port: %s", err)
	}
	return port, nil
}
func (osclient *OpenStackClients) GetSubnet(subnetList []string, ip string) (*subnets.Subnet, error) {
	parsedIp := net.ParseIP(ip)
	if parsedIp == nil {
		return nil, fmt.Errorf("invalid IP address: %s", ip)
	}
	for _, subnet := range subnetList {
		sn, err := subnets.Get(osclient.NetworkingClient, subnet).Extract()
		if err != nil {
			return nil, fmt.Errorf("failed to get subnet: %s", err)
		}
		_, ipNet, err := net.ParseCIDR(sn.CIDR)
		if err != nil {
			return nil, fmt.Errorf("failed to parse CIDR %q for subnet %s : %w", sn.CIDR, sn.ID, err)
		}
		if ipNet.Contains(parsedIp) {
			PrintLog(fmt.Sprintf("Subnet %s contains IP %s", sn.ID, ip))
			return sn, nil
		}
	}
	return nil, fmt.Errorf("IP %s is not in any of the subnets %v", ip, subnetList)
}
func (osclient *OpenStackClients) CreatePort(network *networks.Network, mac, ip, vmname string, securityGroups []string, fallbackToDHCP bool) (*ports.Port, error) {
	PrintLog(fmt.Sprintf("OPENSTACK API: Creating port for network %s, authurl %s, tenant %s with MAC address %s and IP address %s", network.ID, osclient.AuthURL, osclient.Tenant, mac, ip))
	pages, err := ports.List(osclient.NetworkingClient, ports.ListOpts{
		NetworkID:  network.ID,
		MACAddress: mac,
	}).AllPages()
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %s", err)
	}

	portList, err := ports.ExtractPorts(pages)
	if err != nil {
		return nil, err
	}

	for _, port := range portList {
		if port.MACAddress == mac {
			if len(port.FixedIPs) > 0 {
				foundPortIP := port.FixedIPs[0].IPAddress
				if ip != "" && foundPortIP != ip {
					return nil, fmt.Errorf("port conflict: a port with MAC %s already exists but has IP %s, while IP %s was requested", mac, foundPortIP, ip)
				}
			}
			PrintLog(fmt.Sprintf("Port with MAC address %s already exists, ID: %s", mac, port.ID))
			return &port, nil
		}
	}

	PrintLog(fmt.Sprintf("Port with MAC address %s does not exist, creating new port, trying with same IP address: %s", mac, ip))

	// Check if subnet is valid to avoid panic.
	if len(network.Subnets) == 0 {
		return nil, fmt.Errorf("no subnets found for network: %s", network.ID)
	}

	createOpts := ports.CreateOpts{
		Name:           "port-" + vmname,
		NetworkID:      network.ID,
		MACAddress:     mac,
		SecurityGroups: &securityGroups,
	}

	if ip != "" {
		PrintLog(fmt.Sprintf("Subnets in network  : %v", network.Subnets))
		
		subnet, err := osclient.GetSubnet(network.Subnets, ip)
		fixedIP := []ports.IP{}
		if err != nil  && !fallbackToDHCP {
			return nil, fmt.Errorf("failed to get subnet: %s", err)
		}
		
		if subnet != nil{				
			fixedIP = append(fixedIP, ports.IP{
			SubnetID:  subnet.ID,
			IPAddress: ip,
		    })
		}
		createOpts.FixedIPs = fixedIP
	}

	port, err := ports.Create(osclient.NetworkingClient, createOpts).Extract()
	if err != nil {
		// If static IP assignment fails, check the fallback flag
		if !fallbackToDHCP {
			// If fallback is disabled, return the error immediately
			return nil, errors.Wrapf(err, "failed to create port with static IP %s, and fallback to DHCP is disabled", ip)
		}
		// If fallback is enabled, proceed with the DHCP attempt
		PrintLog(fmt.Sprintf("Could Not Use IP: %s, using DHCP to create Port", ip))
		dhcpPort, dhcpErr := ports.Create(osclient.NetworkingClient, ports.CreateOpts{
			Name:           "port-" + vmname,
			NetworkID:      network.ID,
			MACAddress:     mac,
			SecurityGroups: &securityGroups,
		}).Extract()

		if dhcpErr != nil {
			return nil, errors.Wrap(dhcpErr, "failed to create port with DHCP after static IP failed")
		}

		PrintLog(fmt.Sprintf("Port created with DHCP instead of static IP %s. Port ID: %s", ip, dhcpPort.ID))
		return dhcpPort, nil
	}

	PrintLog(fmt.Sprintf("Port created with ID: %s", port.ID))
	return port, nil
}

func (osclient *OpenStackClients) CreateVM(flavor *flavors.Flavor, networkIDs, portIDs []string, vminfo vm.VMInfo, availabilityZone string, securityGroups []string, vjailbreakSettings k8sutils.VjailbreakSettings, useFlavorless bool) (*servers.Server, error) {
	uuid := ""
	bootableDiskIndex := 0
	for idx, disk := range vminfo.VMDisks {
		if disk.Boot {
			uuid = disk.OpenstackVol.ID
			bootableDiskIndex = idx
			break
		}
	}
	if uuid == "" {
		return nil, fmt.Errorf("unable to determine boot volume for VM: %s", vminfo.Name)
	}
	PrintLog(fmt.Sprintf("OPENSTACK API: Creating VM %s, authurl %s, tenant %s with flavor %s in availability zone %s", vminfo.Name, osclient.AuthURL, osclient.Tenant, flavor.ID, availabilityZone))
	blockDevice := bootfromvolume.BlockDevice{
		DeleteOnTermination: false,
		DestinationType:     bootfromvolume.DestinationVolume,
		SourceType:          bootfromvolume.SourceVolume,
		UUID:                uuid,
	}
	// Create the server
	openstacknws := []servers.Network{}
	for idx := range networkIDs {
		openstacknws = append(openstacknws, servers.Network{
			UUID: networkIDs[idx],
			Port: portIDs[idx],
		})
	}
	serverCreateOpts := servers.CreateOpts{
		Name:           vminfo.Name,
		FlavorRef:      flavor.ID,
		Networks:       openstacknws,
		SecurityGroups: securityGroups,
	}

	if useFlavorless {
		PrintLog(fmt.Sprintf("Using flavorless provisioning. Adding hotplug metadata: CPU=%d, Memory=%dMB", vminfo.CPU, vminfo.Memory))
		serverCreateOpts.Metadata = map[string]string{
			constants.HotplugCPUKey:       fmt.Sprintf("%d", vminfo.CPU),
			constants.HotplugMemoryKey:    fmt.Sprintf("%d", vminfo.Memory),
			constants.HotplugCPUMaxKey:    fmt.Sprintf("%d", vminfo.CPU),
			constants.HotplugMemoryMaxKey: fmt.Sprintf("%d", vminfo.Memory),
		}
	}

	if availabilityZone != "" && !strings.Contains(availabilityZone, constants.PCDClusterNameNoCluster) {
		// for PCD, this will be set to cluster name
		serverCreateOpts.AvailabilityZone = availabilityZone
	}
	if len((vminfo.RDMDisks)) > 0 {
		if serverCreateOpts.Metadata == nil {
			serverCreateOpts.Metadata = map[string]string{}
		}
		serverCreateOpts.Metadata["hw_scsi_reservations"] = "true"
	}
	createOpts := bootfromvolume.CreateOptsExt{
		CreateOptsBuilder: serverCreateOpts,
		BlockDevice:       []bootfromvolume.BlockDevice{blockDevice},
	}

	for _, disk := range vminfo.RDMDisks {
		// Set the Nova API version to 2.6
		osclient.ComputeClient.Microversion = "2.60"
		blockDevice := bootfromvolume.BlockDevice{
			DeleteOnTermination: false,
			DestinationType:     bootfromvolume.DestinationVolume,
			SourceType:          bootfromvolume.SourceVolume,
			UUID:                disk.Status.CinderVolumeID,
			DeviceType:          "lun",
			DiskBus:             "scsi",
			BootIndex:           -1,
		}
		createOpts.BlockDevice = append(createOpts.BlockDevice, blockDevice)
	}

	// Wait for disks to become available
	for _, disk := range vminfo.VMDisks {
		err := osclient.WaitForVolume(disk.OpenstackVol.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to wait for volume to become available: %s", err)
		}
	}

	server, err := servers.Create(osclient.ComputeClient, createOpts).Extract()
	if err != nil {
		return nil, fmt.Errorf("failed to create server: %s", err)
	}

	err = servers.WaitForStatus(osclient.ComputeClient, server.ID, "ACTIVE", vjailbreakSettings.VMActiveWaitRetryLimit*vjailbreakSettings.VMActiveWaitIntervalSeconds)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for server to become active: %s", err)
	}

	PrintLog(fmt.Sprintf("Server created with ID: %s, Attaching Additional Disks", server.ID))

	for _, disk := range append(vminfo.VMDisks[:bootableDiskIndex], vminfo.VMDisks[bootableDiskIndex+1:]...) {
		_, err := volumeattach.Create(osclient.ComputeClient, server.ID, volumeattach.CreateOpts{
			VolumeID:            disk.OpenstackVol.ID,
			DeleteOnTermination: false,
		}).Extract()
		if err != nil {
			return nil, fmt.Errorf("failed to attach volume to VM: %s", err)
		}
	}
	return server, nil
}

func (osclient *OpenStackClients) WaitUntilVMActive(vmID string) (bool, error) {
	result, err := servers.Get(osclient.ComputeClient, vmID).Extract()
	if err != nil {
		return false, fmt.Errorf("failed to get server: %s", err)
	}
	if result.Status != "ACTIVE" {
		return false, fmt.Errorf("server is not active")
	}
	return true, nil
}

func (osclient *OpenStackClients) GetSecurityGroupIDs(groupNames []string, projectName string) ([]string, error) {
	if len(groupNames) == 0 {
		return nil, nil
	}

	if projectName == "" {
		return nil, fmt.Errorf("projectName is required for security group lookup")
	}

	//check if string is UUID
	isUUID := func(s string) bool {
		re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
		return re.MatchString(s)
	}

	//build a map name -> ID
	identityClient, err := openstack.NewIdentityV3(osclient.NetworkingClient.ProviderClient, gophercloud.EndpointOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to create identity client: %w", err)
	}

	listOpts := projects.ListOpts{Name: projectName}
	allPages, err := projects.List(identityClient, listOpts).AllPages()
	if err != nil {
		return nil, fmt.Errorf("failed to list projects with name %s: %w", projectName, err)
	}
	allProjects, err := projects.ExtractProjects(allPages)
	if err != nil {
		return nil, fmt.Errorf("failed to extract projects: %w", err)
	}
	if len(allProjects) == 0 {
		return nil, fmt.Errorf("no project found with name %s", projectName)
	}
	projectID := allProjects[0].ID

	allPages, err = groups.List(osclient.NetworkingClient, groups.ListOpts{
		TenantID: projectID,
	}).AllPages()
	if err != nil {
		return nil, fmt.Errorf("failed to list security groups: %w", err)
	}
	allGroups, err := groups.ExtractGroups(allPages)
	if err != nil {
		return nil, fmt.Errorf("failed to extract security groups: %w", err)
	}

	nameToIDMap := make(map[string]string)
	for _, group := range allGroups {
		nameToIDMap[group.Name] = group.ID
	}

	var groupIDs []string
	for _, g := range groupNames {
		if isUUID(g) {
			groupIDs = append(groupIDs, g)
			continue
		}

		id, found := nameToIDMap[g]
		if !found {
			return nil, fmt.Errorf("security group with name '%s' not found in project '%s'", g, projectName)
		}
		groupIDs = append(groupIDs, id)
	}

	return groupIDs, nil
}
