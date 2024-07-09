package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumes"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

type VMInfo struct {
	CPU     int32
	Memory  int32
	State   types.VirtualMachinePowerState
	Mac     []string
	UUID    string
	Host    string
	VM      mo.VirtualMachine
	VMDisks []VMDisk
	VddkURL string
	UEFI    bool
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

func getDatacenters(ctx context.Context, finder *find.Finder) ([]*object.Datacenter, error) {
	// Find all datacenters
	datacenters, err := finder.DatacenterList(ctx, "*")
	if err != nil {
		return nil, err
	}

	return datacenters, nil
}

// func getVMs(ctx context.Context, finder *find.Finder) ([]*object.VirtualMachine, error) {
// 	// Find all virtual machines on the host
// 	vms, err := finder.VirtualMachineList(ctx, "*")
// 	if err != nil {
// 		return nil, err
// 	}
// 	return vms, nil
// }

// get VM by name
func GetVMByName(ctx context.Context, name string) (*object.VirtualMachine, error) {
	client := ctx.Value("govmomi_client").(*vim25.Client)
	finder := find.NewFinder(client, false)
	datacenters, err := getDatacenters(ctx, finder)
	if err != nil {
		return nil, err
	}
	for _, datacenter := range datacenters {
		finder.SetDatacenter(datacenter)
		vm, err := finder.VirtualMachine(ctx, name)
		if err == nil {
			return vm, nil
		}
	}
	return nil, err
}

func GetVMInfo(ctx context.Context) (VMInfo, error) {
	vm := ctx.Value("vm").(*object.VirtualMachine)

	var o mo.VirtualMachine
	vm.Properties(ctx, vm.Reference(), []string{}, &o)
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
		VM:      o,
		VMDisks: vmdisks,
		UEFI:    uefi,
	}
	return vminfo, nil
}

func ParseChangeID(changeId string) (*ChangeID, error) {
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

func GetChangeID(disk *types.VirtualDisk) (*ChangeID, error) {
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
	return ParseChangeID(changeId)
}

func UpdateDiskInfo(ctx context.Context, vminfo VMInfo) (VMInfo, error) {
	client := ctx.Value("govmomi_client").(*vim25.Client)
	pc := property.DefaultCollector(client)
	vm := ctx.Value("vm").(*object.VirtualMachine)
	var snapbackingdisk []string
	var snapname []string
	var snapid []string

	var o mo.VirtualMachine
	vm.Properties(ctx, vm.Reference(), []string{}, &o)

	if o.Snapshot != nil {
		// get backing disk of snapshot
		var s mo.VirtualMachineSnapshot
		pc.RetrieveOne(ctx, o.Snapshot.CurrentSnapshot.Reference(), []string{}, &s)

		for _, device := range s.Config.Hardware.Device {
			switch disk := device.(type) {
			case *types.VirtualDisk:
				backing := disk.Backing.(types.BaseVirtualDeviceFileBackingInfo)
				info := backing.GetVirtualDeviceFileBackingInfo()
				snapbackingdisk = append(snapbackingdisk, info.FileName)
				snapname = append(snapname, o.Snapshot.CurrentSnapshot.Value)
				changeid, err := GetChangeID(disk)
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

func IsCBTEnabled(ctx context.Context) (bool, error) {
	vm := ctx.Value("vm").(*object.VirtualMachine)
	var o mo.VirtualMachine
	vm.Properties(ctx, vm.Reference(), []string{"config.changeTrackingEnabled"}, &o)
	return *o.Config.ChangeTrackingEnabled, nil
}

func EnableCBT(ctx context.Context) error {
	vm := ctx.Value("vm").(*object.VirtualMachine)
	configSpec := types.VirtualMachineConfigSpec{
		ChangeTrackingEnabled: types.NewBool(true),
	}

	task, err := vm.Reconfigure(ctx, configSpec)
	if err != nil {
		return err
	}
	task.Wait(ctx)
	return nil
}

func TakeSnapshot(ctx context.Context, name string) error {
	vm := ctx.Value("vm").(*object.VirtualMachine)
	task, err := vm.CreateSnapshot(ctx, name, "", false, false)
	if err != nil {
		return err
	}
	task.Wait(ctx)
	return nil
}

func DeleteSnapshot(ctx context.Context, name string) error {
	vm := ctx.Value("vm").(*object.VirtualMachine)
	var consolidate = true
	task, err := vm.RemoveSnapshot(ctx, name, false, &consolidate)
	if err != nil {
		return err
	}
	task.Wait(ctx)
	return nil
}

func GetSnapshot(ctx context.Context, name string) (*types.ManagedObjectReference, error) {
	vm := ctx.Value("vm").(*object.VirtualMachine)
	snap, err := vm.FindSnapshot(ctx, name)
	if err != nil {
		return nil, err
	}
	return snap, nil
}

func CustomQueryChangedDiskAreas(ctx context.Context, baseChangeID string, curSnapshot *types.ManagedObjectReference, disk *types.VirtualDisk, offset int64) (types.DiskChangeInfo, error) {
	var noChange types.DiskChangeInfo
	var err error
	v := ctx.Value("vm").(*object.VirtualMachine)

	req := types.QueryChangedDiskAreas{
		This:        v.Reference(),
		Snapshot:    curSnapshot,
		DeviceKey:   disk.Key,
		StartOffset: offset,
		ChangeId:    baseChangeID,
	}

	res, err := methods.QueryChangedDiskAreas(ctx, v.Client(), &req)
	if err != nil {
		return noChange, err
	}

	return res.Returnval, nil
}

// This was used in to get all the information about the VMs in the vCenter
// func getAllInfo(ctx context.Context, client *vim25.Client, vcenterurl, vcenteruser string) (map[string]map[string]map[string]map[string]VMInfo, error) {
// 	data := make(map[string]map[string]map[string]map[string]VMInfo)
// 	finder := find.NewFinder(client, false)
// 	pc := property.DefaultCollector(client)
// 	// get all the datacenters
// 	datacenters, err := getDatacenters(ctx, finder)
// 	if err != nil {
// 		return nil, err
// 	}
// 	// print all the datacenters
// 	for _, datacenter := range datacenters {
// 		data[datacenter.Name()] = make(map[string]map[string]map[string]VMInfo)
// 		// fmt.Printf("%+v\n", data)
// 		// get all the clusters
// 		finder.SetDatacenter(datacenter)

// 		// print all the VMs
// 		vms, err := getVMs(ctx, finder)
// 		if err != nil {
// 			return nil, err
// 		}

// 		for _, vm := range vms {
// 			var o mo.VirtualMachine
// 			vm.Properties(ctx, vm.Reference(), []string{}, &o)

// 			// fmt.Println(o.Runtime.Host)
// 			// get mac address of vm in a list

// 			var h mo.HostSystem
// 			pc.RetrieveOne(ctx, o.Runtime.Host.Reference(), []string{}, &h)
// 			var host = h.Summary.Config.Name
// 			// fmt.Println(h.Summary.Config.Name, h.Parent)

// 			var c mo.ClusterComputeResource
// 			pc.RetrieveOne(ctx, h.Parent.Reference(), []string{}, &c)
// 			var cluster = c.Name

// 			var mac []string
// 			for _, device := range o.Config.Hardware.Device {
// 				if nic, ok := device.(types.BaseVirtualEthernetCard); ok {
// 					mac = append(mac, nic.GetVirtualEthernetCard().MacAddress)
// 				}
// 			}
// 			if o.Snapshot != nil {
// 				// get backing disk of snapshot
// 				var s mo.VirtualMachineSnapshot
// 				pc.RetrieveOne(ctx, o.Snapshot.CurrentSnapshot.Reference(), []string{}, &s)

// 				for _, device := range s.Config.Hardware.Device {
// 					switch disk := device.(type) {
// 					case *types.VirtualDisk:
// 						backing := disk.Backing.(types.BaseVirtualDeviceFileBackingInfo)
// 						info := backing.GetVirtualDeviceFileBackingInfo()
// 						fmt.Println(info.FileName)
// 						fmt.Println(o.Snapshot.CurrentSnapshot.Value)
// 					}
// 				}
// 			}

// 			// fmt.Println("Datacenter:", datacenter.Name(), " Cluster:", cluster, " Host:", host, " VM:", o.Name)

// 			if _, ok := data[datacenter.Name()][cluster]; !ok {
// 				data[datacenter.Name()][cluster] = make(map[string]map[string]VMInfo)
// 			}
// 			if _, ok := data[datacenter.Name()][cluster][host]; !ok {
// 				data[datacenter.Name()][cluster][host] = make(map[string]VMInfo)
// 			}

// 			data[datacenter.Name()][cluster][host][vm.Name()] = VMInfo{
// 				CPU:    o.Config.Hardware.NumCPU,
// 				Memory: o.Config.Hardware.MemoryMB,
// 				State:  o.Runtime.PowerState,
// 				Mac:    mac,
// 				UUID:   o.Config.Uuid,
// 				Host:   o.Runtime.Host.Reference().Value,
// 				VM:     vm,

// 				VddkURL: GenerateVDDKUrl(vcenteruser, vcenterurl, datacenter.Name(), cluster, host),
// 			}
// 		}
// 	}

// 	return data, nil
// }
