// Copyright © 2025 The vjailbreak authors

package migrate

import (
	"context"
	"net"
	"testing"

	"github.com/platform9/vjailbreak/v2v-helper/nbd"
	"github.com/vmware/govmomi/vim25/types"
)

// unusedTCPPort opens a TCP listener, grabs its port, then closes it --
// giving tests a port number nothing is listening on so a connection attempt
// fails fast (connection refused) instead of timing out.
func unusedTCPPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve a port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		t.Fatalf("failed to release reserved port: %v", err)
	}
	return port
}

// TestHotAddNBDServer_Teardown_Idempotent covers both states teardown() must
// handle safely without touching real infra: nothing attached, and something
// attached. session.proxyVMObj is nil (detachProxyDisks returns immediately)
// and migobj.K8sClient is nil (adjustProxyDiskCount returns immediately), so
// this never makes a real vCenter/K8s call -- only the diskKey/nbdPid
// bookkeeping is under test.
func TestHotAddNBDServer_Teardown_Idempotent(t *testing.T) {
	t.Run("nothing attached", func(t *testing.T) {
		h := &hotAddNBDServer{
			migobj:  &Migrate{},
			session: &hotAddSession{},
		}
		h.teardown(context.Background())
		h.teardown(context.Background()) // must not panic or double-decrement
		if h.diskKey != 0 || h.nbdPid != 0 {
			t.Errorf("got diskKey=%d nbdPid=%d, want both 0", h.diskKey, h.nbdPid)
		}
	})

	t.Run("disk attached, nbd not yet serving", func(t *testing.T) {
		h := &hotAddNBDServer{
			migobj:  &Migrate{},
			session: &hotAddSession{},
			diskKey: 4242,
		}
		h.teardown(context.Background())
		if h.diskKey != 0 || h.nbdPid != 0 {
			t.Errorf("got diskKey=%d nbdPid=%d, want both reset to 0", h.diskKey, h.nbdPid)
		}
		// Second call on the now-clean state must still be a no-op.
		h.teardown(context.Background())
		if h.diskKey != 0 || h.nbdPid != 0 {
			t.Errorf("second teardown() changed state: diskKey=%d nbdPid=%d", h.diskKey, h.nbdPid)
		}
	})
}

// TestHotAddNBDServer_CopyDisk_SkipsTeardownOnFailure guards the Finding-2
// regression: CopyDisk must NOT tear down the Proxy VM attachment when the
// copy itself fails, because nbd_copy.go's callers may still need the
// attachment/qemu-nbd for the current round. diskKey/nbdPid are left
// unchanged by a skipped teardown, so they're the observable signal here.
// nbdPid is deliberately left at 0 so that if this regresses and teardown
// runs anyway, killNBDDaemons/detachProxyDisks still no-op safely (nil
// session.sshClient / session.proxyVMObj) instead of panicking.
func TestHotAddNBDServer_CopyDisk_SkipsTeardownOnFailure(t *testing.T) {
	h := &hotAddNBDServer{
		migobj:  &Migrate{},
		session: &hotAddSession{},
		diskKey: 4242,
	}
	// Nothing is listening on this port, so nbdcopy/libnbd fails fast.
	h.NBDServer = *nbd.NewTCPSourceServer("127.0.0.1", unusedTCPPort(t), nil)

	err := h.CopyDisk(context.Background(), t.TempDir()+"/dest", 0, false)
	if err == nil {
		t.Fatal("expected CopyDisk against an unreachable source to fail")
	}
	if h.diskKey != 4242 {
		t.Errorf("teardown ran despite copy failure: diskKey=%d, want unchanged 4242", h.diskKey)
	}
}

// TestHotAddNBDServer_CopyChangedBlocks_SkipsTeardownOnFailure mirrors
// TestHotAddNBDServer_CopyDisk_SkipsTeardownOnFailure for the incremental-sync
// path, which is the one nbd_copy.go actually wraps in a retry loop.
func TestHotAddNBDServer_CopyChangedBlocks_SkipsTeardownOnFailure(t *testing.T) {
	h := &hotAddNBDServer{
		migobj:  &Migrate{},
		session: &hotAddSession{},
		diskKey: 4242,
	}
	h.NBDServer = *nbd.NewTCPSourceServer("127.0.0.1", unusedTCPPort(t), nil)

	err := h.CopyChangedBlocks(context.Background(), types.DiskChangeInfo{}, t.TempDir()+"/dest", false)
	if err == nil {
		t.Fatal("expected CopyChangedBlocks against an unreachable source to fail")
	}
	if h.diskKey != 4242 {
		t.Errorf("teardown ran despite copy failure: diskKey=%d, want unchanged 4242", h.diskKey)
	}
}
