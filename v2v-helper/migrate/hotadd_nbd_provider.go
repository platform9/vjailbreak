// Copyright © 2025 The vjailbreak authors

package migrate

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	esxissh "github.com/platform9/vjailbreak/v2v-helper/esxi-ssh"
	"github.com/platform9/vjailbreak/v2v-helper/nbd"
	k8sutils "github.com/platform9/vjailbreak/v2v-helper/pkg/k8sutils"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/utils"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
)

// hotAddCopyTimeout bounds each vCenter/SSH call StartNBDServer makes.
const hotAddCopyTimeout = 5 * time.Minute

const (
	hotAddStartRetries   = 3
	hotAddStartRetryWait = 10 * time.Second
)

// hotAddSession is the Proxy VM connection shared by every disk's hotAddNBDServer
// for one migration. ctx is retained so StartNBDServer/StopNBDServer stay
// cancellable with the rest of the migration.
type hotAddSession struct {
	ctx        context.Context
	sshClient  *esxissh.Client
	proxyVMObj *object.VirtualMachine
}

// NewHotAddSession opens the SSH connection to migobj's Proxy VM and looks up its
// vCenter object, to be shared across one hotAddNBDServer per disk.
func (migobj *Migrate) NewHotAddSession(ctx context.Context) (*hotAddSession, error) {
	sshKeyBytes, err := k8sutils.GetHotAddPrivateKey(ctx, migobj.K8sClient, migobj.ProxyVMK8sName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get Hot-Add SSH private key")
	}

	sshClient := esxissh.NewClientWithTimeout(30 * time.Second)
	connectCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err := sshClient.Connect(connectCtx, migobj.ProxyVMIP, hotAddSSHUser, sshKeyBytes); err != nil {
		return nil, errors.Wrapf(err, "SSH to Proxy VM %s failed", migobj.ProxyVMIP)
	}

	proxyVMObj, err := migobj.Vcclient.GetVMByName(ctx, migobj.ProxyVMName)
	if err != nil {
		_ = sshClient.Disconnect()
		return nil, errors.Wrapf(err, "failed to locate Proxy VM '%s' in vCenter", migobj.ProxyVMName)
	}

	return &hotAddSession{ctx: ctx, sshClient: sshClient, proxyVMObj: proxyVMObj}, nil
}

// Close disconnects the shared SSH client. Safe to call on a nil session.
func (s *hotAddSession) Close() {
	if s == nil {
		return
	}
	if err := s.sshClient.Disconnect(); err != nil {
		utils.PrintLog("Warning: failed to disconnect SSH client: " + err.Error())
	}
}

// hotAddNBDServer implements nbd.NBDOperations over the Proxy VM (SSH-attach +
// qemu-nbd) instead of local VDDK/nbdkit. It embeds nbd.NBDServer to reuse
// CopyDisk/CopyChangedBlocks/GetProgress unchanged.
type hotAddNBDServer struct {
	nbd.NBDServer

	migobj  *Migrate
	session *hotAddSession
	diskKey int32
	nbdPid  int
}

// NewHotAddNBDServer returns a per-disk nbd.NBDOperations backed by session, which
// callers create once per migration with NewHotAddSession and share across disks.
func NewHotAddNBDServer(migobj *Migrate, session *hotAddSession) nbd.NBDOperations {
	return &hotAddNBDServer{migobj: migobj, session: session}
}

// StartNBDServer attaches the frozen VMDK at file to the Proxy VM, identifies its
// block device, and starts qemu-nbd on it, retrying the whole sequence up to
// hotAddStartRetries times.
func (h *hotAddNBDServer) StartNBDServer(_ *object.VirtualMachine, _, _, _, _, _, file string, progchan chan string) error {
	var lastErr error
	for attempt := 1; attempt <= hotAddStartRetries; attempt++ {
		if attempt > 1 {
			utils.PrintLog(fmt.Sprintf("Hot-Add: retrying NBD server start (attempt %d/%d) after: %v", attempt, hotAddStartRetries, lastErr))
			select {
			case <-h.session.ctx.Done():
				return h.session.ctx.Err()
			case <-time.After(hotAddStartRetryWait):
			}
		}

		if err := h.startNBDServerOnce(file, progchan); err != nil {
			lastErr = err
			// Reset before retrying so we don't attach the same VMDK twice.
			teardownCtx, cancel := context.WithTimeout(context.Background(), hotAddCopyTimeout)
			h.teardown(teardownCtx)
			cancel()
			continue
		}
		return nil
	}
	return errors.Wrapf(lastErr, "failed to start Hot-Add NBD server after %d attempts", hotAddStartRetries)
}

// startNBDServerOnce is StartNBDServer's single-attempt body.
func (h *hotAddNBDServer) startNBDServerOnce(file string, progchan chan string) error {
	ctx, cancel := context.WithTimeout(h.session.ctx, hotAddCopyTimeout)
	defer cancel()

	migrationName, err := utils.GetMigrationObjectName()
	if err != nil {
		return errors.Wrap(err, "failed to get migration object name")
	}

	transfers := []hotAddDiskTransfer{{SnapshotVMDKPath: file}}
	if err := h.migobj.attachAllDisks(ctx, migrationName, h.session.proxyVMObj, transfers); err != nil {
		return err
	}
	h.diskKey = transfers[0].DiskKey
	h.migobj.adjustProxyDiskCount(ctx, 1)

	if err := h.migobj.identifyBlockDevices(ctx, h.session.sshClient, transfers, h.session.proxyVMObj); err != nil {
		return errors.Wrap(err, "failed to identify block device on Proxy VM")
	}

	ports, err := h.migobj.findFreePorts(h.session.sshClient, constants.HotAddPortRangeMin, constants.HotAddPortRangeMax, 1)
	if err != nil {
		return errors.Wrap(err, "failed to allocate NBD port")
	}

	pid, err := h.migobj.serveViaNBD(h.session.sshClient, transfers[0].BlockDevice, ports[0])
	if err != nil {
		return errors.Wrapf(err, "failed to start qemu-nbd on port %d", ports[0])
	}
	h.nbdPid = pid

	h.NBDServer = *nbd.NewTCPSourceServer(h.migobj.ProxyVMIP, ports[0], progchan)
	return nil
}

// teardown kills this round's qemu-nbd process and detaches its frozen VMDK.
// Idempotent -- safe to call repeatedly or with nothing attached.
func (h *hotAddNBDServer) teardown(ctx context.Context) {
	hadAttachment := h.diskKey != 0 || h.nbdPid != 0
	transfers := []hotAddDiskTransfer{{DiskKey: h.diskKey, NBDPid: h.nbdPid}}
	h.migobj.killNBDDaemons(h.session.sshClient, transfers)
	h.migobj.detachProxyDisks(ctx, h.session.proxyVMObj, transfers)
	if hadAttachment {
		h.migobj.adjustProxyDiskCount(ctx, -1)
	}
	h.diskKey = 0
	h.nbdPid = 0
}

// StopNBDServer tears down this round's attachment. Uses context.Background(), not
// h.session.ctx, because cleanup() (migrate.go) calls this after the migration's own
// ctx is already cancelled -- deriving from it here would fail every SSH command
// instantly and leave the disk/qemu-nbd stuck on the Proxy VM.
func (h *hotAddNBDServer) StopNBDServer() error {
	ctx, cancel := context.WithTimeout(context.Background(), hotAddCopyTimeout)
	defer cancel()
	h.teardown(ctx)
	return nil
}

func (h *hotAddNBDServer) CopyDisk(ctx context.Context, dest string, diskindex int, destEncrypted bool) error {
	copyErr := h.NBDServer.CopyDisk(ctx, dest, diskindex, destEncrypted)
	if copyErr == nil {
		teardownCtx, cancel := context.WithTimeout(context.Background(), hotAddCopyTimeout)
		defer cancel()
		h.teardown(teardownCtx)
	}
	return copyErr
}

func (h *hotAddNBDServer) CopyChangedBlocks(ctx context.Context, changedAreas types.DiskChangeInfo, path string, destEncrypted bool) error {
	copyErr := h.NBDServer.CopyChangedBlocks(ctx, changedAreas, path, destEncrypted)
	if copyErr == nil {
		teardownCtx, cancel := context.WithTimeout(context.Background(), hotAddCopyTimeout)
		defer cancel()
		h.teardown(teardownCtx)
	}
	return copyErr
}
