// Copyright © 2026 The vjailbreak authors

package migrate

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/timing"
	"github.com/platform9/vjailbreak/v2v-helper/vm"
)

const gib = int64(1024 * 1024 * 1024)

// TestRecordSourceFootprintSumsProvisionedSize is the guard on the number that
// makes a Hot-Add run and a non-Hot-Add run comparable. Without it the report
// cannot tell whether Hot-Add "won" because it is faster or because the other
// run moved fewer bytes.
func TestRecordSourceFootprintSumsProvisionedSize(t *testing.T) {
	tests := []struct {
		name            string
		disks           []vm.VMDisk
		wantDiskCount   int
		wantProvisioned int64
	}{
		{
			name:            "no disks",
			disks:           nil,
			wantDiskCount:   0,
			wantProvisioned: 0,
		},
		{
			name:            "single 200 GiB disk",
			disks:           []vm.VMDisk{{Name: "disk-0", Size: 200 * gib}},
			wantDiskCount:   1,
			wantProvisioned: 200 * gib,
		},
		{
			name: "multi disk sums",
			disks: []vm.VMDisk{
				{Name: "disk-0", Size: 200 * gib},
				{Name: "disk-1", Size: 50 * gib},
				{Name: "disk-2", Size: 10 * gib},
			},
			wantDiskCount:   3,
			wantProvisioned: 260 * gib,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockVMOps := vm.NewMockVMOperations(ctrl)
			// A nil VM object stands in for a source we cannot query (mock
			// migrations, or a vCenter that dropped the session): committed
			// must fall back to 0 rather than panic.
			mockVMOps.EXPECT().GetVMObj().Return(nil)

			rec := timing.New("test-vm", "HotAdd", "cold", func(string) {})
			migobj := &Migrate{VMops: mockVMOps, Timing: rec}

			migobj.recordSourceFootprint(context.Background(), vm.VMInfo{VMDisks: tt.disks})

			got := rec.Summarise(false)
			if got.DiskCount != tt.wantDiskCount {
				t.Errorf("DiskCount = %d, want %d", got.DiskCount, tt.wantDiskCount)
			}
			if got.DiskBytes != tt.wantProvisioned {
				t.Errorf("DiskBytes = %d, want %d", got.DiskBytes, tt.wantProvisioned)
			}
			if got.AllocatedBytes != 0 {
				t.Errorf("AllocatedBytes = %d, want 0 when the VM object is unavailable", got.AllocatedBytes)
			}
		})
	}
}

// TestRecordSourceFootprintWithNilRecorder: timing must never be load-bearing.
func TestRecordSourceFootprintWithNilRecorder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockVMOps := vm.NewMockVMOperations(ctrl)
	mockVMOps.EXPECT().GetVMObj().Return(nil)

	migobj := &Migrate{VMops: mockVMOps} // Timing left nil
	migobj.recordSourceFootprint(context.Background(), vm.VMInfo{
		VMDisks: []vm.VMDisk{{Name: "disk-0", Size: 200 * gib}},
	})
}

// TestMigrateVMEmitsSummaryWithNilRecorder proves the summary hook added around
// migrateVM cannot itself fail a migration.
func TestMigrateVMEmitsSummaryWithNilRecorder(t *testing.T) {
	migobj := &Migrate{}
	migobj.Timing.EmitSummary(false)
	migobj.Timing.EmitSummary(true)
}

// TestPhaseStepNamesAreDistinct: two phases sharing a name would silently
// collapse into one row and double-count its duration.
func TestPhaseStepNamesAreDistinct(t *testing.T) {
	names := []string{
		StepDiskCopyTotal,
		StepConvertTotal,
		StepCreateInstanceTotal,
		StepHotAddFetchSSHKeySecret,
		StepHotAddEnumerateFrozen,
		StepHotAddSSHConnect,
		StepHotAddLocateProxyInVC,
		StepHotAddAttachToProxy,
		StepHotAddIdentifyDevices,
		StepHotAddAllocatePorts,
		StepHotAddServeNBD,
		StepHotAddNBDCopy,
		StepHotAddCleanupTotal,
	}

	seen := map[string]bool{}
	for _, name := range names {
		if name == "" {
			t.Error("empty step name")
		}
		if seen[name] {
			t.Errorf("duplicate step name %q", name)
		}
		seen[name] = true
	}
}
