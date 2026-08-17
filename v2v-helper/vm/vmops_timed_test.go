// Copyright © 2026 The vjailbreak authors

package vm

import (
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/timing"
	"github.com/platform9/vjailbreak/v2v-helper/vcenter"
	"github.com/vmware/govmomi/vim25/types"
)

func newTestRecorder() *timing.Recorder {
	return timing.New("test-vm", "HotAdd", "cold", func(string) {})
}

// stepsRecorded returns the step names in the order they were first timed.
func stepsRecorded(rec *timing.Recorder) []string {
	stats := rec.Snapshot()
	names := make([]string, 0, len(stats))
	for _, s := range stats {
		names = append(names, s.Step)
	}
	return names
}

// TestTimedVMOperationsRecordsAStepPerCall drives every method on the decorator
// and asserts each one lands under its own step name. A method added to
// VMOperations without a corresponding entry here shows up as a missing row in
// the Hot-Add comparison, not as a build failure — so this test is the guard.
func TestTimedVMOperationsRecordsAStepPerCall(t *testing.T) {
	tests := []struct {
		name     string
		expect   func(m *MockVMOperationsMockRecorder)
		call     func(t *TimedVMOperations)
		wantStep string
	}{
		{
			name:     "GetVMInfo",
			expect:   func(m *MockVMOperationsMockRecorder) { m.GetVMInfo(gomock.Any(), gomock.Any()) },
			call:     func(tv *TimedVMOperations) { _, _ = tv.GetVMInfo("linux", nil) },
			wantStep: StepGetVMInfo,
		},
		{
			name:     "UpdateDiskInfo",
			expect:   func(m *MockVMOperationsMockRecorder) { m.UpdateDiskInfo(gomock.Any(), gomock.Any(), gomock.Any()) },
			call:     func(tv *TimedVMOperations) { _ = tv.UpdateDiskInfo(&VMInfo{}, VMDisk{}, true) },
			wantStep: StepUpdateDiskInfo,
		},
		{
			name:     "UpdateDisksInfo",
			expect:   func(m *MockVMOperationsMockRecorder) { m.UpdateDisksInfo(gomock.Any(), gomock.Any()) },
			call:     func(tv *TimedVMOperations) { _ = tv.UpdateDisksInfo(&VMInfo{}, true) },
			wantStep: StepUpdateDisksInfo,
		},
		{
			name:     "IsCBTEnabled",
			expect:   func(m *MockVMOperationsMockRecorder) { m.IsCBTEnabled() },
			call:     func(tv *TimedVMOperations) { _, _ = tv.IsCBTEnabled() },
			wantStep: StepIsCBTEnabled,
		},
		{
			name:     "GetHardwareVersion",
			expect:   func(m *MockVMOperationsMockRecorder) { m.GetHardwareVersion() },
			call:     func(tv *TimedVMOperations) { _, _ = tv.GetHardwareVersion() },
			wantStep: StepGetHardwareVersion,
		},
		{
			name:     "EnableCBT",
			expect:   func(m *MockVMOperationsMockRecorder) { m.EnableCBT() },
			call:     func(tv *TimedVMOperations) { _ = tv.EnableCBT() },
			wantStep: StepEnableCBT,
		},
		{
			name:     "TakeSnapshot",
			expect:   func(m *MockVMOperationsMockRecorder) { m.TakeSnapshot(gomock.Any()) },
			call:     func(tv *TimedVMOperations) { _ = tv.TakeSnapshot("snap") },
			wantStep: StepTakeSnapshot,
		},
		{
			name:     "DeleteSnapshot",
			expect:   func(m *MockVMOperationsMockRecorder) { m.DeleteSnapshot(gomock.Any()) },
			call:     func(tv *TimedVMOperations) { _ = tv.DeleteSnapshot("snap") },
			wantStep: StepDeleteSnapshot,
		},
		{
			name:     "DeleteSnapshotByRef",
			expect:   func(m *MockVMOperationsMockRecorder) { m.DeleteSnapshotByRef(gomock.Any()) },
			call:     func(tv *TimedVMOperations) { _ = tv.DeleteSnapshotByRef(&types.ManagedObjectReference{}) },
			wantStep: StepDeleteSnapshotByRef,
		},
		{
			name:     "GetSnapshot",
			expect:   func(m *MockVMOperationsMockRecorder) { m.GetSnapshot(gomock.Any()) },
			call:     func(tv *TimedVMOperations) { _, _ = tv.GetSnapshot("snap") },
			wantStep: StepGetSnapshot,
		},
		{
			name:     "ListSnapshots",
			expect:   func(m *MockVMOperationsMockRecorder) { m.ListSnapshots() },
			call:     func(tv *TimedVMOperations) { _, _ = tv.ListSnapshots() },
			wantStep: StepListSnapshots,
		},
		{
			name:     "CleanUpSnapshots",
			expect:   func(m *MockVMOperationsMockRecorder) { m.CleanUpSnapshots(gomock.Any()) },
			call:     func(tv *TimedVMOperations) { _ = tv.CleanUpSnapshots(true) },
			wantStep: StepCleanUpSnapshots,
		},
		{
			name:     "DeleteMigrationSnapshots",
			expect:   func(m *MockVMOperationsMockRecorder) { m.DeleteMigrationSnapshots(gomock.Any(), gomock.Any()) },
			call:     func(tv *TimedVMOperations) { _ = tv.DeleteMigrationSnapshots(nil, true) },
			wantStep: StepDeleteMigrationSnapshots,
		},
		{
			name: "CustomQueryChangedDiskAreas",
			expect: func(m *MockVMOperationsMockRecorder) {
				m.CustomQueryChangedDiskAreas(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
			},
			call: func(tv *TimedVMOperations) {
				_, _ = tv.CustomQueryChangedDiskAreas("cid", &types.ManagedObjectReference{}, &types.VirtualDisk{}, 0)
			},
			wantStep: StepQueryChangedDiskAreas,
		},
		{
			name:     "VMGuestShutdown",
			expect:   func(m *MockVMOperationsMockRecorder) { m.VMGuestShutdown() },
			call:     func(tv *TimedVMOperations) { _ = tv.VMGuestShutdown() },
			wantStep: StepGuestShutdown,
		},
		{
			name:     "VMPowerOff",
			expect:   func(m *MockVMOperationsMockRecorder) { m.VMPowerOff() },
			call:     func(tv *TimedVMOperations) { _ = tv.VMPowerOff() },
			wantStep: StepPowerOff,
		},
		{
			name:     "VMPowerOn",
			expect:   func(m *MockVMOperationsMockRecorder) { m.VMPowerOn() },
			call:     func(tv *TimedVMOperations) { _ = tv.VMPowerOn() },
			wantStep: StepPowerOn,
		},
		{
			name:     "DisconnectNetworkInterfaces",
			expect:   func(m *MockVMOperationsMockRecorder) { m.DisconnectNetworkInterfaces() },
			call:     func(tv *TimedVMOperations) { _ = tv.DisconnectNetworkInterfaces() },
			wantStep: StepDisconnectNetwork,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mock := NewMockVMOperations(ctrl)
			tt.expect(mock.EXPECT())

			rec := newTestRecorder()
			tv := NewTimedVMOperations(mock, rec)
			tt.call(tv)

			got := stepsRecorded(rec)
			if len(got) != 1 || got[0] != tt.wantStep {
				t.Fatalf("recorded steps = %v, want exactly [%q]", got, tt.wantStep)
			}
		})
	}
}

// TestTimedVMOperationsForwardsArgsAndResults proves the decorator is a pure
// pass-through: a wrong argument or a swallowed return value would silently
// change migration behaviour, not just the metrics.
func TestTimedVMOperationsForwardsArgsAndResults(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	wantInfo := VMInfo{Name: "win2k19", CPU: 4, Memory: 8192}
	wantErr := errors.New("vcenter unreachable")

	mock := NewMockVMOperations(ctrl)
	mock.EXPECT().GetVMInfo("windows", []string{"rdm-1"}).Return(wantInfo, wantErr)
	mock.EXPECT().TakeSnapshot("vjailbreak-hotadd-snap").Return(wantErr)
	mock.EXPECT().IsCBTEnabled().Return(true, nil)

	tv := NewTimedVMOperations(mock, newTestRecorder())

	gotInfo, gotErr := tv.GetVMInfo("windows", []string{"rdm-1"})
	if gotInfo.Name != wantInfo.Name || gotInfo.CPU != wantInfo.CPU || gotInfo.Memory != wantInfo.Memory {
		t.Errorf("GetVMInfo returned %+v, want %+v", gotInfo, wantInfo)
	}
	if !errors.Is(gotErr, wantErr) {
		t.Errorf("GetVMInfo error = %v, want %v", gotErr, wantErr)
	}

	if err := tv.TakeSnapshot("vjailbreak-hotadd-snap"); !errors.Is(err, wantErr) {
		t.Errorf("TakeSnapshot error = %v, want %v", err, wantErr)
	}

	enabled, err := tv.IsCBTEnabled()
	if !enabled || err != nil {
		t.Errorf("IsCBTEnabled = (%v, %v), want (true, nil)", enabled, err)
	}
}

// TestTimedVMOperationsCountsErrors: a step that retried is not comparable
// across two runs, so failures must be visible in the summary.
func TestTimedVMOperationsCountsErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := NewMockVMOperations(ctrl)
	mock.EXPECT().VMPowerOff().Return(errors.New("busy"))
	mock.EXPECT().VMPowerOff().Return(nil)

	rec := newTestRecorder()
	tv := NewTimedVMOperations(mock, rec)
	_ = tv.VMPowerOff()
	_ = tv.VMPowerOff()

	stats := rec.Snapshot()
	if len(stats) != 1 {
		t.Fatalf("expected 1 step, got %d", len(stats))
	}
	if stats[0].Count != 2 || stats[0].Errors != 1 {
		t.Errorf("Count/Errors = %d/%d, want 2/1", stats[0].Count, stats[0].Errors)
	}
}

// vcenterClientFake implements VMOperations plus GetVCenterClient, matching the
// concrete *VMOps that the migrate package type-asserts against.
type vcenterClientFake struct {
	VMOperations
	client *vcenter.VCenterClient
}

func (f *vcenterClientFake) GetVCenterClient() *vcenter.VCenterClient { return f.client }

// TestGetVCenterClientPassesThrough is the regression guard for the Hot-Add and
// storage-accelerated copy paths: both reach the concrete vCenter client by
// type-asserting migobj.VMops. If the decorator does not forward this, both
// fail at runtime with "VMops does not implement GetVCenterClient()".
func TestGetVCenterClientPassesThrough(t *testing.T) {
	want := &vcenter.VCenterClient{}
	tv := NewTimedVMOperations(&vcenterClientFake{client: want}, newTestRecorder())

	if got := tv.GetVCenterClient(); got != want {
		t.Fatalf("GetVCenterClient() = %p, want %p", got, want)
	}
}

// TestGetVCenterClientOnUnsupportedInnerReturnsNil: wrapping an implementation
// that has no vCenter client must return nil rather than panic.
func TestGetVCenterClientOnUnsupportedInnerReturnsNil(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tv := NewTimedVMOperations(NewMockVMOperations(ctrl), newTestRecorder())
	if got := tv.GetVCenterClient(); got != nil {
		t.Fatalf("GetVCenterClient() = %v, want nil", got)
	}
}

// TestGetVMObjIsNotTimed: it reads an already-resolved object, so timing it
// would add a zero-duration row that makes the comparison table noisier.
func TestGetVMObjIsNotTimed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := NewMockVMOperations(ctrl)
	mock.EXPECT().GetVMObj().Return(nil)

	rec := newTestRecorder()
	tv := NewTimedVMOperations(mock, rec)
	tv.GetVMObj()

	if got := stepsRecorded(rec); len(got) != 0 {
		t.Fatalf("GetVMObj recorded steps %v, want none", got)
	}
}

// TestNilRecorderStillForwards: timing must never be load-bearing.
func TestNilRecorderStillForwards(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := NewMockVMOperations(ctrl)
	mock.EXPECT().TakeSnapshot("snap").Return(nil)

	tv := NewTimedVMOperations(mock, nil)
	if err := tv.TakeSnapshot("snap"); err != nil {
		t.Fatalf("TakeSnapshot with nil recorder returned %v", err)
	}
}
