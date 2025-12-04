// Copyright © 2024 The vjailbreak authors

package migrate

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage"
	esxissh "github.com/platform9/vjailbreak/v2v-helper/esxi-ssh"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/k8sutils"
	"github.com/platform9/vjailbreak/v2v-helper/vcenter"
	"github.com/platform9/vjailbreak/v2v-helper/vm"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
)

// VAAICopyDisks performs VAAI XCOPY-based disk copy for all VM disks
// This offloads the copy operation to the storage array, which is much faster than NBD
func (migobj *Migrate) VAAICopyDisks(ctx context.Context, vminfo vm.VMInfo) ([]storage.Volume, error) {
	migobj.logMessage("Starting VAAI XCOPY-based disk copy")

	// Validate prerequisites
	if migobj.StorageProvider == nil {
		return []storage.Volume{}, fmt.Errorf("storage provider not initialized for VAAI copy")
	}

	// Get ESXi host information
	host, err := migobj.getESXiHost(ctx)
	if err != nil {
		return []storage.Volume{}, errors.Wrap(err, "failed to get ESXi host")
	}

	hostIP, err := migobj.getHostIPAddress(ctx, host)
	if err != nil {
		return []storage.Volume{}, errors.Wrap(err, "failed to get ESXi host IP")
	}

	migobj.logMessage(fmt.Sprintf("ESXi host: %s (IP: %s)", host.Name(), hostIP))

	// Connect to ESXi via SSH
	esxiClient := esxissh.NewClient()
	defer esxiClient.Disconnect()

	if err := esxiClient.Connect(ctx, hostIP, "root", migobj.ESXiSSHPrivateKey); err != nil {
		return []storage.Volume{}, errors.Wrap(err, "failed to connect to ESXi via SSH")
	}

	// Test the connection
	if err := esxiClient.TestConnection(); err != nil {
		return []storage.Volume{}, errors.Wrap(err, "failed to test ESXi connection")
	}

	migobj.logMessage("Connected to ESXi host via SSH")

	volumes := []storage.Volume{}

	// Process each disk
	for idx, vmdisk := range vminfo.VMDisks {
		migobj.logMessage(fmt.Sprintf("Processing disk %d/%d: %s", idx+1, len(vminfo.VMDisks), vmdisk.Name))

		// Perform VAAI copy for this disk
		clonedVolume, err := migobj.copyDiskViaVAAI(ctx, esxiClient, vminfo.VMDisks[idx], hostIP)
		if err != nil {
			return []storage.Volume{}, errors.Wrapf(err, "failed to copy disk %s via VAAI", vmdisk.Name)
		}

		// Attach the Cinder volume to get the device path
		devicePath, err := migobj.AttachVolume(vmdisk)
		if err != nil {
			return []storage.Volume{}, errors.Wrapf(err, "failed to attach volume for disk %s", vmdisk.Name)
		}
		vminfo.VMDisks[idx].Path = devicePath
		volumes = append(volumes, clonedVolume)

		migobj.logMessage(fmt.Sprintf("Successfully copied disk %s via VAAI XCOPY", vmdisk.Name))
	}

	migobj.logMessage("VAAI XCOPY-based disk copy completed successfully")
	return volumes, nil
}

// copyDiskViaVAAI copies a single disk using VAAI XCOPY
func (migobj *Migrate) copyDiskViaVAAI(ctx context.Context, esxiClient *esxissh.Client, vmDisk vm.VMDisk, hostIP string) (storage.Volume, error) {
	startTime := time.Now()

	// Step 1: Get source VMDK NAA (the backing storage device)
	migobj.logMessage(fmt.Sprintf("Resolving source VMDK backing device: %s", vmDisk.Path))
	sourceNAA, err := esxiClient.GetVMDKBackingNAA(vmDisk.Path)
	if err != nil {
		return storage.Volume{}, errors.Wrapf(err, "failed to get source VMDK backing NAA for %s", vmDisk.Path)
	}
	migobj.logMessage(fmt.Sprintf("Source VMDK backed by NAA: %s", sourceNAA))

	// Step 2: Initialize storage provider
	migobj.InitializeStorageProvider(ctx)

	// Step 3: Get ESXi host IQN for volume mapping
	hostIQN, err := esxiClient.GetHostIQN()
	if err != nil {
		return storage.Volume{}, errors.Wrap(err, "failed to get ESXi host IQN")
	}
	migobj.logMessage(fmt.Sprintf("ESXi host IQN: %s", hostIQN))

	// Step 4: Map host IQN to initiator group
	initiatorGroup := fmt.Sprintf("vjailbreak-xcopy")
	migobj.logMessage(fmt.Sprintf("Creating/updating initiator group: %s", initiatorGroup))

	mappingContext, err := migobj.StorageProvider.CreateOrUpdateInitiatorGroup(initiatorGroup, []string{hostIQN})
	if err != nil {
		return storage.Volume{}, errors.Wrapf(err, "failed to create initiator group %s", initiatorGroup)
	}

	// Step 5: Create target volume
	// Use vmDisk.Size (VMware disk size in bytes) - Pure API expects size in bytes
	diskSizeBytes := vmDisk.Size
	// Ensure size is a multiple of 512 (sector alignment)
	if diskSizeBytes%512 != 0 {
		diskSizeBytes = ((diskSizeBytes / 512) + 1) * 512
	}
	migobj.logMessage(fmt.Sprintf("Creating target volume %s with size %d bytes (%d GB)", vmDisk.Name, diskSizeBytes, diskSizeBytes/(1024*1024*1024)))
	targetVolume, err := migobj.StorageProvider.CreateVolume(vmDisk.Name, diskSizeBytes)
	if err != nil {
		return storage.Volume{}, errors.Wrapf(err, "failed to create target volume %s", vmDisk.Name)
	}

	// Step 6: Cinder manage the volume
	migobj.logMessage(fmt.Sprintf("Cinder managing the volume %s", vmDisk.Name))
	cinderVolumeId, err := migobj.manageVolumeToCinder(ctx, targetVolume.Name, vmDisk)
	if err != nil {
		return storage.Volume{}, errors.Wrapf(err, "failed to Cinder manage volume %s", vmDisk.Name)
	}
	// Step 7: Map target volume to ESXi host
	migobj.logMessage(fmt.Sprintf("Mapping target volume to ESXi host"))
	targetVol := storage.Volume{
		Name: targetVolume.Name,
		NAA:  targetVolume.NAA,
		Size: targetVolume.Size,
		OpenstackVol: storage.OpenstackVolume{
			ID: cinderVolumeId,
		},
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

	// TODO: Add retry logic
	time.Sleep(40 * time.Second)

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
		return storage.Volume{}, fmt.Errorf("target device %s not visible on ESXi after rescan", targetDevicePath)
	}
	migobj.logMessage(fmt.Sprintf("Target device is visible: %s", targetDevicePath))

	// Step 8: Perform VAAI XCOPY clone directly to raw device (RDM format)
	// This clones directly to the raw device without needing a datastore
	// Command format: vmkfstools -i <source> -d rdm:<target_device> <dummy_vmdk_path>
	migobj.logMessage(fmt.Sprintf("Starting VAAI XCOPY clone: %s -> %s (RDM)", vmDisk.Path, targetDevicePath))

	cloneStart := time.Now()
	task, err := esxiClient.StartVmkfstoolsRDMClone(vmDisk.Path, targetDevicePath)
	if err != nil {
		return storage.Volume{}, errors.Wrapf(err, "failed to start VAAI RDM clone for disk %s", vmDisk.Name)
	}

	// Step 9: Monitor clone progress
	tracker := esxissh.NewCloneTracker(esxiClient, task, vmDisk.Path, targetDevicePath)
	tracker.SetPollInterval(2 * time.Second)

	err = tracker.WaitForCompletion()
	if err != nil {
		return storage.Volume{}, errors.Wrapf(err, "VAAI RDM clone failed for disk %s", vmDisk.Name)
	}

	cloneDuration := time.Since(cloneStart)
	totalDuration := time.Since(startTime)

	migobj.logMessage(fmt.Sprintf("VAAI XCOPY completed in %s (total: %s) for disk %s",
		cloneDuration.Round(time.Second), totalDuration.Round(time.Second), vmDisk.Name))

	return targetVolume, nil
}

// ValidateVAAIPrerequisites validates that all prerequisites for VAAI copy are met
func (migobj *Migrate) ValidateVAAIPrerequisites(ctx context.Context) error {
	migobj.logMessage("Validating VAAI prerequisites")

	// Check storage provider
	if migobj.StorageProvider == nil {
		return fmt.Errorf("storage provider not initialized")
	}

	// Load ESXi SSH key from secret if not already loaded
	if len(migobj.ESXiSSHPrivateKey) == 0 {
		if err := migobj.LoadESXiSSHKey(ctx); err != nil {
			return errors.Wrap(err, "failed to load ESXi SSH private key")
		}
	}

	// Validate storage provider credentials
	if err := migobj.StorageProvider.ValidateCredentials(ctx); err != nil {
		return errors.Wrap(err, "storage provider credential validation failed")
	}

	migobj.logMessage("VAAI prerequisites validated successfully")
	return nil
}

// getESXiHost returns the ESXi host object for the VM
func (migobj *Migrate) getESXiHost(ctx context.Context) (*object.HostSystem, error) {
	vm := migobj.VMops.GetVMObj()
	var vmProps mo.VirtualMachine
	err := vm.Properties(ctx, vm.Reference(), []string{"runtime.host"}, &vmProps)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get VM properties")
	}

	if vmProps.Runtime.Host == nil {
		return nil, fmt.Errorf("VM has no host")
	}

	// Access VCenterClient through the VMOps interface
	// VMops is of type vm.VMOperations interface, we need the concrete type
	// to access GetVCenterClient() which is not part of the interface
	type vcenterClientGetter interface {
		GetVCenterClient() *vcenter.VCenterClient
	}

	vcGetter, ok := migobj.VMops.(vcenterClientGetter)
	if !ok {
		return nil, fmt.Errorf("VMops does not implement GetVCenterClient()")
	}

	host := object.NewHostSystem(vcGetter.GetVCenterClient().VCClient, *vmProps.Runtime.Host)
	return host, nil
}

// getHostIPAddress returns the management IP address of an ESXi host
func (migobj *Migrate) getHostIPAddress(ctx context.Context, host *object.HostSystem) (string, error) {
	var hostProps mo.HostSystem
	err := host.Properties(ctx, host.Reference(), []string{"config.network"}, &hostProps)
	if err != nil {
		return "", errors.Wrap(err, "failed to get host properties")
	}

	if hostProps.Config == nil || hostProps.Config.Network == nil {
		return "", fmt.Errorf("host has no network configuration")
	}

	// Get management IP from vNICs
	for _, vnic := range hostProps.Config.Network.Vnic {
		if vnic.Spec.Ip != nil && vnic.Spec.Ip.IpAddress != "" {
			// Prefer management network
			if strings.Contains(strings.ToLower(vnic.Device), "vmk0") {
				return vnic.Spec.Ip.IpAddress, nil
			}
		}
	}

	// Fallback to first available IP
	if len(hostProps.Config.Network.Vnic) > 0 && hostProps.Config.Network.Vnic[0].Spec.Ip != nil {
		return hostProps.Config.Network.Vnic[0].Spec.Ip.IpAddress, nil
	}

	return "", fmt.Errorf("no management IP found for host")
}

// manageVolumeToCinder manages an existing storage array volume into Cinder
func (migobj *Migrate) manageVolumeToCinder(ctx context.Context, volumeName string, vmDisk vm.VMDisk) (string, error) {
	migobj.logMessage(fmt.Sprintf("Managing volume %s into Cinder", volumeName))
	// Get array creds mapping
	arrayCredsMapping, err := k8sutils.GetArrayCredsMapping(ctx, migobj.K8sClient, migobj.ArrayCredsMapping)
	if err != nil {
		return "", errors.Wrap(err, "failed to get array creds mapping")
	}

	dataStoreName := vmDisk.Datastore
	arrayCredsName := ""
	for _, mapping := range arrayCredsMapping.Spec.Mappings {
		if mapping.Source == dataStoreName {
			arrayCredsName = mapping.Target
		}
	}

	if arrayCredsName == "" {
		return "", fmt.Errorf("no array creds found for datastore %s", dataStoreName)
	}

	arrayCreds, err := k8sutils.GetArrayCreds(ctx, migobj.K8sClient, arrayCredsName)
	if err != nil {
		return "", errors.Wrap(err, "failed to get array creds")
	}

	volumeType := arrayCreds.Spec.OpenStackMapping.VolumeType
	cinderBackend := arrayCreds.Spec.OpenStackMapping.CinderBackendPool

	// Create the manage volume request
	// The reference format depends on the storage backend
	// For Pure Storage, it's typically the volume name
	volumeRef := map[string]interface{}{
		"source-name": volumeName,
	}

	// Call OpenStack to manage the volume
	// This uses the Cinder manage API to import the existing volume
	managedVolume, err := migobj.Openstackclients.ManageExistingVolume(
		volumeName,
		volumeRef,
		cinderBackend,
		volumeType,
	)
	if err != nil {
		return "", errors.Wrapf(err, "failed to manage volume %s in Cinder", volumeName)
	}

	migobj.logMessage(fmt.Sprintf("Volume %s managed successfully with Cinder ID: %s", volumeName, managedVolume.ID))

	return managedVolume.ID, nil
}

// getCinderBackendForDatastore returns the Cinder backend host for a given datastore
func (migobj *Migrate) getCinderBackendForDatastore(datastoreName string) string {
	// TODO: This should look up the ArrayCredsMapping to find the correct backend
	// For now, return a placeholder that needs to be configured
	// Format is typically: hostname@backend#pool

	// This mapping should come from the ArrayCredsMapping CR
	// which maps datastores to ArrayCreds, which has the OpenStackMapping
	migobj.logMessage(fmt.Sprintf("Looking up Cinder backend for datastore: %s", datastoreName))

	// Placeholder - this needs to be implemented to read from ArrayCredsMapping
	// The actual value should come from ArrayCreds.Spec.OpenStackMapping.CinderBackendPool
	return fmt.Sprintf("hostname@%s#pool", migobj.VendorType)
}
