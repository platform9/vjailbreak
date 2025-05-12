// Copyright © 2024 The vjailbreak authors

package vm

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/platform9/vjailbreak/v2v-helper/vcenter"

	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumes"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

//go:generate mockgen -source=../vm/vmops.go -destination=../vm/vmops_mock.go -package=vm

type VMOperations interface {
	GetVMInfo(ostype string) (VMInfo, error)
	GetVMObj() *object.VirtualMachine
	UpdateDiskInfo(vminfo VMInfo, namespace string) (VMInfo, error)
	IsCBTEnabled() (bool, error)
	EnableCBT() error
	TakeSnapshot(name string) error
	DeleteSnapshot(name string) error
	GetSnapshot(name string) (*types.ManagedObjectReference, error)
	CustomQueryChangedDiskAreas(baseChangeID string, curSnapshot *types.ManagedObjectReference, disk *types.VirtualDisk, offset int64) (types.DiskChangeInfo, error)
	VMPowerOff() error
	VMPowerOn() error
}

type VMInfo struct {
	CPU      int32
	Memory   int32
	State    types.VirtualMachinePowerState
	Mac      []string
	IPs      []string
	UUID     string
	Host     string
	VMDisks  []VMDisk
	UEFI     bool
	Name     string
	OSType   string
	RDMDisks []RDMDisk
}

type ChangeID struct {
	UUID   string
	Number string
	Value  string
}

type VMDisk struct {
	Name            string
	Size            int64
	OpenstackVol    *volumes.Volume
	Path            string
	Disk            *types.VirtualDisk
	Snapname        string
	SnapBackingDisk string
	ChangeID        string
	Boot            bool
}

type RDMDisk struct {
	// DiskName is the name of the disk
	DiskName string `json:"diskName,omitempty"`
	// DiskSize is the size of the disk in GB
	DiskSize int64 `json:"diskSize,omitempty"`
	// UUID is the unique identifier of the disk
	UUID string `json:"uuid,omitempty"`
	// DisplayName is the display name of the disk
	DisplayName string `json:"displayName,omitempty"`
	// OperationalState is the operational state of the disk
	OperationalState []string `json:"operationalState,omitempty"`
	// CinderBackendPool is the cinder backend pool of the disk
	CinderBackendPool string `json:"cinderBackendPool,omitempty"`
	// VolumeType is the volume type of the disk
	VolumeType string `json:"volumeType,omitempty"`
	// AvailabilityZone is the availability zone of the disk
	AvailabilityZone string `json:"availabilityZone,omitempty"`
	// Bootable indicates if the disk is bootable
	Bootable bool `json:"bootable,omitempty"`
	// DiskMode is the mode of the disk
	Description string `json:"description,omitempty"`

	VolumeId string `json:"volumeId,omitempty"`
}

type VMOps struct {
	vcclient  *vcenter.VCenterClient
	VMObj     *object.VirtualMachine
	ctx       context.Context
	k8sClient client.Client
}

func VMOpsBuilder(ctx context.Context, vcclient vcenter.VCenterClient, name string, k8sClient client.Client) (*VMOps, error) {
	vm, err := vcclient.GetVMByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get VM: %s", err)
	}
	return &VMOps{
		vcclient:  &vcclient,
		VMObj:     vm,
		ctx:       ctx,
		k8sClient: k8sClient,
	}, nil
}

func (vmops *VMOps) GetVMObj() *object.VirtualMachine {
	return vmops.VMObj
}

// func getVMs(ctx context.Context, finder *find.Finder) ([]*object.VirtualMachine, error) {
// 	// Find all virtual machines on the host
// 	// vms, err := finder.VirtualMachineList(ctx, "*")
// 	// if err != nil {
// 	// 	return nil, err
// 	// }
// 	// return vms, nil
// }

func (vmops *VMOps) GetVMInfo(ostype string) (VMInfo, error) {
	vm := vmops.VMObj

	var o mo.VirtualMachine
	err := vm.Properties(vmops.ctx, vm.Reference(), []string{}, &o)
	if err != nil {
		return VMInfo{}, fmt.Errorf("failed to get VM properties: %s", err)
	}
	var mac []string
	for _, device := range o.Config.Hardware.Device {
		if nic, ok := device.(types.BaseVirtualEthernetCard); ok {
			mac = append(mac, nic.GetVirtualEthernetCard().MacAddress)
		}
	}
	// Get IP addresses of the VM
	ips := []string{}
	for _, nic := range o.Guest.Net {
		if nic.IpConfig != nil {
			for _, ip := range nic.IpConfig.IpAddress {
				if !strings.Contains(ip.IpAddress, ":") {
					ips = append(ips, ip.IpAddress)
				}
			}
		}
	}

	vmdisks := []VMDisk{} // Create an empty slice of Disk structs
	for _, device := range o.Config.Hardware.Device {
		if disk, ok := device.(*types.VirtualDisk); ok {
			vmdisks = append(vmdisks, VMDisk{
				Name: disk.DeviceInfo.GetDescription().Label,
				Size: disk.CapacityInBytes,
				Disk: disk,
			},
			)
		}
	}
	uefi := false
	if o.Config.Firmware == "efi" {
		uefi = true
	}
	if ostype == "" {
		if o.Guest.GuestFamily == string(types.VirtualMachineGuestOsFamilyWindowsGuest) {
			ostype = "windows"
		} else if o.Guest.GuestFamily == string(types.VirtualMachineGuestOsFamilyLinuxGuest) {
			ostype = "linux"
		} else {
			return VMInfo{}, fmt.Errorf("no OS type provided and unable to determine OS type")
		}
	}

	vminfo := VMInfo{
		CPU:     o.Config.Hardware.NumCPU,
		Memory:  o.Config.Hardware.MemoryMB,
		State:   o.Runtime.PowerState,
		Mac:     mac,
		IPs:     ips,
		UUID:    o.Config.Uuid,
		Host:    o.Runtime.Host.Reference().Value,
		Name:    o.Name,
		VMDisks: vmdisks,
		UEFI:    uefi,
		OSType:  ostype,
	}
	return vminfo, nil
}

func parseChangeID(changeId string) (*ChangeID, error) {
	changeIdParts := strings.Split(changeId, "/")
	if len(changeIdParts) != 2 {
		return nil, fmt.Errorf("invalid change ID format")
	}

	return &ChangeID{
		UUID:   changeIdParts[0],
		Number: changeIdParts[1],
		Value:  changeId,
	}, nil
}

func getChangeID(disk *types.VirtualDisk) (*ChangeID, error) {
	var changeId string

	if b, ok := disk.Backing.(*types.VirtualDiskFlatVer2BackingInfo); ok {
		changeId = b.ChangeId
	} else if b, ok := disk.Backing.(*types.VirtualDiskSparseVer2BackingInfo); ok {
		changeId = b.ChangeId
	} else if b, ok := disk.Backing.(*types.VirtualDiskRawDiskMappingVer1BackingInfo); ok {
		changeId = b.ChangeId
	} else if b, ok := disk.Backing.(*types.VirtualDiskRawDiskVer2BackingInfo); ok {
		changeId = b.ChangeId
	} else {
		return nil, fmt.Errorf("failed to get change ID")
	}

	if changeId == "" {
		return nil, fmt.Errorf("CBT is not enabled on disk %d", disk.Key)
	}
	return parseChangeID(changeId)
}

func (vmops *VMOps) UpdateDiskInfo(vminfo VMInfo, namespace string) (VMInfo, error) {
	pc := vmops.vcclient.VCPropertyCollector
	vm := vmops.VMObj
	var snapbackingdisk []string
	var snapname []string
	var snapid []string

	var o mo.VirtualMachine
	err := vm.Properties(vmops.ctx, vm.Reference(), []string{}, &o)
	if err != nil {
		return vminfo, fmt.Errorf("failed to get VM properties: %s", err)
	}

	if o.Snapshot != nil {
		// get backing disk of snapshot
		var s mo.VirtualMachineSnapshot
		err := pc.RetrieveOne(vmops.ctx, o.Snapshot.CurrentSnapshot.Reference(), []string{}, &s)
		if err != nil {
			return vminfo, fmt.Errorf("failed to get snapshot properties: %s", err)
		}

		for _, device := range s.Config.Hardware.Device {
			switch disk := device.(type) {
			case *types.VirtualDisk:
				backing := disk.Backing.(types.BaseVirtualDeviceFileBackingInfo)
				info := backing.GetVirtualDeviceFileBackingInfo()
				snapbackingdisk = append(snapbackingdisk, info.FileName)
				snapname = append(snapname, o.Snapshot.CurrentSnapshot.Value)
				changeid, err := getChangeID(disk)
				if err != nil {
					return vminfo, fmt.Errorf("failed to get change ID: %s", err)
				}
				snapid = append(snapid, changeid.Value)
			}
		}
		for idx := range vminfo.VMDisks {
			vminfo.VMDisks[idx].SnapBackingDisk = snapbackingdisk[idx]
			vminfo.VMDisks[idx].Snapname = snapname[idx]
			vminfo.VMDisks[idx].ChangeID = snapid[idx]
		}
		// Based on VMName and diskname fetch DiskInfo
		rdmDIskInfo, err := GetVMwareMachine(vmops.ctx, vmops.k8sClient, vminfo.Name, namespace)
		if err != nil {
			return vminfo, fmt.Errorf("failed to get rdmDisk properties: %s", err)
		}
		copyRDMDisks(&vminfo, rdmDIskInfo)
	}

	return vminfo, nil
}

func (vmops *VMOps) IsCBTEnabled() (bool, error) {
	vm := vmops.VMObj
	var o mo.VirtualMachine
	err := vm.Properties(vmops.ctx, vm.Reference(), []string{"config.changeTrackingEnabled"}, &o)
	if err != nil {
		return false, fmt.Errorf("failed to get VM properties: %s", err)
	}
	return *o.Config.ChangeTrackingEnabled, nil
}

func (vmops *VMOps) EnableCBT() error {
	vm := vmops.VMObj
	configSpec := types.VirtualMachineConfigSpec{
		ChangeTrackingEnabled: types.NewBool(true),
	}

	task, err := vm.Reconfigure(vmops.ctx, configSpec)
	if err != nil {
		return fmt.Errorf("failed to enable CBT: %s", err)
	}
	err = task.Wait(vmops.ctx)
	if err != nil {
		return fmt.Errorf("failed while waiting for task: %s", err)
	}
	return nil
}

func (vmops *VMOps) TakeSnapshot(name string) error {
	vm := vmops.VMObj
	task, err := vm.CreateSnapshot(vmops.ctx, name, "", false, false)
	if err != nil {
		return fmt.Errorf("failed to take snapshot: %s", err)
	}
	err = task.Wait(vmops.ctx)
	if err != nil {
		return fmt.Errorf("failed while waiting for task: %s", err)
	}
	return nil
}

func (vmops *VMOps) DeleteSnapshot(name string) error {
	vm := vmops.VMObj
	var consolidate = true
	task, err := vm.RemoveSnapshot(vmops.ctx, name, false, &consolidate)
	if err != nil {
		return fmt.Errorf("failed to delete snapshot: %s", err)
	}
	err = task.Wait(vmops.ctx)
	if err != nil {
		return fmt.Errorf("failed while waiting for task: %s", err)
	}
	return nil
}

func (vmops *VMOps) GetSnapshot(name string) (*types.ManagedObjectReference, error) {
	vm := vmops.VMObj
	snap, err := vm.FindSnapshot(vmops.ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find snapshot: %s", err)
	}
	return snap, nil
}

func (vmops *VMOps) CustomQueryChangedDiskAreas(baseChangeID string, curSnapshot *types.ManagedObjectReference, disk *types.VirtualDisk, offset int64) (types.DiskChangeInfo, error) {
	var changedblocks types.DiskChangeInfo
	v := vmops.VMObj

	req := types.QueryChangedDiskAreas{
		This:        v.Reference(),
		Snapshot:    curSnapshot,
		DeviceKey:   disk.Key,
		StartOffset: offset,
		ChangeId:    baseChangeID,
	}
	for {
		res, err := methods.QueryChangedDiskAreas(vmops.ctx, v.Client(), &req)
		if err != nil {
			return changedblocks, fmt.Errorf("failed to query changed disk areas: %s", err)
		}
		// If there are no more changes, stop fetching the changed blocks
		if len(res.Returnval.ChangedArea) == 0 {
			break
		}
		// Append the changed blocks to the result
		changedblocks.ChangedArea = append(changedblocks.ChangedArea, res.Returnval.ChangedArea...)
		// Update the total length of the changed blocks
		changedblocks.Length += res.Returnval.Length
		// If the total length of the changed blocks is greater or equal to the disk capacity, break the loop
		if changedblocks.Length >= disk.CapacityInBytes {
			break
		}
		// Update the start offset for the next iteration
		req.StartOffset = changedblocks.Length
	}

	return changedblocks, nil
}

func (vmops *VMOps) VMPowerOff() error {
	currstate, err := vmops.VMObj.PowerState(vmops.ctx)
	if err != nil {
		return fmt.Errorf("failed to get VM power state: %s", err)
	}
	if currstate == types.VirtualMachinePowerStatePoweredOff {
		return nil
	}
	task, err := vmops.VMObj.PowerOff(vmops.ctx)
	if err != nil {
		return err
	}
	err = task.Wait(vmops.ctx)
	if err != nil {
		return fmt.Errorf("failed while waiting for power off task: %s", err)
	}
	return nil
}

func (vmops *VMOps) VMPowerOn() error {
	currstate, err := vmops.VMObj.PowerState(vmops.ctx)
	if err != nil {
		return fmt.Errorf("failed to get VM power state: %s", err)
	}
	if currstate == types.VirtualMachinePowerStatePoweredOn {
		return nil
	}
	task, err := vmops.VMObj.PowerOn(vmops.ctx)
	if err != nil {
		return err
	}
	err = task.Wait(vmops.ctx)
	if err != nil {
		return fmt.Errorf("failed while waiting for power on task: %s", err)
	}
	return nil
}

func GetVMwareMachine(ctx context.Context, client client.Client, vmName string, namespace string) (*vjailbreakv1alpha1.VMwareMachine, error) {
	// Convert VM name to k8s compatible name
	sanitizedVMName := ConvertToK8sName(vmName)

	// Create VMwareMachine objet
	vmwareMachine := &vjailbreakv1alpha1.VMwareMachine{}

	// Create namespaced name for lookup
	namespacedName := k8stypes.NamespacedName{
		Name:      sanitizedVMName,
		Namespace: namespace,
	}

	// Get VMwareMachine object
	if err := client.Get(ctx, namespacedName, vmwareMachine); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, fmt.Errorf("VMwareMachine '%s' not found in namespace '%s'", sanitizedVMName, "migration-system")
		}
		return nil, fmt.Errorf("failed to get VMwareMachine: %w", err)
	}

	return vmwareMachine, nil
}

func ConvertToK8sName(name string) string {
	// Replace invalid characters with dashes
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.ReplaceAll(name, " ", "-")

	// Remove any other invalid characters
	validChar := regexp.MustCompile(`[^a-z0-9-]`)
	name = validChar.ReplaceAllString(name, "")

	return name
}

func copyRDMDisks(vminfo *VMInfo, rdmDiskInfo *vjailbreakv1alpha1.VMwareMachine) {
	if rdmDiskInfo != nil && rdmDiskInfo.Spec.VMs.RDMDisks != nil {
		vminfo.RDMDisks = make([]RDMDisk, len(rdmDiskInfo.Spec.VMs.RDMDisks))
		for i, disk := range rdmDiskInfo.Spec.VMs.RDMDisks {
			vminfo.RDMDisks[i] = RDMDisk{
				DiskName:          disk.DiskName,
				DiskSize:          disk.DiskSize,
				UUID:              disk.UUID,
				DisplayName:       disk.DisplayName,
				OperationalState:  disk.OperationalState,
				CinderBackendPool: disk.CinderBackendPool,
				VolumeType:        disk.VolumeType,
				AvailabilityZone:  disk.AvailabilityZone,
				Bootable:          disk.Bootable,
				Description:       disk.Description,
			}
		}
	}
}
