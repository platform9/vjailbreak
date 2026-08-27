// Copyright © 2024 The vjailbreak authors

package nbd

import (
	"fmt"

	"github.com/platform9/vjailbreak/v2v-helper/pkg/utils"
	"github.com/vmware/govmomi/object"
)

// StartDebugStaleNFCSessions is a fault-injection helper, disabled at count
// <= 0, that opens `count` additional nbdkit/VDDK NFC sessions against the
// given VM/snapshot/disk - purely to consume NFC session slots on the target
// vCenter/ESXi host.
//
// PURPOSE: vCenter caps total concurrent NFC connections at 52 (see VMware's
// "NFC Session Limits and Timeouts"). Reports of migrations stalling or
// hitting "faulted" NFC/compression sessions (e.g. VJAILB-244) are
// consistent with that cap - or ESXi hostd's own NFC memory limit - being
// exceeded, whether by several concurrent migrations or by other NFC
// consumers on the same vCenter. This knob lets a single migration
// deliberately manufacture that condition (by opening many redundant
// sessions against its own disk) so the failure can be reproduced on demand
// against a lab vCenter, without needing 52 real concurrent migrations.
//
// `count` is read by the caller from the migration ConfigMap's
// constants.DebugStaleNFCSessionsKey (see v2v-helper/pkg/utils.MigrationParams.
// DebugStaleNFCSessions), which the controller seeds at "0" (disabled) and an
// operator can hand-edit on a running migration - after triggering it, before
// v2v-helper reaches the CopyingBlocks/nbdcopy phase - to a positive value.
// LiveReplicateDisks re-reads that ConfigMap immediately before calling this,
// so the edit takes effect on the very next migration to reach that point
// without needing to touch the pod spec.
//
// SAFETY: the NFC session cap is global to the vCenter, not per-migration.
// Enabling this pressures/exhausts a shared resource, which can fault or
// stall ANY other NFC consumer on that vCenter at the same time (other
// migrations, backups, clones) - not just this one. Only ever set this
// against a lab/test vCenter, never against a production/customer vCenter.
//
// The extra sessions are opened the same way a real disk copy's NBDServer is
// (including --exit-with-parent, so they die with this process rather than
// leaking on the host) but are never driven by nbdcopy - they just sit there
// holding an NFC session open until StopDebugStaleNFCSessions is called or
// the process exits.
//
// A failure partway through opening these (fewer than `count` sessions
// returned) is not itself treated as an error: once the target vCenter's NFC
// cap is actually being pressured, an open failure here may well be the very
// fault this is trying to reproduce, so it's logged and the caller gets back
// whatever did succeed rather than the whole migration aborting.
func StartDebugStaleNFCSessions(count int, vm *object.VirtualMachine, server, username, password, thumbprint, snapref, file string) []*NBDServer {
	if count <= 0 {
		return nil
	}

	utils.PrintLog(fmt.Sprintf(
		"*** DEBUG FAULT INJECTION ENABLED (%d extra stale NFC session(s) requested via the migration ConfigMap): "+
			"opening %d extra session(s) against %s to intentionally pressure/exceed vCenter's NFC session cap "+
			"from within this single migration. This is NOT normal migration behavior - only run this against "+
			"a lab/test vCenter. ***",
		count, count, server))

	stale := make([]*NBDServer, 0, count)
	for i := 0; i < count; i++ {
		session := &NBDServer{}
		if err := session.StartNBDServer(vm, server, username, password, thumbprint, snapref, file, nil); err != nil {
			utils.PrintLog(fmt.Sprintf(
				"DEBUG: stale NFC session %d/%d failed to open (%v) - if the cap is already exceeded, "+
					"this may itself be the reproduction firing", i+1, count, err))
			continue
		}
		utils.PrintLog(fmt.Sprintf("DEBUG: stale NFC session %d/%d opened", i+1, count))
		stale = append(stale, session)
	}

	utils.PrintLog(fmt.Sprintf("DEBUG: %d/%d requested stale NFC sessions opened successfully", len(stale), count))
	return stale
}

// StopDebugStaleNFCSessions tears down every session returned by a prior
// StartDebugStaleNFCSessions call. Safe to call with a nil or empty slice.
func StopDebugStaleNFCSessions(stale []*NBDServer) {
	for i, session := range stale {
		if err := session.StopNBDServer(); err != nil {
			utils.PrintLog(fmt.Sprintf("DEBUG: failed to stop stale NFC session %d/%d: %v", i+1, len(stale), err))
		}
	}
}
