// Copyright © 2025 The vjailbreak authors

package migrate

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	esxissh "github.com/platform9/vjailbreak/v2v-helper/esxi-ssh"
	k8sutils "github.com/platform9/vjailbreak/v2v-helper/pkg/k8sutils"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/utils"
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

// takeVMSnapshot removes any pre-existing snapshot with the same name, then creates
// a memory-less snapshot of the source VM. quiesce=true requires VMware Tools and
// a powered-off (or quiescence-capable) VM; pass quiesce=false for mock migrations
// where the source VM remains powered on.
func (migobj *Migrate) takeVMSnapshot(ctx context.Context, name string, quiesce bool) error {
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

	vmObj := migobj.VMops.GetVMObj()
	task, err := vmObj.CreateSnapshot(ctx, name, "", false, quiesce)
	if err != nil {
		return errors.Wrap(err, "failed to create snapshot")
	}
	return task.Wait(ctx)
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

	// ReconfigVM requires ControllerKey+UnitNumber; without them vCenter defaults to
	// key=0 (IDE) which rejects the request or fills up.
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

// identifyBlockDevices matches proxy VM NAA WWIDs to vCenter disk UUIDs by device key,
// populating transfer[i].BlockDevice and WWID. Retries up to hotAddIdentifyRetries times.
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
// VirtualDisk devices on the proxy VM.
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

// findFreePorts reads /proc/net/tcp once and returns count free ports in [rangeMin, rangeMax].
// Reading once prevents concurrent goroutines from racing to pick the same port.
func (migobj *Migrate) findFreePorts(sshClient *esxissh.Client, rangeMin, rangeMax, count int) ([]int, error) {
	out, err := sshClient.ExecuteCommand("cat /proc/net/tcp /proc/net/tcp6 2>/dev/null")
	if err != nil {
		return nil, errors.Wrap(err, "failed to read /proc/net/tcp on proxy VM")
	}

	usedPorts := make(map[int]bool)
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		localAddr := fields[1]
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

	var ports []int
	for port := rangeMin; port <= rangeMax && len(ports) < count; port++ {
		if !usedPorts[port] {
			ports = append(ports, port)
		}
	}
	if len(ports) < count {
		return nil, fmt.Errorf("not enough free ports in range %d-%d (need %d, found %d)",
			rangeMin, rangeMax, count, len(ports))
	}
	return ports, nil
}

// serveViaNBD starts qemu-nbd on the Proxy VM via nohup and returns its PID.
// Polls /proc/net/tcp until the port is bound before returning.
func (migobj *Migrate) serveViaNBD(sshClient *esxissh.Client, blockDevice string, port int) (int, error) {
	portHex := strings.ToUpper(fmt.Sprintf("%04x", port))
	cmd := fmt.Sprintf(
		`nohup qemu-nbd --format=raw --port=%d --bind=0.0.0.0 --persistent %s </dev/null >/dev/null 2>&1 & `+
			`pid=$!; i=0; `+
			`while [ $i -lt 20 ] && ! grep -q ":%s " /proc/net/tcp /proc/net/tcp6 2>/dev/null; `+
			`do i=$((i+1)); sleep 0.25; done; `+
			`echo $pid`,
		port, blockDevice, portHex,
	)
	out, err := sshClient.ExecuteCommand(cmd)
	if err != nil {
		return 0, errors.Wrapf(err, "qemu-nbd failed to start on port %d", port)
	}
	pidStr := strings.TrimSpace(out)
	if pidStr == "" {
		return 0, fmt.Errorf("qemu-nbd started on port %d but PID was not captured", port)
	}
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, fmt.Errorf("could not parse qemu-nbd PID from output %q: %w", pidStr, err)
	}
	return pid, nil
}

// runNBDCopy transfers data from an NBD source on the proxy VM to a local block device.
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
	if migobj.K8sClient == nil || migobj.ProxyVMK8sName == "" || delta == 0 {
		return
	}
	key := k8stypes.NamespacedName{
		Name:      migobj.ProxyVMK8sName,
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

// cleanupHotAdd kills NBD daemons, detaches proxy VM disks, and removes the snapshot.
// Individual failures are logged without aborting the cleanup sequence.
func (migobj *Migrate) cleanupHotAdd(ctx context.Context, sshClient *esxissh.Client,
	transfers []hotAddDiskTransfer, proxyVMObj *object.VirtualMachine,
) {
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

// HotAddCopyDisks powers off the source VM, snapshots it, attaches frozen disks to
// the Proxy VM, and transfers data via qemu-nbd+nbdcopy. Cleanup runs on return.
func (migobj *Migrate) HotAddCopyDisks(ctx context.Context, vminfo vm.VMInfo) error {
	migobj.logMessage("Starting Hot-Add disk copy")

	if migobj.ProxyVMIP == "" || migobj.ProxyVMName == "" {
		return errors.New("ProxyVMIP and ProxyVMName must be set for HotAdd copy method")
	}

	// Load the per-proxy-VM SSH private key from the k8s secret named
	// "{proxyVMName}-hot-add-ssh-key", created during Proxy VM onboarding.
	sshKeyBytes, err := k8sutils.GetHotAddPrivateKey(ctx, migobj.K8sClient, migobj.ProxyVMK8sName)
	if err != nil {
		return errors.Wrap(err, "failed to get Hot-Add SSH private key")
	}

	// 1. Power off the source VM — skipped for mock migrations.
	if migobj.MigrationType == "mock" {
		migobj.logMessage("Mock migration detected, skipping VM power off")
	} else {
		migobj.logMessage("Powering off source VM before Hot-Add snapshot")
		if err := migobj.VMops.VMPowerOff(); err != nil {
			return errors.Wrap(err, "failed to power off source VM")
		}
		if err := utils.DoRetryWithExponentialBackoff(ctx, func() error {
			currState, stateErr := migobj.VMops.GetVMObj().PowerState(ctx)
			if stateErr != nil {
				return stateErr
			}
			if currState != govmomitypes.VirtualMachinePowerStatePoweredOff {
				return fmt.Errorf("VM power-off command completed but VM is still in state: %s", currState)
			}
			return nil
		}, constants.MaxPowerOffRetryLimit, constants.PowerOffRetryCap); err != nil {
			return errors.Wrap(err, "failed to verify VM power state after power off")
		}
	}

	// 2. Snapshot the source VM. Use quiesced=true for cold migrations (VM is
	// powered off); use quiesced=false for mock so the powered-on VM is
	// snapshotted crash-consistently without requiring VMware Tools quiescence.
	quiesce := migobj.MigrationType != "mock"
	migobj.logMessage(constants.EventMessageHotAddSnapshotCreate)
	if err := migobj.takeVMSnapshot(ctx, hotAddSnapName, quiesce); err != nil {
		return errors.Wrap(err, "failed to create source VM snapshot")
	}

	// 3. Enumerate frozen VMDKs from the snapshot backing.
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

	// 4. Open SSH connection to Proxy VM.
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

	// 5. Find the Proxy VM object in vCenter.
	proxyVMObj, err := migobj.Vcclient.GetVMByName(ctx, migobj.ProxyVMName)
	if err != nil {
		_ = migobj.VMops.DeleteSnapshot(hotAddSnapName)
		return errors.Wrapf(err, "failed to locate Proxy VM '%s' in vCenter", migobj.ProxyVMName)
	}

	// Cleanup is deferred here so all resources are released on any return path.
	// context.Background() ensures govmomi calls succeed even if parent ctx is cancelled.
	defer func() {
		migobj.logMessage(constants.EventMessageHotAddCleanup)
		migobj.cleanupHotAdd(context.Background(), sshClient, transfers, proxyVMObj)
	}()

	// 6. Attach each frozen disk to the Proxy VM.
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

	// 7. Identify block devices on the Proxy VM by matching NAA WWIDs.
	migobj.logMessage(constants.EventMessageHotAddIdentify)
	if err := migobj.identifyBlockDevices(ctx, sshClient, transfers, proxyVMObj); err != nil {
		return errors.Wrap(err, "failed to identify block devices on Proxy VM")
	}

	// 8. Pre-allocate one NBD port per disk before launching goroutines so concurrent
	//    workers don't race and pick the same port from /proc/net/tcp.
	ports, err := migobj.findFreePorts(sshClient, constants.HotAddPortRangeMin, constants.HotAddPortRangeMax, len(transfers))
	if err != nil {
		return errors.Wrap(err, "failed to allocate NBD ports")
	}
	for i := range transfers {
		transfers[i].NBDPort = ports[i]
	}

	// 9. Copy all disks concurrently — each disk gets its own NBD server and nbdcopy.
	overallStart := time.Now()
	errCh := make(chan error, len(transfers))
	var wg sync.WaitGroup

	for i := range transfers {
		wg.Add(1)
		t := &transfers[i]
		idx := i
		go func() {
			defer wg.Done()
			diskStart := time.Now()

			migobj.logMessage(fmt.Sprintf("%s port %d for %s (disk %d/%d)",
				constants.EventMessageHotAddServing, t.NBDPort, t.BlockDevice, idx+1, len(transfers)))
			pid, err := migobj.serveViaNBD(sshClient, t.BlockDevice, t.NBDPort)
			if err != nil {
				errCh <- errors.Wrapf(err, "disk %d: failed to start NBD server on port %d", idx+1, t.NBDPort)
				return
			}
			t.NBDPid = pid

			migobj.logMessage(fmt.Sprintf("%s nbd://%s:%d → %s (disk %d/%d)",
				constants.EventMessageHotAddCopying, migobj.ProxyVMIP, t.NBDPort, t.DestDevice, idx+1, len(transfers)))
			if err := migobj.runNBDCopy(ctx, migobj.ProxyVMIP, t.NBDPort, t.DestDevice); err != nil {
				errCh <- errors.Wrapf(err, "disk %d: nbdcopy failed", idx+1)
				return
			}

			migobj.logMessage(fmt.Sprintf("Disk %d/%d copied in %s: %s → %s",
				idx+1, len(transfers), time.Since(diskStart).Round(time.Second),
				t.BlockDevice, t.DestDevice))
		}()
	}

	wg.Wait()
	close(errCh)

	overallDuration := time.Since(overallStart)
	var copyErrors []string
	for err := range errCh {
		copyErrors = append(copyErrors, err.Error())
	}
	if len(copyErrors) > 0 {
		return fmt.Errorf("one or more disks failed to copy:\n  %s", strings.Join(copyErrors, "\n  "))
	}
	migobj.logMessage(fmt.Sprintf("All %d disk(s) copied in %s", len(transfers), overallDuration.Round(time.Second)))

	migobj.logMessage("Hot-Add disk copy completed successfully")
	return nil
}
