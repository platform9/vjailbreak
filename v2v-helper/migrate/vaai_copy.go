// Copyright © 2024 The vjailbreak authors

package migrate

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage"
	"github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage/pure"
	esxissh "github.com/platform9/vjailbreak/v2v-helper/esxi-ssh"
	"github.com/platform9/vjailbreak/v2v-helper/vm"
)

// VAAICopyDisks performs VAAI XCOPY-based disk copy for all VM disks
// This offloads the copy operation to the storage array, which is much faster than NBD
func (migobj *Migrate) VAAICopyDisks(ctx context.Context, vminfo vm.VMInfo) (vm.VMInfo, error) {
	migobj.logMessage("Starting VAAI XCOPY-based disk copy")

	// Validate prerequisites
	if migobj.StorageProvider == nil {
		return vminfo, fmt.Errorf("storage provider not initialized for VAAI copy")
	}

	// Get ESXi host information
	host, err := migobj.getESXiHost(ctx)
	if err != nil {
		return vminfo, errors.Wrap(err, "failed to get ESXi host")
	}

	hostIP, err := migobj.getHostIPAddress(ctx, host)
	if err != nil {
		return vminfo, errors.Wrap(err, "failed to get ESXi host IP")
	}

	migobj.logMessage(fmt.Sprintf("ESXi host: %s (IP: %s)", host.Name(), hostIP))

	// Connect to ESXi via SSH
	esxiClient := esxissh.NewClient()
	defer esxiClient.Disconnect()

	if err := esxiClient.Connect(ctx, hostIP, "root", migobj.ESXiSSHPrivateKey); err != nil {
		return vminfo, errors.Wrap(err, "failed to connect to ESXi via SSH")
	}

	migobj.logMessage("Connected to ESXi host via SSH")

	// Process each disk
	for idx, vmdisk := range vminfo.VMDisks {
		migobj.logMessage(fmt.Sprintf("Processing disk %d/%d: %s", idx+1, len(vminfo.VMDisks), vmdisk.Name))

		// Attach the Cinder volume to get the device path
		devicePath, err := migobj.AttachVolume(vmdisk)
		if err != nil {
			return vminfo, errors.Wrapf(err, "failed to attach volume for disk %s", vmdisk.Name)
		}
		vminfo.VMDisks[idx].Path = devicePath

		// Perform VAAI copy for this disk
		if err := migobj.copyDiskViaVAAI(ctx, esxiClient, vminfo.VMDisks[idx], hostIP); err != nil {
			return vminfo, errors.Wrapf(err, "failed to copy disk %s via VAAI", vmdisk.Name)
		}

		migobj.logMessage(fmt.Sprintf("Successfully copied disk %s via VAAI XCOPY", vmdisk.Name))
	}

	// Detach all volumes after copy
	if err := migobj.DetachAllVolumes(vminfo); err != nil {
		return vminfo, errors.Wrap(err, "failed to detach volumes after VAAI copy")
	}

	migobj.logMessage("VAAI XCOPY-based disk copy completed successfully")
	return vminfo, nil
}

// copyDiskViaVAAI copies a single disk using VAAI XCOPY
func (migobj *Migrate) copyDiskViaVAAI(ctx context.Context, esxiClient *esxissh.Client, vmDisk vm.VMDisk, hostIP string) error {
	startTime := time.Now()

	// Step 1: Get source VMDK NAA (the backing storage device)
	migobj.logMessage(fmt.Sprintf("Resolving source VMDK backing device: %s", vmDisk.Path))
	sourceNAA, err := esxiClient.GetVMDKBackingNAA(vmDisk.Path)
	if err != nil {
		return errors.Wrapf(err, "failed to get source VMDK backing NAA for %s", vmDisk.Path)
	}
	migobj.logMessage(fmt.Sprintf("Source VMDK backed by NAA: %s", sourceNAA))

	// Step 2: Get source volume information from storage array
	// Get vendor-specific provider based on VendorType
	var sourceVolume storage.Volume
	var targetVolume storage.Volume

	switch migobj.VendorType {
	case "pure":
		pureProvider, ok := migobj.StorageProvider.(*pure.PureStorageProvider)
		if !ok {
			return fmt.Errorf("storage provider type mismatch: expected Pure provider for vendor type 'pure'")
		}

		accessInfo := storage.StorageAccessInfo{
			Hostname:            migobj.ArrayHost,
			Username:            migobj.ArrayUser,
			Password:            migobj.ArrayPassword,
			SkipSSLVerification: migobj.ArrayInsecure,
			VendorType:          migobj.VendorType,
		}

		// Connect to storage array
		if err := pureProvider.Connect(ctx, accessInfo); err != nil {
			return errors.Wrap(err, "failed to connect to storage array")
		}
		defer pureProvider.Disconnect()

		vol, err := pureProvider.GetVolumeFromNAA(sourceNAA)
		if err != nil {
			return errors.Wrapf(err, "failed to get source volume from NAA %s", sourceNAA)
		}
		sourceVolume = vol
		migobj.logMessage(fmt.Sprintf("Source volume: %s (Serial: %s)", sourceVolume.Name, sourceVolume.SerialNumber))

		// Step 3: Get target Cinder volume NAA
		cinderVolumeID := vmDisk.OpenstackVol.ID
		migobj.logMessage(fmt.Sprintf("Resolving target Cinder volume: %s", cinderVolumeID))

		lun, err := pureProvider.ResolveCinderVolumeToLUN(cinderVolumeID)
		if err != nil {
			return errors.Wrapf(err, "failed to resolve Cinder volume %s to NAA", cinderVolumeID)
		}
		targetVolume = lun

	default:
		return fmt.Errorf("VAAI copy not supported for vendor type '%s'. Currently supported: pure", migobj.VendorType)
	}

	migobj.logMessage(fmt.Sprintf("Target Cinder volume NAA: %s", targetVolume.NAA))

	// Step 4: Get ESXi host IQN for volume mapping
	hostIQN, err := esxiClient.GetHostIQN()
	if err != nil {
		return errors.Wrap(err, "failed to get ESXi host IQN")
	}
	migobj.logMessage(fmt.Sprintf("ESXi host IQN: %s", hostIQN))

	// Step 5: Map target volume to ESXi host
	initiatorGroup := fmt.Sprintf("vjailbreak-vaai-%s", vmDisk.OpenstackVol.ID[:8])
	migobj.logMessage(fmt.Sprintf("Creating/updating initiator group: %s", initiatorGroup))

	mappingContext, err := migobj.StorageProvider.CreateOrUpdateInitiatorGroup(initiatorGroup, []string{hostIQN})
	if err != nil {
		return errors.Wrapf(err, "failed to create initiator group %s", initiatorGroup)
	}

	// Map target volume to ESXi host
	migobj.logMessage(fmt.Sprintf("Mapping target volume to ESXi host"))
	targetVol := storage.Volume{
		Name: targetVolume.Name,
		NAA:  targetVolume.NAA,
		Size: targetVolume.Size,
	}
	_, err = migobj.StorageProvider.MapVolumeToGroup(initiatorGroup, targetVol, mappingContext)
	if err != nil {
		// Volume might already be mapped, log warning and continue
		migobj.logMessage(fmt.Sprintf("Warning: Failed to map target volume (may already be mapped): %v", err))
	}

	// Cleanup function to unmap after copy
	defer func() {
		migobj.logMessage("Cleaning up volume mappings")
		if err := migobj.StorageProvider.UnmapVolumeFromGroup(initiatorGroup, targetVol, mappingContext); err != nil {
			migobj.logMessage(fmt.Sprintf("Warning: Failed to unmap target volume: %v", err))
		}
	}()

	// Step 6: Rescan ESXi storage to detect the target volume
	migobj.logMessage("Rescanning ESXi storage to detect target volume")
	if err := esxiClient.RescanStorage(); err != nil {
		migobj.logMessage(fmt.Sprintf("Warning: Storage rescan failed: %v", err))
	}

	// Wait for device to be visible
	time.Sleep(5 * time.Second)

	// Step 7: Verify target device is visible on ESXi
	targetDevicePath := fmt.Sprintf("/vmfs/devices/disks/naa.%s", targetVolume.NAA)
	migobj.logMessage(fmt.Sprintf("Verifying target device is visible: %s", targetDevicePath))

	checkCmd := fmt.Sprintf("ls %s 2>&1", targetDevicePath)
	checkOutput, checkErr := esxiClient.ExecuteCommand(checkCmd)
	if checkErr != nil || !strings.Contains(checkOutput, targetDevicePath) {
		migobj.logMessage(fmt.Sprintf("Warning: Device not immediately visible: %v, output: %s", checkErr, checkOutput))
		// List available devices for debugging
		allDisks, _ := esxiClient.ExecuteCommand("ls /vmfs/devices/disks/ | grep naa | head -20")
		migobj.logMessage(fmt.Sprintf("Available NAA devices (first 20): %s", allDisks))
		return fmt.Errorf("target device %s not visible on ESXi after rescan", targetDevicePath)
	}
	migobj.logMessage(fmt.Sprintf("Target device is visible: %s", targetDevicePath))

	// Step 8: Perform VAAI XCOPY clone directly to raw device (RDM format)
	// This clones directly to the raw device without needing a datastore
	// Command format: vmkfstools -i <source> -d rdm:<target_device> <dummy_vmdk_path>
	migobj.logMessage(fmt.Sprintf("Starting VAAI XCOPY clone: %s -> %s (RDM)", vmDisk.Path, targetDevicePath))

	cloneStart := time.Now()
	task, err := esxiClient.StartVmkfstoolsRDMClone(vmDisk.Path, targetDevicePath)
	if err != nil {
		return errors.Wrapf(err, "failed to start VAAI RDM clone for disk %s", vmDisk.Name)
	}

	// Step 9: Monitor clone progress
	tracker := esxissh.NewCloneTracker(esxiClient, task, vmDisk.Path, targetDevicePath)
	tracker.SetPollInterval(2 * time.Second)

	err = tracker.WaitForCompletion()
	if err != nil {
		return errors.Wrapf(err, "VAAI RDM clone failed for disk %s", vmDisk.Name)
	}

	cloneDuration := time.Since(cloneStart)
	totalDuration := time.Since(startTime)

	migobj.logMessage(fmt.Sprintf("VAAI XCOPY completed in %s (total: %s) for disk %s",
		cloneDuration.Round(time.Second), totalDuration.Round(time.Second), vmDisk.Name))

	return nil
}

// VAAILiveReplicateDisks performs live replication using VAAI XCOPY
// This is an alternative to the NBD-based LiveReplicateDisks
func (migobj *Migrate) VAAILiveReplicateDisks(ctx context.Context, vminfo vm.VMInfo) (vm.VMInfo, error) {
	vmops := migobj.VMops

	migobj.logMessage("Starting VAAI-based live disk replication")

	// For cold migration, power off VM first
	if migobj.MigrationType == "cold" && !migobj.CheckIfAdminCutoverSelected() {
		if err := vmops.VMPowerOff(); err != nil {
			return vminfo, errors.Wrap(err, "failed to power off VM")
		}
	}

	// Clean up snapshots before starting
	migobj.logMessage("Cleaning up snapshots before copy")
	err := vmops.CleanUpSnapshots(false)
	if err != nil {
		return vminfo, errors.Wrap(err, "failed to clean up snapshots")
	}

	// For VAAI, we don't need snapshots or NBD servers
	// We'll do a direct copy from the source datastore to target volumes

	// Perform VAAI copy
	vminfo, err = migobj.VAAICopyDisks(ctx, vminfo)
	if err != nil {
		if cleanuperror := migobj.cleanup(vminfo, fmt.Sprintf("failed to copy disks via VAAI: %s", err)); cleanuperror != nil {
			return vminfo, errors.Wrapf(err, "failed to cleanup disks: %s", cleanuperror)
		}
		return vminfo, errors.Wrap(err, "failed to copy disks via VAAI")
	}

	migobj.logMessage("VAAI-based live disk replication completed successfully")
	return vminfo, nil
}

// ValidateVAAIPrerequisites validates that all prerequisites for VAAI copy are met
func (migobj *Migrate) ValidateVAAIPrerequisites(ctx context.Context) error {
	migobj.logMessage("Validating VAAI prerequisites")

	// Check storage provider
	if migobj.StorageProvider == nil {
		return fmt.Errorf("storage provider not initialized")
	}

	// Check ESXi SSH key
	if len(migobj.ESXiSSHPrivateKey) == 0 {
		return fmt.Errorf("ESXi SSH private key not provided")
	}

	// Validate storage provider credentials
	if err := migobj.StorageProvider.ValidateCredentials(ctx); err != nil {
		return errors.Wrap(err, "storage provider credential validation failed")
	}

	migobj.logMessage("VAAI prerequisites validated successfully")
	return nil
}
