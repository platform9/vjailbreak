// Copyright © 2024 The vjailbreak authors

package migrate

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/k8sutils"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/utils"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/utils/vmutils"
	"github.com/platform9/vjailbreak/v2v-helper/virtv2v"
	"github.com/platform9/vjailbreak/v2v-helper/vm"
)

// getBootCommand returns the appropriate command to detect boot volume based on OS type
func (migobj *Migrate) getBootCommand(osType string) string {
	switch strings.ToLower(osType) {
	case constants.OSFamilyWindows:
		return "ls /Windows"
	case constants.OSFamilyLinux:
		return "ls /boot"
	default:
		return "inspect-os"
	}
}

// attachAllVolumes attaches all volumes and updates their paths in vminfo
func (migobj *Migrate) attachAllVolumes(ctx context.Context, vminfo *vm.VMInfo) error {
	for idx, vmdisk := range vminfo.VMDisks {
		path, err := migobj.AttachVolume(ctx, vmdisk)
		if err != nil {
			return errors.Wrap(err, "failed to attach volume")
		}
		vminfo.VMDisks[idx].Path = path
	}
	return nil
}

// detectBootVolume identifies which volume contains the boot partition
func (migobj *Migrate) detectBootVolume(vminfo vm.VMInfo, getBootCommand string) (bootVolumeIndex int, osPath string, err error) {
	bootVolumeIndex = -1

	utils.PrintLog(fmt.Sprintf("Detecting boot volume (UEFI: %t)", vminfo.UEFI))

	for idx := range vminfo.VMDisks {
		ans, cmdErr := virtv2v.RunCommandInGuest(vminfo.VMDisks[idx].Path, getBootCommand, false)
		if cmdErr != nil || ans == "" {
			continue
		}

		utils.PrintLog(fmt.Sprintf("Boot volume detected: Disk %d (%s)", idx, vminfo.VMDisks[idx].Name))
		osPath = strings.TrimSpace(ans)
		bootVolumeIndex = idx
		break
	}

	if bootVolumeIndex < 0 {
		utils.PrintLog("WARNING: No boot volume detected")
	}

	return bootVolumeIndex, osPath, nil
}

// handleLinuxOSDetection handles OS detection and validation for Linux systems
func (migobj *Migrate) handleLinuxOSDetection(vminfo vm.VMInfo, bootVolumeIndex int, osPath string, autoFstabUpdate bool) (finalBootIndex int, finalOsPath string, osRelease string, espDiskIndex int, err error) {
	finalBootIndex = bootVolumeIndex
	finalOsPath = osPath

	// Nothing downstream is safe to index if there are no disks.
	if len(vminfo.VMDisks) == 0 {
		return -1, "", "", -1, errors.New("no disks present for VM; cannot detect boot disk")
	}

	// Run get-bootable-partition.sh script
	var ans string
	var cmdErr error

	if ans, cmdErr = virtv2v.RunGetBootablePartitionScript(vminfo.VMDisks); cmdErr != nil {
		utils.PrintLog(fmt.Sprintf("Warning: Failed to run get-bootable-partition.sh: %v", cmdErr))
	} else if ans != "" {
		migobj.logMessage(fmt.Sprintf("Bootable partition: %s", ans))
	}

	if ans == "" {
		return -1, "", "", -1, errors.New("empty bootable partition from the script")
	}

	index, err := virtv2v.RunCommandInGuestAllVolumes(vminfo.VMDisks, "device-index", false, strings.TrimSpace(ans))
	if err != nil {
		fmt.Printf("failed to run command (%s): %v: %s\n", index, err, strings.TrimSpace(index))
		return -1, "", "", -1, err
	}

	finalBootIndex, err = strconv.Atoi(strings.TrimSpace(index))
	if err != nil {
		return -1, "", "", -1, errors.Wrap(err, "failed to convert bootable partition index to int")
	}
	migobj.logMessage(fmt.Sprintf("Bootable partition index: %d", finalBootIndex))

	// device-index can resolve to the libguestfs appliance's own disk (a slot past
	// the guest's disks). Fail the migration rather than indexing vminfo.VMDisks out
	// of range (which would panic) or silently converting the wrong disk.
	if finalBootIndex < 0 || finalBootIndex >= len(vminfo.VMDisks) {
		return -1, "", "", -1, errors.Errorf(
			"detected boot disk index %d is out of range for %d disk(s); aborting migration",
			finalBootIndex, len(vminfo.VMDisks))
	}

	// Detect ESP for UEFI VMs
	espDiskIndex = -1
	if vminfo.UEFI {
		detectedESPIndex, espErr := virtv2v.DetectESPDiskIndex(vminfo.VMDisks)
		if espErr != nil {
			migobj.logMessage(fmt.Sprintf("Error detecting ESP disk: %v", espErr))
		} else if detectedESPIndex >= 0 && detectedESPIndex < len(vminfo.VMDisks) {
			espDiskIndex = detectedESPIndex
			migobj.logMessage(fmt.Sprintf("ESP detected on Disk %d: %s", espDiskIndex, vminfo.VMDisks[espDiskIndex].Name))
		} else {
			migobj.logMessage("WARNING: No ESP detected for UEFI VM")
		}
	}

	// Use boot partition device as OS path for virt-v2v
	// virt-v2v-in-place will auto-detect the actual OS location (LVM or regular partition)
	// when all disks are provided in the libvirt XML
	finalOsPath = strings.TrimSpace(ans)
	migobj.logMessage(fmt.Sprintf("OS path for virt-v2v: %s", finalOsPath))

	osRelease, err = virtv2v.GetOsReleaseAllVolumes(vminfo.VMDisks)
	if err != nil {
		return -1, "", "", -1, errors.Wrapf(err, "failed to get os release: %s", strings.TrimSpace(osRelease))
	}

	if err := migobj.validateLinuxOS(osRelease); err != nil {
		return -1, "", "", -1, err
	}

	// Run generate-mount-persistence.sh script based on AUTO_FSTAB_UPDATE setting.
	// The flag passed to the script varies by OS: SUSE GRUB Legacy guests use
	// --replace-fstab to avoid rewriting device.map before virt-v2v runs (see
	// RunMountPersistenceScript for the full rationale).
	if autoFstabUpdate {
		migobj.logMessage("Running generate-mount-persistence.sh script")
		if err := virtv2v.RunMountPersistenceScript(vminfo.VMDisks, vminfo.VMDisks[finalBootIndex].Path, osRelease); err != nil {
			utils.PrintLog(fmt.Sprintf("Warning: Failed to run generate-mount-persistence.sh: %v", err))
			// Don't fail the migration, just log the warning
		} else {
			migobj.logMessage("Successfully ran generate-mount-persistence.sh script")
		}
	} else {
		migobj.logMessage("Skipping generate-mount-persistence.sh script (AUTO_FSTAB_UPDATE is disabled)")
	}

	if vminfo.UEFI && espDiskIndex >= 0 && espDiskIndex != finalBootIndex {
		migobj.logMessage(fmt.Sprintf("UEFI multi-disk: ESP on Disk %d, Root on Disk %d", espDiskIndex, finalBootIndex))
	}

	return finalBootIndex, finalOsPath, osRelease, espDiskIndex, nil
}

// validateLinuxOS checks if the detected Linux OS is supported
func (migobj *Migrate) validateLinuxOS(osRelease string) error {
	osDetected := strings.ToLower(strings.TrimSpace(osRelease))
	utils.PrintLog(fmt.Sprintf("OS detected by guestfish: %s", osDetected))

	supportedOS := []string{
		"redhat", "red hat", "rhel", "centos", "scientific linux",
		"oracle linux", "fedora", "sles", "sled", "opensuse",
		"alt linux", "debian", "ubuntu", "rocky linux",
		"suse linux enterprise server", "suse linux enterprise desktop", "alma linux",
	}

	for _, s := range supportedOS {
		if strings.Contains(osDetected, s) {
			utils.PrintLog("operating system compatibility check passed")
			return nil
		}
	}

	return errors.Errorf("unsupported OS detected by guestfish: %s", osDetected)
}

// handleWindowsBootDetection handles boot volume detection for Windows systems
func (migobj *Migrate) handleWindowsBootDetection(vminfo vm.VMInfo, bootVolumeIndex int) (int, string, error) {
	utils.PrintLog("operating system compatibility check passed")

	var finalBootIndex int
	var err error

	if len(vminfo.VMDisks) == 0 {
		return -1, "", errors.New("no disks present for VM; cannot detect boot disk")
	}

	utils.PrintLog("checking for bootable volume in case of LDM")
	finalBootIndex, err = virtv2v.GetBootableVolumeIndex(vminfo.VMDisks)
	if err != nil {
		return -1, "", errors.Wrap(err, "Failed to get bootable volume index")
	}

	// Same guard as the Linux path: GetBootableVolumeIndex resolves via device-index,
	// which can point at the libguestfs appliance disk (past the guest's disks).
	// Fail the migration rather than indexing vminfo.VMDisks out of range.
	if finalBootIndex < 0 || finalBootIndex >= len(vminfo.VMDisks) {
		return -1, "", errors.Errorf(
			"detected boot disk index %d is out of range for %d disk(s); aborting migration",
			finalBootIndex, len(vminfo.VMDisks))
	}

	osRelease, err := virtv2v.GetWindowsVersion(vminfo.VMDisks, vminfo.VMDisks[finalBootIndex].Path)
	if err != nil {
		utils.PrintLog(fmt.Sprintf("Warning: Failed to detect Windows version: %v", err))
		osRelease = "Windows (version unknown)"
	}

	utils.PrintLog(fmt.Sprintf("Windows OS detected: %s", osRelease))

	return finalBootIndex, osRelease, nil
}

// performDiskConversion runs virt-v2v conversion on the boot disk
func (migobj *Migrate) performDiskConversion(ctx context.Context, vminfo vm.VMInfo, bootVolumeIndex int, osPath, osRelease string, espDiskIndex int) error {

	persisNetwork := utils.GetNetworkPersistance(ctx, migobj.K8sClient)
	removeVMwareTools := utils.GetRemoveVMwareTools(ctx, migobj.K8sClient)

	if !migobj.Convert {
		return nil
	}

	firstbootscripts := []string{}
	firstbootwinscripts := []virtv2v.FirstBootWindows{}
	// Fix NTFS for Windows
	if strings.ToLower(vminfo.OSType) == constants.OSFamilyWindows {
		if err := virtv2v.NTFSFix(vminfo.VMDisks[bootVolumeIndex].Path); err != nil {
			return errors.Wrap(err, "failed to run ntfsfix")
		}
		firstbootscripts = append(firstbootscripts, "Firstboot-Init-Windows")
		firstbootwinscripts = append(firstbootwinscripts, virtv2v.FirstBootWindows{
			Script: "Firstboot-Scheduler.ps1",
			Async:  false,
		})
		if strings.ToLower(vminfo.OSType) == constants.OSFamilyWindows {
			utils.PrintLog("Successfully added VirtIO PowerShell script to guest")
			firstbootwinscripts = append(firstbootwinscripts, virtv2v.FirstBootWindows{
				Script: "install-virtio-win12.ps1",
				Async:  false,
			})
		}
		if persisNetwork {
			firstbootwinscripts = append(firstbootwinscripts, virtv2v.FirstBootWindows{
				Script: "Orchestrate-NICRecovery.ps1",
				Async:  true,
			})
		}
		firstbootwinscripts = append(firstbootwinscripts, virtv2v.FirstBootWindows{
			Script: "disk-online-fix.ps1",
			Async:  true,
		})
		if removeVMwareTools {
			firstbootwinscripts = append(firstbootwinscripts, virtv2v.FirstBootWindows{
				Script: "vmware-tools-deletion.ps1",
				Async:  true,
			})
		}
		userFirstBootScripts, err := virtv2v.PushWindowsFirstBoot(vminfo.OSType)
		if err != nil {
			return err
		}
		for _, scriptName := range userFirstBootScripts {
			firstbootwinscripts = append(firstbootwinscripts, virtv2v.FirstBootWindows{
				Script: scriptName,
				Async:  true,
			})
		}
	}

	// Add first boot scripts for RHEL family
	if virtv2v.IsRHELFamily(osRelease) {
		versionID := parseVersionID(osRelease)
		if versionID == "" {
			return errors.Errorf("failed to get version ID")
		}
		if !persisNetwork {

			majorVersion, err := strconv.Atoi(strings.Split(versionID, ".")[0])
			if err != nil {
				return fmt.Errorf("failed to parse major version: %v", err)
			}

			if majorVersion >= 7 {
				firstbootscriptname := "rhel_enable_dhcp"
				firstbootscript := constants.RhelFirstBootScript
				firstbootscripts = append(firstbootscripts, firstbootscriptname)

				if err := virtv2v.AddFirstBootScript(firstbootscript, firstbootscriptname); err != nil {
					return errors.Wrap(err, "failed to add first boot script")
				}
				utils.PrintLog("First boot script added successfully")
			}
		}
	}

	// Pre-conversion SUSE fixes: install the LVM mkinitrd wrapper so that
	// virt-v2v's chroot call to mkinitrd can resolve /dev/<vg>/<lv> paths.
	if virtv2v.IsSUSEFamily(osRelease) {
		utils.PrintLog("SUSE guest detected: running pre-conversion FixLegacyMkinitrd")
		if err := virtv2v.FixLegacyMkinitrd(vminfo.VMDisks); err != nil {
			// Non-fatal: log and continue.  The conversion may still succeed
			// on modern SUSE guests that use dracut, or if the root is not LVM.
			utils.PrintLog(fmt.Sprintf("Warning: FixLegacyMkinitrd failed (continuing): %v", err))
		} else {
			utils.PrintLog("FixLegacyMkinitrd completed successfully")
		}
	}
	// Inject VMware Tools cleanup script for Linux guests when requested
	if removeVMwareTools && strings.ToLower(vminfo.OSType) == constants.OSFamilyLinux {
		firstbootscriptname := "vmware_tools_cleanup"
		firstbootscripts = append(firstbootscripts, firstbootscriptname)
		scriptContent, err := os.ReadFile("/home/fedora/vmware-tools-cleanup.sh")
		if err != nil {
			return errors.Wrap(err, "failed to read VMware tools cleanup script")
		}
		if err := virtv2v.AddFirstBootScript(string(scriptContent), firstbootscriptname); err != nil {
			return errors.Wrap(err, "failed to add VMware tools cleanup first boot script")
		}
		utils.PrintLog("VMware Tools cleanup script added for Linux firstboot")
	}

	// Run virt-v2v conversion
	blockDriver := blockDriverFromMetadata(migobj.ImageMetadata)
	utils.PrintLog(fmt.Sprintf("Starting virt-v2v conversion for VM %s (osPath=%s, osType=%s, blockDriver=%q)", vminfo.Name, osPath, vminfo.OSType, blockDriver))
	if err := virtv2v.ConvertDisk(ctx, constants.XMLFileName, osPath, vminfo.OSType, migobj.Virtiowin, firstbootscripts, vminfo.VMDisks[bootVolumeIndex].Path, osRelease, blockDriver); err != nil {
		return errors.Wrap(err, "failed to run virt-v2v")
	}
	utils.PrintLog("virt-v2v conversion completed successfully")

	if strings.ToLower(vminfo.OSType) == constants.OSFamilyWindows {
		if removeVMwareTools {
			if err := virtv2v.RunOfflineVMwareCleanup(vminfo.VMDisks[bootVolumeIndex].Path); err != nil {
				utils.PrintLog(fmt.Sprintf("WARNING: offline VMware Tools cleanup returned error: %v", err))
			}
		}

		if err := virtv2v.InjectFirstBootScriptsFromStore(vminfo.VMDisks, vminfo.VMDisks[bootVolumeIndex].Path, firstbootwinscripts); err != nil {
			return errors.Wrap(err, "failed to inject first boot scripts")
		}
	}

	// Set volume as bootable
	if err := migobj.Openstackclients.SetVolumeBootable(ctx, vminfo.VMDisks[bootVolumeIndex].OpenstackVol); err != nil {
		return errors.Wrap(err, "failed to set volume as bootable")
	}

	// For UEFI multi-disk layouts, also mark the ESP disk as bootable
	// This is required because OpenStack won't allow attaching a non-bootable volume with BootIndex=0
	if vminfo.UEFI && espDiskIndex >= 0 && espDiskIndex != bootVolumeIndex {
		migobj.logMessage(fmt.Sprintf("Marking ESP disk (Disk %d: %s) as bootable in OpenStack", espDiskIndex, vminfo.VMDisks[espDiskIndex].Name))
		if err := migobj.Openstackclients.SetVolumeBootable(ctx, vminfo.VMDisks[espDiskIndex].OpenstackVol); err != nil {
			return errors.Wrap(err, "failed to set ESP volume as bootable")
		}
	}

	return nil
}

// blockDriverFromMetadata returns the virt-v2v --block-driver value that
// matches the hw_disk_bus / hw_scsi_model image properties the caller intends
// to set on the migrated volume.  The two values must agree: if User wants to use
// virtio-scsi controller (hw_disk_bus=scsi, hw_scsi_model=virtio-scsi)
// virt-v2v must make vioscsi boot-critical; or else virtio-blk
// (hw_disk_bus=virtio, the default) virt-v2v must make viostor boot-critical.
// Returning "" lets virt-v2v use its built-in default (virtio-blk / viostor).
func blockDriverFromMetadata(metadata map[string]string) string {
	if metadata["hw_disk_bus"] == "scsi" && metadata["hw_scsi_model"] == "virtio-scsi" {
		return "virtio-scsi"
	}
	return ""
}

func (migobj *Migrate) ConvertVolumes(ctx context.Context, vminfo vm.VMInfo) (int, error) {
	migobj.logMessage("Converting disk")

	// Step 1: Determine boot command based on OS type
	getBootCommand := migobj.getBootCommand(vminfo.OSType)

	// Step 2: Attach all volumes
	if err := migobj.attachAllVolumes(ctx, &vminfo); err != nil {
		return -1, err
	}

	// Step 3: Generate XML configuration for conversion
	if err := vmutils.GenerateXMLConfig(vminfo); err != nil {
		return -1, errors.Wrap(err, "failed to generate XML")
	}

	// Step 3.5: Get vjailbreak settings
	vjailbreakSettings, err := k8sutils.GetVjailbreakSettings(ctx, migobj.K8sClient)
	if err != nil {
		return -1, errors.Wrap(err, "failed to get vjailbreak settings")
	}

	// Step 4: Detect boot volume
	bootVolumeIndex, osPath, err := migobj.detectBootVolume(vminfo, getBootCommand)
	if err != nil {
		return -1, err
	}

	// Step 5: Handle OS-specific detection and validation
	var osRelease string
	var espDiskIndex int = -1

	osType := strings.ToLower(vminfo.OSType)

	switch osType {
	case constants.OSFamilyLinux:
		bootVolumeIndex, osPath, osRelease, espDiskIndex, err = migobj.handleLinuxOSDetection(vminfo, bootVolumeIndex, osPath, vjailbreakSettings.AutoFstabUpdate)
		if err != nil {
			return -1, err
		}

	case constants.OSFamilyWindows:
		bootVolumeIndex, osRelease, err = migobj.handleWindowsBootDetection(vminfo, bootVolumeIndex)
		if err != nil {
			return -1, err
		}

	default:
		return -1, errors.Errorf("unsupported OS type: %s", vminfo.OSType)
	}

	// Step 6: Validate boot volume was found
	if bootVolumeIndex == -1 {
		return -1, errors.Errorf("boot volume not found, cannot create target VM")
	}

	// Step 7: Mark boot volume
	utils.PrintLog(fmt.Sprintf("Boot disk selected: Disk %d (%s)", bootVolumeIndex, vminfo.VMDisks[bootVolumeIndex].Name))
	vminfo.VMDisks[bootVolumeIndex].Boot = true

	// Step 8: Apply merged VolumeImageProfile metadata to the boot volume. Nova/libvirt
	// only read volume_image_metadata from the root disk, so we scope this to the boot volume.
	if len(migobj.ImageMetadata) > 0 {
		bootVol := vminfo.VMDisks[bootVolumeIndex].OpenstackVol
		if bootVol != nil {
			if err := migobj.Openstackclients.ApplyBootVolumeImageMetadata(ctx, bootVol, migobj.ImageMetadata); err != nil {
				return -1, errors.Wrap(err, "failed to apply VolumeImageProfile metadata to boot volume")
			}
			migobj.logMessage(fmt.Sprintf("Applied %d image metadata key(s) from VolumeImageProfiles to boot volume %s: %v",
				len(migobj.ImageMetadata), bootVol.ID, migobj.ImageMetadata))
		}
	}

	// Step 9: Perform disk conversion
	if err := migobj.performDiskConversion(ctx, vminfo, bootVolumeIndex, osPath, osRelease, espDiskIndex); err != nil {
		return -1, err
	}

	// Step 10: Configure network for Linux systems
	if osType == constants.OSFamilyLinux {
		if err := migobj.configureLinuxNetwork(ctx, vminfo, bootVolumeIndex, osRelease); err != nil {
			return -1, err
		}
	} else if osType == constants.OSFamilyWindows {
		if err := migobj.configureWindowsNetwork(ctx, vminfo, bootVolumeIndex, osRelease); err != nil {
			return -1, err
		}
	}

	// Step 11: Detach all volumes
	if err := migobj.DetachAllVolumes(ctx, vminfo); err != nil {
		return -1, errors.Wrap(err, "Failed to detach all volumes from VM")
	}

	migobj.logMessage("Successfully converted disk")
	return espDiskIndex, nil
}

// parseVersionID parses the VERSION_ID from /etc/os-release or /etc/redhat-release format.
// It returns the version ID as a string, or an empty string if not found.
func parseVersionID(osRelease string) string {
	osRelease = strings.TrimSpace(osRelease)

	// Key-value style (os-release, SuSE-release, etc.)
	if strings.Contains(osRelease, "=") {
		var version, patchlevel string
		for _, line := range strings.Split(osRelease, "\n") {
			kv := strings.SplitN(line, "=", 2)
			if len(kv) != 2 {
				continue
			}
			key := strings.TrimSpace(strings.ToUpper(kv[0]))
			val := strings.TrimSpace(strings.Trim(kv[1], `"`)) // Remove quotes and spaces
			switch key {
			case "VERSION_ID":
				return val
			case "VERSION":
				version = val
			case "PATCHLEVEL":
				patchlevel = val
			}
		}
		// If it's SLES style, combine VERSION + PATCHLEVEL if available
		if version != "" {
			if patchlevel != "" {
				return version + "." + patchlevel
			}
			return version
		}
	} else {
		// /etc/redhat-release style
		re := regexp.MustCompile(`release\s+([0-9]+(\.[0-9]+)?)`)
		matches := re.FindStringSubmatch(strings.ToLower(osRelease))
		if len(matches) > 1 {
			return matches[1]
		}
	}

	return ""
}

func isNetplanSupported(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		log.Printf("Warning: unexpected VERSION_ID format: %q", version)
		return true // assume modern if uncertain
	}

	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		log.Printf("Warning: failed to parse VERSION_ID %q: %v %v", version, err1, err2)
		return true
	}

	// Compare with 17.10
	if major > 17 {
		return true
	}
	if major == 17 && minor >= 10 {
		return true
	}
	return false
}
