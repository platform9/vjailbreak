// Copyright © 2025 The vjailbreak authors

package migrate

import (
	"context"
	"fmt"

	"github.com/platform9/vjailbreak/v2v-helper/vcenter"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// hotAddGetVCenterClient extracts the VCenterClient from VMops using the same
// pattern as vaai_copy.go so we do not need to change the VMOperations interface.
func (migobj *Migrate) hotAddGetVCenterClient() *vcenter.VCenterClient {
	type vcenterClientGetter interface {
		GetVCenterClient() *vcenter.VCenterClient
	}
	if g, ok := migobj.VMops.(vcenterClientGetter); ok {
		return g.GetVCenterClient()
	}
	return nil
}

// hotAddCreateLinkedClone creates a powered-off linked clone of sourceVM at the
// given snapshot. The clone is placed in the same folder and datastore as the
// source VM — it shares the parent disk, so creation is instant and uses no
// extra storage. Returns the govmomi object for the new clone VM.
func hotAddCreateLinkedClone(
	ctx context.Context,
	vcClient *vcenter.VCenterClient,
	sourceVM *object.VirtualMachine,
	snapshotRef types.ManagedObjectReference,
	cloneName string,
) (*object.VirtualMachine, error) {
	// Resolve the source VM's parent folder so the clone lands in the same place.
	var vmMo mo.VirtualMachine
	if err := sourceVM.Properties(ctx, sourceVM.Reference(), []string{"parent"}, &vmMo); err != nil {
		return nil, fmt.Errorf("failed to get source VM parent folder: %w", err)
	}
	if vmMo.Parent == nil {
		return nil, fmt.Errorf("source VM has no parent folder")
	}
	folder := object.NewFolder(vcClient.VCClient, *vmMo.Parent)

	cloneSpec := types.VirtualMachineCloneSpec{
		// MoveAllDiskBackingsAndAllowSharing keeps the clone on the same datastore
		// and marks the disks as shared with the parent — this is the linked clone.
		Location: types.VirtualMachineRelocateSpec{
			DiskMoveType: string(types.VirtualMachineRelocateDiskMoveOptionsMoveAllDiskBackingsAndAllowSharing),
		},
		Snapshot: &snapshotRef,
		PowerOn:  false,
		Template: false,
	}

	task, err := sourceVM.Clone(ctx, folder, cloneName, cloneSpec)
	if err != nil {
		return nil, fmt.Errorf("CloneVM_Task failed: %w", err)
	}

	info, err := task.WaitForResult(ctx)
	if err != nil {
		return nil, fmt.Errorf("linked clone task failed: %w", err)
	}

	ref, ok := info.Result.(types.ManagedObjectReference)
	if !ok {
		return nil, fmt.Errorf("unexpected result type from CloneVM_Task: %T", info.Result)
	}

	clone := object.NewVirtualMachine(vcClient.VCClient, ref)
	return clone, nil
}
