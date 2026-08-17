// Copyright © 2026 The vjailbreak authors

package vm

import (
	"github.com/platform9/vjailbreak/v2v-helper/pkg/timing"
	"github.com/platform9/vjailbreak/v2v-helper/vcenter"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
)

// Step names for the vCenter side of the migration. They are the row labels of
// the Hot-Add vs non-Hot-Add comparison, so they are constants rather than
// inline strings — the report script matches on them exactly.
const (
	StepGetVMInfo                = "vCenter: Get VM Properties"
	StepUpdateDiskInfo           = "vCenter: Update Disk Info"
	StepUpdateDisksInfo          = "vCenter: Update Disk Info (all disks)"
	StepIsCBTEnabled             = "vCenter: Check CBT Enabled"
	StepGetHardwareVersion       = "vCenter: Get Hardware Version"
	StepEnableCBT                = "vCenter: Enable CBT (Reconfigure)"
	StepTakeSnapshot             = "vCenter: Take Snapshot"
	StepDeleteSnapshot           = "vCenter: Delete Snapshot (consolidate)"
	StepDeleteSnapshotByRef      = "vCenter: Delete Snapshot By Ref"
	StepGetSnapshot              = "vCenter: Get Snapshot"
	StepListSnapshots            = "vCenter: List Snapshots"
	StepCleanUpSnapshots         = "vCenter: Clean Up Snapshots"
	StepDeleteMigrationSnapshots = "vCenter: Delete Migration Snapshots"
	StepQueryChangedDiskAreas    = "vCenter: Query Changed Disk Areas (CBT)"
	StepGuestShutdown            = "vCenter: Guest Shutdown (poll until off)"
	StepPowerOff                 = "vCenter: Power Off VM"
	StepPowerOn                  = "vCenter: Power On VM"
	StepDisconnectNetwork        = "vCenter: Disconnect Source Network Interfaces"
)

// TimedVMOperations wraps a VMOperations and records how long each vCenter call
// takes. It is a pure pass-through: arguments, return values and errors are
// forwarded unchanged.
type TimedVMOperations struct {
	inner VMOperations
	rec   *timing.Recorder
}

// vcenterClientProvider mirrors the private interface the migrate package
// type-asserts VMops against to reach the concrete vCenter client. The
// decorator must satisfy it, otherwise wrapping VMops silently breaks both the
// Hot-Add and the storage-accelerated copy paths at runtime.
type vcenterClientProvider interface {
	GetVCenterClient() *vcenter.VCenterClient
}

// Compile-time guarantees: the decorator is substitutable for the thing it wraps.
var (
	_ VMOperations          = (*TimedVMOperations)(nil)
	_ vcenterClientProvider = (*TimedVMOperations)(nil)
)

// NewTimedVMOperations wraps inner. A nil recorder makes every call a plain
// pass-through, so callers do not need to branch on whether timing is enabled.
func NewTimedVMOperations(inner VMOperations, rec *timing.Recorder) *TimedVMOperations {
	return &TimedVMOperations{inner: inner, rec: rec}
}

// GetVCenterClient forwards to the wrapped implementation. Not timed: it is a
// field read, not an API call.
func (t *TimedVMOperations) GetVCenterClient() *vcenter.VCenterClient {
	provider, ok := t.inner.(vcenterClientProvider)
	if !ok {
		return nil
	}
	return provider.GetVCenterClient()
}

// GetVMObj is not timed — it returns an already-resolved object with no round trip.
func (t *TimedVMOperations) GetVMObj() *object.VirtualMachine {
	return t.inner.GetVMObj()
}

func (t *TimedVMOperations) GetVMInfo(ostype string, rdmDisks []string) (VMInfo, error) {
	done := t.rec.Start(StepGetVMInfo)
	info, err := t.inner.GetVMInfo(ostype, rdmDisks)
	done(err)
	return info, err
}

func (t *TimedVMOperations) UpdateDiskInfo(info *VMInfo, disk VMDisk, copySuccess bool) error {
	return t.rec.Track(StepUpdateDiskInfo, func() error {
		return t.inner.UpdateDiskInfo(info, disk, copySuccess)
	})
}

func (t *TimedVMOperations) UpdateDisksInfo(info *VMInfo, copySuccess bool) error {
	return t.rec.Track(StepUpdateDisksInfo, func() error {
		return t.inner.UpdateDisksInfo(info, copySuccess)
	})
}

func (t *TimedVMOperations) IsCBTEnabled() (bool, error) {
	done := t.rec.Start(StepIsCBTEnabled)
	enabled, err := t.inner.IsCBTEnabled()
	done(err)
	return enabled, err
}

func (t *TimedVMOperations) GetHardwareVersion() (int, error) {
	done := t.rec.Start(StepGetHardwareVersion)
	version, err := t.inner.GetHardwareVersion()
	done(err)
	return version, err
}

func (t *TimedVMOperations) EnableCBT() error {
	return t.rec.Track(StepEnableCBT, t.inner.EnableCBT)
}

func (t *TimedVMOperations) TakeSnapshot(name string) error {
	return t.rec.Track(StepTakeSnapshot, func() error { return t.inner.TakeSnapshot(name) })
}

func (t *TimedVMOperations) DeleteSnapshot(name string) error {
	return t.rec.Track(StepDeleteSnapshot, func() error { return t.inner.DeleteSnapshot(name) })
}

func (t *TimedVMOperations) DeleteSnapshotByRef(snap *types.ManagedObjectReference) error {
	return t.rec.Track(StepDeleteSnapshotByRef, func() error { return t.inner.DeleteSnapshotByRef(snap) })
}

func (t *TimedVMOperations) GetSnapshot(name string) (*types.ManagedObjectReference, error) {
	done := t.rec.Start(StepGetSnapshot)
	snap, err := t.inner.GetSnapshot(name)
	done(err)
	return snap, err
}

func (t *TimedVMOperations) ListSnapshots() ([]types.VirtualMachineSnapshotTree, error) {
	done := t.rec.Start(StepListSnapshots)
	snaps, err := t.inner.ListSnapshots()
	done(err)
	return snaps, err
}

func (t *TimedVMOperations) CleanUpSnapshots(ignoreerror bool) error {
	return t.rec.Track(StepCleanUpSnapshots, func() error { return t.inner.CleanUpSnapshots(ignoreerror) })
}

func (t *TimedVMOperations) DeleteMigrationSnapshots(snapshots []types.VirtualMachineSnapshotTree, ignoreerror bool) error {
	return t.rec.Track(StepDeleteMigrationSnapshots, func() error {
		return t.inner.DeleteMigrationSnapshots(snapshots, ignoreerror)
	})
}

func (t *TimedVMOperations) CustomQueryChangedDiskAreas(baseChangeID string, curSnapshot *types.ManagedObjectReference,
	disk *types.VirtualDisk, offset int64) (types.DiskChangeInfo, error) {
	done := t.rec.Start(StepQueryChangedDiskAreas)
	info, err := t.inner.CustomQueryChangedDiskAreas(baseChangeID, curSnapshot, disk, offset)
	done(err)
	return info, err
}

func (t *TimedVMOperations) VMGuestShutdown() error {
	return t.rec.Track(StepGuestShutdown, t.inner.VMGuestShutdown)
}

func (t *TimedVMOperations) VMPowerOff() error {
	return t.rec.Track(StepPowerOff, t.inner.VMPowerOff)
}

func (t *TimedVMOperations) VMPowerOn() error {
	return t.rec.Track(StepPowerOn, t.inner.VMPowerOn)
}

func (t *TimedVMOperations) DisconnectNetworkInterfaces() error {
	return t.rec.Track(StepDisconnectNetwork, t.inner.DisconnectNetworkInterfaces)
}
