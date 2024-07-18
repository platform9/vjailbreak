package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"vjailbreak/vm"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/extensions/volumeactions"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumes"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/bootfromvolume"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/volumeattach"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
)

type OpenstackOperations interface {
	CreateVolume(name string, size int64, ostype string, uefi bool) (*volumes.Volume, error)
	WaitForVolume(volumeID string) error
	AttachVolumeToVM(volumeID string) error
	WaitForVolumeAttachment(volumeID string) error
	DetachVolumeFromVM(volumeID string) error
	SetVolumeUEFI(volume *volumes.Volume) error
	SetVolumeImageMetadata(volume *volumes.Volume) error
	SetVolumeBootable(volume *volumes.Volume) error
	GetClosestFlavour(cpu int32, memory int32) (*flavors.Flavor, error)
	GetNetworkID(networkname string) (string, error)
	CreatePort(networkid string, vminfo vm.VMInfo) (*ports.Port, error)
	CreateVM(flavor *flavors.Flavor, networkID string, port *ports.Port, vminfo vm.VMInfo) (*servers.Server, error)
}

type OpenStackClients struct {
	BlockStorageClient *gophercloud.ServiceClient
	ComputeClient      *gophercloud.ServiceClient
	NetworkingClient   *gophercloud.ServiceClient
}

type OpenStackMetadata struct {
	UUID string `json:"uuid"`
}

func validateOpenStack() (*OpenStackClients, error) {
	opts, err := openstack.AuthOptionsFromEnv()
	if err != nil {
		return nil, err
	}
	providerClient, err := openstack.AuthenticatedClient(opts)
	if err != nil {
		return nil, err
	}

	endpoint := gophercloud.EndpointOpts{
		Region: os.Getenv("OS_REGION_NAME"),
	}

	blockStorageClient, err := openstack.NewBlockStorageV3(providerClient, endpoint)
	if err != nil {
		return nil, err
	}

	computeClient, err := openstack.NewComputeV2(providerClient, endpoint)
	if err != nil {
		return nil, err
	}

	networkingClient, err := openstack.NewNetworkV2(providerClient, endpoint)
	if err != nil {
		return nil, err
	}

	return &OpenStackClients{
		BlockStorageClient: blockStorageClient,
		ComputeClient:      computeClient,
		NetworkingClient:   networkingClient,
	}, nil
}

func OpenStackClientsBuilder() (*OpenStackClients, error) {
	ostackclients, err := validateOpenStack()
	if err != nil {
		return nil, err
	}
	return ostackclients, nil
}

func getCurrentInstanceUUID() (string, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://169.254.169.254/openstack/latest/meta_data.json", nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var metadata OpenStackMetadata
	if err := json.Unmarshal(body, &metadata); err != nil {
		return "", err
	}

	return metadata.UUID, nil
}

// create a new volume
func (osclient *OpenStackClients) CreateVolume(name string, size int64, ostype string, uefi bool) (*volumes.Volume, error) {
	blockStorageClient := osclient.BlockStorageClient
	var opts volumes.CreateOpts

	opts = volumes.CreateOpts{
		Size: int(float64(size) / (1024 * 1024 * 1024)),
		Name: name,
	}
	volume, err := volumes.Create(blockStorageClient, opts).Extract()
	if err != nil {
		return nil, err
	}

	err = osclient.WaitForVolume(volume.ID)
	if err != nil {
		return nil, err
	}
	if uefi {
		err = osclient.SetVolumeUEFI(volume)
		if err != nil {
			return nil, err
		}
	}
	if ostype == "windows" {
		err = osclient.SetVolumeImageMetadata(volume)
		if err != nil {
			return nil, err
		}
	}

	return volume, nil
}

func (osclient *OpenStackClients) WaitForVolume(volumeID string) error {
	for i := 0; i < 10; i++ {
		volume, err := volumes.Get(osclient.BlockStorageClient, volumeID).Extract()
		if err != nil {
			return err
		}

		if volume.Status == "available" {
			return nil
		}
		time.Sleep(5 * time.Second) // Wait for 5 seconds before checking again
	}
	return fmt.Errorf("volume did not become available within 500 seconds")
}

func (osclient *OpenStackClients) AttachVolumeToVM(volumeID string) error {
	instanceID, err := getCurrentInstanceUUID()
	if err != nil {
		return err
	}
	_, err = volumeattach.Create(osclient.ComputeClient, instanceID, volumeattach.CreateOpts{
		VolumeID:            volumeID,
		DeleteOnTermination: false,
	}).Extract()
	if err != nil {
		return err
	}

	log.Println("Waiting for volume attachment")
	err = osclient.WaitForVolumeAttachment(volumeID)
	if err != nil {
		return err
	}

	return nil
}

func findDevice(volumeID string) (string, error) {
	files, err := os.ReadDir("/dev/disk/by-id/")
	if err != nil {
		return "", err
	}

	for _, file := range files {
		if strings.Contains(file.Name(), volumeID[:18]) {
			devicePath, err := filepath.EvalSymlinks(filepath.Join("/dev/disk/by-id/", file.Name()))
			if err != nil {
				return "", err
			}

			return devicePath, nil
		}
	}

	return "", nil
}

func (osclient *OpenStackClients) WaitForVolumeAttachment(volumeID string) error {
	for i := 0; i < 6; i++ {
		devicePath, _ := findDevice(volumeID)
		if devicePath != "" {
			return nil
		}
		time.Sleep(5 * time.Second) // Wait for 5 seconds before checking again
	}
	return fmt.Errorf("volume attachment not found within 30 seconds")
}

func (osclient *OpenStackClients) DetachVolumeFromVM(volumeID string) error {
	instanceID, err := getCurrentInstanceUUID()
	if err != nil {
		return err
	}
	err = volumeattach.Delete(osclient.ComputeClient, instanceID, volumeID).ExtractErr()
	if err != nil {
		return err
	}
	return nil
}

func (osclient *OpenStackClients) SetVolumeUEFI(volume *volumes.Volume) error {
	options := volumeactions.ImageMetadataOpts{
		Metadata: map[string]string{
			"hw_firmware_type": "uefi",
		},
	}
	err := volumeactions.SetImageMetadata(osclient.BlockStorageClient, volume.ID, options).ExtractErr()
	if err != nil {
		return err
	}
	return nil
}

func (osclient *OpenStackClients) SetVolumeImageMetadata(volume *volumes.Volume) error {
	options := volumeactions.ImageMetadataOpts{
		Metadata: map[string]string{
			"hw_disk_bus": "virtio",
			"os_type":     "windows",
		},
	}
	err := volumeactions.SetImageMetadata(osclient.BlockStorageClient, volume.ID, options).ExtractErr()
	if err != nil {
		return err
	}
	return nil
}

func (osclient *OpenStackClients) SetVolumeBootable(volume *volumes.Volume) error {
	options := volumeactions.BootableOpts{
		Bootable: true,
	}
	err := volumeactions.SetBootable(osclient.BlockStorageClient, volume.ID, options).ExtractErr()
	if err != nil {
		return err
	}
	return nil
}

func (osclient *OpenStackClients) GetClosestFlavour(cpu int32, memory int32) (*flavors.Flavor, error) {
	allPages, err := flavors.ListDetail(osclient.ComputeClient, nil).AllPages()
	if err != nil {
		return nil, err
	}

	allFlavors, err := flavors.ExtractFlavors(allPages)
	if err != nil {
		return nil, err
	}

	log.Println("Current requirements:", cpu, "CPUs and", memory, "MB of RAM")

	bestFlavor := new(flavors.Flavor)
	bestFlavor.VCPUs = 9999999
	bestFlavor.RAM = 9999999
	// Find the smallest flavor that meets the requirements
	for _, flavor := range allFlavors {
		if flavor.VCPUs >= int(cpu) && flavor.RAM >= int(memory) {
			if flavor.VCPUs < bestFlavor.VCPUs || (flavor.VCPUs == bestFlavor.VCPUs && flavor.RAM < bestFlavor.RAM) {
				bestFlavor = &flavor
			}
		}
	}

	if bestFlavor != nil {
		log.Printf("The best flavor is:\nName: %s, ID: %s, RAM: %dMB, VCPUs: %d, Disk: %dGB\n",
			bestFlavor.Name, bestFlavor.ID, bestFlavor.RAM, bestFlavor.VCPUs, bestFlavor.Disk)
	} else {
		log.Println("No suitable flavor found.")
	}

	return bestFlavor, nil
}

func (osclient *OpenStackClients) GetNetworkID(networkname string) (string, error) {
	allPages, err := networks.List(osclient.NetworkingClient, nil).AllPages()
	if err != nil {
		return "", err
	}

	allNetworks, err := networks.ExtractNetworks(allPages)
	if err != nil {
		return "", err
	}

	for _, network := range allNetworks {
		if network.Name == networkname {
			return network.ID, nil
		}
	}
	return "", fmt.Errorf("network not found")
}

func (osclient *OpenStackClients) CreatePort(networkid string, vminfo vm.VMInfo) (*ports.Port, error) {
	// Get the list of networks
	allPages, err := networks.List(osclient.NetworkingClient, nil).AllPages()
	if err != nil {
		log.Printf("Failed to list networks: %v", err)
		return nil, err
	}

	allNetworks, err := networks.ExtractNetworks(allPages)
	if err != nil {
		log.Printf("Failed to extract networks: %v", err)
		return nil, err
	}

	for _, network := range allNetworks {
		if network.ID == networkid {
			for _, m := range vminfo.Mac {
				pages, err := ports.List(osclient.NetworkingClient, ports.ListOpts{
					NetworkID:  networkid,
					MACAddress: m,
				}).AllPages()
				if err != nil {
					return nil, err
				}

				portList, err := ports.ExtractPorts(pages)
				if err != nil {
					return nil, err
				}

				for _, port := range portList {
					if port.MACAddress == m {
						log.Printf("Port with MAC address %s already exists, ID: %s\n", m, port.ID)
						return &port, nil
					}
				}
				log.Printf("Port with MAC address %s does not exist, creating new port\n", m)
				port, err := ports.Create(osclient.NetworkingClient, ports.CreateOpts{
					Name:       "port-" + vminfo.Name,
					NetworkID:  networkid,
					MACAddress: m,
				}).Extract()
				if err != nil {
					return nil, err
				}
				log.Println("Port created with ID: ", port.ID)
				return port, nil
			}
		}
	}
	return nil, fmt.Errorf("network not found")
}

func (osclient *OpenStackClients) CreateVM(flavor *flavors.Flavor, networkID string, port *ports.Port, vminfo vm.VMInfo) (*servers.Server, error) {
	blockDevice := bootfromvolume.BlockDevice{
		DeleteOnTermination: false,
		DestinationType:     bootfromvolume.DestinationVolume,
		SourceType:          bootfromvolume.SourceVolume,
		UUID:                vminfo.VMDisks[0].OpenstackVol.ID,
	}
	// Create the server
	serverCreateOpts := servers.CreateOpts{
		Name:      vminfo.Name,
		FlavorRef: flavor.ID,
		Networks: []servers.Network{
			{
				UUID: networkID,
				Port: port.ID,
			},
		},
	}

	createOpts := bootfromvolume.CreateOptsExt{
		CreateOptsBuilder: serverCreateOpts,
		BlockDevice:       []bootfromvolume.BlockDevice{blockDevice},
	}

	// Wait for disks to become available
	for _, disk := range vminfo.VMDisks {
		err := osclient.WaitForVolume(disk.OpenstackVol.ID)
		if err != nil {
			return nil, err
		}
	}

	server, err := servers.Create(osclient.ComputeClient, createOpts).Extract()
	if err != nil {
		return nil, err
	}

	err = servers.WaitForStatus(osclient.ComputeClient, server.ID, "ACTIVE", 60)
	if err != nil {
		return nil, err
	}

	log.Println("Server created with ID: ", server.ID)

	log.Println("Attaching Additional Disks")

	for _, disk := range vminfo.VMDisks[1:] {
		_, err := volumeattach.Create(osclient.ComputeClient, server.ID, volumeattach.CreateOpts{
			VolumeID:            disk.OpenstackVol.ID,
			DeleteOnTermination: false,
		}).Extract()
		if err != nil {
			return nil, err
		}
	}

	return server, nil
}
