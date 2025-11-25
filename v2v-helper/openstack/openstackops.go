// Copyright © 2024 The vjailbreak authors

package openstack

import (
	context "context"
	"fmt"
	"os"
	"time"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
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
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"
	netutils "github.com/platform9/vjailbreak/common/utils"
)

//go:generate mockgen -source=../openstack/openstackops.go -destination=../openstack/openstackops_mock.go -package=openstack

type OpenstackOperations interface {
	CreateVolume(ctx context.Context, name string, size int64, ostype string, uefi bool, volumetype string, setRDMLabel bool) (*volumes.Volume, error)
	WaitForVolume(ctx context.Context, volumeID string) error
	AttachVolumeToVM(ctx context.Context, volumeID string) error
	WaitForVolumeAttachment(ctx context.Context, volumeID string) error
	DetachVolumeFromVM(ctx context.Context, volumeID string) error
	SetVolumeUEFI(ctx context.Context, volume *volumes.Volume) error
	EnableQGA(ctx context.Context, volume *volumes.Volume) error
	SetVolumeImageMetadata(ctx context.Context, volume *volumes.Volume, setRDMLabel bool) error
	SetVolumeBootable(ctx context.Context, volume *volumes.Volume) error
	GetClosestFlavour(ctx context.Context, cpu int32, memory int32) (*flavors.Flavor, error)
	GetFlavor(ctx context.Context, flavorId string) (*flavors.Flavor, error)
	GetNetwork(ctx context.Context, networkname string) (*networks.Network, error)
	GetPort(ctx context.Context, portID string) (*ports.Port, error)
	ValidateAndCreatePort(ctx context.Context, networkid *networks.Network, mac string, ipPerMac map[string][]vm.IpEntry, vmname string, securityGroups []string, fallbackToDHCP bool, gatewayIP map[string]string) (*ports.Port, error)
	DeletePort(ctx context.Context, portID string) error
	GetSubnet(ctx context.Context, network []string, ip string) (*subnets.Subnet, error)
	CreateVM(ctx context.Context, flavor *flavors.Flavor, networkIDs, portIDs []string, vminfo vm.VMInfo, availabilityZone string, securityGroups []string, serverGroupID string, vjailbreakSettings k8sutils.VjailbreakSettings, useFlavorless bool) (*servers.Server, error)
	GetServerGroups(ctx context.Context, projectName string) ([]vjailbreakv1alpha1.ServerGroupInfo, error)
	GetSecurityGroupIDs(ctx context.Context, groupNames []string, projectName string) ([]string, error)
	DeleteVolume(ctx context.Context, volumeID string) error
	FindDevice(volumeID string) (string, error)
	WaitUntilVMActive(ctx context.Context, vmID string) (bool, error)
}

func validateOpenStack(ctx context.Context, insecure bool) (*utils.OpenStackClients, error) {
	opts, err := openstack.AuthOptionsFromEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to get OpenStack auth options: %s", err)
	}
	opts.AllowReauth = true
	providerClient, err := openstack.NewClient(opts.IdentityEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider client: %s", err)
	}

	vjbNet := netutils.NewVjbNet()
	if insecure {
		vjbNet.Insecure = true
	}
	if vjbNet.CreateSecureHTTPClient() == nil {
		providerClient.HTTPClient = *vjbNet.GetClient()
	} else {
		return nil, fmt.Errorf("failed to create secure HTTP client")
	}
	// Connection Retry Block
	for i := 0; i < constants.MaxIntervalCount; i++ {
		err = openstack.Authenticate(ctx, providerClient, opts)
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

func NewOpenStackClients(ctx context.Context, insecure bool) (*utils.OpenStackClients, error) {
	ostackclients, err := validateOpenStack(ctx, insecure)
	if err != nil {
		return nil, fmt.Errorf("failed to validate OpenStack connection: %s", err)
	}
	return ostackclients, nil
}
