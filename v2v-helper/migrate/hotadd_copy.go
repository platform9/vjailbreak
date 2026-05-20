// Copyright © 2025 The vjailbreak authors

package migrate

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	esxissh "github.com/platform9/vjailbreak/v2v-helper/esxi-ssh"
	"github.com/platform9/vjailbreak/v2v-helper/vcenter"
	"github.com/platform9/vjailbreak/v2v-helper/vm"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	govmomitypes "github.com/vmware/govmomi/vim25/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	hotAddSSHKeyPath        = "/home/fedora/.ssh/id_rsa"
	hotAddSnapName          = "vjailbreak-hotadd-snap"
	hotAddSSHUser           = "root"
	hotAddIdentifyRetries   = 3
	hotAddIdentifyRetryWait = 5 * time.Second
	hotAddNBDCopyRetries    = 3
	hotAddNBDCopyRetryWait  = 10 * time.Second
)

// hotAddDiskTransfer holds per-disk state for the Hot-Add copy process.
type hotAddDiskTransfer struct {
	BlockDevice      string // /dev/sdX on the Proxy VM
	DestDevice       string // /dev/sdX on the vJailbreak appliance (destination)
	SnapshotVMDKPath string // frozen parent VMDK datastore path
	DiskKey          int32  // vCenter device key — used to detach during cleanup
	WWID             string // normalised NAA UUID (no dashes, lowercase)
	NBDPort          int    // port qemu-nbd is listening on
	NBDPid           int    // PID of qemu-nbd daemon on proxy VM
}

// vcenterClientGetter is a local interface allowing us to retrieve the concrete
// VCenterClient from the VMOperations implementation via type assertion.
type vcenterClientGetter interface {
	GetVCenterClient() *vcenter.VCenterClient
}

// getVCenterClient extracts the concrete VCenterClient from migobj.VMops.
func (migobj *Migrate) getVCenterClient() (*vcenter.VCenterClient, error) {
	getter, ok := migobj.VMops.(vcenterClientGetter)
	if !ok {
		return nil, errors.New("VMops does not implement GetVCenterClient()")
	}
	return getter.GetVCenterClient(), nil
}

// takeVMSnapshot creates a quiesced-off, memory-less snapshot on the source VM.
// If a snapshot with the same name already exists it is removed first.
func (migobj *Migrate) takeVMSnapshot(name string) error {
	snapshots, err := migobj.VMops.ListSnapshots()
	if err == nil {
		for _, snap := range snapshots {
			if snap.Name == name {
				migobj.logMessage(fmt.Sprintf("Removing pre-existing snapshot '%s'", name))
				if delErr := migobj.VMops.DeleteSnapshot(name); delErr != nil {
					return errors.Wrapf(delErr, "failed to remove pre-existing snapshot '%s'", name)
				}
				break
			}
		}
	}
	return migobj.VMops.TakeSnapshot(name)
}

// getFrozenVMDKs returns one hotAddDiskTransfer per data disk on the source VM,
// populated with the frozen parent VMDK path and device key after the snapshot.
func (migobj *Migrate) getFrozenVMDKs(ctx context.Context, vminfo vm.VMInfo) ([]hotAddDiskTransfer, error) {
	srcVM := migobj.VMops.GetVMObj()

	var vmProps mo.VirtualMachine
	if err := srcVM.Properties(ctx, srcVM.Reference(), []string{"config.hardware.device"}, &vmProps); err != nil {
		return nil, errors.Wrap(err, "failed to read source VM hardware config")
	}

	transfers := make([]hotAddDiskTransfer, 0, len(vminfo.VMDisks))

	for _, device := range vmProps.Config.Hardware.Device {
		vDisk, ok := device.(*govmomitypes.VirtualDisk)
		if !ok {
			continue
		}
		backing, ok := vDisk.Backing.(*govmomitypes.VirtualDiskFlatVer2BackingInfo)
		if !ok {
			continue
		}
		// After snapshot the current disk's backing.Parent holds the frozen pre-snapshot VMDK.
		frozenPath := backing.FileName
		if backing.Parent != nil && backing.Parent.FileName != "" {
			frozenPath = backing.Parent.FileName
		}
		transfers = append(transfers, hotAddDiskTransfer{
			SnapshotVMDKPath: frozenPath,
			DiskKey:          0, // filled in after attachment to proxy
		})
	}

	if len(transfers) == 0 {
		return nil, errors.New("no virtual disks found on source VM after snapshot")
	}
	return transfers, nil
}

// attachDiskToProxy attaches the frozen VMDK at vmdkPath to the proxy VM in
// independent_nonpersistent mode. Returns the vCenter device key of the new disk.
func (migobj *Migrate) attachDiskToProxy(ctx context.Context, proxyVMObj *object.VirtualMachine, vmdkPath string) (int32, error) {
	// Record existing disk keys before attach so we can identify the new one.
	keysBefore, err := getDiskKeys(ctx, proxyVMObj)
	if err != nil {
		return 0, errors.Wrap(err, "failed to list proxy VM disks before attach")
	}

	// Get the proxy VM's current device list to find an available SCSI controller.
	// ReconfigVM requires ControllerKey+UnitNumber to be set; without them vCenter
	// defaults to key=0 (IDE controller) which either rejects the request or fills up.
	deviceList, err := proxyVMObj.Device(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get proxy VM device list")
	}
	controller, err := deviceList.FindDiskController("")
	if err != nil {
		return 0, errors.Wrap(err, "failed to find available disk controller on proxy VM")
	}

	disk := &govmomitypes.VirtualDisk{
		VirtualDevice: govmomitypes.VirtualDevice{
			Backing: &govmomitypes.VirtualDiskFlatVer2BackingInfo{
				VirtualDeviceFileBackingInfo: govmomitypes.VirtualDeviceFileBackingInfo{
					FileName: vmdkPath,
				},
				DiskMode: string(govmomitypes.VirtualDiskModeIndependent_nonpersistent),
			},
		},
	}
	deviceList.AssignController(disk, controller)

	if err := proxyVMObj.AddDevice(ctx, disk); err != nil {
		return 0, errors.Wrapf(err, "failed to attach disk %s to proxy VM", vmdkPath)
	}

	// Find the key that was added.
	keysAfter, err := getDiskKeys(ctx, proxyVMObj)
	if err != nil {
		return 0, errors.Wrap(err, "failed to list proxy VM disks after attach")
	}
	for key := range keysAfter {
		if !keysBefore[key] {
			return key, nil
		}
	}
	return 0, fmt.Errorf("could not determine device key for newly attached disk %s", vmdkPath)
}

// getDiskKeys returns a set of VirtualDisk device keys currently on a VM.
func getDiskKeys(ctx context.Context, vmObj *object.VirtualMachine) (map[int32]bool, error) {
	var props mo.VirtualMachine
	if err := vmObj.Properties(ctx, vmObj.Reference(), []string{"config.hardware.device"}, &props); err != nil {
		return nil, err
	}
	keys := make(map[int32]bool)
	for _, dev := range props.Config.Hardware.Device {
		if _, ok := dev.(*govmomitypes.VirtualDisk); ok {
			keys[dev.GetVirtualDevice().Key] = true
		}
	}
	return keys, nil
}

// identifyBlockDevices SSH-queries the Proxy VM for NAA WWIDs and matches them
// against the vCenter disk backing UUIDs to populate transfer[i].BlockDevice and
// transfer[i].WWID. Each transfer is matched by its vCenter device key (DiskKey),
// which was recorded at attach time, so the mapping is deterministic regardless of
// map iteration order or leftover disks from prior runs.
// Retries up to hotAddIdentifyRetries times.
func (migobj *Migrate) identifyBlockDevices(ctx context.Context, sshClient *esxissh.Client,
	transfers []hotAddDiskTransfer, proxyVMObj *object.VirtualMachine,
) error {
	// Get per-device-key UUIDs from vCenter for this proxy VM.
	keyToUUID, err := migobj.getProxyDiskUUIDs(ctx, proxyVMObj)
	if err != nil {
		return errors.Wrap(err, "failed to get proxy VM disk UUIDs from vCenter")
	}

	for attempt := 1; attempt <= hotAddIdentifyRetries; attempt++ {
		guestMap, err := readGuestWWIDs(sshClient)
		if err != nil {
			migobj.logMessage(fmt.Sprintf("WWID read attempt %d/%d failed: %v", attempt, hotAddIdentifyRetries, err))
			time.Sleep(hotAddIdentifyRetryWait)
			continue
		}

		allMatched := true
		for i := range transfers {
			if transfers[i].BlockDevice != "" {
				continue // already identified
			}
			normUUID, ok := keyToUUID[transfers[i].DiskKey]
			if !ok {
				migobj.logMessage(fmt.Sprintf("Warning: no UUID found for proxy disk key %d (transfer %d)", transfers[i].DiskKey, i))
				allMatched = false
				continue
			}
			dev, ok := guestMap[normUUID]
			if !ok {
				allMatched = false
				continue
			}
			transfers[i].BlockDevice = "/dev/" + dev
			transfers[i].WWID = normUUID
		}

		if allMatched {
			return nil
		}
		migobj.logMessage(fmt.Sprintf("Not all disks identified yet (attempt %d/%d), retrying...", attempt, hotAddIdentifyRetries))
		time.Sleep(hotAddIdentifyRetryWait)
	}
	return errors.New("could not match all proxy VM disks to block devices after retries")
}

// getProxyDiskUUIDs returns a map of vCenter device key → normalised UUID for all
// VirtualDisk devices on the proxy VM. Keying by device key (not UUID) lets
// identifyBlockDevices do a precise per-disk lookup via transfers[i].DiskKey.
func (migobj *Migrate) getProxyDiskUUIDs(ctx context.Context, proxyVMObj *object.VirtualMachine) (map[int32]string, error) {
	var props mo.VirtualMachine
	if err := proxyVMObj.Properties(ctx, proxyVMObj.Reference(), []string{"config.hardware.device"}, &props); err != nil {
		return nil, err
	}
	result := make(map[int32]string)
	for _, dev := range props.Config.Hardware.Device {
		vDisk, ok := dev.(*govmomitypes.VirtualDisk)
		if !ok {
			continue
		}
		backing, ok := vDisk.Backing.(*govmomitypes.VirtualDiskFlatVer2BackingInfo)
		if !ok {
			continue
		}
		if backing.Uuid == "" {
			continue
		}
		key := dev.GetVirtualDevice().Key
		norm := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(backing.Uuid, "-", ""), " ", ""))
		result[key] = norm
	}
	return result, nil
}

// readGuestWWIDs SSHes into the proxy VM and returns a map of normalised NAA UUID
// to block device name (e.g. "sdb").
func readGuestWWIDs(sshClient *esxissh.Client) (map[string]string, error) {
	cmd := `for d in /sys/block/sd*; do w=$(cat "$d/device/wwid" 2>/dev/null); case "$w" in naa.*) echo "$(basename $d)|${w#naa.}";; esac; done`
	out, err := sshClient.ExecuteCommand(cmd)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}
		dev := strings.TrimSpace(parts[0])
		rawWWID := strings.TrimSpace(parts[1])
		norm := strings.ToLower(strings.ReplaceAll(rawWWID, "-", ""))
		result[norm] = dev
	}
	return result, nil
}

// findFreePort reads /proc/net/tcp and /proc/net/tcp6 on the Proxy VM and returns
// the first port in [rangeMin, rangeMax] not currently in use.
func (migobj *Migrate) findFreePort(sshClient *esxissh.Client, rangeMin, rangeMax int) (int, error) {
	out, err := sshClient.ExecuteCommand("cat /proc/net/tcp /proc/net/tcp6 2>/dev/null")
	if err != nil {
		return 0, errors.Wrap(err, "failed to read /proc/net/tcp on proxy VM")
	}

	usedPorts := make(map[int]bool)
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		localAddr := fields[1]
		// localAddr format: XXXXXXXX:PPPP (hex IP:hex port)
		colonIdx := strings.LastIndex(localAddr, ":")
		if colonIdx < 0 {
			continue
		}
		portHex := localAddr[colonIdx+1:]
		port, err := strconv.ParseInt(portHex, 16, 32)
		if err != nil {
			continue
		}
		usedPorts[int(port)] = true
	}

	for port := rangeMin; port <= rangeMax; port++ {
		if !usedPorts[port] {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no free port found in range %d-%d on proxy VM", rangeMin, rangeMax)
}

// serveViaNBD starts qemu-nbd on the Proxy VM in fork+persistent mode and returns
// the PID of the background daemon.
func (migobj *Migrate) serveViaNBD(sshClient *esxissh.Client, blockDevice string, port int) (int, error) {
	cmd := fmt.Sprintf("qemu-nbd --format=raw --port=%d --bind=0.0.0.0 --fork --persistent %s", port, blockDevice)
	out, err := sshClient.ExecuteCommand(cmd)
	if err != nil {
		return 0, errors.Wrapf(err, "qemu-nbd failed to start on port %d", port)
	}
	// qemu-nbd --fork prints the child PID to stdout.
	pidStr := strings.TrimSpace(out)
	if pidStr == "" {
		// Some versions don't print anything; return 0 so cleanup skips the kill.
		return 0, nil
	}
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		// Non-fatal: we may not be able to kill it explicitly, but the process
		// will be cleaned up when the disk is detached.
		migobj.logMessage(fmt.Sprintf("Warning: could not parse qemu-nbd PID from output %q: %v", pidStr, err))
		return 0, nil
	}
	return pid, nil
}

// runNBDCopy executes nbdcopy locally to transfer data from the NBD source on the
// proxy VM to the destination block device on the vJailbreak appliance.
// Retries up to hotAddNBDCopyRetries times.
func (migobj *Migrate) runNBDCopy(ctx context.Context, proxyIP string, port int, destDevice string) error {
	nbdURL := fmt.Sprintf("nbd://%s:%d", proxyIP, port)
	for attempt := 1; attempt <= hotAddNBDCopyRetries; attempt++ {
		migobj.logMessage(fmt.Sprintf("nbdcopy attempt %d/%d: %s → %s", attempt, hotAddNBDCopyRetries, nbdURL, destDevice))
		//nolint:gosec // nbdURL and destDevice come from validated internal state, not user input
		cmd := exec.CommandContext(ctx, "nbdcopy", nbdURL, destDevice)
		out, err := cmd.CombinedOutput()
		if err == nil {
			return nil
		}
		migobj.logMessage(fmt.Sprintf("nbdcopy attempt %d failed: %v\n%s", attempt, err, string(out)))
		if attempt < hotAddNBDCopyRetries {
			time.Sleep(hotAddNBDCopyRetryWait)
		}
	}
	return fmt.Errorf("nbdcopy failed after %d attempts: %s → %s", hotAddNBDCopyRetries, nbdURL, destDevice)
}

// adjustProxyDiskCount atomically adds delta to the ProxyVM's AttachedDiskCount status.
// Failures are non-fatal and are logged without aborting the caller.
func (migobj *Migrate) adjustProxyDiskCount(ctx context.Context, delta int) {
	if migobj.K8sClient == nil || migobj.ProxyVMName == "" || delta == 0 {
		return
	}
	key := k8stypes.NamespacedName{
		Name:      migobj.ProxyVMName,
		Namespace: constants.NamespaceMigrationSystem,
	}
	for attempt := 0; attempt < 3; attempt++ {
		proxyVM := &vjailbreakv1alpha1.ProxyVM{}
		if err := migobj.K8sClient.Get(ctx, key, proxyVM); err != nil {
			migobj.logMessage(fmt.Sprintf("Warning: could not get ProxyVM %s to update disk count: %v", migobj.ProxyVMName, err))
			return
		}
		patch := client.MergeFrom(proxyVM.DeepCopy())
		newCount := proxyVM.Status.AttachedDiskCount + delta
		if newCount < 0 {
			newCount = 0
		}
		proxyVM.Status.AttachedDiskCount = newCount
		if err := migobj.K8sClient.Status().Patch(ctx, proxyVM, patch); err != nil {
			if apierrors.IsConflict(err) {
				time.Sleep(200 * time.Millisecond)
				continue
			}
			migobj.logMessage(fmt.Sprintf("Warning: could not update ProxyVM %s disk count: %v", migobj.ProxyVMName, err))
			return
		}
		return
	}
	migobj.logMessage(fmt.Sprintf("Warning: gave up updating ProxyVM %s disk count after conflicts", migobj.ProxyVMName))
}

// cleanupHotAdd releases all resources acquired during Hot-Add copy:
// kills NBD daemon processes, detaches disks from proxy VM, removes the snapshot.
// Individual failures are logged but do not abort the cleanup sequence.
func (migobj *Migrate) cleanupHotAdd(ctx context.Context, sshClient *esxissh.Client,
	transfers []hotAddDiskTransfer, proxyVMObj *object.VirtualMachine,
) {
	// Kill NBD daemon processes.
	for _, t := range transfers {
		if t.NBDPid <= 0 {
			continue
		}
		if _, err := sshClient.ExecuteCommand(fmt.Sprintf("kill %d 2>/dev/null; true", t.NBDPid)); err != nil {
			migobj.logMessage(fmt.Sprintf("Warning: failed to kill qemu-nbd PID %d: %v", t.NBDPid, err))
		}
	}

	// Detach disks from proxy VM (keepFiles=true — we must NOT delete the frozen VMDK).
	if proxyVMObj != nil {
		var vmProps mo.VirtualMachine
		if err := proxyVMObj.Properties(ctx, proxyVMObj.Reference(), []string{"config.hardware.device"}, &vmProps); err != nil {
			migobj.logMessage(fmt.Sprintf("Warning: failed to read proxy VM devices during cleanup: %v", err))
		} else {
			for _, t := range transfers {
				if t.DiskKey == 0 {
					continue
				}
				for _, dev := range vmProps.Config.Hardware.Device {
					if dev.GetVirtualDevice().Key == t.DiskKey {
						if err := proxyVMObj.RemoveDevice(ctx, true, dev); err != nil {
							migobj.logMessage(fmt.Sprintf("Warning: failed to detach disk key %d from proxy VM: %v", t.DiskKey, err))
						}
						break
					}
				}
			}
		}
	}

	// Remove the source VM snapshot.
	if err := migobj.VMops.DeleteSnapshot(hotAddSnapName); err != nil {
		migobj.logMessage(fmt.Sprintf("Warning: failed to remove snapshot '%s': %v", hotAddSnapName, err))
	}

	// Decrement the proxy VM's attached disk count for every disk we attached.
	attached := 0
	for _, t := range transfers {
		if t.DiskKey != 0 {
			attached++
		}
	}
	// Use Background context: cleanup runs in a defer and the parent ctx may be cancelled.
	migobj.adjustProxyDiskCount(context.Background(), -attached)
}

// HotAddCopyDisks is the top-level entry point for the Hot-Add copy method.
// It snapshots the source VM, attaches frozen disks to the Proxy VM,
// identifies block devices, serves them via qemu-nbd, and transfers data
// using nbdcopy. Cleanup runs on return (success or failure).
func (migobj *Migrate) HotAddCopyDisks(ctx context.Context, vminfo vm.VMInfo) error {
	migobj.logMessage("Starting Hot-Add disk copy")

	if migobj.ProxyVMIP == "" || migobj.ProxyVMName == "" {
		return errors.New("ProxyVMIP and ProxyVMName must be set for HotAdd copy method")
	}

	// Read the vJailbreak appliance SSH key used to connect to the Proxy VM.
	sshKeyBytes, err := os.ReadFile(hotAddSSHKeyPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read SSH key at %s", hotAddSSHKeyPath)
	}

	// 1. Snapshot the source VM.
	migobj.logMessage(constants.EventMessageHotAddSnapshotCreate)
	if err := migobj.takeVMSnapshot(hotAddSnapName); err != nil {
		return errors.Wrap(err, "failed to create source VM snapshot")
	}

	// 2. Enumerate frozen VMDKs from the snapshot backing.
	transfers, err := migobj.getFrozenVMDKs(ctx, vminfo)
	if err != nil {
		_ = migobj.VMops.DeleteSnapshot(hotAddSnapName)
		return errors.Wrap(err, "failed to enumerate frozen VMDKs")
	}
	for i := range transfers {
		if i < len(vminfo.VMDisks) {
			transfers[i].DestDevice = vminfo.VMDisks[i].Path
		}
	}

	// 3. Open SSH connection to Proxy VM.
	sshClient := esxissh.NewClientWithTimeout(30 * time.Second)
	connectCtx, cancelConnect := context.WithTimeout(ctx, 60*time.Second)
	defer cancelConnect()
	if err := sshClient.Connect(connectCtx, migobj.ProxyVMIP, hotAddSSHUser, sshKeyBytes); err != nil {
		_ = migobj.VMops.DeleteSnapshot(hotAddSnapName)
		return errors.Wrapf(err, "SSH to Proxy VM %s failed", migobj.ProxyVMIP)
	}
	defer func() {
		if err := sshClient.Disconnect(); err != nil {
			migobj.logMessage(fmt.Sprintf("Warning: failed to disconnect SSH client: %v", err))
		}
	}()

	// 4. Find the Proxy VM object in vCenter.
	proxyVMObj, err := migobj.Vcclient.GetVMByName(ctx, migobj.ProxyVMName)
	if err != nil {
		_ = migobj.VMops.DeleteSnapshot(hotAddSnapName)
		return errors.Wrapf(err, "failed to locate Proxy VM '%s' in vCenter", migobj.ProxyVMName)
	}

	// Cleanup deferred after proxy VM is found so all resources are released.
	defer func() {
		migobj.logMessage(constants.EventMessageHotAddCleanup)
		migobj.cleanupHotAdd(ctx, sshClient, transfers, proxyVMObj)
	}()

	// 5. Attach each frozen disk to the Proxy VM.
	migobj.logMessage(constants.EventMessageHotAddAttachDisks)
	for i := range transfers {
		migobj.logMessage(fmt.Sprintf("Attaching disk %d/%d: %s", i+1, len(transfers), transfers[i].SnapshotVMDKPath))
		key, err := migobj.attachDiskToProxy(ctx, proxyVMObj, transfers[i].SnapshotVMDKPath)
		if err != nil {
			return errors.Wrapf(err, "failed to attach disk %s to Proxy VM", transfers[i].SnapshotVMDKPath)
		}
		transfers[i].DiskKey = key
		migobj.logMessage(fmt.Sprintf("Disk attached with vCenter key %d", key))
	}

	migobj.adjustProxyDiskCount(ctx, len(transfers))

	// 6. Identify block devices on the Proxy VM by matching NAA WWIDs.
	migobj.logMessage(constants.EventMessageHotAddIdentify)
	if err := migobj.identifyBlockDevices(ctx, sshClient, transfers, proxyVMObj); err != nil {
		return errors.Wrap(err, "failed to identify block devices on Proxy VM")
	}

	// 7. For each disk: find free port, start NBD server, run nbdcopy.
	for i := range transfers {
		migobj.logMessage(fmt.Sprintf("Copying disk %d/%d: %s → %s",
			i+1, len(transfers), transfers[i].BlockDevice, transfers[i].DestDevice))

		port, err := migobj.findFreePort(sshClient, constants.HotAddPortRangeMin, constants.HotAddPortRangeMax)
		if err != nil {
			return errors.Wrapf(err, "no free NBD port available for disk %d", i+1)
		}
		transfers[i].NBDPort = port

		migobj.logMessage(fmt.Sprintf("%s port %d for %s", constants.EventMessageHotAddServing, port, transfers[i].BlockDevice))
		pid, err := migobj.serveViaNBD(sshClient, transfers[i].BlockDevice, port)
		if err != nil {
			return errors.Wrapf(err, "failed to start qemu-nbd for disk %d on port %d", i+1, port)
		}
		transfers[i].NBDPid = pid

		migobj.logMessage(fmt.Sprintf("%s nbd://%s:%d → %s", constants.EventMessageHotAddCopying,
			migobj.ProxyVMIP, port, transfers[i].DestDevice))
		if err := migobj.runNBDCopy(ctx, migobj.ProxyVMIP, port, transfers[i].DestDevice); err != nil {
			return errors.Wrapf(err, "nbdcopy failed for disk %d", i+1)
		}
		migobj.logMessage(fmt.Sprintf("Disk %d/%d copied successfully", i+1, len(transfers)))
	}

	migobj.logMessage("Hot-Add disk copy completed successfully")
	return nil
}
