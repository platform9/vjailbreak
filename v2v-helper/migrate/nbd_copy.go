// Copyright © 2024 The vjailbreak authors

package migrate

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	"github.com/platform9/vjailbreak/v2v-helper/nbd"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/k8sutils"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/utils"
	"github.com/platform9/vjailbreak/v2v-helper/vm"
	"github.com/vmware/govmomi/vim25/types"
)

// String returns a human-readable name for the state
func (s PeriodicSyncStates) String() string {
	switch s {
	case StateIdle:
		return "Idle"
	case StateCleaningSnapshots:
		return "CleaningSnapshots"
	case StateTakingSnapshot:
		return "TakingSnapshot"
	case StateSyncingCBT:
		return "SyncingCBT"
	default:
		return "Unknown"
	}
}

// This function enables CBT on the VM if it is not enabled and takes a snapshot for initializing CBT
func (migobj *Migrate) EnableCBTWrapper() error {
	vmops := migobj.VMops

	// If it is cold migration we do not need to enable CBT.
	if migobj.MigrationType == "cold" {
		migobj.logMessage("Cold migration selected: skipping CBT check and enablement")
		return nil
	}

	// CBT requires virtual hardware >= 7. Fail with a clear error if
	// changeTrackingEnabled is unavailable, instead of panicking.
	hwVersion, VersionErr := vmops.GetHardwareVersion()
	if VersionErr != nil {
		migobj.logMessage(fmt.Sprintf("Could not determine hardware version, Trying to enable CBT: %s", VersionErr))
	}
	migobj.logMessage(fmt.Sprintf("Hardware version detected: %s", hwVersion))

	if hwVersion > 0 && hwVersion < vm.MinCBTHardwareVersion {
		return fmt.Errorf(
			"changed block tracking (CBT) is not supported on virtual hardware version %d "+
				"(requires version %d or newer); please use cold migration for this VM",
			hwVersion, vm.MinCBTHardwareVersion)
	}

	cbt, err := vmops.IsCBTEnabled()
	if err != nil {
		return errors.Wrap(err, "failed to check if CBT is enabled")
	}
	migobj.logMessage(fmt.Sprintf("CBT Enabled: %t", cbt))

	if !cbt {
		// 7.5. Enable CBT
		migobj.logMessage("CBT is not enabled. Enabling CBT")
		err = vmops.EnableCBT()
		if err != nil {
			return errors.Wrap(err, "failed to enable CBT")
		}
		_, err := vmops.IsCBTEnabled()
		if err != nil {
			return errors.Wrap(err, "failed to check if CBT is enabled")
		}
		migobj.logMessage("Creating temporary snapshot of the source VM")
		err = vmops.TakeSnapshot("tmp-snap")
		if err != nil {
			return errors.Wrap(err, "failed to take snapshot of source VM")
		}
		utils.PrintLog("Snapshot created successfully")
		err = vmops.DeleteSnapshot("tmp-snap")
		if err != nil {
			return errors.Wrap(err, "failed to delete snapshot of source VM")
		}
		utils.PrintLog("Snapshot deleted successfully")
		migobj.logMessage("CBT enabled successfully")
	}
	return nil
}

func (migobj *Migrate) SyncCBT(ctx context.Context, vminfo vm.VMInfo) error {
	migobj.logMessage("Starting Periodic sync process")
	defer migobj.logMessage("Periodic sync process completed")
	vmops := migobj.VMops
	nbdops := migobj.Nbdops
	envURL := migobj.URL
	envUserName := migobj.UserName
	envPassword := migobj.Password
	thumbprint := migobj.Thumbprint
	migration_snapshot, err := vmops.GetSnapshot(constants.MigrationSnapshotName)
	if err != nil {
		return errors.Wrap(err, "failed to get snapshot")
	}

	var changedAreas types.DiskChangeInfo

	for idx := range vminfo.VMDisks {
		changedAreas, err = vmops.CustomQueryChangedDiskAreas(vminfo.VMDisks[idx].ChangeID, migration_snapshot, vminfo.VMDisks[idx].Disk, 0)
		if err != nil {
			return errors.Wrap(err, "failed to get changed disk areas")
		}

		if len(changedAreas.ChangedArea) == 0 {
			migobj.logMessage(fmt.Sprintf("Periodic Sync: Disk %d: No changed blocks found. Skipping copy", idx))
		} else {
			migobj.logMessage(fmt.Sprintf("Periodic Sync: Disk %d: Blocks have Changed.", idx))

			// Before starting NBD server, update disk info with new snapshot details
			// We have marked block copy as false, in order to not update changeID.
			// This should now update the snapname and snapBackingDisk with the new snapshot details and copy correctly.
			err = vmops.UpdateDiskInfo(&vminfo, vminfo.VMDisks[idx], false)
			if err != nil {
				return errors.Wrap(err, "failed to update disk info")
			}

			utils.PrintLog("Restarting NBD server")
			err = nbdops[idx].StopNBDServer()
			if err != nil {
				return errors.Wrap(err, "failed to stop NBD server")
			}

			err = nbdops[idx].StartNBDServer(vmops.GetVMObj(), envURL, envUserName, envPassword, thumbprint, vminfo.VMDisks[idx].Snapname, vminfo.VMDisks[idx].SnapBackingDisk, migobj.EventReporter)
			if err != nil {
				return errors.Wrap(err, "failed to start NBD server")
			}
			// sleep for 2 seconds to allow the NBD server to start
			time.Sleep(2 * time.Second)

			// 11. Copy Changed Blocks over
			changedBlockCopySuccess := true
			startTime := time.Now()
			migobj.logMessage(fmt.Sprintf("Periodic Sync: Starting incremental block copy for disk %d at %s", idx, startTime))
			err = nbdops[idx].CopyChangedBlocks(ctx, changedAreas, vminfo.VMDisks[idx].Path, vminfo.VMDisks[idx].OpenstackVol.Encrypted)
			if err != nil {
				migobj.logMessage(fmt.Sprintf("Periodic Sync: Failed to copy changed blocks for disk %d: %v", idx, err))
				select {
				case <-ctx.Done():
					err = vmops.CleanUpSnapshots(false)
					changedBlockCopySuccess = false
					if err != nil {
						return errors.Wrap(err, "failed to cleanup snapshot of source VM")
					}
				default:
					return errors.Wrap(err, "failed to copy changed blocks")
				}
			}

			duration := time.Since(startTime)

			migobj.logMessage(fmt.Sprintf("Periodic Sync: Incremental block copy for disk %d completed in %s", idx, duration))

			err = vmops.UpdateDiskInfo(&vminfo, vminfo.VMDisks[idx], changedBlockCopySuccess)
			if err != nil {
				return errors.Wrap(err, "failed to update disk info")
			}
			if !changedBlockCopySuccess {
				migobj.logMessage(fmt.Sprintf("Periodic Sync: Failed to copy changed blocks: %s", err))
				migobj.logMessage(fmt.Sprintf("Periodic Sync: Since full copy has completed, Retrying copy of changed blocks for disk: %d", idx))
			}
		}
	}
	// Cleanup the snapshot taken for incremental copy
	return nil
}

func (migobj *Migrate) getSyncEnabled() bool {
	var enabled bool
	enabled = false
	migrationParams, err := utils.GetMigrationParams(context.Background(), migobj.K8sClient)
	if err != nil {
		return enabled
	}
	if migrationParams.PeriodicSyncEnabled {
		enabled = true
	}
	return enabled
}

func (migobj *Migrate) getSyncDuration() time.Duration {
	const defaultInterval = "1h"

	migobj.logMessage("Periodic Sync: Setting up sync interval")

	migrationParams, err := utils.GetMigrationParams(context.Background(), migobj.K8sClient)
	if err != nil {
		migobj.logMessage(fmt.Sprintf("WARNING: Failed to get migration params: %v, using default interval (%s)",
			err, defaultInterval))
	}
	// Get sync interval settings
	interval := migrationParams.PeriodicSyncInterval
	if interval == "" {
		vjailbreakSettings, err := k8sutils.GetVjailbreakSettings(context.Background(), migobj.K8sClient)
		if err != nil {
			migobj.logMessage(fmt.Sprintf("WARNING: Failed to get vjailbreak settings: %v, using default interval (%s)",
				err, defaultInterval))
		}
		interval = vjailbreakSettings.PeriodicSyncInterval
		if interval == "" {
			interval = defaultInterval
		}
	}
	// Calculate wait time based on unit
	waitTime, err := time.ParseDuration(interval)
	if err != nil {
		migobj.logMessage(fmt.Sprintf("WARNING: Failed to parse interval %s, using default interval (%s)", interval, defaultInterval))
		interval = defaultInterval
		waitTime, _ = time.ParseDuration(interval)
	} else if waitTime < 5*time.Minute {
		migobj.logMessage(fmt.Sprintf("WARNING: Interval %s is less than 5 minutes, falling back to 5m", interval))
		waitTime = 5 * time.Minute
	}
	return waitTime
}

func (migobj *Migrate) WaitforAdminCutover(ctx context.Context, vminfo vm.VMInfo) error {
	vmops := migobj.VMops
	maxRetries, capInterval := utils.GetRetryLimits()
	migobj.logMessage(constants.EventMessageWaitingForAdminCutOver)

	// Initialize state machine context
	syncCtx := &PeriodicSyncContext{
		CurrentState: StateIdle,
	}

	elapsed := time.Duration(0)

	for {
		syncEnabled := migobj.getSyncEnabled()
		var syncInterval time.Duration
		if syncEnabled {
			syncInterval = migobj.getSyncDuration()
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case label := <-migobj.PodLabelWatcher:
			if label == "yes" && syncCtx.CurrentState == StateIdle {
				migobj.logMessage("Admin cutover triggered")
				return nil
			}
		default:
			if !syncEnabled {
				// Small sleep to prevent busy-waiting when sync is disabled
				time.Sleep(1 * time.Second)
				continue
			}

			// Calculate wait time based on elapsed time
			if elapsed >= syncInterval {
				migobj.logMessage("Periodic Sync: Previous sync took longer than interval, starting next cycle immediately")
				elapsed = syncInterval
			}

			waitTime := syncInterval - elapsed
			stateInfo := syncCtx.CurrentState.String()
			if syncCtx.WarningMessage != "" {
				stateInfo = fmt.Sprintf("%s (WARNING: %s)", stateInfo, syncCtx.WarningMessage)
			}
			migobj.logMessage(fmt.Sprintf("Periodic Sync: Waiting %s before next sync cycle (state: %s)", waitTime, stateInfo))

			select {
			case <-ctx.Done():
				return ctx.Err()
			case label := <-migobj.PodLabelWatcher:
				if label == "yes" {
					migobj.logMessage("Admin cutover triggered during wait")
					return nil
				}
			case <-time.After(waitTime):
				// wait completed → proceed to sync
			}

			// Execute state machine - always start fresh from Idle each cycle
			migobj.logMessage(fmt.Sprintf("Periodic Sync: Starting sync cycle (interval: %s)", syncInterval))
			start := time.Now()

			// Reset state to start fresh each cycle
			syncCtx.CurrentState = StateCleaningSnapshots

			if err := migobj.Vcclient.EnsureSessionActive(ctx); err != nil {
				syncCtx.LastError = err
				syncCtx.WarningMessage = fmt.Sprintf("vCenter session refresh failed: %v. Will retry on next sync interval.", err)
				syncCtx.CurrentState = StateIdle
				migobj.logMessage(fmt.Sprintf("Periodic Sync: WARNING - %s", syncCtx.WarningMessage))
				elapsed = time.Since(start)
				continue
			}

			// State: CleaningSnapshots
			err := utils.DoRetryWithExponentialBackoff(ctx, func() error {
				return vmops.CleanUpSnapshots(false)
			}, maxRetries, capInterval)
			if err != nil {
				syncCtx.LastError = err
				syncCtx.WarningMessage = fmt.Sprintf("Snapshot cleanup failed after %d retries: %v. Will retry on next sync interval.", maxRetries, err)
				syncCtx.CurrentState = StateIdle
				migobj.logMessage(fmt.Sprintf("Periodic Sync: WARNING - %s", syncCtx.WarningMessage))
				elapsed = time.Since(start)
				continue
			}

			// State: TakingSnapshot
			syncCtx.CurrentState = StateTakingSnapshot
			err = utils.DoRetryWithExponentialBackoff(ctx, func() error {
				return vmops.TakeSnapshot(constants.MigrationSnapshotName)
			}, maxRetries, capInterval)
			if err != nil {
				syncCtx.LastError = err
				syncCtx.WarningMessage = fmt.Sprintf("Snapshot creation '%s' failed after %d retries: %v. Will retry on next sync interval.", constants.MigrationSnapshotName, maxRetries, err)
				syncCtx.CurrentState = StateIdle
				migobj.logMessage(fmt.Sprintf("Periodic Sync: WARNING - %s", syncCtx.WarningMessage))
				elapsed = time.Since(start)
				continue
			}

			// State: SyncingCBT
			syncCtx.CurrentState = StateSyncingCBT
			err = utils.DoRetryWithExponentialBackoff(ctx, func() error {
				return migobj.SyncCBT(ctx, vminfo)
			}, maxRetries, capInterval)
			if err != nil {
				syncCtx.LastError = err
				syncCtx.WarningMessage = fmt.Sprintf("CBT sync failed after %d retries: %v. Will retry on next sync interval.", maxRetries, err)
				syncCtx.CurrentState = StateIdle
				migobj.logMessage(fmt.Sprintf("Periodic Sync: WARNING - %s", syncCtx.WarningMessage))
				elapsed = time.Since(start)
				continue
			}

			// Sync completed successfully - clear warning state
			syncCtx.CurrentState = StateIdle
			syncCtx.WarningMessage = ""
			syncCtx.LastError = nil
			migobj.logMessage("Periodic Sync: Sync cycle completed successfully")

			elapsed = time.Since(start)
		}
	}
}

func (migobj *Migrate) LiveReplicateDisks(ctx context.Context, vminfo vm.VMInfo) (vm.VMInfo, error) {
	vmops := migobj.VMops
	nbdops := migobj.Nbdops
	envURL := migobj.URL
	envUserName := migobj.UserName
	envPassword := migobj.Password
	thumbprint := migobj.Thumbprint

	// Get migration parameters to check if user acknowledged network conflict risk
	migrationParams, err := utils.GetMigrationParams(ctx, migobj.K8sClient)
	if err != nil {
		migobj.logMessage(fmt.Sprintf("WARNING: Failed to get migration params: %v, continuing with migration", err))
	} else {
		if migobj.MigrationType == "mock" {

			if migrationParams.AcknowledgeNetworkConflictRisk {
				migobj.logMessage("User acknowledged the risk involved")
			} else {
				migobj.logMessage("User did not acknowledge the risk involved")
			}
		}
	}

	cutoverLabelPresent, cutoverLabelValue := migobj.CheckCutoverOptions()
	// if the cutover immediately is selected with cold migration type then the migration will happen like cold migration
	var currentCutoverOption string
	if migobj.MigrationType == "cold" {
		if cutoverLabelValue != "" {
			if cutoverLabelValue == "yes" {

				currentCutoverOption = "Immediately After Data Copy"
			} else if cutoverLabelValue == "no" {
				currentCutoverOption = "Admin Initiated Cutover"
			}
			migobj.logMessage(fmt.Sprintf("Migration Type : %s | Cutover Option %s", migobj.MigrationType, currentCutoverOption))
		}
		if err := vmops.VMPowerOff(); err != nil {
			return vminfo, errors.Wrap(err, "failed to power off VM")
		}
		// Verify VM is actually powered off
		if err := utils.DoRetryWithExponentialBackoff(ctx, func() error {
			currState, stateErr := vmops.GetVMObj().PowerState(ctx)
			if stateErr != nil {
				return stateErr
			}
			if currState != types.VirtualMachinePowerStatePoweredOff {
				return fmt.Errorf("VM power-off command completed but VM is still in state: %s", currState)
			}
			return nil
		}, constants.MaxPowerOffRetryLimit, constants.PowerOffRetryCap); err != nil {
			return vminfo, errors.Wrap(err, "failed to verify VM power state after power off")
		}
	}

	// clean up snapshots
	utils.PrintLog("Cleaning up snapshots before copy")
	err = vmops.CleanUpSnapshots(false)
	if err != nil {
		return vminfo, errors.Wrap(err, "failed to clean up snapshots: %s, please delete manually before starting again")
	}

	utils.PrintLog("Starting NBD server")
	err = vmops.TakeSnapshot(constants.MigrationSnapshotName)
	if err != nil {
		return vminfo, errors.Wrap(err, "failed to take snapshot of source VM")
	}

	// Cold migrations never use CBT, so the snapshot disks have no change IDs
	// (especially on legacy hardware version < 7). Only require change IDs for
	// non-cold migrations, which rely on them for incremental copies.
	err = vmops.UpdateDisksInfo(&vminfo, migobj.MigrationType != "cold")
	if err != nil {
		return vminfo, errors.Wrap(err, "failed to update disk info")
	}

	for idx, vmdisk := range vminfo.VMDisks {
		migobj.logMessage(fmt.Sprintf("Copying disk %d, Completed: 0%%", idx))
		err = nbdops[idx].StartNBDServer(vmops.GetVMObj(), envURL, envUserName, envPassword, thumbprint, vmdisk.Snapname, vmdisk.SnapBackingDisk, migobj.EventReporter)
		if err != nil {
			return vminfo, errors.Wrap(err, "failed to start NBD server")
		}
	}

	// DEBUG fault-injection: re-read the migration ConfigMap right before the
	// real copy starts (rather than reusing the migrationParams fetched above)
	// so an operator can `kubectl edit configmap` the DEBUG_STALE_NFC_SESSIONS
	// key on this running migration - any time after triggering it, up until
	// this point - and have it take effect. A positive value opens that many
	// extra stale NFC sessions against this migration's first disk to
	// intentionally pressure/exceed vCenter's NFC session cap from within a
	// single migration, for reproducing session-cap/"faulted session" failures
	// (e.g. VJAILB-244) against a lab vCenter. Only ever set it against a
	// lab/test vCenter - see nbd.StartDebugStaleNFCSessions.
	if len(vminfo.VMDisks) > 0 {
		debugParams, debugErr := utils.GetMigrationParams(ctx, migobj.K8sClient)
		if debugErr != nil {
			migobj.logMessage(fmt.Sprintf("DEBUG: failed to re-read migration ConfigMap for stale NFC session count, skipping: %v", debugErr))
		} else if debugParams.DebugStaleNFCSessions > 0 {
			staleNFCSessions := nbd.StartDebugStaleNFCSessions(debugParams.DebugStaleNFCSessions, vmops.GetVMObj(), envURL, envUserName, envPassword, thumbprint,
				vminfo.VMDisks[0].Snapname, vminfo.VMDisks[0].SnapBackingDisk)
			if len(staleNFCSessions) > 0 {
				defer nbd.StopDebugStaleNFCSessions(staleNFCSessions)
			}
		}
	}

	// sleep for 2 seconds to allow the NBD server to start
	time.Sleep(2 * time.Second)
	final := false

	for idx, vmdisk := range vminfo.VMDisks {
		vminfo.VMDisks[idx].Path, err = migobj.AttachVolume(ctx, vmdisk)
		if err != nil {
			return vminfo, errors.Wrap(err, "failed to attach volume")
		}
	}

	// Validate disk mapping before starting copy
	if err := migobj.validateDiskMapping(vminfo); err != nil {
		return vminfo, errors.Wrap(err, "disk mapping validation failed")
	}

	// Log the disk copy plan for debugging
	migobj.logDiskCopyPlan(vminfo)

	vcenterSettings, err := k8sutils.GetVjailbreakSettings(ctx, migobj.K8sClient)
	if err != nil {
		return vminfo, errors.Wrap(err, "failed to get vcenter settings")
	}
	utils.PrintLog(fmt.Sprintf("Fetched vjailbreak settings for Changed Blocks Copy Iteration Threshold: %d", vcenterSettings.ChangedBlocksCopyIterationThreshold))

	// Check if migration has admin cutover if so don't copy any more changed blocks
	adminInitiatedCutover := cutoverLabelPresent && (cutoverLabelValue == "no")
	incrementalCopyCount := 0
	for {
		// If its the first copy, copy the entire disk
		if incrementalCopyCount == 0 {
			for idx := range vminfo.VMDisks {
				startTime := time.Now()
				disk := vminfo.VMDisks[idx]

				migobj.logMessage(fmt.Sprintf("Starting full disk copy [%d/%d]: %s (DeviceKey=%d)",
					idx+1, len(vminfo.VMDisks), disk.Name, disk.Disk.Key))
				migobj.logMessage(fmt.Sprintf("  Source: %s", extractFileName(disk.SnapBackingDisk)))
				migobj.logMessage(fmt.Sprintf("  Target: %s (Volume ID: %s)", disk.Path, disk.OpenstackVol.ID))

				err = nbdops[idx].CopyDisk(ctx, disk.Path, idx, disk.OpenstackVol.Encrypted)
				if err != nil {
					return vminfo, errors.Wrap(err, fmt.Sprintf("failed to copy disk %s (DeviceKey=%d)", disk.Name, disk.Disk.Key))
				}
				duration := time.Since(startTime)
				if migobj.MigrationType == "cold" {
					migobj.logMessage(fmt.Sprintf("✓ Disk %d (%s) copied successfully in %s", idx, disk.Name, duration))
				} else {
					migobj.logMessage(fmt.Sprintf("✓ Disk %d (%s) copied successfully in %s, copying changed blocks now", idx, disk.Name, duration))
				}
			}

			if adminInitiatedCutover {
				utils.PrintLog("Admin initiated cutover detected, skipping changed blocks copy")
				if err := migobj.WaitforAdminCutover(ctx, vminfo); err != nil {
					return vminfo, errors.Wrap(err, "failed to start VM Cutover")
				}
				if err := migobj.Vcclient.EnsureSessionActive(ctx); err != nil {
					return vminfo, errors.Wrap(err, "failed to refresh vCenter session post-cutover")
				}
				if migobj.MigrationType == "mock" {
					utils.PrintLog("Mock migration detected, skipping VM power off")
				} else {
					utils.PrintLog("Shutting down source VM and performing final copy")
					err = vmops.VMPowerOff()
					if err != nil {
						return vminfo, errors.Wrap(err, "failed to power off VM")
					}
					// Verify VM is actually powered off
					if err := utils.DoRetryWithExponentialBackoff(ctx, func() error {
						currState, stateErr := vmops.GetVMObj().PowerState(ctx)
						if stateErr != nil {
							return stateErr
						}
						if currState != types.VirtualMachinePowerStatePoweredOff {
							return fmt.Errorf("VM power-off command completed but VM is still in state: %s", currState)
						}
						return nil
					}, constants.MaxPowerOffRetryLimit, constants.PowerOffRetryCap); err != nil {
						return vminfo, errors.Wrap(err, "failed to verify VM power state after power off")
					}
				}
			}
			if err := migobj.WaitforCutover(); err != nil {
				return vminfo, errors.Wrap(err, "failed to start VM Cutover")
			}
		} else {
			migration_snapshot, err := vmops.GetSnapshot(constants.MigrationSnapshotName)
			if err != nil {
				return vminfo, errors.Wrap(err, "failed to get snapshot")
			}

			var changedAreas types.DiskChangeInfo
			done := true

			for idx := range vminfo.VMDisks {
				err := vmops.UpdateDiskInfo(&vminfo, vminfo.VMDisks[idx], false)
				if err != nil {
					return vminfo, errors.Wrap(err, "failed to update disk info")
				}

				changedAreas, err = vmops.CustomQueryChangedDiskAreas(vminfo.VMDisks[idx].ChangeID, migration_snapshot, vminfo.VMDisks[idx].Disk, 0)
				if err != nil {
					return vminfo, errors.Wrap(err, "failed to get changed disk areas")
				}

				if len(changedAreas.ChangedArea) == 0 {
					if migobj.MigrationType != "cold" {
						migobj.logMessage(fmt.Sprintf("Disk %d: No changed blocks found. Skipping copy", idx))
					}
				} else {
					migobj.logMessage(fmt.Sprintf("Disk %d: Blocks have Changed.", idx))

					utils.PrintLog("Restarting NBD server")
					err = nbdops[idx].StopNBDServer()
					if err != nil {
						return vminfo, errors.Wrap(err, "failed to stop NBD server")
					}

					err = nbdops[idx].StartNBDServer(vmops.GetVMObj(), envURL, envUserName, envPassword, thumbprint, vminfo.VMDisks[idx].Snapname, vminfo.VMDisks[idx].SnapBackingDisk, migobj.EventReporter)
					if err != nil {
						return vminfo, errors.Wrap(err, "failed to start NBD server")
					}
					// sleep for 2 seconds to allow the NBD server to start
					time.Sleep(2 * time.Second)

					// 11. Copy Changed Blocks over
					done = false
					changedBlockCopySuccess := true
					migobj.logMessage("Copying changed blocks")

					startTime := time.Now()
					migobj.logMessage(fmt.Sprintf("Starting incremental block copy for disk %d at %s", idx, startTime))

					// Use exponential backoff for retry logic (3 retries, 30 second cap)
					copyErr := utils.DoRetryWithExponentialBackoff(ctx, func() error {
						return nbdops[idx].CopyChangedBlocks(ctx, changedAreas, vminfo.VMDisks[idx].Path, vminfo.VMDisks[idx].OpenstackVol.Encrypted)
					}, 3, 30*time.Second)

					if copyErr != nil {
						changedBlockCopySuccess = false
						// Fail the migration if copy fails after 3 retries
						return vminfo, errors.Wrap(copyErr, fmt.Sprintf("failed to copy changed blocks for disk %d after 3 attempts", idx))
					}

					duration := time.Since(startTime)

					migobj.logMessage(fmt.Sprintf("Incremental block copy for disk %d completed in %s", idx, duration))

					err = vmops.UpdateDiskInfo(&vminfo, vminfo.VMDisks[idx], changedBlockCopySuccess)
					if err != nil {
						return vminfo, errors.Wrap(err, "failed to update disk info")
					}
					migobj.logMessage(fmt.Sprintf("Finished copying and syncing changed blocks for disk %d in %s [Progress: %d/20]", idx, duration, incrementalCopyCount))
				}
			}
			if final {
				break
			}
			if done || incrementalCopyCount > vcenterSettings.ChangedBlocksCopyIterationThreshold {
				if migobj.MigrationType == "mock" {
					utils.PrintLog("Mock migration detected, skipping VM power off")
				} else {
					utils.PrintLog("Shutting down source VM and performing final copy")
					err = vmops.VMPowerOff()
					if err != nil {
						return vminfo, errors.Wrap(err, "failed to power off VM")
					}
					// Verify VM is actually powered off
					if err := utils.DoRetryWithExponentialBackoff(ctx, func() error {
						currState, stateErr := vmops.GetVMObj().PowerState(ctx)
						if stateErr != nil {
							return stateErr
						}
						if currState != types.VirtualMachinePowerStatePoweredOff {
							return fmt.Errorf("VM power-off command completed but VM is still in state: %s", currState)
						}
						return nil
					}, constants.MaxPowerOffRetryLimit, constants.PowerOffRetryCap); err != nil {
						return vminfo, errors.Wrap(err, "failed to verify VM power state after power off")
					}
				}
				final = true
			}
		}

		// Update old change id to the new base change id value
		// Only do this after you have gone through all disks with old change id.
		// If you dont, only your first disk will have the updated changes

		err = vmops.CleanUpSnapshots(false)
		if err != nil {
			return vminfo, errors.Wrap(err, "failed to cleanup snapshot of source VM")
		}

		// If migration type is cold, check power state and exit the live replicate function.
		if migobj.MigrationType == "cold" {
			// Verify VM is actually powered off
			if err := utils.DoRetryWithExponentialBackoff(ctx, func() error {
				currState, stateErr := vmops.GetVMObj().PowerState(ctx)
				if stateErr != nil {
					return stateErr
				}
				if currState != types.VirtualMachinePowerStatePoweredOff {
					return fmt.Errorf("Cold migration requires the VM to remain powered off. However, the VM was found in the '%s' state after power-off and snapshot copy. Aborting migration to avoid data inconsistency.", currState)
				}
				return nil
			}, constants.MaxPowerOffRetryLimit, constants.PowerOffRetryCap); err != nil {
				return vminfo, errors.Wrap(err, "failed to verify VM power state after power off")
			}
			break
		}

		err = vmops.TakeSnapshot(constants.MigrationSnapshotName)
		if err != nil {
			return vminfo, errors.Wrap(err, "failed to take snapshot of source VM")
		}

		incrementalCopyCount += 1

	}

	err = migobj.DetachAllVolumes(ctx, vminfo)
	if err != nil {
		return vminfo, errors.Wrap(err, "Failed to detach all volumes from VM")
	}

	utils.PrintLog("Stopping NBD server")
	for _, nbdserver := range nbdops {
		err = nbdserver.StopNBDServer()
		if err != nil {
			return vminfo, errors.Wrap(err, "failed to stop NBD server")
		}
	}

	utils.PrintLog("Deleting migration snapshot")
	err = vmops.CleanUpSnapshots(true)
	if err != nil {
		migobj.logMessage(fmt.Sprintf(`Failed to cleanup snapshot of source VM: %s, since copy is completed,
        continuing with the migration`, err))
	}
	return vminfo, nil
}
