// Copyright © 2024 The vjailbreak authors

package migrate

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	"github.com/platform9/vjailbreak/v2v-helper/nbd"
	"github.com/platform9/vjailbreak/v2v-helper/openstack"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/k8sutils"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/utils"
	"github.com/platform9/vjailbreak/v2v-helper/reporter"
	"github.com/platform9/vjailbreak/v2v-helper/vcenter"
	"github.com/platform9/vjailbreak/v2v-helper/vm"
	"github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage"
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
	Reporter                reporter.ReporterOps
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
}

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

func (migobj *Migrate) gracefulTerminate(ctx context.Context, vminfo vm.VMInfo, cancel context.CancelFunc) {
	gracefulShutdown := make(chan os.Signal, 1)
	// Handle SIGTERM
	signal.Notify(gracefulShutdown, syscall.SIGTERM, syscall.SIGINT)
	<-gracefulShutdown
	migobj.logMessage("Gracefully terminating")
	cancel()
	migobj.cleanup(ctx, vminfo, "Migration terminated", nil, nil)
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

	if err := migobj.DisconnectSourceNetworkIfRequested(); err != nil {
		utils.PrintLog(fmt.Sprintf("Warning: Failed to disconnect source VM network interfaces: %v", err))
	}

	return nil
}

func (migobj *Migrate) cleanup(ctx context.Context, vminfo vm.VMInfo, message string, portids []string, vcenterSettings *k8sutils.VjailbreakSettings) error {
	migobj.logMessage(fmt.Sprintf("%s. Trying to perform cleanup", message))
	err := migobj.DetachAllVolumesWithCleanup(ctx, vminfo)
	if err != nil {
		utils.PrintLog(fmt.Sprintf("Failed to detach all volumes from VM: %s\n", err))
	}
	err = migobj.DeleteAllVolumes(ctx, vminfo)
	if err != nil {
		utils.PrintLog(fmt.Sprintf("Failed to delete all volumes from host: %s\n", err))
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
