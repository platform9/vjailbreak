// Copyright © 2024 The vjailbreak authors

package nbd

import (
	"fmt"
	"time"

	"github.com/platform9/vjailbreak/v2v-helper/pkg/utils"
	"github.com/vmware/govmomi/object"
	"libguestfs.org/libnbd"
)

// StaleNFCSession is one deliberately-idle NFC connection opened by
// StartDebugStaleNFCSessions: an nbdkit/VDDK server process, plus a live NBD
// client handle connected to it and held open without ever being closed.
//
// The client handle is what actually matters. nbdkit's vddk plugin does not
// call VixDiskLib_Open - the point at which an NFC session is actually
// established on the ESXi/vCenter side - until something connects to it and
// completes the NBD handshake; a bare nbdkit process with nothing connected
// to its socket sits there idle and consumes no NFC session at all. (This is
// exactly why an earlier version of this knob, which only started the
// nbdkit processes and never connected to them, opened 0 real NFC sessions
// no matter how high the requested count was - see StartDebugStaleNFCSessions.)
type StaleNFCSession struct {
	server *NBDServer
	handle *libnbd.Libnbd
}

// staleSessionConnectAttempts/-Delay bound how long StartDebugStaleNFCSessions
// waits for each freshly-started nbdkit's unix socket to come up before
// giving up on that one session. nbdkit is typically ready within
// milliseconds, but spinning up dozens of VDDK/NFC connections concurrently
// (the whole point of this knob) can make that noticeably slower.
const (
	staleSessionConnectAttempts = 15
	staleSessionConnectDelay    = 200 * time.Millisecond
)

// StartDebugStaleNFCSessions is a fault-injection helper, disabled at count
// <= 0, that opens `count` additional, genuinely-live NFC sessions against
// the given VM/snapshot/disk - purely to consume NFC session slots on the
// target vCenter/ESXi host.
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
// operator can hand-edit on a running migration at any time. The caller is
// Migrate.SyncCBT (the periodic sync cycle), which re-reads that ConfigMap
// immediately before each cycle's NBD server restart - not LiveReplicateDisks'
// one-time initial copy - because that is where real NFC session-cap/"faulted
// session" failures have actually been observed (VJAILB-243, VJAILB-244):
// periodic sync opens a fresh NFC session on every cycle that has changed
// blocks, so session-cap pressure builds up over many cycles rather than
// showing up on the first copy. An edit takes effect on the next cycle
// without needing to touch the pod spec.
//
// SAFETY: the NFC session cap is global to the vCenter, not per-migration.
// Enabling this pressures/exhausts a shared resource, which can fault or
// stall ANY other NFC consumer on that vCenter at the same time (other
// migrations, backups, clones) - not just this one. Only ever set this
// against a lab/test vCenter, never against a production/customer vCenter.
//
// Each session is: an nbdkit/VDDK server started the same way a real disk
// copy's NBDServer is (including --exit-with-parent, so it dies with this
// process rather than leaking on the host), plus a libnbd client handle
// connected to it and never closed - which is what actually drives nbdkit's
// vddk plugin to open the VMDK (VixDiskLib_Open) and establish the real NFC
// session on the vCenter/ESXi side. Neither side is ever driven with real
// I/O; they just sit there holding the session open until
// StopDebugStaleNFCSessions is called or the process exits.
//
// A failure opening or connecting one session (fewer than `count` sessions
// returned) is not itself treated as an error: once the target vCenter's NFC
// cap is actually being pressured, a failure here may well be the very fault
// this is trying to reproduce, so it's logged and the caller gets back
// whatever did succeed rather than the whole migration aborting.
func StartDebugStaleNFCSessions(count int, vm *object.VirtualMachine, server, username, password, thumbprint, snapref, file string) []*StaleNFCSession {
	if count <= 0 {
		return nil
	}

	utils.PrintLog(fmt.Sprintf(
		"*** DEBUG FAULT INJECTION ENABLED (%d extra stale NFC session(s) requested via the migration ConfigMap): "+
			"opening %d extra live session(s) against %s to intentionally pressure/exceed vCenter's NFC session cap "+
			"from within this single migration. This is NOT normal migration behavior - only run this against "+
			"a lab/test vCenter. ***",
		count, count, server))

	stale := make([]*StaleNFCSession, 0, count)
	for i := 0; i < count; i++ {
		nbdServer := &NBDServer{}
		if err := nbdServer.StartNBDServer(vm, server, username, password, thumbprint, snapref, file, nil); err != nil {
			utils.PrintLog(fmt.Sprintf(
				"DEBUG: stale NFC session %d/%d: nbdkit failed to start (%v) - if the cap is already exceeded, "+
					"this may itself be the reproduction firing", i+1, count, err))
			continue
		}

		handle, err := connectStaleNFCHandle(nbdServer.tmp_dir)
		if err != nil {
			utils.PrintLog(fmt.Sprintf(
				"DEBUG: stale NFC session %d/%d: nbdkit started but the NFC/VDDK open failed (%v) - if the cap "+
					"is already exceeded, THIS is likely the reproduction firing", i+1, count, err))
			_ = nbdServer.StopNBDServer()
			continue
		}

		utils.PrintLog(fmt.Sprintf("DEBUG: stale NFC session %d/%d opened (nbdkit connected, NFC/VDDK session established)", i+1, count))
		stale = append(stale, &StaleNFCSession{server: nbdServer, handle: handle})
	}

	utils.PrintLog(fmt.Sprintf("DEBUG: %d/%d requested stale NFC sessions actually established", len(stale), count))
	return stale
}

// connectStaleNFCHandle connects a libnbd client to the nbdkit instance
// listening in tmpDir, retrying briefly while the unix socket comes up. A
// successful ConnectUri is what makes nbdkit's vddk plugin call
// VixDiskLib_Open - the real, session-consuming step - not just start the
// server process.
func connectStaleNFCHandle(tmpDir string) (*libnbd.Libnbd, error) {
	handle, err := libnbd.Create()
	if err != nil {
		return nil, fmt.Errorf("failed to create libnbd handle: %w", err)
	}

	sockUrl := generateSockUrl(tmpDir)
	var connectErr error
	for attempt := 0; attempt < staleSessionConnectAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(staleSessionConnectDelay)
		}
		if connectErr = handle.ConnectUri(sockUrl); connectErr == nil {
			return handle, nil
		}
	}

	handle.Close()
	return nil, connectErr
}

// StopDebugStaleNFCSessions tears down every session returned by a prior
// StartDebugStaleNFCSessions call: closes the client handle (releasing the
// NFC session) and then stops the backing nbdkit process. Safe to call with
// a nil or empty slice.
func StopDebugStaleNFCSessions(stale []*StaleNFCSession) {
	for i, session := range stale {
		if session.handle != nil {
			if err := session.handle.Close(); err != nil {
				utils.PrintLog(fmt.Sprintf("DEBUG: failed to close stale NFC session %d/%d handle: %v", i+1, len(stale), err))
			}
		}
		if err := session.server.StopNBDServer(); err != nil {
			utils.PrintLog(fmt.Sprintf("DEBUG: failed to stop stale NFC session %d/%d nbdkit process: %v", i+1, len(stale), err))
		}
	}
}
