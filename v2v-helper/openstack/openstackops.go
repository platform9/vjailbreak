// Copyright © 2024 The vjailbreak authors

package openstack

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/platform9/vjailbreak/v2v-helper/pkg/constants"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/k8sutils"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/utils"
	"github.com/platform9/vjailbreak/v2v-helper/vm"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumes"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
)

//go:generate mockgen -source=../openstack/openstackops.go -destination=../openstack/openstackops_mock.go -package=openstack

type OpenstackOperations interface {
	CreateVolume(name string, size int64, ostype string, uefi bool, volumetype string, setRDMLabel bool) (*volumes.Volume, error)
	WaitForVolume(volumeID string) error
	AttachVolumeToVM(volumeID string) error
	WaitForVolumeAttachment(volumeID string) error
	DetachVolumeFromVM(volumeID string) error
	SetVolumeUEFI(volume *volumes.Volume) error
	EnableQGA(volume *volumes.Volume) error
	SetVolumeImageMetadata(volume *volumes.Volume, setRDMLabel bool) error
	SetVolumeBootable(volume *volumes.Volume) error
	GetClosestFlavour(cpu int32, memory int32) (*flavors.Flavor, error)
	GetFlavor(flavorId string) (*flavors.Flavor, error)
	GetNetwork(networkname string) (*networks.Network, error)
	GetPort(portID string) (*ports.Port, error)
	CreatePort(networkid *networks.Network, mac string, ip []string, vmname string, securityGroups []string, fallbackToDHCP bool) (*ports.Port, error)
	CreateVM(flavor *flavors.Flavor, networkIDs, portIDs []string, vminfo vm.VMInfo, availabilityZone string, securityGroups []string, vjailbreakSettings k8sutils.VjailbreakSettings, useFlavorless bool) (*servers.Server, error)
	GetSecurityGroupIDs(groupNames []string, projectName string) ([]string, error)
	DeleteVolume(volumeID string) error
	FindDevice(volumeID string) (string, error)
	WaitUntilVMActive(vmID string) (bool, error)
}

func validateOpenStack(insecure bool) (*utils.OpenStackClients, error) {
	opts, err := openstack.AuthOptionsFromEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to get OpenStack auth options: %s", err)
	}
	opts.AllowReauth = true
	providerClient, err := openstack.NewClient(opts.IdentityEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider client: %s", err)
	}
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	if insecure {
		tlsConfig.InsecureSkipVerify = true
	}
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	providerClient.HTTPClient = http.Client{
		Transport: transport,
	}

	// Connection Retry Block
	for i := 0; i < constants.MaxIntervalCount; i++ {
		err = openstack.Authenticate(providerClient, opts)
		if err == nil {
			break
		}
		time.Sleep(5 * time.Second) // Wait for 5 seconds before checking again
	}
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate OpenStack client: %s", err)
	}

	endpoint := gophercloud.EndpointOpts{
		Region: os.Getenv("OS_REGION_NAME"),
	}

	blockStorageClient, err := openstack.NewBlockStorageV3(providerClient, endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create block storage client: %s", err)
	}

	computeClient, err := openstack.NewComputeV2(providerClient, endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create compute client: %s", err)
	}

	networkingClient, err := openstack.NewNetworkV2(providerClient, endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create networking client: %s", err)
	}

	return &utils.OpenStackClients{
		BlockStorageClient: blockStorageClient,
		ComputeClient:      computeClient,
		NetworkingClient:   networkingClient,
		K8sClient:          nil,
		AuthURL:            opts.IdentityEndpoint,
		Tenant:             opts.TenantName,
	}, nil
}

func NewOpenStackClients(insecure bool) (*utils.OpenStackClients, error) {
	ostackclients, err := validateOpenStack(insecure)
	if err != nil {
		return nil, fmt.Errorf("failed to validate OpenStack connection: %s", err)
	}
	return ostackclients, nil
}
