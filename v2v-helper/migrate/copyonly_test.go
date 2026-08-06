// Copyright © 2024 The vjailbreak authors
package migrate

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/gophercloud/gophercloud/v2/openstack/blockstorage/v3/volumes"
	"github.com/stretchr/testify/assert"

	"github.com/platform9/vjailbreak/v2v-helper/openstack"
	"github.com/platform9/vjailbreak/v2v-helper/vm"
)

// TestIsCopyOnly verifies copy-only is the inverse of the Convert flag that the controller sets via
// the CONVERT ConfigMap key.
func TestIsCopyOnly(t *testing.T) {
	tests := []struct {
		name    string
		convert bool
		want    bool
	}{
		{name: "convert enabled is a normal migration", convert: true, want: false},
		{name: "convert disabled is copy-only", convert: false, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			migobj := Migrate{Convert: tt.convert}
			assert.Equal(t, tt.want, migobj.isCopyOnly())
		})
	}
}

// TestWindowsBootVolumeMetadata asserts that the copy-only boot-volume metadata never advertises a
// virtio disk bus. An unconverted Windows guest has no viostor driver loaded, so hw_disk_bus=virtio
// (or hw_scsi_model=virtio-scsi) would make the instance stop with INACCESSIBLE_BOOT_DEVICE (0x7B).
func TestWindowsBootVolumeMetadata(t *testing.T) {
	tests := []struct {
		name   string
		osType string
		want   map[string]string
	}{
		{
			name:   "windows keeps os_type only",
			osType: "windows",
			want:   map[string]string{"os_type": "windows"},
		},
		{
			name:   "windows os type is matched case-insensitively",
			osType: "Windows",
			want:   map[string]string{"os_type": "windows"},
		},
		{
			name:   "linux gets no hardcoded metadata",
			osType: "linux",
			want:   nil,
		},
		{
			name:   "unknown os type gets no hardcoded metadata",
			osType: "",
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := windowsBootVolumeMetadata(tt.osType)
			assert.Equal(t, tt.want, got)
			assert.NotContains(t, got, "hw_disk_bus", "copy-only must not pin a disk bus")
			assert.NotContains(t, got, "hw_scsi_model", "copy-only must not pin a SCSI model")
		})
	}
}

// TestCreateVolumes_WindowsVirtioMetadataGatedOnCopyOnly is the regression guard for the actual
// boot failure: SetVolumeImageMetadata is what writes hw_disk_bus=virtio, and it must not be called
// for a copy-only migration.
func TestCreateVolumes_WindowsVirtioMetadataGatedOnCopyOnly(t *testing.T) {
	tests := []struct {
		name              string
		convert           bool
		osType            string
		wantMetadataCalls int
	}{
		{
			name:              "windows conversion applies virtio metadata",
			convert:           true,
			osType:            "windows",
			wantMetadataCalls: 1,
		},
		{
			name:              "windows copy-only skips virtio metadata",
			convert:           false,
			osType:            "windows",
			wantMetadataCalls: 0,
		},
		{
			name:              "linux conversion does not apply windows metadata",
			convert:           true,
			osType:            "linux",
			wantMetadataCalls: 0,
		},
		{
			name:              "linux copy-only does not apply windows metadata",
			convert:           false,
			osType:            "linux",
			wantMetadataCalls: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			inputvminfo := vm.VMInfo{
				Name:    "test-vm",
				OSType:  tt.osType,
				UEFI:    false,
				VMDisks: []vm.VMDisk{{Name: "disk1", Size: int64(1024)}},
			}

			mockOpenStackOps := openstack.NewMockOpenstackOperations(ctrl)
			mockOpenStackOps.EXPECT().
				CreateVolume(gomock.Any(), "test-vm-disk1", int64(1024), tt.osType, false, "voltype-1", false).
				Return(&volumes.Volume{ID: "id1", Name: "test-vm-disk1"}, nil).
				Times(1)
			mockOpenStackOps.EXPECT().
				SetVolumeImageMetadata(gomock.Any(), gomock.Any(), false).
				Return(nil).
				Times(tt.wantMetadataCalls)

			migobj := Migrate{
				Openstackclients: mockOpenStackOps,
				InPod:            false,
				Convert:          tt.convert,
				Volumetypes:      []string{"voltype-1"},
			}

			_, err := migobj.CreateVolumes(context.Background(), inputvminfo)
			assert.NoError(t, err)
		})
	}
}

// stubInspection replaces the guestfs-backed inspection seams for the duration of a test so no
// libguestfs appliance is required.
func stubInspection(t *testing.T, bootIndex int, bootErr error, espIndex int, espErr error) {
	t.Helper()
	origBoot, origESP := inspectBootVolume, inspectESPDisk
	inspectBootVolume = func(_ *Migrate, _ vm.VMInfo, _ string) (int, string, error) {
		return bootIndex, "", bootErr
	}
	inspectESPDisk = func(_ []vm.VMDisk) (int, error) {
		return espIndex, espErr
	}
	t.Cleanup(func() {
		inspectBootVolume, inspectESPDisk = origBoot, origESP
	})
}

// TestDetectBootDiskReadOnly covers the fallback contract: inspection is preferred, but an
// inconclusive or failing inspection must fall back to Disk 0 instead of aborting the migration.
func TestDetectBootDiskReadOnly(t *testing.T) {
	tests := []struct {
		name      string
		uefi      bool
		bootIndex int
		bootErr   error
		espIndex  int
		espErr    error
		wantBoot  int
		wantESP   int
	}{
		{
			name:      "inspection result is used when conclusive",
			bootIndex: 1,
			espIndex:  -1,
			wantBoot:  1,
			wantESP:   -1,
		},
		{
			name:      "inconclusive inspection falls back to disk 0",
			bootIndex: -1,
			espIndex:  -1,
			wantBoot:  0,
			wantESP:   -1,
		},
		{
			name:      "failed inspection falls back to disk 0",
			bootIndex: 1,
			bootErr:   assert.AnError,
			espIndex:  -1,
			wantBoot:  0,
			wantESP:   -1,
		},
		{
			name:      "uefi guest reports the detected ESP disk",
			uefi:      true,
			bootIndex: 1,
			espIndex:  0,
			wantBoot:  1,
			wantESP:   0,
		},
		{
			name:      "uefi esp detection failure is tolerated",
			uefi:      true,
			bootIndex: 1,
			espIndex:  -1,
			espErr:    assert.AnError,
			wantBoot:  1,
			wantESP:   -1,
		},
		{
			name:      "esp index out of range is ignored",
			uefi:      true,
			bootIndex: 0,
			espIndex:  99,
			wantBoot:  0,
			wantESP:   -1,
		},
		{
			name:      "bios guest never inspects for an ESP",
			uefi:      false,
			bootIndex: 0,
			espIndex:  0,
			wantBoot:  0,
			wantESP:   -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubInspection(t, tt.bootIndex, tt.bootErr, tt.espIndex, tt.espErr)

			migobj := Migrate{Convert: false}
			vminfo := vm.VMInfo{
				Name:   "test-vm",
				OSType: "linux",
				UEFI:   tt.uefi,
				VMDisks: []vm.VMDisk{
					{Name: "disk1"},
					{Name: "disk2"},
				},
			}

			gotBoot, gotESP := migobj.detectBootDiskReadOnly(vminfo)
			assert.Equal(t, tt.wantBoot, gotBoot, "boot disk index")
			assert.Equal(t, tt.wantESP, gotESP, "ESP disk index")
		})
	}
}

// TestStageVolumesWithoutConversion_UEFIMarksESPBootable verifies that a UEFI guest whose ESP lives
// on a separate disk gets both that disk and the root disk marked bootable, mirroring what the
// conversion path does, so Nova can build a working block device mapping.
func TestStageVolumesWithoutConversion_UEFIMarksESPBootable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	stubInspection(t, 1, nil, 0, nil)

	espVol := &volumes.Volume{ID: "id-esp"}
	rootVol := &volumes.Volume{ID: "id-root"}
	vminfo := vm.VMInfo{
		Name:   "test-vm",
		OSType: "linux",
		UEFI:   true,
		VMDisks: []vm.VMDisk{
			{Name: "disk-esp", OpenstackVol: espVol},
			{Name: "disk-root", OpenstackVol: rootVol},
		},
	}

	mockOpenStackOps := openstack.NewMockOpenstackOperations(ctrl)
	mockOpenStackOps.EXPECT().AttachVolumeToVM(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockOpenStackOps.EXPECT().FindDevice(gomock.Any()).Return("/dev/sda", nil).AnyTimes()
	mockOpenStackOps.EXPECT().DetachVolumeFromVM(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockOpenStackOps.EXPECT().WaitForVolume(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockOpenStackOps.EXPECT().SetVolumeBootable(gomock.Any(), rootVol).Return(nil).Times(1)
	mockOpenStackOps.EXPECT().SetVolumeBootable(gomock.Any(), espVol).Return(nil).Times(1)

	migobj := Migrate{Openstackclients: mockOpenStackOps, Convert: false}

	espDiskIndex, err := migobj.stageVolumesWithoutConversion(context.Background(), vminfo)
	assert.NoError(t, err)
	assert.Equal(t, 0, espDiskIndex)
	assert.True(t, vminfo.VMDisks[1].Boot, "root disk must be flagged as the boot disk")
}

// TestStageVolumesWithoutConversion_BootDiskFallback covers the copy-only staging path end to end
// with mocked OpenStack calls, using an inconclusive inspection result to exercise the documented
// fallback to Disk 0 rather than failing the migration.
func TestStageVolumesWithoutConversion_BootDiskFallback(t *testing.T) {
	tests := []struct {
		name          string
		osType        string
		imageMetadata map[string]string
		wantMetadata  map[string]string
	}{
		{
			name:          "linux with no profiles applies no boot metadata",
			osType:        "linux",
			imageMetadata: nil,
			wantMetadata:  nil,
		},
		{
			name:          "windows with no profiles applies os_type only",
			osType:        "windows",
			imageMetadata: nil,
			wantMetadata:  map[string]string{"os_type": "windows"},
		},
		{
			name:          "linux profile properties are applied",
			osType:        "linux",
			imageMetadata: map[string]string{"hw_disk_bus": "sata"},
			wantMetadata:  map[string]string{"hw_disk_bus": "sata"},
		},
		{
			name:          "profile overrides the hardcoded os_type",
			osType:        "windows",
			imageMetadata: map[string]string{"os_type": "linux", "hw_disk_bus": "sata"},
			wantMetadata:  map[string]string{"os_type": "linux", "hw_disk_bus": "sata"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			stubInspection(t, -1, nil, -1, nil)

			bootVol := &volumes.Volume{ID: "id1", Name: "test-vm-disk1"}
			vminfo := vm.VMInfo{
				Name:   "test-vm",
				OSType: tt.osType,
				UEFI:   false,
				VMDisks: []vm.VMDisk{
					{Name: "disk1", Size: int64(1024), OpenstackVol: bootVol},
				},
			}

			mockOpenStackOps := openstack.NewMockOpenstackOperations(ctrl)
			mockOpenStackOps.EXPECT().AttachVolumeToVM(gomock.Any(), "id1").Return(nil).AnyTimes()
			mockOpenStackOps.EXPECT().FindDevice("id1").Return("/dev/sda", nil).AnyTimes()
			mockOpenStackOps.EXPECT().DetachVolumeFromVM(gomock.Any(), "id1").Return(nil).AnyTimes()
			mockOpenStackOps.EXPECT().WaitForVolume(gomock.Any(), "id1").Return(nil).AnyTimes()

			// The boot volume must be marked bootable, otherwise Nova has no root device.
			mockOpenStackOps.EXPECT().SetVolumeBootable(gomock.Any(), bootVol).Return(nil).Times(1)

			if tt.wantMetadata == nil {
				mockOpenStackOps.EXPECT().
					ApplyBootVolumeImageMetadata(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)
			} else {
				mockOpenStackOps.EXPECT().
					ApplyBootVolumeImageMetadata(gomock.Any(), bootVol, tt.wantMetadata).
					Return(nil).
					Times(1)
			}

			migobj := Migrate{
				Openstackclients: mockOpenStackOps,
				InPod:            false,
				Convert:          false,
				ImageMetadata:    tt.imageMetadata,
				Volumetypes:      []string{"voltype-1"},
			}

			espDiskIndex, err := migobj.stageVolumesWithoutConversion(context.Background(), vminfo)
			assert.NoError(t, err)
			assert.Equal(t, -1, espDiskIndex, "no ESP is expected for a BIOS guest")
			assert.True(t, vminfo.VMDisks[0].Boot, "Disk 0 must be flagged as the boot disk by fallback")
		})
	}
}

// TestStageVolumesWithoutConversion_NoDisks guards against indexing a VM with no disks.
func TestStageVolumesWithoutConversion_NoDisks(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	migobj := Migrate{
		Openstackclients: openstack.NewMockOpenstackOperations(ctrl),
		Convert:          false,
	}

	_, err := migobj.stageVolumesWithoutConversion(context.Background(), vm.VMInfo{Name: "test-vm"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no disks present")
}
