package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumes"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

type VMOperations interface {
	GetVMInfo() (VMInfo, error)
	UpdateDiskInfo(vminfo VMInfo) (VMInfo, error)
	IsCBTEnabled() (bool, error)
	EnableCBT() error
	TakeSnapshot(name string) error
	DeleteSnapshot(name string) error
	GetSnapshot(name string) (*types.ManagedObjectReference, error)
	CustomQueryChangedDiskAreas(baseChangeID string, curSnapshot *types.ManagedObjectReference, disk *types.VirtualDisk, offset int64) (types.DiskChangeInfo, error)
}

type VMInfo struct {
	CPU     int32
	Memory  int32
	State   types.VirtualMachinePowerState
	Mac     []string
	UUID    string
	Host    string
	VMDisks []VMDisk
	VddkURL string
	UEFI    bool
	Name    string
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
}

type VMOps struct {
	vcclient *VCenterClient
	VMObj    *object.VirtualMachine
}

func VMOpsBuilder(vcclient VCenterClient, name string) (*VMOps, error) {
	vm, err := vcclient.GetVMByName(name)
	if err != nil {
		return nil, err
	}
	if vm == nil {
		return nil, fmt.Errorf("VM %s not found", name)
	}
	return &VMOps{vcclient: &vcclient, VMObj: vm}, nil

}

// func getVMs(ctx context.Context, finder *find.Finder) ([]*object.VirtualMachine, error) {
// 	// Find all virtual machines on the host
// 	vms, err := finder.VirtualMachineList(ctx, "*")
// 	if err != nil {
// 		return nil, err
// 	}
// 	return vms, nil
// }

func (vmops *VMOps) GetVMInfo() (VMInfo, error) {
	vm := vmops.VMObj

	var o mo.VirtualMachine
	vm.Properties(context.Background(), vm.Reference(), []string{}, &o)
	var mac []string
	for _, device := range o.Config.Hardware.Device {
		if nic, ok := device.(types.BaseVirtualEthernetCard); ok {
			mac = append(mac, nic.GetVirtualEthernetCard().MacAddress)
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
	vminfo := VMInfo{
		CPU:     o.Config.Hardware.NumCPU,
		Memory:  o.Config.Hardware.MemoryMB,
		State:   o.Runtime.PowerState,
		Mac:     mac,
		UUID:    o.Config.Uuid,
		Host:    o.Runtime.Host.Reference().Value,
		Name:    o.Name,
		VMDisks: vmdisks,
		UEFI:    uefi,
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

func (vmops *VMOps) UpdateDiskInfo(vminfo VMInfo) (VMInfo, error) {
	pc := vmops.vcclient.VCPropertyCollector
	vm := vmops.VMObj
	var snapbackingdisk []string
	var snapname []string
	var snapid []string

	var o mo.VirtualMachine
	vm.Properties(context.Background(), vm.Reference(), []string{}, &o)

	if o.Snapshot != nil {
		// get backing disk of snapshot
		var s mo.VirtualMachineSnapshot
		pc.RetrieveOne(context.Background(), o.Snapshot.CurrentSnapshot.Reference(), []string{}, &s)

		for _, device := range s.Config.Hardware.Device {
			switch disk := device.(type) {
			case *types.VirtualDisk:
				backing := disk.Backing.(types.BaseVirtualDeviceFileBackingInfo)
				info := backing.GetVirtualDeviceFileBackingInfo()
				snapbackingdisk = append(snapbackingdisk, info.FileName)
				snapname = append(snapname, o.Snapshot.CurrentSnapshot.Value)
				changeid, err := getChangeID(disk)
				if err != nil {
					return vminfo, err
				}
				snapid = append(snapid, changeid.Value)
			}
		}
		for idx, _ := range vminfo.VMDisks {
			vminfo.VMDisks[idx].SnapBackingDisk = snapbackingdisk[idx]
			vminfo.VMDisks[idx].Snapname = snapname[idx]
			vminfo.VMDisks[idx].ChangeID = snapid[idx]
		}
	}

	return vminfo, nil
}

func (vmops *VMOps) IsCBTEnabled() (bool, error) {
	vm := vmops.VMObj
	var o mo.VirtualMachine
	vm.Properties(context.Background(), vm.Reference(), []string{"config.changeTrackingEnabled"}, &o)
	return *o.Config.ChangeTrackingEnabled, nil
}

func (vmops *VMOps) EnableCBT() error {
	vm := vmops.VMObj
	configSpec := types.VirtualMachineConfigSpec{
		ChangeTrackingEnabled: types.NewBool(true),
	}

	task, err := vm.Reconfigure(context.Background(), configSpec)
	if err != nil {
		return err
	}
	task.Wait(context.Background())
	return nil
}

func (vmops *VMOps) TakeSnapshot(name string) error {
	vm := vmops.VMObj
	task, err := vm.CreateSnapshot(context.Background(), name, "", false, false)
	if err != nil {
		return err
	}
	task.Wait(context.Background())
	return nil
}

func (vmops *VMOps) DeleteSnapshot(name string) error {
	vm := vmops.VMObj
	var consolidate = true
	task, err := vm.RemoveSnapshot(context.Background(), name, false, &consolidate)
	if err != nil {
		return err
	}
	task.Wait(context.Background())
	return nil
}

func (vmops *VMOps) GetSnapshot(name string) (*types.ManagedObjectReference, error) {
	vm := vmops.VMObj
	snap, err := vm.FindSnapshot(context.Background(), name)
	if err != nil {
		return nil, err
	}
	return snap, nil
}

func (vmops *VMOps) CustomQueryChangedDiskAreas(baseChangeID string, curSnapshot *types.ManagedObjectReference, disk *types.VirtualDisk, offset int64) (types.DiskChangeInfo, error) {
	var noChange types.DiskChangeInfo
	var err error
	v := vmops.VMObj

	req := types.QueryChangedDiskAreas{
		This:        v.Reference(),
		Snapshot:    curSnapshot,
		DeviceKey:   disk.Key,
		StartOffset: offset,
		ChangeId:    baseChangeID,
	}

	res, err := methods.QueryChangedDiskAreas(context.Background(), v.Client(), &req)
	if err != nil {
		return noChange, err
	}

	return res.Returnval, nil
}

// func (vmops *VMOps) LiveReplicateDisks() (string, error) {

// }
