// Copyright © 2026 The vjailbreak authors

package openstack

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/gophercloud/gophercloud/v2/openstack/blockstorage/v3/volumes"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/networks"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/k8sutils"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/timing"
	"github.com/platform9/vjailbreak/v2v-helper/vm"
)

func newTestRecorder() *timing.Recorder {
	return timing.New("test-vm", "HotAdd", "cold", func(string) {})
}

func stepsRecorded(rec *timing.Recorder) []string {
	stats := rec.Snapshot()
	names := make([]string, 0, len(stats))
	for _, s := range stats {
		names = append(names, s.Step)
	}
	return names
}

// TestTimedOpenstackOperationsRecordsAStepPerCall drives every method on the
// decorator and asserts each lands under its own step name. A method added to
// OpenstackOperations without an entry here silently drops a row from the
// Hot-Add comparison rather than failing the build — this test is the guard.
func TestTimedOpenstackOperationsRecordsAStepPerCall(t *testing.T) {
	ctx := context.Background()
	vol := &volumes.Volume{ID: "vol-1"}

	tests := []struct {
		name     string
		expect   func(m *MockOpenstackOperationsMockRecorder)
		call     func(o *TimedOpenstackOperations)
		wantStep string
	}{
		{
			"CreateVolume",
			func(m *MockOpenstackOperationsMockRecorder) {
				m.CreateVolume(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
			},
			func(o *TimedOpenstackOperations) { _, _ = o.CreateVolume(ctx, "d", 10, "linux", false, "lvm", false) },
			StepCreateVolume,
		},
		{
			"WaitForVolume",
			func(m *MockOpenstackOperationsMockRecorder) { m.WaitForVolume(gomock.Any(), gomock.Any()) },
			func(o *TimedOpenstackOperations) { _ = o.WaitForVolume(ctx, "vol-1") },
			StepWaitForVolume,
		},
		{
			"AttachVolumeToVM",
			func(m *MockOpenstackOperationsMockRecorder) { m.AttachVolumeToVM(gomock.Any(), gomock.Any()) },
			func(o *TimedOpenstackOperations) { _ = o.AttachVolumeToVM(ctx, "vol-1") },
			StepAttachVolumeToVM,
		},
		{
			"WaitForVolumeAttachment",
			func(m *MockOpenstackOperationsMockRecorder) { m.WaitForVolumeAttachment(gomock.Any(), gomock.Any()) },
			func(o *TimedOpenstackOperations) { _ = o.WaitForVolumeAttachment(ctx, "vol-1") },
			StepWaitForVolumeAttachment,
		},
		{
			"DetachVolumeFromVM",
			func(m *MockOpenstackOperationsMockRecorder) { m.DetachVolumeFromVM(gomock.Any(), gomock.Any()) },
			func(o *TimedOpenstackOperations) { _ = o.DetachVolumeFromVM(ctx, "vol-1") },
			StepDetachVolumeFromVM,
		},
		{
			"SetVolumeUEFI",
			func(m *MockOpenstackOperationsMockRecorder) { m.SetVolumeUEFI(gomock.Any(), gomock.Any()) },
			func(o *TimedOpenstackOperations) { _ = o.SetVolumeUEFI(ctx, vol) },
			StepSetVolumeUEFI,
		},
		{
			"EnableQGA",
			func(m *MockOpenstackOperationsMockRecorder) { m.EnableQGA(gomock.Any(), gomock.Any()) },
			func(o *TimedOpenstackOperations) { _ = o.EnableQGA(ctx, vol) },
			StepEnableQGA,
		},
		{
			"SetVolumeImageMetadata",
			func(m *MockOpenstackOperationsMockRecorder) {
				m.SetVolumeImageMetadata(gomock.Any(), gomock.Any(), gomock.Any())
			},
			func(o *TimedOpenstackOperations) { _ = o.SetVolumeImageMetadata(ctx, vol, false) },
			StepSetVolumeImageMetadata,
		},
		{
			"ApplyBootVolumeImageMetadata",
			func(m *MockOpenstackOperationsMockRecorder) {
				m.ApplyBootVolumeImageMetadata(gomock.Any(), gomock.Any(), gomock.Any())
			},
			func(o *TimedOpenstackOperations) { _ = o.ApplyBootVolumeImageMetadata(ctx, vol, nil) },
			StepApplyBootVolumeMetadata,
		},
		{
			"SetVolumeBootable",
			func(m *MockOpenstackOperationsMockRecorder) { m.SetVolumeBootable(gomock.Any(), gomock.Any()) },
			func(o *TimedOpenstackOperations) { _ = o.SetVolumeBootable(ctx, vol) },
			StepSetVolumeBootable,
		},
		{
			"GetClosestFlavour",
			func(m *MockOpenstackOperationsMockRecorder) {
				m.GetClosestFlavour(gomock.Any(), gomock.Any(), gomock.Any())
			},
			func(o *TimedOpenstackOperations) { _, _ = o.GetClosestFlavour(ctx, 4, 8192) },
			StepGetClosestFlavour,
		},
		{
			"GetFlavor",
			func(m *MockOpenstackOperationsMockRecorder) { m.GetFlavor(gomock.Any(), gomock.Any()) },
			func(o *TimedOpenstackOperations) { _, _ = o.GetFlavor(ctx, "f-1") },
			StepGetFlavor,
		},
		{
			"GetNetwork",
			func(m *MockOpenstackOperationsMockRecorder) { m.GetNetwork(gomock.Any(), gomock.Any()) },
			func(o *TimedOpenstackOperations) { _, _ = o.GetNetwork(ctx, "net") },
			StepGetNetwork,
		},
		{
			"GetPort",
			func(m *MockOpenstackOperationsMockRecorder) { m.GetPort(gomock.Any(), gomock.Any()) },
			func(o *TimedOpenstackOperations) { _, _ = o.GetPort(ctx, "port-1") },
			StepGetPort,
		},
		{
			"ValidateAndCreatePort",
			func(m *MockOpenstackOperationsMockRecorder) {
				m.ValidateAndCreatePort(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
			},
			func(o *TimedOpenstackOperations) {
				_, _ = o.ValidateAndCreatePort(ctx, &networks.Network{}, "mac", nil, "vm", nil, false, nil, nil)
			},
			StepValidateAndCreatePort,
		},
		{
			"DeletePort",
			func(m *MockOpenstackOperationsMockRecorder) { m.DeletePort(gomock.Any(), gomock.Any()) },
			func(o *TimedOpenstackOperations) { _ = o.DeletePort(ctx, "port-1") },
			StepDeletePort,
		},
		{
			"GetSubnet",
			func(m *MockOpenstackOperationsMockRecorder) { m.GetSubnet(gomock.Any(), gomock.Any(), gomock.Any()) },
			func(o *TimedOpenstackOperations) { _, _ = o.GetSubnet(ctx, []string{"net"}, "10.0.0.5") },
			StepGetSubnet,
		},
		{
			"CreatePort",
			func(m *MockOpenstackOperationsMockRecorder) {
				m.CreatePort(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
					gomock.Any(), gomock.Any(), gomock.Any())
			},
			func(o *TimedOpenstackOperations) {
				_, _ = o.CreatePort(ctx, &networks.Network{}, "mac", nil, "vm", nil, false, nil)
			},
			StepCreatePort,
		},
		{
			"CreateVM",
			func(m *MockOpenstackOperationsMockRecorder) {
				m.CreateVM(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
			},
			func(o *TimedOpenstackOperations) {
				_, _ = o.CreateVM(ctx, &flavors.Flavor{}, nil, nil, vm.VMInfo{}, "az", nil, "",
					k8sutils.VjailbreakSettings{}, 0)
			},
			StepCreateVM,
		},
		{
			"GetServerGroups",
			func(m *MockOpenstackOperationsMockRecorder) { m.GetServerGroups(gomock.Any(), gomock.Any()) },
			func(o *TimedOpenstackOperations) { _, _ = o.GetServerGroups(ctx, "proj") },
			StepGetServerGroups,
		},
		{
			"GetSecurityGroupIDs",
			func(m *MockOpenstackOperationsMockRecorder) {
				m.GetSecurityGroupIDs(gomock.Any(), gomock.Any(), gomock.Any())
			},
			func(o *TimedOpenstackOperations) { _, _ = o.GetSecurityGroupIDs(ctx, nil, "proj") },
			StepGetSecurityGroupIDs,
		},
		{
			"DeleteVolume",
			func(m *MockOpenstackOperationsMockRecorder) { m.DeleteVolume(gomock.Any(), gomock.Any()) },
			func(o *TimedOpenstackOperations) { _ = o.DeleteVolume(ctx, "vol-1") },
			StepDeleteVolume,
		},
		{
			"FindDevice",
			func(m *MockOpenstackOperationsMockRecorder) { m.FindDevice(gomock.Any()) },
			func(o *TimedOpenstackOperations) { _, _ = o.FindDevice("vol-1") },
			StepFindDevice,
		},
		{
			"ManageExistingVolume",
			func(m *MockOpenstackOperationsMockRecorder) {
				m.ManageExistingVolume(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
			},
			func(o *TimedOpenstackOperations) { _, _ = o.ManageExistingVolume("n", nil, "h", "vt") },
			StepManageExistingVolume,
		},
		{
			"WaitUntilVMActive",
			func(m *MockOpenstackOperationsMockRecorder) { m.WaitUntilVMActive(gomock.Any(), gomock.Any()) },
			func(o *TimedOpenstackOperations) { _, _ = o.WaitUntilVMActive(ctx, "srv-1") },
			StepWaitUntilVMActive,
		},
		{
			"GetIsSimpleNetwork",
			func(m *MockOpenstackOperationsMockRecorder) { m.GetIsSimpleNetwork(gomock.Any(), gomock.Any()) },
			func(o *TimedOpenstackOperations) { _, _ = o.GetIsSimpleNetwork(ctx, "net-1") },
			StepGetIsSimpleNetwork,
		},
		{
			"GetCinderVolumeServices",
			func(m *MockOpenstackOperationsMockRecorder) { m.GetCinderVolumeServices(gomock.Any()) },
			func(o *TimedOpenstackOperations) { _, _ = o.GetCinderVolumeServices(ctx) },
			StepGetCinderVolumeServices,
		},
		{
			"GetVolume",
			func(m *MockOpenstackOperationsMockRecorder) { m.GetVolume(gomock.Any(), gomock.Any()) },
			func(o *TimedOpenstackOperations) { _, _ = o.GetVolume(ctx, "vol-1") },
			StepGetVolume,
		},
		{
			"DeleteServer",
			func(m *MockOpenstackOperationsMockRecorder) { m.DeleteServer(gomock.Any(), gomock.Any()) },
			func(o *TimedOpenstackOperations) { _ = o.DeleteServer(ctx, "srv-1") },
			StepDeleteServer,
		},
		{
			"StopServer",
			func(m *MockOpenstackOperationsMockRecorder) { m.StopServer(gomock.Any(), gomock.Any()) },
			func(o *TimedOpenstackOperations) { _ = o.StopServer(ctx, "srv-1") },
			StepStopServer,
		},
		{
			"DetachVolumeFromServer",
			func(m *MockOpenstackOperationsMockRecorder) {
				m.DetachVolumeFromServer(gomock.Any(), gomock.Any(), gomock.Any())
			},
			func(o *TimedOpenstackOperations) { _ = o.DetachVolumeFromServer(ctx, "srv-1", "vol-1") },
			StepDetachVolumeFromServer,
		},
		{
			"WaitForVolumeDetached",
			func(m *MockOpenstackOperationsMockRecorder) {
				m.WaitForVolumeDetached(gomock.Any(), gomock.Any(), gomock.Any())
			},
			func(o *TimedOpenstackOperations) { _ = o.WaitForVolumeDetached(ctx, "vol-1", time.Second) },
			StepWaitForVolumeDetached,
		},
		{
			"GetServerStatus",
			func(m *MockOpenstackOperationsMockRecorder) { m.GetServerStatus(gomock.Any(), gomock.Any()) },
			func(o *TimedOpenstackOperations) { _, _ = o.GetServerStatus(ctx, "srv-1") },
			StepGetServerStatus,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mock := NewMockOpenstackOperations(ctrl)
			tt.expect(mock.EXPECT())

			rec := newTestRecorder()
			o := NewTimedOpenstackOperations(mock, rec)
			tt.call(o)

			got := stepsRecorded(rec)
			if len(got) != 1 || got[0] != tt.wantStep {
				t.Fatalf("recorded steps = %v, want exactly [%q]", got, tt.wantStep)
			}
		})
	}
}

// TestTimedOpenstackOperationsForwardsArgsAndResults proves the decorator is a
// pure pass-through — a dropped argument would change migration behaviour, not
// just the metrics.
func TestTimedOpenstackOperationsForwardsArgsAndResults(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	wantVol := &volumes.Volume{ID: "vol-abc", Size: 200}
	wantErr := errors.New("cinder quota exceeded")

	mock := NewMockOpenstackOperations(ctrl)
	mock.EXPECT().
		CreateVolume(ctx, "disk-0", int64(200), "windows", true, "ceph", false).
		Return(wantVol, nil)
	mock.EXPECT().AttachVolumeToVM(ctx, "vol-abc").Return(wantErr)

	o := NewTimedOpenstackOperations(mock, newTestRecorder())

	gotVol, err := o.CreateVolume(ctx, "disk-0", 200, "windows", true, "ceph", false)
	if err != nil {
		t.Fatalf("CreateVolume returned unexpected error: %v", err)
	}
	if gotVol != wantVol {
		t.Errorf("CreateVolume returned %+v, want %+v", gotVol, wantVol)
	}

	if got := o.AttachVolumeToVM(ctx, "vol-abc"); !errors.Is(got, wantErr) {
		t.Errorf("AttachVolumeToVM error = %v, want %v", got, wantErr)
	}
}

// TestPerDiskStepsAccumulate: CreateVolume runs once per disk, so the report
// needs both the call count and the summed duration, not one flat number.
func TestPerDiskStepsAccumulate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	mock := NewMockOpenstackOperations(ctrl)
	mock.EXPECT().
		CreateVolume(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&volumes.Volume{}, nil).
		Times(3)

	rec := newTestRecorder()
	o := NewTimedOpenstackOperations(mock, rec)
	for i := 0; i < 3; i++ {
		_, _ = o.CreateVolume(ctx, "d", 200, "linux", false, "ceph", false)
	}

	stats := rec.Snapshot()
	if len(stats) != 1 {
		t.Fatalf("expected 1 step, got %d", len(stats))
	}
	if stats[0].Count != 3 {
		t.Errorf("Count = %d, want 3", stats[0].Count)
	}
}

// TestOpenstackNilRecorderStillForwards: timing must never be load-bearing.
func TestOpenstackNilRecorderStillForwards(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := NewMockOpenstackOperations(ctrl)
	mock.EXPECT().DeleteVolume(gomock.Any(), "vol-1").Return(nil)

	o := NewTimedOpenstackOperations(mock, nil)
	if err := o.DeleteVolume(context.Background(), "vol-1"); err != nil {
		t.Fatalf("DeleteVolume with nil recorder returned %v", err)
	}
}
