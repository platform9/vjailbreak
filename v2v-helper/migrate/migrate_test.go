// Copyright © 2024 The vjailbreak authors
package migrate

import (
	"context"
	"testing"
	"time"

	"github.com/platform9/vjailbreak/v2v-helper/nbd"
	"github.com/platform9/vjailbreak/v2v-helper/openstack"
	"github.com/platform9/vjailbreak/v2v-helper/vm"

	"github.com/golang/mock/gomock"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumes"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
	"github.com/stretchr/testify/assert"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
)

func TestCreateVolumes(t *testing.T) {
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
			CreateVolume(inputvminfo.Name+"-"+inputvminfo.VMDisks[0].Name, inputvminfo.VMDisks[0].Size, "linux", false, "voltype-1").
			Return(&volumes.Volume{ID: "id1", Name: "test-vm-disk1"}, nil).
			Times(1),
		mockOpenStackOps.EXPECT().
			CreateVolume(inputvminfo.Name+"-"+inputvminfo.VMDisks[1].Name, inputvminfo.VMDisks[1].Size, "linux", false, "voltype-2").
			Return(&volumes.Volume{ID: "id2", Name: "test-vm-disk2"}, nil).
			Times(1),
	)
	mockOpenStackOps.EXPECT().
		SetVolumeBootable(&volumes.Volume{ID: "id1", Name: "test-vm-disk1"}).
		Return(nil).
		AnyTimes()
	gomock.InOrder(
		mockOpenStackOps.EXPECT().AttachVolumeToVM("id1").Return(nil).Times(1),
		mockOpenStackOps.EXPECT().AttachVolumeToVM("id2").Return(nil).Times(1),
	)
	gomock.InOrder(
		mockOpenStackOps.EXPECT().FindDevice("id1").Return("/dev/sda", nil).Times(1),
		mockOpenStackOps.EXPECT().FindDevice("id2").Return("/dev/sdb", nil).Times(1),
	)
	migobj := Migrate{
		Openstackclients: mockOpenStackOps,
		InPod:            false,
		Volumetypes:      []string{"voltype-1", "voltype-2"},
	}

	outputvminfo, err := migobj.CreateVolumes(inputvminfo)
	assert.NoError(t, err)
	outputvminfo.VMDisks[0].Path, err = migobj.AttachVolume(inputvminfo.VMDisks[0])
	assert.NoError(t, err)
	assert.Equal(t, "id1", outputvminfo.VMDisks[0].OpenstackVol.ID)
	assert.Equal(t, "/dev/sda", outputvminfo.VMDisks[0].Path)
	outputvminfo.VMDisks[1].Path, err = migobj.AttachVolume(inputvminfo.VMDisks[1])
	assert.NoError(t, err)
	assert.Equal(t, "id2", outputvminfo.VMDisks[1].OpenstackVol.ID)
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

	migobj := Migrate{
		VMops:         mockVMOps,
		EventReporter: make(chan string),
	}
	err := migobj.EnableCBTWrapper()
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
			{Name: "disk1", Size: int64(1024), Disk: &types.VirtualDisk{}, OpenstackVol: &volumes.Volume{ID: "id1"}},
			{Name: "disk2", Size: int64(2048), Disk: &types.VirtualDisk{}, OpenstackVol: &volumes.Volume{ID: "id2"}},
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
	dummychan := make(chan string)
	dummychan2 := make(chan string)

	mockVMOps := vm.NewMockVMOperations(ctrl)
	mockNBD := nbd.NewMockNBDOperations(ctrl)
	mockOpenStackOps := openstack.NewMockOpenstackOperations(ctrl)

	gomock.InOrder(
		mockVMOps.EXPECT().TakeSnapshot("migration-snap").Return(nil).Times(1),
		mockVMOps.EXPECT().UpdateDiskInfo(inputvminfo).Return(vm.VMInfo{
			Name:   "test-vm",
			OSType: "linux",
			UEFI:   false,
			VMDisks: []vm.VMDisk{
				{Name: "disk1", Size: int64(1024), OpenstackVol: &volumes.Volume{ID: "id1"}, Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm.vmdk", ChangeID: "1"},
				{Name: "disk2", Size: int64(2048), OpenstackVol: &volumes.Volume{ID: "id2"}, Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm_1.vmdk", ChangeID: "2"},
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
				"[ds1] test_vm/test_vm.vmdk",
				dummychan).
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
				"[ds1] test_vm/test_vm_1.vmdk",
				dummychan).
			Return(nil).
			Times(1),
		mockOpenStackOps.EXPECT().AttachVolumeToVM("id1").Return(nil).Times(1),
		mockOpenStackOps.EXPECT().FindDevice("id1").Return("/dev/sda", nil).Times(1),
		mockNBD.EXPECT().CopyDisk(context.TODO(), "/dev/sda").Return(nil).Times(1),
		mockOpenStackOps.EXPECT().DetachVolumeFromVM("id1").Return(nil).Times(1),
		mockOpenStackOps.EXPECT().WaitForVolume("id1").Return(nil).Times(1),

		mockOpenStackOps.EXPECT().AttachVolumeToVM("id2").Return(nil).Times(1),
		mockOpenStackOps.EXPECT().FindDevice("id2").Return("/dev/sda", nil).Times(1),
		mockNBD.EXPECT().CopyDisk(context.TODO(), "/dev/sda").Return(nil).Times(1),
		mockOpenStackOps.EXPECT().DetachVolumeFromVM("id2").Return(nil).Times(1),
		mockOpenStackOps.EXPECT().WaitForVolume("id2").Return(nil).Times(1),
		// 1. Both Disks Change
		mockVMOps.EXPECT().
			UpdateDiskInfo(vm.VMInfo{
				Name:   "test-vm",
				OSType: "linux",
				UEFI:   false,
				VMDisks: []vm.VMDisk{
					{Name: "disk1", Size: int64(1024), Path: "/dev/sda", OpenstackVol: &volumes.Volume{ID: "id1"}, Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm.vmdk", ChangeID: "1"},
					{Name: "disk2", Size: int64(2048), Path: "/dev/sda", OpenstackVol: &volumes.Volume{ID: "id2"}, Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm_1.vmdk", ChangeID: "2"},
				},
			}).
			Return(vm.VMInfo{
				Name:   "test-vm",
				OSType: "linux",
				UEFI:   false,
				VMDisks: []vm.VMDisk{
					{Name: "disk1", Size: int64(1024), Path: "/dev/sda", OpenstackVol: &volumes.Volume{ID: "id1"}, Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm.vmdk", ChangeID: "3"},
					{Name: "disk2", Size: int64(2048), Path: "/dev/sda", OpenstackVol: &volumes.Volume{ID: "id2"}, Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm_1.vmdk", ChangeID: "4"},
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
				"[ds1] test_vm/test_vm.vmdk",
				dummychan).
			Return(nil).
			Times(1),
		mockOpenStackOps.EXPECT().AttachVolumeToVM("id1").Return(nil).Times(1),
		mockOpenStackOps.EXPECT().FindDevice("id1").Return("/dev/sda", nil).Times(1),
		mockNBD.EXPECT().CopyChangedBlocks(context.TODO(), changedAreasexample, "/dev/sda").Return(nil).Times(1),
		mockOpenStackOps.EXPECT().DetachVolumeFromVM("id1").Return(nil).Times(1),
		mockOpenStackOps.EXPECT().WaitForVolume("id1").Return(nil).Times(1),
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
				"[ds1] test_vm/test_vm_1.vmdk",
				dummychan).
			Return(nil).
			Times(1),
		mockOpenStackOps.EXPECT().AttachVolumeToVM("id2").Return(nil).Times(1),
		mockOpenStackOps.EXPECT().FindDevice("id2").Return("/dev/sda", nil).Times(1),
		mockNBD.EXPECT().CopyChangedBlocks(context.TODO(), changedAreasexample, "/dev/sda").Return(nil).Times(1),
		mockOpenStackOps.EXPECT().DetachVolumeFromVM("id2").Return(nil).Times(1),
		mockOpenStackOps.EXPECT().WaitForVolume("id2").Return(nil).Times(1),
		// 2. Only Disk 1 Changes
		mockVMOps.EXPECT().
			UpdateDiskInfo(vm.VMInfo{
				Name:   "test-vm",
				OSType: "linux",
				UEFI:   false,
				VMDisks: []vm.VMDisk{
					{Name: "disk1", Size: int64(1024), Path: "/dev/sda", OpenstackVol: &volumes.Volume{ID: "id1"}, Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm.vmdk", ChangeID: "3"},
					{Name: "disk2", Size: int64(2048), Path: "/dev/sda", OpenstackVol: &volumes.Volume{ID: "id2"}, Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm_1.vmdk", ChangeID: "4"},
				},
			}).
			Return(vm.VMInfo{
				Name:   "test-vm",
				OSType: "linux",
				UEFI:   false,
				VMDisks: []vm.VMDisk{
					{Name: "disk1", Size: int64(1024), Path: "/dev/sda", OpenstackVol: &volumes.Volume{ID: "id1"}, Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm.vmdk", ChangeID: "5"},
					{Name: "disk2", Size: int64(2048), Path: "/dev/sda", OpenstackVol: &volumes.Volume{ID: "id2"}, Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm_1.vmdk", ChangeID: "4"},
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
		mockNBD.EXPECT().StartNBDServer(&object.VirtualMachine{}, envURL, envUserName, envPassword, thumbprint, "migration-snap", "[ds1] test_vm/test_vm.vmdk", dummychan).Return(nil).Times(1),
		mockOpenStackOps.EXPECT().AttachVolumeToVM("id1").Return(nil).Times(1),
		mockOpenStackOps.EXPECT().FindDevice("id1").Return("/dev/sda", nil).Times(1),
		mockNBD.EXPECT().CopyChangedBlocks(context.TODO(), changedAreasexample, "/dev/sda").Return(nil).Times(1),
		mockOpenStackOps.EXPECT().DetachVolumeFromVM("id1").Return(nil).Times(1),
		mockOpenStackOps.EXPECT().WaitForVolume("id1").Return(nil).Times(1),
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
					{Name: "disk1", Size: int64(1024), Path: "/dev/sda", OpenstackVol: &volumes.Volume{ID: "id1"}, Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm.vmdk", ChangeID: "5"},
					{Name: "disk2", Size: int64(2048), Path: "/dev/sda", OpenstackVol: &volumes.Volume{ID: "id2"}, Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm_1.vmdk", ChangeID: "4"},
				},
			}).
			Return(vm.VMInfo{
				Name:   "test-vm",
				OSType: "linux",
				UEFI:   false,
				VMDisks: []vm.VMDisk{
					{Name: "disk1", Size: int64(1024), Path: "/dev/sda", OpenstackVol: &volumes.Volume{ID: "id1"}, Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm.vmdk", ChangeID: "5"},
					{Name: "disk2", Size: int64(2048), Path: "/dev/sda", OpenstackVol: &volumes.Volume{ID: "id2"}, Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm_1.vmdk", ChangeID: "4"},
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
		// Final Copy
		mockVMOps.EXPECT().VMPowerOff().Return(nil).Times(1),
		mockVMOps.EXPECT().
			UpdateDiskInfo(vm.VMInfo{
				Name:   "test-vm",
				OSType: "linux",
				UEFI:   false,
				VMDisks: []vm.VMDisk{
					{Name: "disk1", Size: int64(1024), Path: "/dev/sda", OpenstackVol: &volumes.Volume{ID: "id1"}, Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm.vmdk", ChangeID: "5"},
					{Name: "disk2", Size: int64(2048), Path: "/dev/sda", OpenstackVol: &volumes.Volume{ID: "id2"}, Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm_1.vmdk", ChangeID: "4"},
				},
			}).
			Return(vm.VMInfo{
				Name:   "test-vm",
				OSType: "linux",
				UEFI:   false,
				VMDisks: []vm.VMDisk{
					{Name: "disk1", Size: int64(1024), Path: "/dev/sda", OpenstackVol: &volumes.Volume{ID: "id1"}, Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm.vmdk", ChangeID: "5"},
					{Name: "disk2", Size: int64(2048), Path: "/dev/sda", OpenstackVol: &volumes.Volume{ID: "id2"}, Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm_1.vmdk", ChangeID: "4"},
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

	migobj := Migrate{
		VMops:            mockVMOps,
		Nbdops:           []nbd.NBDOperations{mockNBD, mockNBD},
		Openstackclients: mockOpenStackOps,
		URL:              envURL,
		UserName:         envUserName,
		Password:         envPassword,
		Thumbprint:       thumbprint,
		EventReporter:    dummychan,
		PodLabelWatcher:  dummychan2,
		MigrationType:    "hot",
	}
	go func() {
		time.Sleep(30 * time.Second)
		migobj.PodLabelWatcher <- "yes"
	}()
	updatedVMInfo, err := migobj.LiveReplicateDisks(context.TODO(), inputvminfo)
	assert.NoError(t, err)
	assert.Equal(t, vm.VMInfo{
		Name:   "test-vm",
		OSType: "linux",
		UEFI:   false,
		VMDisks: []vm.VMDisk{
			{Name: "disk1", Size: int64(1024), Path: "/dev/sda", OpenstackVol: &volumes.Volume{ID: "id1"}, Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm.vmdk", ChangeID: "5"},
			{Name: "disk2", Size: int64(2048), Path: "/dev/sda", OpenstackVol: &volumes.Volume{ID: "id2"}, Snapname: "migration-snap", Disk: &types.VirtualDisk{}, SnapBackingDisk: "[ds1] test_vm/test_vm_1.vmdk", ChangeID: "4"},
		},
	}, updatedVMInfo)
}

func TestDetachAllVolumes(t *testing.T) {
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

	migobj := Migrate{
		Openstackclients: mockOpenStackOps,
		InPod:            false,
	}

	err := migobj.DetachAllVolumes(inputvminfo)
	assert.NoError(t, err)
}

func TestDeleteAllVolumes(t *testing.T) {
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

	migobj := Migrate{
		Openstackclients: mockOpenStackOps,
		InPod:            false,
	}

	err := migobj.DeleteAllVolumes(vminfo)
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
	mockOpenStackOps.EXPECT().GetNetwork(gomock.Any()).Return(&networks.Network{}, nil).Times(1)
	mockOpenStackOps.EXPECT().CreatePort(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&ports.Port{
		MACAddress: "mac-address",
		FixedIPs: []ports.IP{
			{IPAddress: "ip-address"},
		},
	}, nil).Times(1)
	mockOpenStackOps.EXPECT().GetNetwork(gomock.Any()).Return(&networks.Network{}, nil).Times(1)
	mockOpenStackOps.EXPECT().CreatePort(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&ports.Port{
		MACAddress: "mac-address",
		FixedIPs: []ports.IP{
			{IPAddress: "ip-address"},
		},
	}, nil).Times(1)
	mockOpenStackOps.EXPECT().CreateVM(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&servers.Server{}, nil).Times(1)

	inputvminfo := vm.VMInfo{
		Name:   "test-vm",
		OSType: "linux",
		Mac: []string{
			"mac-address-1",
			"mac-address-2",
		},
		IPs: []string{
			"ip-address-1",
			"ip-address-2",
		},
	}

	migobj := Migrate{
		Openstackclients: mockOpenStackOps,
		Networknames:     []string{"network-name-1", "network-name-2"},
		InPod:            false,
	}
	err := migobj.CreateTargetInstance(inputvminfo)
	assert.NoError(t, err)
}

func TestCreateTargetInstance_AdvancedMapping_Ports(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOpenStackOps := openstack.NewMockOpenstackOperations(ctrl)
	mockOpenStackOps.EXPECT().GetClosestFlavour(gomock.Any(), gomock.Any()).Return(&flavors.Flavor{
		VCPUs: 2,
		RAM:   2048,
	}, nil).Times(1)
	mockOpenStackOps.EXPECT().GetPort("port-1").Return(&ports.Port{
		ID:        "port-1-id",
		NetworkID: "network-1",
		FixedIPs: []ports.IP{
			{IPAddress: "ip-address-1"},
		},
	}, nil).Times(1)
	mockOpenStackOps.EXPECT().GetPort("port-2").Return(&ports.Port{
		ID:        "port-2-id",
		NetworkID: "network-2",
		FixedIPs: []ports.IP{
			{IPAddress: "ip-address-2"},
		},
	}, nil).Times(1)
	mockOpenStackOps.EXPECT().CreateVM(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&servers.Server{}, nil).Times(1)

	inputvminfo := vm.VMInfo{
		Name:   "test-vm",
		OSType: "linux",
		Mac: []string{
			"mac-address-1",
			"mac-address-2",
		},
		IPs: []string{
			"ip-address-1",
			"ip-address-2",
		},
	}

	migobj := Migrate{
		Openstackclients: mockOpenStackOps,
		Networknames:     []string{"network-name-1", "network-name-2"},
		Networkports:     []string{"port-1", "port-2"},
		InPod:            false,
	}
	err := migobj.CreateTargetInstance(inputvminfo)
	assert.NoError(t, err)
}

func TestCreateTargetInstance_AdvancedMapping_InsufficientPorts(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOpenStackOps := openstack.NewMockOpenstackOperations(ctrl)
	mockOpenStackOps.EXPECT().GetClosestFlavour(gomock.Any(), gomock.Any()).Return(&flavors.Flavor{
		VCPUs: 2,
		RAM:   2048,
	}, nil).Times(1)

	inputvminfo := vm.VMInfo{
		Name:   "test-vm",
		OSType: "linux",
		Mac: []string{
			"mac-address-1",
			"mac-address-2",
		},
		IPs: []string{
			"ip-address-1",
			"ip-address-2",
		},
	}

	migobj := Migrate{
		Openstackclients: mockOpenStackOps,
		Networknames:     []string{"network-name-1", "network-name-2"},
		Networkports:     []string{"port-1"},
		InPod:            false,
	}
	err := migobj.CreateTargetInstance(inputvminfo)
	assert.Contains(t, err.Error(), "number of network ports does not match number of network names")
}
