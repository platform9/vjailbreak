// Copyright © 2024 The vjailbreak authors

package migrate

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	"github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage"
	"github.com/platform9/vjailbreak/v2v-helper/nbd"
	"github.com/platform9/vjailbreak/v2v-helper/openstack"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/k8sutils"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/utils"
	"github.com/platform9/vjailbreak/v2v-helper/reporter"
	"github.com/platform9/vjailbreak/v2v-helper/vcenter"
	"github.com/platform9/vjailbreak/v2v-helper/vm"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Migrate struct {
	URL                     string
	UserName                string
	Password                string
	Insecure                bool
	Networknames            []string
	Networkports            []string
	Volumetypes             []string
	Virtiowin               string
	Ostype                  string
	Thumbprint              string
	Convert                 bool
	DisconnectSourceNetwork bool
	Openstackclients        openstack.OpenstackOperations
	Vcclient                vcenter.VCenterOperations
	VMops                   vm.VMOperations
	Nbdops                  []nbd.NBDOperations
	EventReporter           chan string
	PodLabelWatcher         chan string
	InPod                   bool
	MigrationTimes          MigrationTimes
	MigrationType           string
	PerformHealthChecks     bool
	HealthCheckPort         string
	K8sClient               client.Client
	TargetFlavorId          string
	TargetAvailabilityZone  string
	SecurityGroups          []string
	ServerGroup             string
	RDMDisks                []string
	TenantName              string
	Reporter                *reporter.Reporter
	FallbackToDHCP          bool
	StorageCopyMethod       string
	// Array credentials for StorageAcceleratedCopy storage migration
	ArrayHost         string
	ArrayUser         string
	ArrayPassword     string
	ArrayInsecure     bool
	VendorType        string
	ArrayCredsMapping string
	// Hot-Add copy method: Proxy VM coordinates
	ProxyVMIP      string
	ProxyVMName    string // vCenter display name — used to locate VM in vCenter
	ProxyVMK8sName string // Kubernetes resource name — used to patch ProxyVM status
	// NetApp-only. Left empty for non-NetApp vendors; when empty for NetApp
	// the provider falls back to auto-detection from existing LUNs or a
	// single-SVM/single-FlexVol auto-pick.
	NetAppSVM         string
	NetAppFlexVol     string
	StorageProvider   storage.StorageProvider
	ESXiSSHPrivateKey []byte
	ESXiSSHSecretName string // Name of the Kubernetes secret containing ESXi SSH private key
	NetworkOverrides  []NICOverride
	isSimpleNetwork   bool
	ImageMetadata     map[string]string
	// TargetMetadata is the merged instance metadata (preserved source tags/attributes
	// plus user-entered custom metadata) applied to the target VM at create time.
	TargetMetadata map[string]string

	// DataOnly indicates no OpenStack VM should be created after disk conversion.
	// When true, port reservation and VM creation are skipped and a DataCopied
	// phase is reported instead of Succeeded.
	DataOnly bool

	// isLDMGuest is set once during ConvertVolumes when the Windows system volume
	// is found on a Dynamic Disk (LDM). ConvertVolumes must know this before it
	// applies image metadata, and performDiskConversion needs the same answer
	// later, so it is resolved once and carried here rather than probed twice.
	isLDMGuest bool
	// ldmProbeVolumeID is the scratch volume created for an LDM guest and attached
	// on the virtio bus at server create. See vm.VMInfo.LDMProbeVolumeID.
	ldmProbeVolumeID string
	// LDMBootStatusWatcher delivers the admin's answer at the
	// WaitingForLDMBootSuccess gate. Separate from PodLabelWatcher so the cutover
	// wait loop and its channel type are untouched.
	LDMBootStatusWatcher chan string
}

const (
	// ldmProbeVolumeSuffix names the scratch volume so it is recognisable as ours.
	ldmProbeVolumeSuffix = "-virtio-probe"
	// diskBusSATA is what an LDM guest must boot on initially: virt-v2v cannot
	// convert it, so it has no virtio storage driver loaded and storahci is in-box.
	diskBusSATA = "sata"
	// diskBusVirtio is the bus an LDM guest is promoted to once the virtio storage
	// driver has actually been installed against the probe device.
	diskBusVirtio = "virtio"
	// imagePropDiskBus is the Nova/Cinder image property naming the disk bus.
	imagePropDiskBus = "hw_disk_bus"
)

// NICOverride defines per-NIC overrides for IP and MAC preservation during migration
type NICOverride struct {
	InterfaceIndex int    `json:"interfaceIndex"`
	PreserveIP     *bool  `json:"preserveIP,omitempty"`
	PreserveMAC    *bool  `json:"preserveMAC,omitempty"`
	UserAssignedIP string `json:"UserAssignedIP,omitempty"`
}

type MigrationTimes struct {
	DataCopyStart  time.Time
	VMCutoverStart time.Time
	VMCutoverEnd   time.Time
}

type PeriodicSyncStates int

const (
	// StateIdle is the initial state, ready to start a new sync cycle
	StateIdle PeriodicSyncStates = iota
	// StateCleaningSnapshots indicates we are cleaning up old snapshots
	StateCleaningSnapshots
	// StateTakingSnapshot indicates we are taking a new migration snapshot
	StateTakingSnapshot
	// StateSyncingCBT indicates we are syncing changed blocks
	StateSyncingCBT
)

// PeriodicSyncContext holds the state machine context for periodic sync operations
type PeriodicSyncContext struct {
	CurrentState   PeriodicSyncStates
	LastError      error
	WarningMessage string // Non-empty indicates sync is in warning state (failed but will retry)
}

func (migobj *Migrate) WaitforCutover() error {
	var zerotime time.Time
	if !migobj.MigrationTimes.VMCutoverStart.Equal(zerotime) && migobj.MigrationTimes.VMCutoverStart.After(time.Now()) {
		migobj.logMessage("Waiting for VM Cutover start time")
		time.Sleep(time.Until(migobj.MigrationTimes.VMCutoverStart))
		migobj.logMessage("VM Cutover start time reached")
	} else {
		if !migobj.MigrationTimes.VMCutoverEnd.Equal(zerotime) && migobj.MigrationTimes.VMCutoverEnd.Before(time.Now()) {
			return errors.New("VM Cutover End time has already passed")
		}
	}
	return nil
}

func (migobj *Migrate) CheckIfAdminCutoverSelected() bool {
	if migobj.Reporter == nil {
		return false
	}
	value, err := migobj.Reporter.GetCutoverLabel()
	if err != nil {
		utils.PrintLog(fmt.Sprintf("Failed to get pod labels: %v", err))
		return false
	}
	// If label is set to no, return true. because that time the admin has initiated cutover
	if value == "no" {
		return true
	}
	return false
}

func (migobj *Migrate) CheckCutoverOptions() (bool, string) {
	if migobj.Reporter == nil {
		return false, ""
	}
	value, err := migobj.Reporter.GetCutoverLabel()
	if err != nil {
		utils.PrintLog(fmt.Sprintf("Failed to get pod labels: %v", err))
		return false, ""
	}
	// If label is set to no or yes, return true. because that time the admin has initiated cutover
	if value != "" {
		return true, value
	}
	return false, ""
}

// gracefulCleanupTimeout bounds the OpenStack/Neutron calls cleanup makes after a
// SIGTERM/SIGINT, once they're on their own fresh context (see gracefulTerminate).
const gracefulCleanupTimeout = 5 * time.Minute

func (migobj *Migrate) gracefulTerminate(ctx context.Context, vminfo vm.VMInfo, cancel context.CancelFunc) {
	gracefulShutdown := make(chan os.Signal, 1)
	// Handle SIGTERM
	signal.Notify(gracefulShutdown, syscall.SIGTERM, syscall.SIGINT)
	<-gracefulShutdown
	migobj.logMessage("Gracefully terminating")
	cancel()
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), gracefulCleanupTimeout)
	migobj.cleanup(cleanupCtx, vminfo, "Migration terminated", nil, nil)
	cleanupCancel()
	os.Exit(0)
}

func (migobj *Migrate) MigrateVM(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Wait until the data copy start time
	var zerotime time.Time
	if !migobj.MigrationTimes.DataCopyStart.Equal(zerotime) && migobj.MigrationTimes.DataCopyStart.After(time.Now()) {
		migobj.logMessage("Waiting for data copy start time")
		time.Sleep(time.Until(migobj.MigrationTimes.DataCopyStart))
		migobj.logMessage("Data copy start time reached")
	}
	fmt.Println("Starting VM Migration with RDM disks : ", migobj.RDMDisks)
	// Get Info about VM
	vminfo, err := migobj.VMops.GetVMInfo(migobj.Ostype, migobj.RDMDisks)

	if err != nil {
		cancel()
		return errors.Wrap(err, "failed to get all info")
	}
	if (len(vminfo.VMDisks) != len(migobj.Volumetypes)) &&
		migobj.StorageCopyMethod != constants.StorageCopyMethod &&
		migobj.StorageCopyMethod != constants.HotAddCopyMethod {
		return errors.Errorf("number of volume types does not match number of disks vm(%d) volume(%d)", len(vminfo.VMDisks), len(migobj.Volumetypes))
	}
	if !migobj.DataOnly && len(vminfo.Mac) != len(migobj.Networknames) {
		return errors.Errorf("number of mac addresses does not match number of network names mac(%d) network(%d)", len(vminfo.Mac), len(migobj.Networknames))
	}
	// Graceful Termination clean-up volumes and snapshots
	go migobj.gracefulTerminate(ctx, vminfo, cancel)

	// Reserve ports for VM
	var networkids, portids, ipaddresses []string
	if !migobj.DataOnly {
		networkids, portids, ipaddresses, err = migobj.ReservePortsForVM(ctx, &vminfo)
		if err != nil {
			return errors.Wrap(err, "failed to reserve ports for VM")
		}
	}
	vcenterSettings, err := k8sutils.GetVjailbreakSettings(ctx, migobj.K8sClient)
	if err != nil {
		return errors.Wrap(err, "failed to get vcenter settings")
	}

	if migobj.StorageCopyMethod == constants.StorageCopyMethod {
		// Initialize storage provider if using StorageAcceleratedCopy migration
		if err := migobj.InitializeStorageProvider(ctx); err != nil {
			if cleanuperror := migobj.cleanup(ctx, vminfo, fmt.Sprintf("failed to initialize storage provider: %s", err), portids, vcenterSettings); cleanuperror != nil {
				return errors.Wrapf(err, "failed to cleanup after storage provider init failure: %s", cleanuperror)
			}
			return errors.Wrap(err, "failed to initialize storage provider")
		}
		defer func() {
			if migobj.StorageProvider != nil {
				migobj.StorageProvider.Disconnect()
			}
		}()
		if err := migobj.ValidateStorageAcceleratedCopyPrerequisites(ctx); err != nil {
			if cleanuperror := migobj.cleanup(ctx, vminfo, fmt.Sprintf("StorageAcceleratedCopy prerequisites validation failed: %s", err), portids, vcenterSettings); cleanuperror != nil {
				return errors.Wrapf(err, "failed to cleanup after prerequisites validation failure: %s", cleanuperror)
			}
			return errors.Wrap(err, "StorageAcceleratedCopy prerequisites validation failed")
		}

		// Perform the copy here.
		if _, err := migobj.StorageAcceleratedCopyCopyDisks(ctx, vminfo); err != nil {
			if cleanuperror := migobj.cleanup(ctx, vminfo, fmt.Sprintf("failed to perform StorageAcceleratedCopy copy: %s", err), portids, vcenterSettings); cleanuperror != nil {
				return errors.Wrapf(err, "failed to cleanup after StorageAcceleratedCopy failure: %s", cleanuperror)
			}
			return errors.Wrap(err, "failed to perform StorageAcceleratedCopy copy")
		}
		// Apply image tags to the volumes we cinder managed.
		if err := migobj.applyImageMetadataForXCOPYVolumes(ctx, vminfo); err != nil {
			if cleanuperror := migobj.cleanup(ctx, vminfo, fmt.Sprintf("failed to apply image metadata to XCOPY volumes: %s", err), portids, vcenterSettings); cleanuperror != nil {
				return errors.Wrapf(err, "failed to cleanup after image metadata failure: %s", cleanuperror)
			}
			return errors.Wrap(err, "failed to apply image metadata to XCOPY volumes")
		}

	} else if migobj.StorageCopyMethod == constants.HotAddCopyMethod {

		vminfo, err = migobj.CreateVolumes(ctx, vminfo)
		if err != nil {
			if cleanuperror := migobj.cleanup(ctx, vminfo, fmt.Sprintf("failed to create volumes for HotAdd migration: %s", err), portids, vcenterSettings); cleanuperror != nil {
				return errors.Wrapf(err, "failed to cleanup after HotAdd volume creation failure: %s", cleanuperror)
			}
			return errors.Wrap(err, "failed to create volumes for HotAdd migration")
		}

		if migobj.MigrationType == "cold" {
			// Cold never needs incremental sync, so it keeps the original one-shot
			// power-off-then-copy path: attach destination volumes up front, then
			// HotAddCopyDisks does a single pass with no CBT involved.
			for idx, vmdisk := range vminfo.VMDisks {
				path, err := migobj.AttachVolume(ctx, vmdisk)
				if err != nil {
					if cleanuperror := migobj.cleanup(ctx, vminfo, fmt.Sprintf("failed to attach volume for HotAdd migration: %s", err), portids, vcenterSettings); cleanuperror != nil {
						return errors.Wrapf(err, "failed to cleanup after HotAdd attach failure: %s", cleanuperror)
					}
					return errors.Wrap(err, "failed to attach volume for HotAdd migration")
				}
				vminfo.VMDisks[idx].Path = path
			}
			if err := migobj.HotAddCopyDisks(ctx, vminfo); err != nil {
				if cleanuperror := migobj.cleanup(ctx, vminfo, fmt.Sprintf("failed to perform HotAdd disk copy: %s", err), portids, vcenterSettings); cleanuperror != nil {
					return errors.Wrapf(err, "failed to cleanup after HotAdd disk copy failure: %s", cleanuperror)
				}
				return errors.Wrap(err, "failed to perform HotAdd disk copy")
			}
		} else {
			// Hot/mock: share the same live-replicate loop normal-hot uses, backed by
			// hotAddNBDServer instead of VDDK -- destination volumes are attached by
			// LiveReplicateDisks itself, same as the normal-hot branch below.
			if err := migobj.EnableCBTWrapper(); err != nil {
				migobj.cleanup(ctx, vminfo, fmt.Sprintf("CBT Failure: %s", err), portids, vcenterSettings)
				return errors.Wrap(err, "CBT Failure")
			}

			session, err := migobj.NewHotAddSession(ctx)
			if err != nil {
				if cleanuperror := migobj.cleanup(ctx, vminfo, fmt.Sprintf("failed to open Hot-Add Proxy VM session: %s", err), portids, vcenterSettings); cleanuperror != nil {
					return errors.Wrapf(err, "failed to cleanup after Hot-Add session failure: %s", cleanuperror)
				}
				return errors.Wrap(err, "failed to open Hot-Add Proxy VM session")
			}
			defer session.Close()

			for range vminfo.VMDisks {
				migobj.Nbdops = append(migobj.Nbdops, NewHotAddNBDServer(migobj, session))
			}

			vminfo, err = migobj.LiveReplicateDisks(ctx, vminfo)
			if err != nil {
				if cleanuperror := migobj.cleanup(ctx, vminfo, fmt.Sprintf("failed to live replicate disks: %s", err), portids, vcenterSettings); cleanuperror != nil {
					return errors.Wrapf(err, "failed to cleanup disks: %s", cleanuperror)
				}
				return errors.Wrap(err, "failed to live replicate disks")
			}
		}

	} else {

		// Create and Add Volumes to Host
		vminfo, err = migobj.CreateVolumes(ctx, vminfo)
		if err != nil {
			if cleanuperror := migobj.cleanup(ctx, vminfo, fmt.Sprintf("failed to create volumes: %s", err), portids, vcenterSettings); cleanuperror != nil {
				return errors.Wrapf(err, "failed to cleanup after volume creation failure: %s", cleanuperror)
			}
			return errors.Wrap(err, "failed to add volumes to host")
		}
		// Enable CBT
		err = migobj.EnableCBTWrapper()
		if err != nil {
			migobj.cleanup(ctx, vminfo, fmt.Sprintf("CBT Failure: %s", err), portids, vcenterSettings)
			return errors.Wrap(err, "CBT Failure")
		}

		// Create NBD servers
		for range vminfo.VMDisks {
			migobj.Nbdops = append(migobj.Nbdops, &nbd.NBDServer{})
		}

		// Live Replicate Disks
		vminfo, err = migobj.LiveReplicateDisks(ctx, vminfo)
		if err != nil {
			if cleanuperror := migobj.cleanup(ctx, vminfo, fmt.Sprintf("failed to live replicate disks: %s", err), portids, vcenterSettings); cleanuperror != nil {
				// combine both errors
				return errors.Wrapf(err, "failed to cleanup disks: %s", cleanuperror)
			}
			return errors.Wrap(err, "failed to live replicate disks")
		}
	}
	// Convert the Boot Disk to raw format
	espDiskIndex, err := migobj.ConvertVolumes(ctx, vminfo)
	if err != nil {
		if !vcenterSettings.CleanupVolumesAfterConvertFailure {
			migobj.logMessage("Cleanup volumes after convert failure is disabled, detaching volumes and cleaning up snapshots")
			detachErr := migobj.DetachAllVolumes(ctx, vminfo)
			if detachErr != nil {
				utils.PrintLog(fmt.Sprintf("Failed to detach all volumes from VM: %s\n", detachErr))
			}

			cleanUpErr := migobj.VMops.CleanUpSnapshots(true)
			if cleanUpErr != nil {
				utils.PrintLog(fmt.Sprintf("Failed to cleanup snapshot of source VM: %s\n", cleanUpErr))
				return errors.Wrap(cleanUpErr, "Failed to cleanup snapshot of source VM")
			}
			return errors.Wrap(err, "failed to convert disks")
		}
		if cleanuperror := migobj.cleanup(ctx, vminfo, fmt.Sprintf("failed to convert disks: %s", err), portids, vcenterSettings); cleanuperror != nil {
			// combine both errors
			return errors.Wrapf(err, "failed to cleanup disks: %s", cleanuperror)
		}
		return errors.Wrap(err, "failed to convert disks")
	}

	if migobj.DataOnly {
		migobj.logMessage("DataOnly mode: disk copy and conversion complete, skipping VM creation")
		if err := migobj.reportStagedVolumeIDs(ctx, vminfo); err != nil {
			migobj.logMessage(fmt.Sprintf("Warning: failed to report staged volume IDs: %v", err))
		}
		return nil
	}

	err = migobj.CreateTargetInstance(ctx, vminfo, networkids, portids, ipaddresses, espDiskIndex)
	if err != nil {
		if serverID, recoveryErr := migobj.verifyVMCreatedDespiteTimeout(ctx, vminfo); recoveryErr == nil {
			utils.PrintLog(fmt.Sprintf("VM created despite CreateTargetInstance error (%v), skipping cleanup", err))
			migobj.logMessage(fmt.Sprintf("VM created successfully: ID: %s", serverID))
		} else {
			if cleanuperror := migobj.cleanup(ctx, vminfo, fmt.Sprintf("failed to create target instance: %s", err), portids, vcenterSettings); cleanuperror != nil {
				// combine both errors
				return errors.Wrapf(err, "failed to cleanup disks: %s", cleanuperror)
			}
			return errors.Wrap(err, "failed to create target instance")
		}
	}

	// An LDM guest was created on an emulated SATA bus with a virtio probe disk
	// attached. Give the admin a chance to confirm from the booted guest that the
	// virtio storage driver installed, then move the root disk onto virtio.
	if migobj.isLDMGuest {
		if err := migobj.waitForLDMBootAndPromote(ctx, vminfo, networkids, portids, ipaddresses, espDiskIndex, vcenterSettings); err != nil {
			return err
		}
	}

	if err := migobj.DisconnectSourceNetworkIfRequested(); err != nil {
		utils.PrintLog(fmt.Sprintf("Warning: Failed to disconnect source VM network interfaces: %v", err))
	}

	return nil
}

// waitForLDMBootAndPromote blocks at the WaitingForLDMBootSuccess gate and acts on
// the admin's answer. It never returns an error for "finish": by the time this runs
// a booted, network-connected VM already exists, and only an optimisation is
// outstanding.
func (migobj *Migrate) waitForLDMBootAndPromote(ctx context.Context, vminfo vm.VMInfo,
	networkids, portids, ipaddresses []string, espDiskIndex int, vcenterSettings *k8sutils.VjailbreakSettings) error {

	// Emitted first and matched by the controller to hold the phase at
	// WaitingForLDMBootSuccess; without it the migration reports Succeeded from the
	// earlier "VM created successfully" event while this gate is still waiting.
	migobj.logMessage(constants.EventMessageWaitingForLDMBootSuccess)

	migobj.logMessage(fmt.Sprintf(
		"VM is running on an emulated SATA bus with a virtio probe disk attached. "+
			"Log in and confirm the virtio storage driver installed, then answer this gate. "+
			"In the guest: Get-PnpDevice -Class SCSIAdapter (expect a Red Hat VirtIO SCSI controller, status OK) "+
			"and sc.exe query viostor (expect STATE: RUNNING). Leave the VM running - answering '%s' "+
			"shuts it down cleanly before recreating it. This gate does not expire; it waits until you answer.",
		constants.LDMBootStatusSuccess))

	answer := migobj.waitForLDMBootStatus(ctx)

	switch answer {
	case constants.LDMBootStatusFailed:
		migobj.logMessage("LDM boot reported as failed by the admin; cleaning up the migrated VM")
		if err := migobj.cleanup(ctx, vminfo, "LDM boot reported as failed", portids, vcenterSettings); err != nil {
			return errors.Wrap(err, "failed to clean up after a failed LDM boot")
		}
		return errors.New("migration failed: admin reported the LDM guest did not boot usably")

	case constants.LDMBootStatusSuccess:
		if err := migobj.promoteLDMGuestToVirtio(ctx, vminfo, networkids, portids, ipaddresses, espDiskIndex); err != nil {
			// Promotion is an optimisation on top of a working VM. Report the
			// failure but do not fail a migration that already succeeded.
			migobj.logMessage(fmt.Sprintf("WARNING: promotion to virtio did not complete (%v); the VM remains on the SATA bus", err))
		}
		return nil

	default:
		migobj.logMessage("Leaving the VM on the emulated SATA bus and completing the migration")
		migobj.detachAndDeleteProbeVolume(ctx, vminfo)

		// Re-emit the terminal event with a fresh timestamp, which is what moves the
		// phase off WaitingForLDMBootSuccess. The original was emitted before the
		// gate opened, and the gate has no time limit: once it ages past the API
		// server's event TTL nothing matches any more, the controller leaves
		// Status.Phase at its stored value, and the UI sits on "waiting" forever
		// even though the migration is finished. The promotion path gets this for
		// free because it recreates the server and emits the event again.
		migobj.logMessage(fmt.Sprintf("%s: left on the emulated SATA bus", constants.EventMessageMigrationSucessful))
		return nil
	}
}

// waitForLDMBootStatus blocks until the admin answers. There is no deadline, for
// the same reason the admin cutover gate has none: the operator may need a
// maintenance window to reach the guest, and expiring the gate on their behalf
// would either strand a VM on SATA or act without them. The wait ends only when
// they answer or the migration context is cancelled.
func (migobj *Migrate) waitForLDMBootStatus(ctx context.Context) string {
	for {
		select {
		case <-ctx.Done():
			migobj.logMessage("Context cancelled while waiting for LDM boot confirmation; leaving the VM on SATA")
			return constants.LDMBootStatusFinish

		case status := <-migobj.LDMBootStatusWatcher:
			switch status {
			case constants.LDMBootStatusSuccess, constants.LDMBootStatusFinish, constants.LDMBootStatusFailed:
				migobj.logMessage(fmt.Sprintf("LDM boot gate answered: %s", status))
				return status
			default:
				// Ignore anything unrecognised rather than resolving the gate on it.
				continue
			}
		}
	}
}

// promoteLDMGuestToVirtio recreates the instance with its root disk on virtio.
// Nova fixes disk_bus in the BDM at server create, so editing volume metadata on
// a running instance does nothing - the server must be rebuilt. Volumes and the
// port survive the delete, so MAC and IP are preserved, and CreateTargetInstance
// is reused so ESP ordering and instance metadata are not reimplemented.
func (migobj *Migrate) promoteLDMGuestToVirtio(ctx context.Context, vminfo vm.VMInfo,
	networkids, portids, ipaddresses []string, espDiskIndex int) error {

	// Resolved from the boot volume's attachment, with no state requirement. The
	// guest is still running at this point - stopServerAndWait below shuts it
	// down - so a lookup that insisted on ACTIVE or on SHUTOFF would be wrong.
	serverID, err := migobj.resolveTargetServerID(ctx, vminfo)
	if err != nil {
		return errors.Wrap(err, "failed to locate the migrated VM before promotion")
	}

	// Matched by the controller to hold the phase for the duration of the rebuild.
	// Must be emitted before any of the work below, and stays the newest matching
	// event until CreateTargetInstance reports success again at the end.
	migobj.logMessage(constants.EventMessagePromotingLDMGuest)

	// Stop before deleting. Deleting a running instance is a hard destroy, which
	// leaves NTFS dirty with no ntfsfix to follow - the conversion phase is long
	// past by now. An ACPI stop lets Windows flush and shut down properly.
	if err := migobj.stopServerAndWait(ctx, serverID); err != nil {
		return err
	}

	migobj.logMessage(fmt.Sprintf("Deleting server %s so it can be recreated on the virtio bus", serverID))
	if err := migobj.Openstackclients.DeleteServer(ctx, serverID); err != nil {
		return errors.Wrap(err, "failed to delete the server before promotion")
	}
	if err := migobj.waitForVolumesAvailable(ctx, vminfo); err != nil {
		return err
	}

	// No offline re-verification here on purpose. The operator has already run
	// "sc.exe query viostor" against the live guest, which reports the driver
	// actually running - a stronger signal than any file check from outside.
	// Re-checking cost an appliance boot and an attach/detach cycle per disk.
	// If the guest does not come up, recreate it with hw_disk_bus=sata.

	// Clear the field so CreateVM leaves the probe off the recreated server, but
	// keep the ID: it is now detached and can finally be deleted.
	probeID := migobj.ldmProbeVolumeID
	migobj.ldmProbeVolumeID = ""

	if err := migobj.recreateLDMGuest(ctx, vminfo, networkids, portids, ipaddresses, espDiskIndex, diskBusVirtio); err != nil {
		return err
	}
	migobj.deleteProbeVolume(ctx, probeID)
	return nil
}

// stopServerAndWait issues an ACPI shutdown and waits for the instance to reach
// SHUTOFF, so the subsequent delete is not a hard destroy.
//
// A timeout is not fatal. The guest may be ignoring ACPI, or may have hung; the
// delete still has to happen for the promotion to proceed, and the cost is a dirty
// filesystem that Windows will chkdsk on the next boot rather than lost data.
func (migobj *Migrate) stopServerAndWait(ctx context.Context, serverID string) error {
	migobj.logMessage(fmt.Sprintf("Stopping server %s before recreating it on the virtio bus", serverID))
	if err := migobj.Openstackclients.StopServer(ctx, serverID); err != nil {
		return errors.Wrap(err, "failed to stop the server before promotion")
	}

	deadline := time.After(constants.LDMShutdownTimeout)
	ticker := time.NewTicker(constants.LDMShutdownPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-deadline:
			migobj.logMessage(fmt.Sprintf(
				"WARNING: server %s did not reach SHUTOFF within %s; continuing with the delete. "+
					"Windows may run chkdsk on the next boot.", serverID, constants.LDMShutdownTimeout))
			return nil

		case <-ticker.C:
			status, err := migobj.Openstackclients.GetServerStatus(ctx, serverID)
			if err != nil {
				migobj.logMessage(fmt.Sprintf("WARNING: could not read the status of server %s: %v", serverID, err))
				continue
			}
			if strings.EqualFold(status, "SHUTOFF") {
				migobj.logMessage(fmt.Sprintf("Server %s is SHUTOFF", serverID))
				return nil
			}
		}
	}
}

// recreateLDMGuest re-applies the boot volume's disk bus and rebuilds the instance.
func (migobj *Migrate) recreateLDMGuest(ctx context.Context, vminfo vm.VMInfo,
	networkids, portids, ipaddresses []string, espDiskIndex int, diskBus string) error {

	bootIdx := -1
	for i := range vminfo.VMDisks {
		if vminfo.VMDisks[i].Boot {
			bootIdx = i
			break
		}
	}
	if bootIdx < 0 || vminfo.VMDisks[bootIdx].OpenstackVol == nil {
		return errors.New("could not identify the boot volume while recreating the VM")
	}
	bootVol := vminfo.VMDisks[bootIdx].OpenstackVol

	metadata := mergeBootVolumeImageMetadata(map[string]string{imagePropDiskBus: diskBus}, migobj.ImageMetadata)
	if err := migobj.Openstackclients.ApplyBootVolumeImageMetadata(ctx, bootVol, metadata); err != nil {
		return errors.Wrap(err, "failed to set the disk bus before recreating the VM")
	}

	if err := migobj.CreateTargetInstance(ctx, vminfo, networkids, portids, ipaddresses, espDiskIndex); err != nil {
		return errors.Wrapf(err, "failed to recreate the VM on the %s bus", diskBus)
	}
	migobj.logMessage(fmt.Sprintf("VM recreated with its root disk on the %s bus", diskBus))
	return nil
}

// waitForVolumesAvailable blocks until every volume has been released by the
// deleted server, so the next create is not rejected for a volume still in use.
func (migobj *Migrate) waitForVolumesAvailable(ctx context.Context, vminfo vm.VMInfo) error {
	for _, disk := range vminfo.VMDisks {
		if disk.OpenstackVol == nil {
			continue
		}
		if err := migobj.Openstackclients.WaitForVolume(ctx, disk.OpenstackVol.ID); err != nil {
			return errors.Wrapf(err, "volume %s was not released after deleting the server", disk.OpenstackVol.ID)
		}
	}
	return nil
}

// deleteProbeVolume removes the scratch volume once it is detached. Best effort:
// a leftover 1GB volume is not worth failing a successful migration over. Callers
// must detach first - the promotion path by deleting the server, the "keep on SATA"
// path via detachAndDeleteProbeVolume.
func (migobj *Migrate) deleteProbeVolume(ctx context.Context, volumeID string) {
	if volumeID == "" {
		return
	}
	// DeleteVolume treats a 404 as success, which is the usual outcome here: the
	// probe carries delete_on_termination, so Nova removes it along with the
	// server the promotion just deleted.
	if err := migobj.Openstackclients.DeleteVolume(ctx, volumeID); err != nil {
		migobj.logMessage(fmt.Sprintf("WARNING: failed to delete the virtio probe volume %s: %v", volumeID, err))
		return
	}
	migobj.logMessage(fmt.Sprintf("Deleted the virtio probe volume %s", volumeID))
}

// detachAndDeleteProbeVolume removes the probe from the still-running instance on
// the "keep on SATA" path, where there is no rebuild to carry it away.
//
// Safe to hot-detach: the probe is a raw, unformatted disk that Windows only needed
// in order to bind viostor once, and the driver stays in the DriverStore whether or
// not the device is present. Best effort throughout - a working VM must not be
// failed over a leftover 1GB volume.
func (migobj *Migrate) detachAndDeleteProbeVolume(ctx context.Context, vminfo vm.VMInfo) {
	probeID := migobj.ldmProbeVolumeID
	if probeID == "" {
		return
	}
	// Cleared up front so a later cleanup() cannot delete it a second time.
	migobj.ldmProbeVolumeID = ""

	// Must be the migrated VM: DetachVolumeFromVM/WaitForVolume resolve the
	// vJailbreak appliance and would detach from the wrong server, then wait out the
	// timeout on an attachment that is not theirs to release.
	serverID, err := migobj.resolveTargetServerID(ctx, vminfo)
	if err != nil {
		migobj.logMessage(fmt.Sprintf(
			"WARNING: could not locate the migrated VM to detach the virtio probe volume %s (%v); it stays "+
				"attached and is removed when the instance is deleted", probeID, err))
		return
	}

	migobj.detachProbeFromServer(ctx, serverID, probeID)
}

// detachProbeFromServer is the OpenStack half of detachAndDeleteProbeVolume, split
// out so it can be unit tested - resolveTargetServerID goes through
// GetCurrentInstanceUUID, which reads cluster state and cannot be mocked.
func (migobj *Migrate) detachProbeFromServer(ctx context.Context, serverID, probeID string) {
	migobj.logMessage(fmt.Sprintf("Detaching the virtio probe volume %s from server %s", probeID, serverID))
	if err := migobj.Openstackclients.DetachVolumeFromServer(ctx, serverID, probeID); err != nil {
		migobj.logMessage(fmt.Sprintf(
			"WARNING: could not detach the virtio probe volume %s (%v); it stays attached and is removed "+
				"when the instance is deleted", probeID, err))
		return
	}

	// Bounded: a guest that never loaded viostor may not acknowledge the unplug, and
	// that is usually why "keep on SATA" was chosen in the first place.
	if err := migobj.Openstackclients.WaitForVolumeDetached(ctx, probeID, constants.LDMProbeDetachTimeout); err != nil {
		migobj.logMessage(fmt.Sprintf(
			"WARNING: the virtio probe volume %s did not detach (%v); it stays attached and is removed "+
				"when the instance is deleted", probeID, err))
		return
	}

	migobj.deleteProbeVolume(ctx, probeID)
}

func (migobj *Migrate) cleanup(ctx context.Context, vminfo vm.VMInfo, message string, portids []string, vcenterSettings *k8sutils.VjailbreakSettings) error {
	migobj.logMessage(fmt.Sprintf("%s. Trying to perform cleanup", message))

	// Stop whatever each disk's NBD provider still has open -- for Hot-Add this is
	// what kills qemu-nbd and detaches the frozen VMDK from the Proxy VM, so a
	// mid-flight termination doesn't leave those behind. No-op for cold Hot-Add
	// (Nbdops is never populated there) and for normal/VDDK when nothing was started.
	for idx, nbdop := range migobj.Nbdops {
		if nbdop == nil {
			continue
		}
		if err := nbdop.StopNBDServer(); err != nil {
			utils.PrintLog(fmt.Sprintf("Warning: failed to stop NBD server for disk %d during cleanup: %v", idx, err))
		}
	}

	err := migobj.DetachAllVolumesWithCleanup(ctx, vminfo)
	if err != nil {
		utils.PrintLog(fmt.Sprintf("Failed to detach all volumes from VM: %s\n", err))
	}
	err = migobj.DeleteAllVolumes(ctx, vminfo)
	if err != nil {
		utils.PrintLog(fmt.Sprintf("Failed to delete all volumes from host: %s\n", err))
	}

	// The probe is tracked outside vminfo.VMDisks, so DeleteAllVolumes cannot see
	// it. It carries delete_on_termination, but that only helps once a server
	// exists to terminate - a failure anywhere between createLDMProbeVolume and
	// the boot gate would otherwise orphan it.
	if probeID := migobj.ldmProbeVolumeID; probeID != "" {
		migobj.ldmProbeVolumeID = ""
		migobj.deleteProbeVolume(ctx, probeID)
	}

	err = migobj.VMops.CleanUpSnapshots(true)
	if err != nil {
		utils.PrintLog(fmt.Sprintf("Failed to cleanup snapshot of source VM: %s\n", err))
		return errors.Wrap(err, fmt.Sprintf("Failed to cleanup snapshot of source VM: %s\n", err))
	}

	// Delete ports if cleanup is enabled
	if vcenterSettings != nil && vcenterSettings.CleanupPortsAfterMigrationFailure && len(portids) > 0 {
		migobj.logMessage("Cleanup ports after migration failure is enabled, deleting ports")
		if portCleanupErr := migobj.DeleteAllPorts(ctx, portids); portCleanupErr != nil {
			utils.PrintLog(fmt.Sprintf("Failed to delete ports: %s\n", portCleanupErr))
		}
	} else if vcenterSettings != nil && !vcenterSettings.CleanupPortsAfterMigrationFailure {
		migobj.logMessage("Cleanup ports after migration failure is disabled, ports will not be deleted")
	}

	return nil
}
