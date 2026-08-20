// Copyright © 2024 The vjailbreak authors

package migrate

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/flavors"
	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	"github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage"
	netappsdk "github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage/netapp"
	_ "github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage/providers"
	vantarasdk "github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage/vantara"
	"github.com/platform9/vjailbreak/v2v-helper/openstack"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/k8sutils"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/utils"
	"github.com/platform9/vjailbreak/v2v-helper/vm"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	netutils "github.com/platform9/vjailbreak/pkg/common/utils"
	probing "github.com/prometheus-community/pro-bing"
)

func (migobj *Migrate) logMessage(message string) {
	if migobj.InPod {
		migobj.EventReporter <- message
	}
	utils.PrintLog(message)
}

// This function creates volumes in OpenStack and attaches them to the helper vm
func (migobj *Migrate) CreateVolumes(ctx context.Context, vminfo vm.VMInfo) (vm.VMInfo, error) {
	openstackops := migobj.Openstackclients
	migobj.logMessage("Creating volumes in OpenStack")

	for idx, vmdisk := range vminfo.VMDisks {
		setRDMLabel := false
		if len(vminfo.RDMDisks) > 0 {
			setRDMLabel = true
		}
		volume, err := openstackops.CreateVolume(ctx, vminfo.Name+"-"+vmdisk.Name, vmdisk.Size, vminfo.OSType, vminfo.UEFI, migobj.Volumetypes[idx], setRDMLabel)
		if err != nil {
			return vminfo, errors.Wrap(err, "failed to create volume")
		}
		vminfo.VMDisks[idx].OpenstackVol = volume
		if vminfo.VMDisks[idx].Boot {
			err = openstackops.SetVolumeBootable(ctx, volume)
			if err != nil {
				return vminfo, errors.Wrap(err, "failed to set volume as bootable")
			}
		}
	}
	migobj.logMessage("Volumes created successfully")
	return vminfo, nil
}

// applyImageMetadataForXCOPYVolumes mirrors the metadata calls that CreateVolume
// makes on the standard (non-accelerated) path.
func (migobj *Migrate) applyImageMetadataForXCOPYVolumes(ctx context.Context, vminfo vm.VMInfo) error {
	openstackops := migobj.Openstackclients
	setRDMLabel := len(vminfo.RDMDisks) > 0

	for idx := range vminfo.VMDisks {
		vmdisk := vminfo.VMDisks[idx]
		if vmdisk.OpenstackVol == nil {
			return fmt.Errorf("XCOPY volume for disk %s has no OpenStack volume reference", vmdisk.Name)
		}
		volume := vmdisk.OpenstackVol

		if vminfo.UEFI {
			if err := openstackops.SetVolumeUEFI(ctx, volume); err != nil {
				return errors.Wrapf(err, "failed to set UEFI metadata on XCOPY volume %s", volume.ID)
			}
		}

		if strings.ToLower(vminfo.OSType) == constants.OSFamilyWindows {
			if err := openstackops.SetVolumeImageMetadata(ctx, volume, setRDMLabel); err != nil {
				return errors.Wrapf(err, "failed to set Windows image metadata on XCOPY volume %s", volume.ID)
			}
		}

		if err := openstackops.EnableQGA(ctx, volume); err != nil {
			return errors.Wrapf(err, "failed to enable QGA on XCOPY volume %s", volume.ID)
		}

		if vmdisk.Boot {
			if err := openstackops.SetVolumeBootable(ctx, volume); err != nil {
				return errors.Wrapf(err, "failed to set XCOPY volume %s as bootable", volume.ID)
			}
		}
	}

	migobj.logMessage("Applied image metadata to XCOPY-managed volumes")
	return nil
}

func (migobj *Migrate) AttachVolume(ctx context.Context, disk vm.VMDisk) (string, error) {
	openstackops := migobj.Openstackclients
	migobj.logMessage(fmt.Sprintf("Attaching volumes to VM: %s", disk.Name))
	if disk.OpenstackVol == nil {
		return "", errors.Wrap(fmt.Errorf("OpenStack volume is nil"), "failed to attach volume to VM")
	}
	volumeID := disk.OpenstackVol.ID
	if err := openstackops.AttachVolumeToVM(ctx, volumeID); err != nil {
		return "", errors.Wrap(err, "failed to attach volume to VM")
	}

	// Get the Path of the attached volume
	devicePath, err := openstackops.FindDevice(volumeID)
	if err != nil {
		return "", errors.Wrap(err, "failed to find device")
	}
	return devicePath, nil
}

func (migobj *Migrate) DetachVolume(ctx context.Context, disk vm.VMDisk) error {
	openstackops := migobj.Openstackclients

	if err := openstackops.DetachVolumeFromVM(ctx, disk.OpenstackVol.ID); err != nil {
		return errors.Wrap(err, "failed to detach volume from VM")
	}

	err := openstackops.WaitForVolume(ctx, disk.OpenstackVol.ID)
	if err != nil {
		return errors.Wrap(err, "failed to wait for volume to become available")
	}
	return nil
}

func (migobj *Migrate) DetachAllVolumes(ctx context.Context, vminfo vm.VMInfo) error {
	openstackops := migobj.Openstackclients
	for _, vmdisk := range vminfo.VMDisks {
		if vmdisk.OpenstackVol == nil {
			migobj.logMessage(fmt.Sprintf("Skipping detach for disk %s: no OpenStack volume was created", vmdisk.Name))
			continue
		}
		migobj.logMessage(fmt.Sprintf("Detaching volume %s from VM", vmdisk.Name))
		if err := openstackops.DetachVolumeFromVM(ctx, vmdisk.OpenstackVol.ID); err != nil && !strings.Contains(err.Error(), "is not attached to volume") {
			return errors.Wrap(err, "failed to detach volume from VM")
		}

		err := openstackops.WaitForVolume(ctx, vmdisk.OpenstackVol.ID)
		if err != nil {
			return errors.Wrap(err, "failed to wait for volume to become available")
		}
		migobj.logMessage(fmt.Sprintf("Volume %s detached from VM", vmdisk.Name))
	}
	time.Sleep(1 * time.Second)
	return nil
}

// DetachAllVolumesWithCleanup is like DetachAllVolumes but handles the case where
// a volume is attached to a foreign server (e.g. an orphaned target VM created by
// a timed-out CreateTargetInstance call). Boot volumes cannot be hot-detached from
// a running VM, so the foreign server is deleted first, then WaitForVolume polls
// until Cinder reports the volume available.
func (migobj *Migrate) DetachAllVolumesWithCleanup(ctx context.Context, vminfo vm.VMInfo) error {
	openstackops := migobj.Openstackclients

	vjailbreakUUID, err := openstack.GetCurrentInstanceUUID()
	if err != nil {
		return errors.Wrap(err, "failed to get vJailbreak instance UUID")
	}

	deletedServers := map[string]bool{}

	for _, vmdisk := range vminfo.VMDisks {
		if vmdisk.OpenstackVol == nil {
			migobj.logMessage(fmt.Sprintf("Skipping detach for disk %s: no OpenStack volume was created", vmdisk.Name))
			continue
		}

		volume, err := openstackops.GetVolume(ctx, vmdisk.OpenstackVol.ID)
		if err != nil {
			return errors.Wrapf(err, "failed to get volume state for disk %s", vmdisk.Name)
		}

		if len(volume.Attachments) == 0 {
			migobj.logMessage(fmt.Sprintf("Volume %s is already detached, skipping", vmdisk.Name))
			continue
		}

		attachedServerID := volume.Attachments[0].ServerID

		if attachedServerID == vjailbreakUUID {
			migobj.logMessage(fmt.Sprintf("Detaching volume %s from vJailbreak VM", vmdisk.Name))
			if err := openstackops.DetachVolumeFromVM(ctx, vmdisk.OpenstackVol.ID); err != nil && !strings.Contains(err.Error(), "is not attached to volume") {
				return errors.Wrap(err, "failed to detach volume from vJailbreak VM")
			}
		} else {
			if !deletedServers[attachedServerID] {
				migobj.logMessage(fmt.Sprintf("Volume %s is attached to orphaned server %s, deleting server", vmdisk.Name, attachedServerID))
				if err := openstackops.DeleteServer(ctx, attachedServerID); err != nil {
					return errors.Wrapf(err, "failed to delete orphaned server %s", attachedServerID)
				}
				deletedServers[attachedServerID] = true
			}
		}

		if err := openstackops.WaitForVolume(ctx, vmdisk.OpenstackVol.ID); err != nil {
			return errors.Wrapf(err, "failed to wait for volume %s to become available", vmdisk.Name)
		}
		migobj.logMessage(fmt.Sprintf("Volume %s detached from VM", vmdisk.Name))
	}

	time.Sleep(1 * time.Second)
	return nil
}

// resolveTargetServerID returns the ID of the server the boot volume is attached
// to, in whatever state it is. Kept separate from verifyVMCreatedDespiteTimeout,
// which also insists on ACTIVE - wrong for the LDM promotion, where the guest is
// deliberately stopped first and an ACTIVE check would reject every promotion.
func (migobj *Migrate) resolveTargetServerID(ctx context.Context, vminfo vm.VMInfo) (string, error) {
	vjailbreakUUID, err := openstack.GetCurrentInstanceUUID()
	if err != nil {
		return "", errors.Wrap(err, "failed to get vJailbreak instance UUID")
	}

	var bootDisk *vm.VMDisk
	for i := range vminfo.VMDisks {
		if vminfo.VMDisks[i].Boot {
			bootDisk = &vminfo.VMDisks[i]
			break
		}
	}
	if bootDisk == nil || bootDisk.OpenstackVol == nil {
		return "", fmt.Errorf("no boot volume found")
	}

	volume, err := migobj.Openstackclients.GetVolume(ctx, bootDisk.OpenstackVol.ID)
	if err != nil {
		return "", errors.Wrap(err, "failed to get boot volume")
	}

	if len(volume.Attachments) == 0 {
		return "", fmt.Errorf("boot volume is not attached to any server")
	}

	attachedServerID := volume.Attachments[0].ServerID
	if attachedServerID == vjailbreakUUID {
		return "", fmt.Errorf("boot volume is still attached to vJailbreak VM")
	}
	return attachedServerID, nil
}

func (migobj *Migrate) verifyVMCreatedDespiteTimeout(ctx context.Context, vminfo vm.VMInfo) (string, error) {
	attachedServerID, err := migobj.resolveTargetServerID(ctx, vminfo)
	if err != nil {
		return "", err
	}

	status, err := migobj.Openstackclients.GetServerStatus(ctx, attachedServerID)
	if err != nil {
		return "", errors.Wrap(err, "failed to get target VM status")
	}

	if strings.ToUpper(status) != "ACTIVE" {
		return "", fmt.Errorf("target VM %s is in %s state", attachedServerID, status)
	}

	migobj.logMessage(fmt.Sprintf("Boot volume attached to active target VM %s", attachedServerID))
	return attachedServerID, nil
}

func (migobj *Migrate) DeleteAllVolumes(ctx context.Context, vminfo vm.VMInfo) error {
	openstackops := migobj.Openstackclients
	for _, vmdisk := range vminfo.VMDisks {
		if vmdisk.OpenstackVol == nil {
			migobj.logMessage(fmt.Sprintf("Skipping delete for disk %s: no OpenStack volume was created", vmdisk.Name))
			continue
		}
		err := openstackops.DeleteVolume(ctx, vmdisk.OpenstackVol.ID)
		if err != nil {
			return errors.Wrap(err, "failed to delete volume")
		}
		migobj.logMessage(fmt.Sprintf("Volume %s deleted", vmdisk.Name))
	}
	return nil
}

// extractFileName extracts the file name from a full VMDK path
func extractFileName(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return path
}

// logDiskCopyPlan logs the disk copy plan showing source to target mapping
// Helps with debugging by showing exactly which VMDK goes to which volume
func (migobj *Migrate) logDiskCopyPlan(vminfo vm.VMInfo) {
	migobj.logMessage("=== Disk Copy Plan ===")
	for idx, disk := range vminfo.VMDisks {
		migobj.logMessage(fmt.Sprintf("[%d] %s (DeviceKey=%d): %s -> Volume %s (%s)",
			idx,
			disk.Name,
			disk.Disk.Key,
			extractFileName(disk.SnapBackingDisk),
			disk.OpenstackVol.Name,
			disk.Path))
	}
}

// validateDiskMapping validates that disk mapping is correct before starting copy
// Cross-checks vminfo data with nbdops to ensure correct source-to-target mapping
func (migobj *Migrate) validateDiskMapping(vminfo vm.VMInfo) error {
	migobj.logMessage("Validating disk mapping before copy operation...")

	// Verify number of disks matches number of NBD servers
	if len(vminfo.VMDisks) != len(migobj.Nbdops) {
		return fmt.Errorf("disk count mismatch: vminfo has %d disks but %d NBD servers configured", len(vminfo.VMDisks), len(migobj.Nbdops))
	}

	for idx, vmdisk := range vminfo.VMDisks {
		// Validate volume exists
		if vmdisk.OpenstackVol == nil {
			return fmt.Errorf("OpenStack volume is nil for disk %s (DeviceKey=%d)", vmdisk.Name, vmdisk.Disk.Key)
		}

		// Validate device path exists
		if vmdisk.Path == "" {
			return fmt.Errorf("device path is empty for disk %s (DeviceKey=%d)", vmdisk.Name, vmdisk.Disk.Key)
		}

		// Validate snapshot backing disk exists
		if vmdisk.SnapBackingDisk == "" {
			return fmt.Errorf("snapshot backing disk is empty for disk %s (DeviceKey=%d)", vmdisk.Name, vmdisk.Disk.Key)
		}

		// Cross-check: verify NBD server at this index is initialized
		if migobj.Nbdops[idx] == nil {
			return fmt.Errorf("NBD server not initialized for disk %d (%s)", idx, vmdisk.Name)
		}

		// Log validation details for this disk
		utils.PrintLog(fmt.Sprintf("[%d] Validated %s (DeviceKey=%d): SnapFile=%s, Volume=%s, Path=%s",
			idx, vmdisk.Name, vmdisk.Disk.Key,
			extractFileName(vmdisk.SnapBackingDisk),
			vmdisk.OpenstackVol.ID, vmdisk.Path))
	}

	migobj.logMessage("Disk mapping validation passed")
	return nil
}

func (migobj *Migrate) CreateTargetInstance(ctx context.Context, vminfo vm.VMInfo, networkids, portids []string, ipaddresses []string, espDiskIndex int) error {
	migobj.logMessage("Creating target instance")
	openstackops := migobj.Openstackclients
	var flavor *flavors.Flavor
	var err error

	if migobj.TargetFlavorId != "" {
		flavor, err = openstackops.GetFlavor(ctx, migobj.TargetFlavorId)
		if err != nil {
			return errors.Wrap(err, "failed to get OpenStack flavor")
		}
		if openstack.IsHotplugFlavor(flavor) {
			migobj.logMessage(fmt.Sprintf("Assigned flavor %s is a hotplug base flavor (0 vCPU, 0 RAM); VM will be created with hotplug metadata", flavor.Name))
		}
	} else {
		flavor, err = openstackops.GetClosestFlavour(ctx, vminfo.CPU, vminfo.Memory)
		if err != nil {
			return errors.Wrap(err, "failed to get closest OpenStack flavor")
		}
		utils.PrintLog(fmt.Sprintf("Closest OpenStack flavor: %s: CPU: %dvCPUs\tMemory: %dMB\n", flavor.Name, flavor.VCPUs, flavor.RAM))
	}
	// Get security group IDs
	securityGroupIDs, err := openstackops.GetSecurityGroupIDs(ctx, migobj.SecurityGroups, migobj.TenantName)
	if err != nil {
		return errors.Wrap(err, "failed to resolve security group names to IDs")
	}
	utils.PrintLog(fmt.Sprintf("Using security group IDs: %v", securityGroupIDs))

	if migobj.ServerGroup != "" {
		utils.PrintLog(fmt.Sprintf("Using server group ID: %s", migobj.ServerGroup))
	} else {
		utils.PrintLog("No server group specified - VMs will be placed based on default scheduling")
	}

	// Get vjailbreak settings
	vjailbreakSettings, err := k8sutils.GetVjailbreakSettings(context.Background(), migobj.K8sClient)
	if err != nil {
		return errors.Wrap(err, "failed to get vjailbreak settings")
	}
	utils.PrintLog(fmt.Sprintf("Fetched vjailbreak settings for VM active wait retry limit: %d, VM active wait interval seconds: %d", vjailbreakSettings.VMActiveWaitRetryLimit, vjailbreakSettings.VMActiveWaitIntervalSeconds))

	// Create a new VM in OpenStack
	if len(migobj.TargetMetadata) > 0 {
		vminfo.TargetMetadata = migobj.TargetMetadata
		migobj.logMessage(fmt.Sprintf("Applying %d instance metadata entries (preserved source tags/custom metadata) to target VM", len(migobj.TargetMetadata)))
	}
	newVM, err := openstackops.CreateVM(ctx, flavor, networkids, portids, vminfo, migobj.TargetAvailabilityZone, securityGroupIDs, migobj.ServerGroup, *vjailbreakSettings, espDiskIndex)
	if err != nil {
		return errors.Wrap(err, "failed to create VM")
	}

	// Wait for VM to become active
	for i := 0; i < vjailbreakSettings.VMActiveWaitRetryLimit; i++ {
		migobj.logMessage(fmt.Sprintf("Waiting for VM to become active: %d/%d retries\n", i+1, vjailbreakSettings.VMActiveWaitRetryLimit))
		active, err := openstackops.WaitUntilVMActive(ctx, newVM.ID)
		if err != nil {
			return errors.Wrap(err, "failed to wait for VM to become active")
		}
		if active {
			break
		}
		if i == vjailbreakSettings.VMActiveWaitRetryLimit-1 {
			return errors.Errorf("VM is not active after %d retries", vjailbreakSettings.VMActiveWaitRetryLimit)
		}
		time.Sleep(time.Duration(vjailbreakSettings.VMActiveWaitIntervalSeconds) * time.Second)
	}

	migobj.logMessage(fmt.Sprintf("VM created successfully: ID: %s", newVM.ID))

	if migobj.PerformHealthChecks {
		err = migobj.HealthCheck(vminfo, ipaddresses)
		if err != nil {
			migobj.logMessage(fmt.Sprintf("Health Check failed: %s", err))
		}
	} else {
		migobj.logMessage("Skipping Health Checks")
	}

	return nil
}

func (migobj *Migrate) HealthCheck(vminfo vm.VMInfo, ips []string) error {
	migobj.logMessage("Performing Health Checks")
	healthChecks := make(map[string]bool)
	healthChecks["Ping"] = false
	healthChecks["HTTP Get"] = false
	for i := 0; i < 10; i++ {
		migobj.logMessage(fmt.Sprintf("Health Check Attempt %d", i+1))
		// 1. Ping
		if !healthChecks["Ping"] {
			err := migobj.pingVM(ips)
			if err != nil {
				migobj.logMessage(fmt.Sprintf("Ping(s) failed: %s", err))
			} else {
				healthChecks["Ping"] = true
			}
		}
		// 2. HTTP GET check
		if !healthChecks["HTTP Get"] {
			err := migobj.checkHTTPGet(ips, migobj.HealthCheckPort)
			if err != nil {
				migobj.logMessage(fmt.Sprintf("HTTP Get failed: %s", err))
			} else {
				healthChecks["HTTP Get"] = true
			}
		}
		if healthChecks["Ping"] && healthChecks["HTTP Get"] {
			break
		}
		migobj.logMessage("Waiting for 60 seconds before retrying health checks")
		time.Sleep(60 * time.Second)
	}
	for key, value := range healthChecks {
		if !value {
			migobj.logMessage(fmt.Sprintf("Health Check %s failed", key))
		} else {
			migobj.logMessage(fmt.Sprintf("Health Check %s succeeded", key))
		}
	}
	return nil
}

func (migobj *Migrate) pingVM(ips []string) error {
	for _, ip := range ips {
		migobj.logMessage(fmt.Sprintf("Pinging VM: %s", ip))
		pinger, err := probing.NewPinger(ip)
		if err != nil {
			return errors.Wrap(err, "failed to create pinger")
		}
		pinger.Count = 1
		pinger.Timeout = time.Second * 10
		err = pinger.Run()
		if err != nil {
			return errors.Wrap(err, "failed to run pinger")
		}
		if pinger.Statistics().PacketLoss == 0 {
			migobj.logMessage("Ping succeeded")
		} else {
			return errors.Errorf("Ping failed")
		}
	}
	return nil
}

func (migobj *Migrate) checkHTTPGet(ips []string, port string) error {
	var client *http.Client
	vjbNet := netutils.NewVjbNet()
	if migobj.Insecure {
		vjbNet.Insecure = true
	}
	if vjbNet.CreateSecureHTTPClient() == nil {
		client = vjbNet.GetClient()
	} else {
		return errors.Errorf("Both HTTP and HTTPS failed ")
	}
	for _, ip := range ips {
		// Try HTTP first
		httpURL := fmt.Sprintf("http://%s:%s", ip, port)
		if err := migobj.tryConnection(client, httpURL); err == nil {
			migobj.logMessage("HTTP succeeded")
			continue // Success with HTTP, move to next IP
		}

		// If HTTP fails, try HTTPS
		httpsURL := fmt.Sprintf("https://%s:%s", ip, port)
		if err := migobj.tryConnection(client, httpsURL); err == nil {
			migobj.logMessage("HTTPS succeeded")
			continue // Success with HTTPS, move to next IP
		}

		// Both HTTP and HTTPS failed
		return errors.Errorf("Both HTTP and HTTPS failed for %s:%s", ip, port)
	}

	return nil
}

func (migobj *Migrate) tryConnection(client *http.Client, url string) error {
	resp, err := client.Get(url)
	if err != nil {
		migobj.logMessage(fmt.Sprintf("GET failed for %s: %v", url, err))
		return errors.Wrap(err, "failed to get url")
	}
	defer resp.Body.Close()

	migobj.logMessage(fmt.Sprintf("GET response for %s: %d", url, resp.StatusCode))

	if resp.StatusCode != http.StatusOK {
		return errors.Errorf("GET returned non-OK status for %s: %d", url, resp.StatusCode)
	}

	return nil
}

// LogMessage is an exported wrapper for logMessage that satisfies the esxissh.ProgressLogger interface.
func (migobj *Migrate) LogMessage(message string) {
	migobj.logMessage(message)
}

// buildProviderOptions projects vendor-specific Migrate fields into the
// generic ProviderOptions map passed to the storage SDK. Returns nil when no
// options apply. New vendors add their case here.
func (migobj *Migrate) buildProviderOptions() map[string]string {
	opts := map[string]string{}
	if migobj.VendorType == netappsdk.VendorName {
		if migobj.NetAppSVM != "" {
			opts[netappsdk.OptionSVM] = migobj.NetAppSVM
		}
		if migobj.NetAppFlexVol != "" {
			opts[netappsdk.OptionFlexVol] = migobj.NetAppFlexVol
		}
	}
	if migobj.VendorType == vantarasdk.VendorName {
		if migobj.VantaraPoolID != "" {
			opts[vantarasdk.OptionPoolID] = migobj.VantaraPoolID
		}
		if migobj.VantaraRESTPort != "" {
			opts[vantarasdk.OptionRESTPort] = migobj.VantaraRESTPort
		}
	}
	if len(opts) == 0 {
		return nil
	}
	return opts
}

// InitializeStorageProvider initializes and validates the storage provider for StorageAcceleratedCopy migration
func (migobj *Migrate) InitializeStorageProvider(ctx context.Context) error {
	if migobj.StorageCopyMethod != constants.StorageCopyMethod {
		migobj.logMessage("Storage copy method is not StorageAcceleratedCopy, skipping storage provider initialization")
		return nil
	}

	migobj.logMessage("Initializing storage provider for StorageAcceleratedCopy migration")

	// Validate required credentials
	if migobj.ArrayHost == "" {
		return fmt.Errorf("ARRAY_HOST is required for StorageAcceleratedCopy storage migration")
	}
	if migobj.ArrayUser == "" {
		return fmt.Errorf("ARRAY_USER is required for StorageAcceleratedCopy storage migration")
	}
	if migobj.ArrayPassword == "" {
		return fmt.Errorf("ARRAY_PASSWORD is required for StorageAcceleratedCopy storage migration")
	}

	// Create storage access info
	accessInfo := storage.StorageAccessInfo{
		Hostname:            migobj.ArrayHost,
		Username:            migobj.ArrayUser,
		Password:            migobj.ArrayPassword,
		SkipSSLVerification: migobj.ArrayInsecure,
		VendorType:          migobj.VendorType,
		ProviderOptions:     migobj.buildProviderOptions(),
	}

	// Create storage provider
	provider, err := storage.NewStorageProvider(accessInfo.VendorType)
	if err != nil {
		return fmt.Errorf("failed to create storage provider: %w", err)
	}

	// Connect to storage array
	migobj.logMessage(fmt.Sprintf("Connecting to storage array: %s", migobj.ArrayHost))
	if err := provider.Connect(ctx, accessInfo); err != nil {
		return fmt.Errorf("failed to connect to storage array: %w", err)
	}

	// Validate credentials
	migobj.logMessage("Validating storage array credentials...")
	if err := provider.ValidateCredentials(ctx); err != nil {
		return fmt.Errorf("storage array credential validation failed: %w", err)
	}

	migobj.StorageProvider = provider
	migobj.logMessage(fmt.Sprintf("Storage provider initialized successfully: %s", provider.WhoAmI()))

	return nil
}

// LoadESXiSSHKey loads the ESXi SSH private key from the Kubernetes secret
func (migobj *Migrate) LoadESXiSSHKey(ctx context.Context) error {

	migobj.logMessage(fmt.Sprintf("Loading ESXi SSH private key from secret: %s", constants.ESXiSSHSecretName))

	privateKey, err := k8sutils.GetESXiSSHPrivateKey(ctx, migobj.K8sClient, constants.ESXiSSHSecretName)
	if err != nil {
		return errors.Wrapf(err, "failed to load ESXi SSH private key from secret %s", constants.ESXiSSHSecretName)
	}

	migobj.ESXiSSHPrivateKey = privateKey
	migobj.logMessage("ESXi SSH private key loaded successfully")

	return nil
}

// reportStagedVolumeIDs collects the Cinder volume IDs from vminfo and patches
// them onto Migration.Status.StagedVolumeIDs. It then sends the DataCopied
// event message via the EventReporter channel so the controller can update the
// migration phase. Failures are non-fatal — the caller logs a warning and
// continues.
func (migobj *Migrate) reportStagedVolumeIDs(ctx context.Context, vminfo vm.VMInfo) error {
	// Collect volume IDs
	var volumeIDs []string
	for _, disk := range vminfo.VMDisks {
		if disk.OpenstackVol != nil && disk.OpenstackVol.ID != "" {
			volumeIDs = append(volumeIDs, disk.OpenstackVol.ID)
		}
	}

	// Patch Migration.Status.StagedVolumeIDs if we have a K8s client
	if migobj.K8sClient != nil {
		migrationName, err := utils.GetMigrationObjectName()
		if err != nil {
			return errors.Wrap(err, "failed to get migration object name for staged volume IDs patch")
		}
		migration := &vjailbreakv1alpha1.Migration{}
		if err := migobj.K8sClient.Get(ctx, k8stypes.NamespacedName{
			Name:      migrationName,
			Namespace: constants.NamespaceMigrationSystem,
		}, migration); err != nil {
			return errors.Wrapf(err, "failed to get migration %s to patch staged volume IDs", migrationName)
		}
		patch := client.MergeFrom(migration.DeepCopy())
		migration.Status.StagedVolumeIDs = volumeIDs
		if err := migobj.K8sClient.Status().Patch(ctx, migration, patch); err != nil {
			return errors.Wrapf(err, "failed to patch staged volume IDs on migration %s", migrationName)
		}
		migobj.logMessage(fmt.Sprintf("Patched StagedVolumeIDs %v on migration %s", volumeIDs, migrationName))
	}

	// Send DataCopied event so the controller can advance the migration phase
	migobj.logMessage(constants.EventMessageDataCopied)

	return nil
}
