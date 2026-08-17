// Copyright © 2026 The vjailbreak authors

package openstack

import (
	"context"
	"time"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/k8sutils"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/timing"
	"github.com/platform9/vjailbreak/v2v-helper/vm"

	"github.com/gophercloud/gophercloud/v2/openstack/blockstorage/v3/volumes"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/ports"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/subnets"
)

// Step names for the PCD/OpenStack side. These are row labels in the Hot-Add
// comparison report, so they are constants — the report script matches exactly.
const (
	StepCreateVolume            = "PCD: Create Cinder Volume"
	StepWaitForVolume           = "PCD: Wait for Volume Active"
	StepAttachVolumeToVM        = "PCD: Attach Volume to Helper VM"
	StepWaitForVolumeAttachment = "PCD: Wait for Volume Attachment"
	StepDetachVolumeFromVM      = "PCD: Detach Volume from Helper VM"
	StepSetVolumeUEFI           = "PCD: Set Volume Image Metadata (UEFI/Windows)"
	StepEnableQGA               = "PCD: Enable QGA"
	StepSetVolumeImageMetadata  = "PCD: Set Volume Image Metadata"
	StepApplyBootVolumeMetadata = "PCD: Apply Boot Volume Image Metadata"
	StepSetVolumeBootable       = "PCD: Set Volume Bootable"
	StepGetClosestFlavour       = "PCD: Get Closest Flavor"
	StepGetFlavor               = "PCD: Get Flavor"
	StepGetNetwork              = "PCD: Get Network"
	StepGetPort                 = "PCD: Get Port"
	StepValidateAndCreatePort   = "PCD: Create Network Port (per NIC)"
	StepDeletePort              = "PCD: Delete Port"
	StepGetSubnet               = "PCD: Get Subnet"
	StepCreatePort              = "PCD: Create Network Port (per NIC, no validation)"
	StepCreateVM                = "PCD: Create Nova Instance"
	StepGetServerGroups         = "PCD: Get Server Groups"
	StepGetSecurityGroupIDs     = "PCD: Get Security Group IDs"
	StepDeleteVolume            = "PCD: Delete Volume"
	StepFindDevice              = "PCD: Find Device"
	StepManageExistingVolume    = "PCD: Cinder Manage Existing Volume"
	StepWaitUntilVMActive       = "PCD: Wait for VM Active"
	StepGetIsSimpleNetwork      = "PCD: Get Is Simple Network"
	StepGetCinderVolumeServices = "PCD: Get Cinder Volume Services"
	StepGetVolume               = "PCD: Get Volume"
	StepDeleteServer            = "PCD: Delete Server"
	StepStopServer              = "PCD: Stop Server"
	StepDetachVolumeFromServer  = "PCD: Detach Volume from Server"
	StepWaitForVolumeDetached   = "PCD: Wait for Volume Detached"
	StepGetServerStatus         = "PCD: Get Server Status"
)

// TimedOpenstackOperations wraps an OpenstackOperations and records how long
// each PCD call takes. Pure pass-through: arguments, return values and errors
// are forwarded unchanged.
type TimedOpenstackOperations struct {
	inner OpenstackOperations
	rec   *timing.Recorder
}

var _ OpenstackOperations = (*TimedOpenstackOperations)(nil)

// NewTimedOpenstackOperations wraps inner. A nil recorder makes every call a
// plain pass-through.
func NewTimedOpenstackOperations(inner OpenstackOperations, rec *timing.Recorder) *TimedOpenstackOperations {
	return &TimedOpenstackOperations{inner: inner, rec: rec}
}

func (t *TimedOpenstackOperations) CreateVolume(ctx context.Context, name string, size int64, ostype string,
	uefi bool, volumetype string, setRDMLabel bool) (*volumes.Volume, error) {
	done := t.rec.Start(StepCreateVolume)
	vol, err := t.inner.CreateVolume(ctx, name, size, ostype, uefi, volumetype, setRDMLabel)
	done(err)
	return vol, err
}

func (t *TimedOpenstackOperations) WaitForVolume(ctx context.Context, volumeID string) error {
	return t.rec.Track(StepWaitForVolume, func() error { return t.inner.WaitForVolume(ctx, volumeID) })
}

func (t *TimedOpenstackOperations) AttachVolumeToVM(ctx context.Context, volumeID string) error {
	return t.rec.Track(StepAttachVolumeToVM, func() error { return t.inner.AttachVolumeToVM(ctx, volumeID) })
}

func (t *TimedOpenstackOperations) WaitForVolumeAttachment(ctx context.Context, volumeID string) error {
	return t.rec.Track(StepWaitForVolumeAttachment, func() error { return t.inner.WaitForVolumeAttachment(ctx, volumeID) })
}

func (t *TimedOpenstackOperations) DetachVolumeFromVM(ctx context.Context, volumeID string) error {
	return t.rec.Track(StepDetachVolumeFromVM, func() error { return t.inner.DetachVolumeFromVM(ctx, volumeID) })
}

func (t *TimedOpenstackOperations) SetVolumeUEFI(ctx context.Context, volume *volumes.Volume) error {
	return t.rec.Track(StepSetVolumeUEFI, func() error { return t.inner.SetVolumeUEFI(ctx, volume) })
}

func (t *TimedOpenstackOperations) EnableQGA(ctx context.Context, volume *volumes.Volume) error {
	return t.rec.Track(StepEnableQGA, func() error { return t.inner.EnableQGA(ctx, volume) })
}

func (t *TimedOpenstackOperations) SetVolumeImageMetadata(ctx context.Context, volume *volumes.Volume, setRDMLabel bool) error {
	return t.rec.Track(StepSetVolumeImageMetadata, func() error {
		return t.inner.SetVolumeImageMetadata(ctx, volume, setRDMLabel)
	})
}

func (t *TimedOpenstackOperations) ApplyBootVolumeImageMetadata(ctx context.Context, volume *volumes.Volume,
	metadata map[string]string) error {
	return t.rec.Track(StepApplyBootVolumeMetadata, func() error {
		return t.inner.ApplyBootVolumeImageMetadata(ctx, volume, metadata)
	})
}

func (t *TimedOpenstackOperations) SetVolumeBootable(ctx context.Context, volume *volumes.Volume) error {
	return t.rec.Track(StepSetVolumeBootable, func() error { return t.inner.SetVolumeBootable(ctx, volume) })
}

func (t *TimedOpenstackOperations) GetClosestFlavour(ctx context.Context, cpu int32, memory int32) (*flavors.Flavor, error) {
	done := t.rec.Start(StepGetClosestFlavour)
	flavor, err := t.inner.GetClosestFlavour(ctx, cpu, memory)
	done(err)
	return flavor, err
}

func (t *TimedOpenstackOperations) GetFlavor(ctx context.Context, flavorID string) (*flavors.Flavor, error) {
	done := t.rec.Start(StepGetFlavor)
	flavor, err := t.inner.GetFlavor(ctx, flavorID)
	done(err)
	return flavor, err
}

func (t *TimedOpenstackOperations) GetNetwork(ctx context.Context, networkname string) (*networks.Network, error) {
	done := t.rec.Start(StepGetNetwork)
	network, err := t.inner.GetNetwork(ctx, networkname)
	done(err)
	return network, err
}

func (t *TimedOpenstackOperations) GetPort(ctx context.Context, portID string) (*ports.Port, error) {
	done := t.rec.Start(StepGetPort)
	port, err := t.inner.GetPort(ctx, portID)
	done(err)
	return port, err
}

func (t *TimedOpenstackOperations) ValidateAndCreatePort(ctx context.Context, networkid *networks.Network, mac string,
	ipPerMac map[string][]vm.IpEntry, vmname string, securityGroups []string, fallbackToDHCP bool,
	gatewayIP map[string]string, subnetPortIndex map[string]int) (*ports.Port, error) {
	done := t.rec.Start(StepValidateAndCreatePort)
	port, err := t.inner.ValidateAndCreatePort(ctx, networkid, mac, ipPerMac, vmname, securityGroups,
		fallbackToDHCP, gatewayIP, subnetPortIndex)
	done(err)
	return port, err
}

func (t *TimedOpenstackOperations) DeletePort(ctx context.Context, portID string) error {
	return t.rec.Track(StepDeletePort, func() error { return t.inner.DeletePort(ctx, portID) })
}

func (t *TimedOpenstackOperations) GetSubnet(ctx context.Context, network []string, ip string) (*subnets.Subnet, error) {
	done := t.rec.Start(StepGetSubnet)
	subnet, err := t.inner.GetSubnet(ctx, network, ip)
	done(err)
	return subnet, err
}

func (t *TimedOpenstackOperations) CreatePort(ctx context.Context, networkid *networks.Network, mac string, ip []string,
	vmname string, securityGroups []string, fallbackToDHCP bool, gatewayIP map[string]string) (*ports.Port, error) {
	done := t.rec.Start(StepCreatePort)
	port, err := t.inner.CreatePort(ctx, networkid, mac, ip, vmname, securityGroups, fallbackToDHCP, gatewayIP)
	done(err)
	return port, err
}

func (t *TimedOpenstackOperations) CreateVM(ctx context.Context, flavor *flavors.Flavor, networkIDs, portIDs []string,
	vminfo vm.VMInfo, availabilityZone string, securityGroups []string, serverGroupID string,
	vjailbreakSettings k8sutils.VjailbreakSettings, espDiskIndex int) (*servers.Server, error) {
	done := t.rec.Start(StepCreateVM)
	server, err := t.inner.CreateVM(ctx, flavor, networkIDs, portIDs, vminfo, availabilityZone, securityGroups,
		serverGroupID, vjailbreakSettings, espDiskIndex)
	done(err)
	return server, err
}

func (t *TimedOpenstackOperations) GetServerGroups(ctx context.Context, projectName string) ([]vjailbreakv1alpha1.ServerGroupInfo, error) {
	done := t.rec.Start(StepGetServerGroups)
	groups, err := t.inner.GetServerGroups(ctx, projectName)
	done(err)
	return groups, err
}

func (t *TimedOpenstackOperations) GetSecurityGroupIDs(ctx context.Context, groupNames []string, projectName string) ([]string, error) {
	done := t.rec.Start(StepGetSecurityGroupIDs)
	ids, err := t.inner.GetSecurityGroupIDs(ctx, groupNames, projectName)
	done(err)
	return ids, err
}

func (t *TimedOpenstackOperations) DeleteVolume(ctx context.Context, volumeID string) error {
	return t.rec.Track(StepDeleteVolume, func() error { return t.inner.DeleteVolume(ctx, volumeID) })
}

func (t *TimedOpenstackOperations) FindDevice(volumeID string) (string, error) {
	done := t.rec.Start(StepFindDevice)
	device, err := t.inner.FindDevice(volumeID)
	done(err)
	return device, err
}

func (t *TimedOpenstackOperations) ManageExistingVolume(name string, ref map[string]interface{}, host string,
	volumeType string) (*volumes.Volume, error) {
	done := t.rec.Start(StepManageExistingVolume)
	vol, err := t.inner.ManageExistingVolume(name, ref, host, volumeType)
	done(err)
	return vol, err
}

func (t *TimedOpenstackOperations) WaitUntilVMActive(ctx context.Context, vmID string) (bool, error) {
	done := t.rec.Start(StepWaitUntilVMActive)
	active, err := t.inner.WaitUntilVMActive(ctx, vmID)
	done(err)
	return active, err
}

func (t *TimedOpenstackOperations) GetIsSimpleNetwork(ctx context.Context, networkID string) (bool, error) {
	done := t.rec.Start(StepGetIsSimpleNetwork)
	simple, err := t.inner.GetIsSimpleNetwork(ctx, networkID)
	done(err)
	return simple, err
}

func (t *TimedOpenstackOperations) GetCinderVolumeServices(ctx context.Context) (interface{}, error) {
	done := t.rec.Start(StepGetCinderVolumeServices)
	services, err := t.inner.GetCinderVolumeServices(ctx)
	done(err)
	return services, err
}

func (t *TimedOpenstackOperations) GetVolume(ctx context.Context, volumeID string) (*volumes.Volume, error) {
	done := t.rec.Start(StepGetVolume)
	vol, err := t.inner.GetVolume(ctx, volumeID)
	done(err)
	return vol, err
}

func (t *TimedOpenstackOperations) DeleteServer(ctx context.Context, serverID string) error {
	return t.rec.Track(StepDeleteServer, func() error { return t.inner.DeleteServer(ctx, serverID) })
}

func (t *TimedOpenstackOperations) StopServer(ctx context.Context, serverID string) error {
	return t.rec.Track(StepStopServer, func() error { return t.inner.StopServer(ctx, serverID) })
}

func (t *TimedOpenstackOperations) DetachVolumeFromServer(ctx context.Context, serverID, volumeID string) error {
	return t.rec.Track(StepDetachVolumeFromServer, func() error {
		return t.inner.DetachVolumeFromServer(ctx, serverID, volumeID)
	})
}

func (t *TimedOpenstackOperations) WaitForVolumeDetached(ctx context.Context, volumeID string, timeout time.Duration) error {
	return t.rec.Track(StepWaitForVolumeDetached, func() error {
		return t.inner.WaitForVolumeDetached(ctx, volumeID, timeout)
	})
}

func (t *TimedOpenstackOperations) GetServerStatus(ctx context.Context, serverID string) (string, error) {
	done := t.rec.Start(StepGetServerStatus)
	status, err := t.inner.GetServerStatus(ctx, serverID)
	done(err)
	return status, err
}
