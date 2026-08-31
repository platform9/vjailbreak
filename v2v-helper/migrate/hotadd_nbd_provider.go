// Copyright © 2025 The vjailbreak authors

package migrate

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	esxissh "github.com/platform9/vjailbreak/v2v-helper/esxi-ssh"
	"github.com/platform9/vjailbreak/v2v-helper/nbd"
	k8sutils "github.com/platform9/vjailbreak/v2v-helper/pkg/k8sutils"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/utils"
	"github.com/vmware/govmomi/object"
)

// hotAddCopyTimeout bounds each individual vCenter/SSH call StartNBDServer and
// StopNBDServer make, so one wedged call can't hang forever even though the
// session's own context has no deadline.
const hotAddCopyTimeout = 5 * time.Minute

// hotAddSession is the Proxy VM connection shared by every disk's hotAddNBDServer for
// one migration. NewHotAddSession opens it once; Close tears it down once, at the end.
// It retains the migration's own ctx -- storing a context is normally an anti-pattern,
// but nbd.NBDOperations.StartNBDServer/StopNBDServer (shared with the VDDK
// implementation) take none, so this is how their calls stay cancellable with the rest
// of the migration instead of running on an unrelated context.Background().
type hotAddSession struct {
	ctx        context.Context
	sshClient  *esxissh.Client
	proxyVMObj *object.VirtualMachine
}

// NewHotAddSession opens the SSH connection to migobj's Proxy VM and looks up its
// vCenter object, ready to be shared across one hotAddNBDServer per disk. ctx should
// be the migration's own long-lived context -- it is retained on the session (see
// hotAddSession) for StartNBDServer/StopNBDServer to derive their per-call contexts from.
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

// hotAddNBDServer implements nbd.NBDOperations by serving the source VM's frozen
// VMDK through the Proxy VM (SSH-attach + qemu-nbd) instead of local VDDK/nbdkit.
// It embeds nbd.NBDServer to reuse CopyDisk/CopyChangedBlocks/GetProgress unchanged --
// that engine already works identically against a TCP qemu-nbd source.
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
// block device, and starts qemu-nbd on it. vm/server/username/password/thumbprint/
// snapref are VDDK-specific and unused here -- file already names the exact frozen
// VMDK to attach.
func (h *hotAddNBDServer) StartNBDServer(_ *object.VirtualMachine, _, _, _, _, _, file string, progchan chan string) error {
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

// StopNBDServer kills this round's qemu-nbd process and detaches its frozen VMDK
// from the Proxy VM, leaving the Proxy VM ready for the next round's StartNBDServer.
func (h *hotAddNBDServer) StopNBDServer() error {
	ctx, cancel := context.WithTimeout(h.session.ctx, hotAddCopyTimeout)
	defer cancel()

	transfers := []hotAddDiskTransfer{{DiskKey: h.diskKey, NBDPid: h.nbdPid}}
	h.migobj.killNBDDaemons(h.session.sshClient, transfers)
	h.migobj.detachProxyDisks(ctx, h.session.proxyVMObj, transfers)
	h.migobj.adjustProxyDiskCount(ctx, -1)
	h.diskKey = 0
	h.nbdPid = 0
	return nil
}
