package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

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

type OpenStackClients struct {
	BlockStorageClient *gophercloud.ServiceClient
	ComputeClient      *gophercloud.ServiceClient
	NetworkingClient   *gophercloud.ServiceClient
}

type OpenStackMetadata struct {
	UUID string `json:"uuid"`
}

func ValidateOpenStack(ctx context.Context) (*OpenStackClients, error) {
	opts, err := openstack.AuthOptionsFromEnv()
	if err != nil {
		return nil, err
	}

	providerClient, err := openstack.AuthenticatedClient(opts)
	if err != nil {
		return nil, err
	}

	blockStorageClient, err := openstack.NewBlockStorageV3(providerClient, gophercloud.EndpointOpts{})
	if err != nil {
		return nil, err
	}

	computeClient, err := openstack.NewComputeV2(providerClient, gophercloud.EndpointOpts{})
	if err != nil {
		return nil, err
	}

	networkingClient, err := openstack.NewNetworkV2(providerClient, gophercloud.EndpointOpts{})
	if err != nil {
		return nil, err
	}

	return &OpenStackClients{
		BlockStorageClient: blockStorageClient,
		ComputeClient:      computeClient,
		NetworkingClient:   networkingClient,
	}, nil
}

func GetCurrentInstanceUUID() (string, error) {
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
func CreateVolume(ctx context.Context, name string, size int64, ostype string, uefi bool) (*volumes.Volume, error) {
	blockStorageClient := ctx.Value("openstack_clients").(*OpenStackClients).BlockStorageClient
	var opts volumes.CreateOpts

	opts = volumes.CreateOpts{
		Size: int(float64(size) / (1024 * 1024 * 1024)),
		Name: name,
	}
	volume, err := volumes.Create(blockStorageClient, opts).Extract()
	if err != nil {
		return nil, err
	}

	err = WaitForVolume(ctx, volume.ID)
	if err != nil {
		return nil, err
	}
	if uefi {
		err = SetVolumeUEFI(ctx, volume)
		if err != nil {
			return nil, err
		}
	}
	if ostype == "windows" {
		err = SetVolumeSATA(ctx, volume)
		if err != nil {
			return nil, err
		}
	}

	return volume, nil
}

func WaitForVolume(ctx context.Context, volumeID string) error {
	blockStorageClient := ctx.Value("openstack_clients").(*OpenStackClients).BlockStorageClient

	for i := 0; i < 6; i++ {
		volume, err := volumes.Get(blockStorageClient, volumeID).Extract()
		if err != nil {
			return err
		}

		if volume.Status == "available" {
			return nil
		}

		time.Sleep(5 * time.Second) // Wait for 5 seconds before checking again
	}
	return fmt.Errorf("volume did not become available within 30 seconds")
}

func AttachVolumeToVM(ctx context.Context, volumeID, instanceID string) error {
	computeClient := ctx.Value("openstack_clients").(*OpenStackClients).ComputeClient
	_, err := volumeattach.Create(computeClient, instanceID, volumeattach.CreateOpts{
		VolumeID:            volumeID,
		DeleteOnTermination: false,
	}).Extract()
	if err != nil {
		return err
	}

	log.Println("Waiting for volume attachment")
	err = WaitForVolumeAttachment(ctx, volumeID)
	if err != nil {
		return err
	}

	return nil
}

func WaitForVolumeAttachment(ctx context.Context, volumeID string) error {
	for i := 0; i < 6; i++ {
		devicePath, _ := findDevice(volumeID)
		if devicePath != "" {
			return nil
		}
		time.Sleep(5 * time.Second) // Wait for 5 seconds before checking again
	}
	return fmt.Errorf("volume attachment not found within 30 seconds")
}

func DetachVolumeFromVM(ctx context.Context, volumeID, instanceID string) error {
	computeClient := ctx.Value("openstack_clients").(*OpenStackClients).ComputeClient
	err := volumeattach.Delete(computeClient, instanceID, volumeID).ExtractErr()
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

func SetVolumeUEFI(ctx context.Context, volume *volumes.Volume) error {
	blockStorageClient := ctx.Value("openstack_clients").(*OpenStackClients).BlockStorageClient

	options := volumeactions.ImageMetadataOpts{
		Metadata: map[string]string{
			"hw_firmware_type": "uefi",
		},
	}
	err := volumeactions.SetImageMetadata(blockStorageClient, volume.ID, options).ExtractErr()
	if err != nil {
		return err
	}
	return nil
}

func SetVolumeSATA(ctx context.Context, volume *volumes.Volume) error {
	blockStorageClient := ctx.Value("openstack_clients").(*OpenStackClients).BlockStorageClient

	options := volumeactions.ImageMetadataOpts{
		Metadata: map[string]string{
			"hw_disk_bus": "sata",
		},
	}
	err := volumeactions.SetImageMetadata(blockStorageClient, volume.ID, options).ExtractErr()
	if err != nil {
		return err
	}
	return nil
}

func SetVolumeBootable(ctx context.Context, volume *volumes.Volume) error {
	blockStorageClient := ctx.Value("openstack_clients").(*OpenStackClients).BlockStorageClient

	options := volumeactions.BootableOpts{
		Bootable: true,
	}
	err := volumeactions.SetBootable(blockStorageClient, volume.ID, options).ExtractErr()
	if err != nil {
		return err
	}
	return nil
}

func GetClosestFlavour(ctx context.Context, cpu int32, memory int32) (*flavors.Flavor, error) {
	computeClient := ctx.Value("openstack_clients").(*OpenStackClients).ComputeClient
	allPages, err := flavors.ListDetail(computeClient, nil).AllPages()
	if err != nil {
		return nil, err
	}

	allFlavors, err := flavors.ExtractFlavors(allPages)
	if err != nil {
		return nil, err
	}

	// Print the list of flavors
	// for _, flavor := range allFlavors {
	// 	fmt.Printf("Name: %s, ID: %s, RAM: %dMB, VCPUs: %d, Disk: %dGB\n",
	// 		flavor.Name, flavor.ID, flavor.RAM, flavor.VCPUs, flavor.Disk)
	// }

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

func CreatePort(ctx context.Context, networkid string, vminfo VMInfo) (*ports.Port, error) {
	networkingClient := ctx.Value("openstack_clients").(*OpenStackClients).NetworkingClient

	// Get the list of networks
	allPages, err := networks.List(networkingClient, nil).AllPages()
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
		// fmt.Printf("Name: %s, ID: %s Given ID: %s\n", network.Name, network.ID, networkid)

		if network.ID == networkid {
			for _, m := range vminfo.Mac {
				pages, err := ports.List(networkingClient, ports.ListOpts{
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
				port, err := ports.Create(networkingClient, ports.CreateOpts{
					Name:       "port-" + vminfo.VM.Name,
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

func CreateVM(ctx context.Context, flavor *flavors.Flavor, networkID string, port *ports.Port, vminfo VMInfo) (*servers.Server, error) {
	computeClient := ctx.Value("openstack_clients").(*OpenStackClients).ComputeClient
	// blockDevices := []bootfromvolume.BlockDevice{}
	// for _, disk := range vminfo.VMDisks {
	// 	blockDevices = append(blockDevices, bootfromvolume.BlockDevice{
	// 		DeleteOnTermination: true,
	// 		DestinationType:     bootfromvolume.DestinationVolume,
	// 		SourceType:          bootfromvolume.SourceVolume,
	// 		UUID:                disk.OpenstackVol.ID,
	// 	})
	// }
	blockDevice := bootfromvolume.BlockDevice{
		DeleteOnTermination: false,
		DestinationType:     bootfromvolume.DestinationVolume,
		SourceType:          bootfromvolume.SourceVolume,
		UUID:                vminfo.VMDisks[0].OpenstackVol.ID,
	}
	// Create the server
	serverCreateOpts := servers.CreateOpts{
		Name:      vminfo.VM.Name,
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

	server, err := servers.Create(computeClient, createOpts).Extract()
	if err != nil {
		return nil, err
	}

	err = servers.WaitForStatus(computeClient, server.ID, "ACTIVE", 60)
	if err != nil {
		return nil, err
	}

	log.Println("Server created with ID: ", server.ID)

	log.Println("Attaching Additional Disks")

	for _, disk := range vminfo.VMDisks[1:] {
		_, err := volumeattach.Create(computeClient, server.ID, volumeattach.CreateOpts{
			VolumeID:            disk.OpenstackVol.ID,
			DeleteOnTermination: false,
		}).Extract()
		if err != nil {
			return nil, err
		}
	}

	return server, nil
}
