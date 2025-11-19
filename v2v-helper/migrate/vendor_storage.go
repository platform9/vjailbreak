// Copyright © 2024 The vjailbreak authors

package migrate

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumes"
	"github.com/pkg/errors"
	"github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage"
	esxissh "github.com/platform9/vjailbreak/v2v-helper/esxi-ssh"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/k8sutils"
	"github.com/platform9/vjailbreak/v2v-helper/vcenter"
	"github.com/platform9/vjailbreak/v2v-helper/vm"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
)

const (
	xcopyInitiatorGroup = "vjailbreak-xcopy-group"
	rescanRetries       = 5
	rescanSleepInterval = 5 * time.Second
)

// VendorBasedStorageCopy performs vendor-based storage copy using storage array APIs
func (migobj *Migrate) VendorBasedStorageCopy(ctx context.Context, vminfo vm.VMInfo) error {
	migobj.logMessage("Starting vendor-based storage copy")

	// Store VM name for volume naming
	vmName := vminfo.Name

	if migobj.StorageProvider == nil {
		return fmt.Errorf("storage provider not initialized")
	}

	// Get ESXi host information
	host, err := migobj.getESXiHost(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get ESXi host")
	}

	// Get ESXi host IP
	hostIP, err := migobj.getHostIPAddress(ctx, host)
	if err != nil {
		return errors.Wrap(err, "failed to get host IP address")
	}

	migobj.logMessage(fmt.Sprintf("ESXi host: %s (IP: %s)", host.Name(), hostIP))

	// Get HBA UIDs from ESXi host
	hbaUIDs, err := migobj.getESXiHBAUIDs(ctx, hostIP)
	if err != nil {
		return errors.Wrap(err, "failed to get ESXi HBA UIDs")
	}

	if len(hbaUIDs) == 0 {
		return fmt.Errorf("no valid HBA UIDs found for host %s", host.Name())
	}

	migobj.logMessage(fmt.Sprintf("Found HBA UIDs: %+v", hbaUIDs))

	// Process each disk
	for i, disk := range vminfo.VMDisks {
		migobj.logMessage(fmt.Sprintf("Processing disk %d/%d: %s", i+1, len(vminfo.VMDisks), disk.Name))

		if err := migobj.copyDiskVendorBased(ctx, vmName, disk, hbaUIDs, hostIP); err != nil {
			return errors.Wrapf(err, "failed to copy disk %s", disk.Name)
		}

		migobj.logMessage(fmt.Sprintf("Successfully copied disk %s", disk.Name))
	}

	migobj.logMessage("Vendor-based storage copy completed successfully")
	return nil
}

// copyDiskVendorBased copies a single disk using vendor-based storage copy
func (migobj *Migrate) copyDiskVendorBased(ctx context.Context, vmName string, vmDisk vm.VMDisk, hbaUIDs []string, hostIP string) error {
	// Step 1: Create volume directly on the storage array
	volumeName := fmt.Sprintf("vjailbreak-%s-%s", vmName, vmDisk.Name)
	volumeSize := vmDisk.Size // Size in bytes

	migobj.logMessage(fmt.Sprintf("Creating volume %s on storage array (size: %d bytes)", volumeName, volumeSize))

	err := migobj.StorageProvider.CreateVolume(volumeName, volumeSize)
	if err != nil {
		return errors.Wrapf(err, "failed to create volume %s on storage array", volumeName)
	}

	migobj.logMessage(fmt.Sprintf("Volume %s created successfully on storage array", volumeName))

	// Get the LUN information for the newly created volume
	lunInfo, err := migobj.StorageProvider.GetVolumeInfo(volumeName)
	if err != nil {
		return errors.Wrapf(err, "failed to get volume info for %s", volumeName)
	}

	// Convert VolumeInfo to Volume for mapping operations
	lun := storage.Volume{
		Name: lunInfo.Name,
		Size: lunInfo.Size,
		NAA:  lunInfo.NAA,
	}

	migobj.logMessage(fmt.Sprintf("Volume info: Name=%s, NAA=%s", lun.Name, lun.NAA))

	// Create or update initiator group with ESXi HBA UIDs
	migobj.logMessage(fmt.Sprintf("Creating/updating initiator group %s with HBA UIDs", xcopyInitiatorGroup))
	mappingContext, err := migobj.StorageProvider.CreateOrUpdateInitiatorGroup(xcopyInitiatorGroup, hbaUIDs)
	if err != nil {
		return errors.Wrapf(err, "failed to create initiator group %s", xcopyInitiatorGroup)
	}

	// Get current mapped groups before we modify anything
	originalGroups, err := migobj.StorageProvider.GetMappedGroups(lun, mappingContext)
	if err != nil {
		return errors.Wrapf(err, "failed to get current mapped groups for LUN %s", lun.Name)
	}
	migobj.logMessage(fmt.Sprintf("LUN %s is currently mapped to groups: %+v", lun.Name, originalGroups))

	// Map the LUN to the xcopy initiator group
	migobj.logMessage(fmt.Sprintf("Mapping LUN %s to initiator group %s", lun.Name, xcopyInitiatorGroup))
	mappedLUN, err := migobj.StorageProvider.MapVolumeToGroup(xcopyInitiatorGroup, lun, mappingContext)
	if err != nil {
		return errors.Wrapf(err, "failed to map LUN %s to group %s", lun.Name, xcopyInitiatorGroup)
	}

	// Cleanup function to unmap and restore original mappings
	defer func() {
		migobj.logMessage("Cleaning up - unmapping LUN and restoring original mappings")

		// Unmap from xcopy group
		if err := migobj.StorageProvider.UnmapVolumeFromGroup(xcopyInitiatorGroup, mappedLUN, mappingContext); err != nil {
			migobj.logMessage(fmt.Sprintf("WARNING: Failed to unmap LUN %s: %v", lun.Name, err))
		}

		// Map back to original groups
		for _, group := range originalGroups {
			migobj.logMessage(fmt.Sprintf("Mapping LUN %s back to original group %s", lun.Name, group))
			if _, err := migobj.StorageProvider.MapVolumeToGroup(group, mappedLUN, mappingContext); err != nil {
				migobj.logMessage(fmt.Sprintf("WARNING: Failed to map LUN back to group %s: %v", group, err))
			}
		}

		// Rescan to clean dead devices
		if err := migobj.rescanESXiStorage(ctx, hostIP, true); err != nil {
			migobj.logMessage(fmt.Sprintf("WARNING: Failed to rescan for dead devices: %v", err))
		}
	}()

	// Rescan ESXi storage to discover the newly mapped LUN
	targetDevice := fmt.Sprintf("/vmfs/devices/disks/naa.%s", strings.ToLower(mappedLUN.NAA))
	migobj.logMessage(fmt.Sprintf("Rescanning ESXi storage to discover device: %s", targetDevice))

	if err := migobj.rescanAndWaitForDevice(ctx, hostIP, targetDevice); err != nil {
		return errors.Wrapf(err, "failed to discover device %s on ESXi", targetDevice)
	}

	migobj.logMessage(fmt.Sprintf("Device %s is now accessible on ESXi", targetDevice))

	// Step 2: Perform the actual copy using vmkfstools
	sourcePath := vmDisk.Path
	migobj.logMessage(fmt.Sprintf("Copying from %s to %s", sourcePath, targetDevice))

	if err := migobj.performESXiCopy(ctx, hostIP, sourcePath, targetDevice); err != nil {
		return errors.Wrapf(err, "failed to copy data from %s to %s", sourcePath, targetDevice)
	}

	migobj.logMessage(fmt.Sprintf("Successfully copied data to %s", targetDevice))

	// Step 3: Manage the volume into Cinder
	migobj.logMessage(fmt.Sprintf("Managing volume %s into Cinder", volumeName))

	cinderVolumeID, err := migobj.manageVolumeToCinder(ctx, volumeName, vmDisk)
	if err != nil {
		return errors.Wrapf(err, "failed to manage volume %s into Cinder", volumeName)
	}

	migobj.logMessage(fmt.Sprintf("Volume %s successfully managed into Cinder with ID: %s", volumeName, cinderVolumeID))

	// Update the VMDisk with the Cinder volume information
	// This will be used later for attaching to the target VM
	vmDisk.OpenstackVol = &volumes.Volume{
		ID:   cinderVolumeID,
		Name: volumeName,
		Size: int(volumeSize / (1024 * 1024 * 1024)), // Convert bytes to GB
	}

	return nil
}

// getESXiHBAUIDs retrieves HBA UIDs from ESXi host via SSH
func (migobj *Migrate) getESXiHBAUIDs(ctx context.Context, hostIP string) ([]string, error) {
	// Create SSH client
	sshClient := esxissh.NewClient()
	defer sshClient.Disconnect()

	if err := sshClient.Connect(ctx, hostIP, "root", migobj.ESXiSSHPrivateKey); err != nil {
		return nil, errors.Wrap(err, "failed to connect to ESXi via SSH")
	}

	// Execute esxcli command to list storage adapters
	output, err := sshClient.ExecuteCommand("esxcli storage core adapter list")
	if err != nil {
		return nil, errors.Wrap(err, "failed to list storage adapters")
	}

	// Parse the output to extract HBA UIDs
	hbaUIDs := []string{}
	uniqueUIDs := make(map[string]bool)

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "HBA Name") {
			continue
		}

		// Parse adapter line - format varies, but UID is typically in a column
		// We're looking for IQN, FC WWN, or NQN identifiers
		fields := strings.Fields(line)
		for _, field := range fields {
			field = strings.ToLower(strings.TrimSpace(field))
			// Check if this looks like a valid HBA UID
			if strings.HasPrefix(field, "iqn.") || strings.HasPrefix(field, "fc.") || strings.HasPrefix(field, "nqn.") {
				if _, exists := uniqueUIDs[field]; !exists {
					uniqueUIDs[field] = true
					hbaUIDs = append(hbaUIDs, field)
				}
			}
		}
	}

	// Alternative: Try to get iSCSI initiator name
	if len(hbaUIDs) == 0 {
		iscsiOutput, err := sshClient.ExecuteCommand("esxcli iscsi adapter list")
		if err == nil {
			// Parse iSCSI adapter output for IQN
			re := regexp.MustCompile(`iqn\.[^\s]+`)
			matches := re.FindAllString(iscsiOutput, -1)
			for _, match := range matches {
				match = strings.ToLower(strings.TrimSpace(match))
				if _, exists := uniqueUIDs[match]; !exists {
					uniqueUIDs[match] = true
					hbaUIDs = append(hbaUIDs, match)
				}
			}
		}
	}

	return hbaUIDs, nil
}

// rescanAndWaitForDevice rescans ESXi storage and waits for device to appear
func (migobj *Migrate) rescanAndWaitForDevice(ctx context.Context, hostIP, devicePath string) error {
	sshClient := esxissh.NewClient()
	defer sshClient.Disconnect()

	if err := sshClient.Connect(ctx, hostIP, "root", migobj.ESXiSSHPrivateKey); err != nil {
		return errors.Wrap(err, "failed to connect to ESXi")
	}

	for attempt := 1; attempt <= rescanRetries; attempt++ {
		// Check if device exists
		_, err := sshClient.ExecuteCommand(fmt.Sprintf("ls -l %s", devicePath))
		if err == nil {
			migobj.logMessage(fmt.Sprintf("Device %s found on attempt %d", devicePath, attempt))
			return nil
		}

		// Rescan storage adapters
		migobj.logMessage(fmt.Sprintf("Rescanning storage adapters (attempt %d/%d)", attempt, rescanRetries))
		_, err = sshClient.ExecuteCommand("esxcli storage core adapter rescan --adapter=vmhba64")
		if err != nil {
			migobj.logMessage(fmt.Sprintf("WARNING: Rescan failed: %v", err))
		}

		// Also try all-adapters rescan
		_, _ = sshClient.ExecuteCommand("esxcli storage core adapter rescan --all")

		time.Sleep(rescanSleepInterval)
	}

	return fmt.Errorf("device %s not found after %d rescan attempts", devicePath, rescanRetries)
}

// rescanESXiStorage rescans ESXi storage to clean up dead devices
func (migobj *Migrate) rescanESXiStorage(ctx context.Context, hostIP string, deleteDeadDevices bool) error {
	sshClient := esxissh.NewClient()
	defer sshClient.Disconnect()

	if err := sshClient.Connect(ctx, hostIP, "root", migobj.ESXiSSHPrivateKey); err != nil {
		return errors.Wrap(err, "failed to connect to ESXi")
	}

	if deleteDeadDevices {
		migobj.logMessage("Rescanning to delete dead devices")
		_, err := sshClient.ExecuteCommand("esxcli storage core adapter rescan --adapter=vmhba64 --type=delete")
		if err != nil {
			return errors.Wrap(err, "failed to rescan for dead devices")
		}
	} else {
		migobj.logMessage("Rescanning to add new devices")
		_, err := sshClient.ExecuteCommand("esxcli storage core adapter rescan --all")
		if err != nil {
			return errors.Wrap(err, "failed to rescan storage")
		}
	}

	return nil
}

// performESXiCopy performs the actual data copy on ESXi using vmkfstools or dd
func (migobj *Migrate) performESXiCopy(ctx context.Context, hostIP, sourcePath, targetDevice string) error {
	sshClient := esxissh.NewClientWithTimeout(24 * time.Hour) // Long timeout for copy operations
	defer sshClient.Disconnect()

	if err := sshClient.Connect(ctx, hostIP, "root", migobj.ESXiSSHPrivateKey); err != nil {
		return errors.Wrap(err, "failed to connect to ESXi")
	}

	// Use vmkfstools for VMDK to block device copy
	// vmkfstools -i source.vmdk -d thin /vmfs/devices/disks/naa.xxx
	copyCmd := fmt.Sprintf("vmkfstools -i %s -d thin %s", sourcePath, targetDevice)
	migobj.logMessage(fmt.Sprintf("Executing copy command: %s", copyCmd))

	output, err := sshClient.ExecuteCommandWithContext(ctx, copyCmd)
	if err != nil {
		return errors.Wrapf(err, "copy command failed: %s", output)
	}

	migobj.logMessage(fmt.Sprintf("Copy output: %s", output))
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
