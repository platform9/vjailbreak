package migrate

import (
	"testing"
	"vjailbreak/nbd"
	"vjailbreak/openstack"
	"vjailbreak/vm"

	"github.com/golang/mock/gomock"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumes"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
	"github.com/stretchr/testify/assert"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
)

func TestAddVolumestoHost(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	inputvminfo := vm.VMInfo{
		Name:   "test-vm",
		OSType: "linux",
		UEFI:   false,
		VMDisks: []vm.VMDisk{
			{Name: "disk1", Size: int64(1024)},
			{Name: "disk2", Size: int64(2048)},
		},
	}

	mockOpenStackOps := openstack.NewMockOpenstackOperations(ctrl)

	gomock.InOrder(
		mockOpenStackOps.EXPECT().
			CreateVolume(inputvminfo.Name+"-"+inputvminfo.VMDisks[0].Name, inputvminfo.VMDisks[0].Size, "linux", false).
			Return(&volumes.Volume{ID: "id1", Name: "test-vm-disk1"}, nil).
			Times(1),
		mockOpenStackOps.EXPECT().
			CreateVolume(inputvminfo.Name+"-"+inputvminfo.VMDisks[1].Name, inputvminfo.VMDisks[1].Size, "linux", false).
			Return(&volumes.Volume{ID: "id2", Name: "test-vm-disk2"}, nil).
			Times(1),
	)
	mockOpenStackOps.EXPECT().
		SetVolumeBootable(&volumes.Volume{ID: "id1", Name: "test-vm-disk1"}).
		Return(nil).
		Times(1)
	gomock.InOrder(
		mockOpenStackOps.EXPECT().AttachVolumeToVM("id1").Return(nil).Times(1),
		mockOpenStackOps.EXPECT().AttachVolumeToVM("id2").Return(nil).Times(1),
	)
	gomock.InOrder(
		mockOpenStackOps.EXPECT().FindDevice("id1").Return("/dev/sda", nil).Times(1),
		mockOpenStackOps.EXPECT().FindDevice("id2").Return("/dev/sdb", nil).Times(1),
	)

	outputvminfo, err := AddVolumestoHost(inputvminfo, mockOpenStackOps)
	assert.NoError(t, err)
	assert.Equal(t, "id1", outputvminfo.VMDisks[0].OpenstackVol.ID)
	assert.Equal(t, "id2", outputvminfo.VMDisks[1].OpenstackVol.ID)
	assert.Equal(t, "/dev/sda", outputvminfo.VMDisks[0].Path)
	assert.Equal(t, "/dev/sdb", outputvminfo.VMDisks[1].Path)

}

func TestEnableCBTWrapper(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockVMOps := vm.NewMockVMOperations(ctrl)
	gomock.InOrder(
		mockVMOps.EXPECT().IsCBTEnabled().Return(false, nil).Times(1),
		mockVMOps.EXPECT().EnableCBT().Return(nil).Times(1),
		mockVMOps.EXPECT().IsCBTEnabled().Return(true, nil).Times(1),
		mockVMOps.EXPECT().TakeSnapshot(gomock.Any()).Return(nil).Times(1),
		mockVMOps.EXPECT().DeleteSnapshot(gomock.Any()).Return(nil).Times(1),
	)

	err := EnableCBTWrapper(mockVMOps)
	assert.NoError(t, err)
}

func TestLiveReplicateDisks(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	inputvminfo := vm.VMInfo{
		Name:   "test-vm",
		OSType: "linux",
		UEFI:   false,
		VMDisks: []vm.VMDisk{
			{Name: "disk1", Size: int64(1024), Disk: &types.VirtualDisk{}, Path: "/dev/sda"},
			{Name: "disk2", Size: int64(2048), Disk: &types.VirtualDisk{}, Path: "/dev/sdb"},
		},
	}

	changedAreasexample := types.DiskChangeInfo{
		StartOffset: int64(0),
		Length:      int64(1024),
		ChangedArea: []types.DiskChangeExtent{
			{
				Start:  int64(0),
				Length: int64(10),
			},
			{
				Start:  int64(1000),
				Length: int64(15),
			},
		},
	}
	envURL := "envURL"
	envUserName := "envUserName"
	envPassword := "envPassword"
	thumbprint := "thumbprint"

	mockVMOps := vm.NewMockVMOperations(ctrl)
	mockNBD := nbd.NewMockNBDOperations(ctrl)

	gomock.InOrder(
		mockVMOps.EXPECT().TakeSnapshot("migration-snap").Return(nil).Times(1),
		mockVMOps.EXPECT().UpdateDiskInfo(inputvminfo).Return(vm.VMInfo{
			Name:   "test-vm",
			OSType: "linux",
			UEFI:   false,
			VMDisks: []vm.VMDisk{
				{Name: "disk1", Size: int64(1024), Path: "/dev/sda", Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm.vmdk", ChangeID: "1"},
				{Name: "disk2", Size: int64(2048), Path: "/dev/sdb", Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm_1.vmdk", ChangeID: "2"},
			},
		}, nil).Times(1),
		mockVMOps.EXPECT().GetVMObj().Return(&object.VirtualMachine{}).Times(1),
		mockNBD.EXPECT().
			StartNBDServer(
				&object.VirtualMachine{},
				envURL,
				envUserName,
				envPassword,
				thumbprint,
				"migration-snap",
				"[ds1] test_vm/test_vm.vmdk").
			Return(nil).
			Times(1),
		mockVMOps.EXPECT().GetVMObj().Return(&object.VirtualMachine{}).Times(1),
		mockNBD.EXPECT().
			StartNBDServer(
				&object.VirtualMachine{},
				envURL,
				envUserName,
				envPassword,
				thumbprint,
				"migration-snap",
				"[ds1] test_vm/test_vm_1.vmdk").
			Return(nil).
			Times(1),
		mockNBD.EXPECT().CopyDisk(inputvminfo.VMDisks[0].Path).Return(nil).Times(1),
		mockNBD.EXPECT().CopyDisk(inputvminfo.VMDisks[1].Path).Return(nil).Times(1),
		// 1. Both Disks Change
		mockVMOps.EXPECT().
			UpdateDiskInfo(vm.VMInfo{
				Name:   "test-vm",
				OSType: "linux",
				UEFI:   false,
				VMDisks: []vm.VMDisk{
					{Name: "disk1", Size: int64(1024), Path: "/dev/sda", Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm.vmdk", ChangeID: "1"},
					{Name: "disk2", Size: int64(2048), Path: "/dev/sdb", Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm_1.vmdk", ChangeID: "2"},
				},
			}).
			Return(vm.VMInfo{
				Name:   "test-vm",
				OSType: "linux",
				UEFI:   false,
				VMDisks: []vm.VMDisk{
					{Name: "disk1", Size: int64(1024), Path: "/dev/sda", Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm.vmdk", ChangeID: "3"},
					{Name: "disk2", Size: int64(2048), Path: "/dev/sdb", Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm_1.vmdk", ChangeID: "4"},
				},
			}, nil).
			Times(1),
		mockVMOps.EXPECT().DeleteSnapshot("migration-snap").Return(nil).Times(1),
		mockVMOps.EXPECT().TakeSnapshot("migration-snap").Return(nil).Times(1),
		mockVMOps.EXPECT().GetSnapshot("migration-snap").Return(&types.ManagedObjectReference{}, nil).Times(1),
		// Incremental Copy Disk 1
		mockVMOps.EXPECT().
			CustomQueryChangedDiskAreas("3", &types.ManagedObjectReference{}, &types.VirtualDisk{}, int64(0)).
			Return(changedAreasexample, nil).Times(1),
		mockNBD.EXPECT().StopNBDServer().Return(nil).Times(1),
		mockVMOps.EXPECT().GetVMObj().Return(&object.VirtualMachine{}).Times(1),
		mockNBD.EXPECT().
			StartNBDServer(
				&object.VirtualMachine{},
				envURL,
				envUserName,
				envPassword,
				thumbprint,
				"migration-snap",
				"[ds1] test_vm/test_vm.vmdk").
			Return(nil).
			Times(1),
		mockNBD.EXPECT().CopyChangedBlocks(changedAreasexample, inputvminfo.VMDisks[0].Path).Return(nil).Times(1),
		// Incremental Copy Disk 2
		mockVMOps.EXPECT().
			CustomQueryChangedDiskAreas("4", &types.ManagedObjectReference{}, &types.VirtualDisk{}, int64(0)).
			Return(changedAreasexample, nil).Times(1),
		mockNBD.EXPECT().StopNBDServer().Return(nil).Times(1),
		mockVMOps.EXPECT().GetVMObj().Return(&object.VirtualMachine{}).Times(1),
		mockNBD.EXPECT().
			StartNBDServer(
				&object.VirtualMachine{},
				envURL,
				envUserName,
				envPassword,
				thumbprint,
				"migration-snap",
				"[ds1] test_vm/test_vm_1.vmdk").
			Return(nil).
			Times(1),
		mockNBD.EXPECT().CopyChangedBlocks(changedAreasexample, inputvminfo.VMDisks[1].Path).Return(nil).Times(1),
		// 2. Only Disk 1 Changes
		mockVMOps.EXPECT().
			UpdateDiskInfo(vm.VMInfo{
				Name:   "test-vm",
				OSType: "linux",
				UEFI:   false,
				VMDisks: []vm.VMDisk{
					{Name: "disk1", Size: int64(1024), Path: "/dev/sda", Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm.vmdk", ChangeID: "3"},
					{Name: "disk2", Size: int64(2048), Path: "/dev/sdb", Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm_1.vmdk", ChangeID: "4"},
				},
			}).
			Return(vm.VMInfo{
				Name:   "test-vm",
				OSType: "linux",
				UEFI:   false,
				VMDisks: []vm.VMDisk{
					{Name: "disk1", Size: int64(1024), Path: "/dev/sda", Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm.vmdk", ChangeID: "5"},
					{Name: "disk2", Size: int64(2048), Path: "/dev/sdb", Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm_1.vmdk", ChangeID: "4"},
				},
			}, nil).
			Times(1),
		mockVMOps.EXPECT().DeleteSnapshot("migration-snap").Return(nil).Times(1),
		mockVMOps.EXPECT().TakeSnapshot("migration-snap").Return(nil).Times(1),
		mockVMOps.EXPECT().GetSnapshot("migration-snap").Return(&types.ManagedObjectReference{}, nil).Times(1),
		// Incremental Copy Disk 1
		mockVMOps.EXPECT().
			CustomQueryChangedDiskAreas("5", &types.ManagedObjectReference{}, &types.VirtualDisk{}, int64(0)).
			Return(changedAreasexample, nil).Times(1),
		mockNBD.EXPECT().StopNBDServer().Return(nil).Times(1),
		mockVMOps.EXPECT().GetVMObj().Return(&object.VirtualMachine{}).Times(1),
		mockNBD.EXPECT().StartNBDServer(&object.VirtualMachine{}, envURL, envUserName, envPassword, thumbprint, "migration-snap", "[ds1] test_vm/test_vm.vmdk").Return(nil).Times(1),
		mockNBD.EXPECT().CopyChangedBlocks(changedAreasexample, inputvminfo.VMDisks[0].Path).Return(nil).Times(1),
		// No copy for Disk 2
		mockVMOps.EXPECT().
			CustomQueryChangedDiskAreas("4", &types.ManagedObjectReference{}, &types.VirtualDisk{}, int64(0)).
			Return(types.DiskChangeInfo{ChangedArea: []types.DiskChangeExtent{}}, nil).Times(1),
		// 3. No disk changes
		mockVMOps.EXPECT().
			UpdateDiskInfo(vm.VMInfo{
				Name:   "test-vm",
				OSType: "linux",
				UEFI:   false,
				VMDisks: []vm.VMDisk{
					{Name: "disk1", Size: int64(1024), Path: "/dev/sda", Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm.vmdk", ChangeID: "5"},
					{Name: "disk2", Size: int64(2048), Path: "/dev/sdb", Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm_1.vmdk", ChangeID: "4"},
				},
			}).
			Return(vm.VMInfo{
				Name:   "test-vm",
				OSType: "linux",
				UEFI:   false,
				VMDisks: []vm.VMDisk{
					{Name: "disk1", Size: int64(1024), Path: "/dev/sda", Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm.vmdk", ChangeID: "5"},
					{Name: "disk2", Size: int64(2048), Path: "/dev/sdb", Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm_1.vmdk", ChangeID: "4"},
				},
			}, nil).
			Times(1),
		mockVMOps.EXPECT().DeleteSnapshot("migration-snap").Return(nil).Times(1),
		mockVMOps.EXPECT().TakeSnapshot("migration-snap").Return(nil).Times(1),
		mockVMOps.EXPECT().GetSnapshot("migration-snap").Return(&types.ManagedObjectReference{}, nil).Times(1),
		// No copy for Disk 1
		mockVMOps.EXPECT().
			CustomQueryChangedDiskAreas("5", &types.ManagedObjectReference{}, &types.VirtualDisk{}, int64(0)).
			Return(types.DiskChangeInfo{ChangedArea: []types.DiskChangeExtent{}}, nil).Times(1),
		// No copy for Disk 2
		mockVMOps.EXPECT().
			CustomQueryChangedDiskAreas("4", &types.ManagedObjectReference{}, &types.VirtualDisk{}, int64(0)).
			Return(types.DiskChangeInfo{ChangedArea: []types.DiskChangeExtent{}}, nil).Times(1),
		mockNBD.EXPECT().StopNBDServer().Return(nil).Times(1),
		mockNBD.EXPECT().StopNBDServer().Return(nil).Times(1),
		mockVMOps.EXPECT().DeleteSnapshot("migration-snap").Return(nil).Times(1),
	)

	updatedVMInfo, err := LiveReplicateDisks(inputvminfo, mockVMOps, []nbd.NBDOperations{mockNBD, mockNBD}, envURL, envUserName, envPassword, thumbprint)
	assert.NoError(t, err)
	assert.Equal(t, vm.VMInfo{
		Name:   "test-vm",
		OSType: "linux",
		UEFI:   false,
		VMDisks: []vm.VMDisk{
			{Name: "disk1", Size: int64(1024), Path: "/dev/sda", Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm.vmdk", ChangeID: "5"},
			{Name: "disk2", Size: int64(2048), Path: "/dev/sdb", Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm_1.vmdk", ChangeID: "4"},
		},
	}, updatedVMInfo)
}

func TestDetachAllDisks(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOpenStackOps := openstack.NewMockOpenstackOperations(ctrl)
	gomock.InOrder(
		mockOpenStackOps.EXPECT().DetachVolumeFromVM(gomock.Any()).Return(nil).Times(1),
		mockOpenStackOps.EXPECT().DetachVolumeFromVM(gomock.Any()).Return(nil).Times(1),
	)
	gomock.InOrder(
		mockOpenStackOps.EXPECT().WaitForVolume(gomock.Any()).Return(nil).Times(1),
		mockOpenStackOps.EXPECT().WaitForVolume(gomock.Any()).Return(nil).Times(1),
	)

	inputvminfo := vm.VMInfo{
		Name:   "test-vm",
		OSType: "linux",
		UEFI:   false,
		VMDisks: []vm.VMDisk{
			{Name: "disk1", Size: int64(1024), OpenstackVol: &volumes.Volume{ID: "id1"}},
			{Name: "disk2", Size: int64(2048), OpenstackVol: &volumes.Volume{ID: "id2"}},
		},
	}

	err := DetachAllDisks(inputvminfo, mockOpenStackOps)
	assert.NoError(t, err)
}

func TestDeleteAllDisks(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOpenStackOps := openstack.NewMockOpenstackOperations(ctrl)
	mockOpenStackOps.EXPECT().DeleteVolume(gomock.Any()).Return(nil).AnyTimes()

	vminfo := vm.VMInfo{
		VMDisks: []vm.VMDisk{
			{OpenstackVol: &volumes.Volume{ID: "id1"}},
			{OpenstackVol: &volumes.Volume{ID: "id2"}},
		},
	}

	err := DeleteAllDisks(vminfo, mockOpenStackOps)
	assert.NoError(t, err)
}

func TestCreateTargetInstance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOpenStackOps := openstack.NewMockOpenstackOperations(ctrl)
	mockOpenStackOps.EXPECT().GetClosestFlavour(gomock.Any(), gomock.Any()).Return(&flavors.Flavor{
		VCPUs: 2,
		RAM:   2048,
	}, nil).Times(1)
	mockOpenStackOps.EXPECT().GetNetworkID(gomock.Any()).Return("network-id", nil).Times(1)
	mockOpenStackOps.EXPECT().CreatePort(gomock.Any(), gomock.Any()).Return(&ports.Port{
		MACAddress: "mac-address",
		FixedIPs: []ports.IP{
			{IPAddress: "ip-address"},
		},
	}, nil).Times(1)
	mockOpenStackOps.EXPECT().CreateVM(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&servers.Server{}, nil).Times(1)

	vminfo := vm.VMInfo{
		CPU:    2,
		Memory: 2048,
	}

	err := CreateTargetInstance(vminfo, mockOpenStackOps, "network-name")
	assert.NoError(t, err)
}
