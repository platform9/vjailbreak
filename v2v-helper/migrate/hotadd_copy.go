// Copyright © 2025 The vjailbreak authors

package migrate

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/platform9/vjailbreak/pkg/common/constants"
	esxissh "github.com/platform9/vjailbreak/v2v-helper/esxi-ssh"
	"github.com/platform9/vjailbreak/v2v-helper/vcenter"
	"github.com/platform9/vjailbreak/v2v-helper/vm"
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

// hotAddDestroyVM deletes a VM and all of its disk files. Used to clean up the
// linked clone after the migration copy is complete (or on failure).
func hotAddDestroyVM(ctx context.Context, vm *object.VirtualMachine) error {
	task, err := vm.Destroy(ctx)
	if err != nil {
		return fmt.Errorf("Destroy_Task failed: %w", err)
	}
	if err := task.Wait(ctx); err != nil {
		return fmt.Errorf("destroy task failed: %w", err)
	}
	return nil
}

// hotAddGetVMDisks returns all VirtualDisk devices attached to a VM.
func hotAddGetVMDisks(ctx context.Context, vm *object.VirtualMachine) ([]*types.VirtualDisk, error) {
	var vmMo mo.VirtualMachine
	if err := vm.Properties(ctx, vm.Reference(), []string{"config.hardware.device"}, &vmMo); err != nil {
		return nil, fmt.Errorf("failed to get VM device list: %w", err)
	}

	var disks []*types.VirtualDisk
	for _, device := range vmMo.Config.Hardware.Device {
		if d, ok := device.(*types.VirtualDisk); ok {
			disks = append(disks, d)
		}
	}
	return disks, nil
}

// hotAddGetVMIP returns the first non-link-local IPv4 address reported by
// VMware Tools for the given VM.
func hotAddGetVMIP(ctx context.Context, targetVM *object.VirtualMachine) (string, error) {
	var vmMo mo.VirtualMachine
	if err := targetVM.Properties(ctx, targetVM.Reference(), []string{"guest.net"}, &vmMo); err != nil {
		return "", fmt.Errorf("failed to get VM guest info: %w", err)
	}

	for _, nic := range vmMo.Guest.Net {
		for _, ip := range nic.IpAddress {
			if strings.Contains(ip, ":") || strings.HasPrefix(ip, "169.254.") {
				continue
			}
			return ip, nil
		}
	}

	return "", fmt.Errorf("no IPv4 address found for VM (is VMware Tools running?)")
}

// ValidateHotAddPrerequisites checks that all requirements for HotAdd copy are
// met before the migration starts, failing fast with a clear error message.
func (migobj *Migrate) ValidateHotAddPrerequisites(ctx context.Context) error {
	migobj.logMessage("[HotAdd] Validating prerequisites")

	if migobj.ProxyVMName == "" {
		return fmt.Errorf("PROXY_VM_NAME is required for HotAdd copy method")
	}

	if len(migobj.ESXiSSHPrivateKey) == 0 {
		if err := migobj.LoadESXiSSHKey(ctx); err != nil {
			return errors.Wrap(err, "failed to load SSH private key")
		}
	}

	vcClient := migobj.hotAddGetVCenterClient()
	if vcClient == nil {
		return fmt.Errorf("cannot access vCenter client from VMops")
	}

	proxyVM, err := vcClient.GetVMByName(ctx, migobj.ProxyVMName)
	if err != nil {
		return errors.Wrapf(err, "proxy VM %q not found in vCenter", migobj.ProxyVMName)
	}

	state, err := proxyVM.PowerState(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get proxy VM power state")
	}
	if state != types.VirtualMachinePowerStatePoweredOn {
		return fmt.Errorf("proxy VM %q must be powered on (current state: %s)", migobj.ProxyVMName, state)
	}

	proxyIP, err := hotAddGetVMIP(ctx, proxyVM)
	if err != nil {
		return errors.Wrapf(err, "cannot determine IP of proxy VM %q (is VMware Tools running?)", migobj.ProxyVMName)
	}

	sshClient := esxissh.NewClient()
	defer sshClient.Disconnect()
	if err := sshClient.Connect(ctx, proxyIP, "root", migobj.ESXiSSHPrivateKey); err != nil {
		return errors.Wrapf(err, "cannot SSH to proxy VM %q at %s", migobj.ProxyVMName, proxyIP)
	}
	if err := sshClient.TestConnectionGeneric(); err != nil {
		return errors.Wrapf(err, "SSH test failed for proxy VM %q", migobj.ProxyVMName)
	}

	migobj.logMessage(fmt.Sprintf("[HotAdd] Proxy VM %q validated (IP: %s)", migobj.ProxyVMName, proxyIP))
	return nil
}

// hotAddAddDiskToVM hot-adds a disk (identified by its VMDK backing path) to a
// running VM. It finds an existing SCSI controller and a free unit number, then
// issues a ReconfigVM_Task. Returns the device key assigned by vCenter so the
// disk can be removed later with hotAddRemoveDiskFromVM.
func hotAddAddDiskToVM(
	ctx context.Context,
	targetVM *object.VirtualMachine,
	backingPath string,
) (int32, error) {
	var vmMo mo.VirtualMachine
	if err := targetVM.Properties(ctx, targetVM.Reference(), []string{"config.hardware.device"}, &vmMo); err != nil {
		return 0, fmt.Errorf("failed to get proxy VM devices: %w", err)
	}

	// Find the first SCSI controller on the proxy VM.
	var controllerKey int32
	usedUnits := map[int32]bool{7: true} // unit 7 is reserved for the controller itself

	for _, dev := range vmMo.Config.Hardware.Device {
		switch sc := dev.(type) {
		case *types.VirtualLsiLogicController,
			*types.VirtualLsiLogicSASController,
			*types.ParaVirtualSCSIController,
			*types.VirtualBusLogicController:
			vc := sc.(types.BaseVirtualDevice).GetVirtualDevice()
			if controllerKey == 0 {
				controllerKey = vc.Key
			}
		}
		// Track used unit numbers on the chosen controller.
		vd := dev.GetVirtualDevice()
		if controllerKey != 0 && vd.ControllerKey == controllerKey {
			if vd.UnitNumber != nil {
				usedUnits[*vd.UnitNumber] = true
			}
		}
	}

	if controllerKey == 0 {
		return 0, fmt.Errorf("no SCSI controller found on proxy VM %s", targetVM.Reference().Value)
	}

	// Find the lowest free unit number (0-15, skip 7).
	var unitNumber int32
	for usedUnits[unitNumber] {
		unitNumber++
		if unitNumber > 15 {
			return 0, fmt.Errorf("no free SCSI slots on proxy VM (controller key=%d)", controllerKey)
		}
	}

	disk := &types.VirtualDisk{
		VirtualDevice: types.VirtualDevice{
			Backing: &types.VirtualDiskFlatVer2BackingInfo{
				VirtualDeviceFileBackingInfo: types.VirtualDeviceFileBackingInfo{
					FileName: backingPath,
				},
				DiskMode: string(types.VirtualDiskModePersistent),
			},
			ControllerKey: controllerKey,
			UnitNumber:    &unitNumber,
		},
	}

	spec := types.VirtualMachineConfigSpec{
		DeviceChange: []types.BaseVirtualDeviceConfigSpec{
			&types.VirtualDeviceConfigSpec{
				Operation: types.VirtualDeviceConfigSpecOperationAdd,
				Device:    disk,
			},
		},
	}

	task, err := targetVM.Reconfigure(ctx, spec)
	if err != nil {
		return 0, fmt.Errorf("ReconfigVM hot-add failed: %w", err)
	}
	if err := task.Wait(ctx); err != nil {
		return 0, fmt.Errorf("hot-add task failed: %w", err)
	}

	// Re-read devices to find the key vCenter assigned to the new disk.
	var after mo.VirtualMachine
	if err := targetVM.Properties(ctx, targetVM.Reference(), []string{"config.hardware.device"}, &after); err != nil {
		return 0, fmt.Errorf("failed to read devices after hot-add: %w", err)
	}
	for _, dev := range after.Config.Hardware.Device {
		if d, ok := dev.(*types.VirtualDisk); ok {
			if b, ok := d.Backing.(*types.VirtualDiskFlatVer2BackingInfo); ok {
				if b.FileName == backingPath {
					return d.Key, nil
				}
			}
		}
	}

	return 0, fmt.Errorf("disk was hot-added but could not find its device key (backing: %s)", backingPath)
}

// hotAddRemoveDiskFromVM hot-removes a disk from a VM by its device key.
// The disk file is NOT deleted — only the device attachment is removed.
func hotAddRemoveDiskFromVM(
	ctx context.Context,
	targetVM *object.VirtualMachine,
	deviceKey int32,
) error {
	var vmMo mo.VirtualMachine
	if err := targetVM.Properties(ctx, targetVM.Reference(), []string{"config.hardware.device"}, &vmMo); err != nil {
		return fmt.Errorf("failed to get proxy VM devices: %w", err)
	}

	var diskToRemove types.BaseVirtualDevice
	for _, dev := range vmMo.Config.Hardware.Device {
		if dev.GetVirtualDevice().Key == deviceKey {
			diskToRemove = dev
			break
		}
	}
	if diskToRemove == nil {
		return fmt.Errorf("device key %d not found on VM %s", deviceKey, targetVM.Reference().Value)
	}

	spec := types.VirtualMachineConfigSpec{
		DeviceChange: []types.BaseVirtualDeviceConfigSpec{
			&types.VirtualDeviceConfigSpec{
				Operation: types.VirtualDeviceConfigSpecOperationRemove,
				Device:    diskToRemove,
			},
		},
	}

	task, err := targetVM.Reconfigure(ctx, spec)
	if err != nil {
		return fmt.Errorf("ReconfigVM hot-remove failed: %w", err)
	}
	if err := task.Wait(ctx); err != nil {
		return fmt.Errorf("hot-remove task failed: %w", err)
	}
	return nil
}

// HotAddCopyDisks orchestrates the full HotAdd disk copy for all VM disks.
// It is called from StartMigration after CreateVolumes has already created
// and set vminfo.VMDisks[idx].OpenstackVol for each disk.
func (migobj *Migrate) HotAddCopyDisks(ctx context.Context, vminfo *vm.VMInfo) error {
	migobj.logMessage("[HotAdd] *** BETA: HotAdd SCSI transport ***")
	migobj.logMessage("[HotAdd] Limitations: SCSI disks only, cold migration only, proxy VM must share datastore with source VM")

	vcClient := migobj.hotAddGetVCenterClient()
	if vcClient == nil {
		return fmt.Errorf("[HotAdd] cannot access vCenter client from VMops")
	}

	// Step 1: Power off source VM (cold migration only for Beta).
	migobj.logMessage("[HotAdd] Powering off source VM")
	if err := migobj.VMops.VMPowerOff(); err != nil {
		return errors.Wrap(err, "failed to power off source VM")
	}
	migobj.logMessage("[HotAdd] Source VM powered off")

	// Step 2: Clean up any leftover snapshots, then take a fresh one.
	migobj.logMessage("[HotAdd] Cleaning up existing snapshots")
	if err := migobj.VMops.CleanUpSnapshots(false); err != nil {
		return errors.Wrap(err, "failed to clean up snapshots")
	}

	migobj.logMessage(fmt.Sprintf("[HotAdd] Taking snapshot %q", constants.MigrationSnapshotName))
	if err := migobj.VMops.TakeSnapshot(constants.MigrationSnapshotName); err != nil {
		return errors.Wrap(err, "failed to take snapshot")
	}
	if err := migobj.VMops.UpdateDisksInfo(vminfo); err != nil {
		return errors.Wrap(err, "failed to update disk info after snapshot")
	}
	migobj.logMessage(fmt.Sprintf("[HotAdd] Snapshot taken, %d disk(s) found", len(vminfo.VMDisks)))
	for i, d := range vminfo.VMDisks {
		migobj.logMessage(fmt.Sprintf("[HotAdd]   disk[%d] %s: backing=%s", i, d.Name, d.SnapBackingDisk))
	}

	// Step 3: Get snapshot reference for linked clone creation.
	snapRef, err := migobj.VMops.GetSnapshot(constants.MigrationSnapshotName)
	if err != nil {
		fmt.Sprintf("vjailbreak-hotadd-%s-%d", vminfo.Name, time.Now().Unix())
		return errors.Wrap(err, "failed to get snapshot reference")
	}

	// Step 4: Locate proxy VM and get its IP.
	proxyVM, err := vcClient.GetVMByName(ctx, migobj.ProxyVMName)
	if err != nil {
		return errors.Wrapf(err, "failed to find proxy VM %q", migobj.ProxyVMName)
	}
	proxyIP, err := hotAddGetVMIP(ctx, proxyVM)
	if err != nil {
		return errors.Wrapf(err, "failed to get IP of proxy VM %q", migobj.ProxyVMName)
	}
	migobj.logMessage(fmt.Sprintf("[HotAdd] Proxy VM %q found at %s", migobj.ProxyVMName, proxyIP))

	// Step 5: Create linked clone — instant, shares parent disk, never powered on.
	cloneName := fmt.Sprintf("vjailbreak-hotadd-%s-%d", vminfo.Name, time.Now().Unix())
	migobj.logMessage(fmt.Sprintf("[HotAdd] Creating linked clone %q", cloneName))
	linkedClone, err := hotAddCreateLinkedClone(ctx, vcClient, migobj.VMops.GetVMObj(), *snapRef, cloneName)
	if err != nil {
		return errors.Wrap(err, "failed to create linked clone")
	}
	migobj.logMessage(fmt.Sprintf("[HotAdd] Linked clone %q created", cloneName))

	defer func() {
		migobj.logMessage(fmt.Sprintf("[HotAdd] Destroying linked clone %q", cloneName))
		if err := hotAddDestroyVM(ctx, linkedClone); err != nil {
			migobj.logMessage(fmt.Sprintf("[HotAdd] WARNING: failed to destroy linked clone: %v", err))
		}
		migobj.logMessage("[HotAdd] Cleaning up snapshot")
		if err := migobj.VMops.CleanUpSnapshots(true); err != nil {
			migobj.logMessage(fmt.Sprintf("[HotAdd] WARNING: failed to clean up snapshot: %v", err))
		}
	}()
	migobj.logMessage("[HotAdd] All disks copied successfully")
	return nil
}
