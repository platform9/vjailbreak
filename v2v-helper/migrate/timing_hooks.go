// Copyright © 2026 The vjailbreak authors

package migrate

import (
	"context"
	"fmt"

	"github.com/platform9/vjailbreak/v2v-helper/pkg/utils"
	"github.com/platform9/vjailbreak/v2v-helper/vm"
	"github.com/vmware/govmomi/vim25/mo"
)

// Coarse phase names. The per-API steps recorded by the vm and openstack
// decorators say where the time went inside a phase; these say how the phases
// compare to each other, which is the headline number in the Hot-Add report.
const (
	// StepDiskCopyTotal covers the whole data-copy phase whichever path ran:
	// Hot-Add proxy, nbdkit/VDDK, or storage-accelerated copy. It is the single
	// row that answers "is Hot-Add faster".
	StepDiskCopyTotal = "Data Copy: total (all disks)"
	// StepConvertTotal covers ConvertVolumes — boot-disk detection plus
	// virt-v2v-in-place. Should be near-identical between the two runs; if it
	// is not, the runs were not comparable.
	StepConvertTotal = "Convert: total (virt-v2v-in-place)"
	// StepCreateInstanceTotal covers CreateTargetInstance end to end.
	StepCreateInstanceTotal = "Target: create instance (total)"

	// Hot-Add sub-steps. These have no equivalent on the VDDK path, which is
	// exactly why they need their own rows — they are the overhead Hot-Add
	// pays to buy a faster read.
	StepHotAddFetchSSHKeySecret = "HotAdd: fetch Proxy VM SSH key secret"
	StepHotAddEnumerateFrozen   = "HotAdd: enumerate frozen VMDKs"
	StepHotAddSSHConnect        = "HotAdd: SSH connect to Proxy VM"
	StepHotAddLocateProxyInVC   = "HotAdd: locate Proxy VM in vCenter"
	StepHotAddAttachToProxy     = "HotAdd: attach frozen VMDK to Proxy VM"
	StepHotAddIdentifyDevices   = "HotAdd: identify block devices on Proxy VM"
	StepHotAddAllocatePorts     = "HotAdd: allocate NBD ports on Proxy VM"
	StepHotAddServeNBD          = "HotAdd: start qemu-nbd on Proxy VM"
	StepHotAddNBDCopy           = "HotAdd: nbdcopy per disk"
	StepHotAddCleanupTotal      = "HotAdd: cleanup (kill nbd, detach, delete snapshot)"
)

// recordSourceFootprint captures how much disk the source VM actually holds and
// hands it to the timing recorder.
//
// This is what makes two runs comparable. The Hot-Add path serves a raw block
// device over qemu-nbd, so nbdcopy moves every byte of the provisioned size.
// The VDDK path reads allocated extents and skips holes. On a thin-provisioned
// 200 GB disk holding 30 GB the two runs are moving wildly different amounts of
// data, and comparing their wall clock without recording this is meaningless.
//
// committed comes from the VM's summary.storage and is an upper bound on guest
// data: it also counts snapshot deltas, swap and log files.
func (migobj *Migrate) recordSourceFootprint(ctx context.Context, vminfo vm.VMInfo) {
	var provisioned int64
	for _, disk := range vminfo.VMDisks {
		provisioned += disk.Size
	}

	var committed int64
	if vmObj := migobj.VMops.GetVMObj(); vmObj != nil {
		var props mo.VirtualMachine
		if err := vmObj.Properties(ctx, vmObj.Reference(), []string{"summary.storage"}, &props); err != nil {
			utils.PrintLog(fmt.Sprintf("Warning: could not read source VM storage summary for timing report: %v", err))
		} else if props.Summary.Storage != nil {
			committed = props.Summary.Storage.Committed
		}
	}

	migobj.Timing.SetDiskInfo(len(vminfo.VMDisks), provisioned, committed)
	migobj.logMessage(fmt.Sprintf(
		"Source footprint: %d disk(s), provisioned=%d bytes, committed=%d bytes (committed includes snapshot/swap/log files)",
		len(vminfo.VMDisks), provisioned, committed))
}
