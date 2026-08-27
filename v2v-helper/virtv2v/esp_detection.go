// Copyright © 2024 The vjailbreak authors

package virtv2v

import (
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/platform9/vjailbreak/v2v-helper/vm"
)

// deriveBaseDisk strips a partition suffix from espDevice, e.g. /dev/sdc1 ->
// /dev/sdc, /dev/nvme0n1p1 -> /dev/nvme0n1.
func deriveBaseDisk(espDevice string) string {
	if espDevice == "" {
		return ""
	}
	baseDisk := strings.TrimRight(espDevice, "0123456789")
	if strings.Contains(baseDisk, "nvme") && strings.HasSuffix(baseDisk, "p") {
		baseDisk = strings.TrimSuffix(baseDisk, "p")
	}
	return baseDisk
}

// validateESPDiskIndex rejects a diskIndex that device-index could have
// resolved to the libguestfs appliance's own disk, past the guest's disks.
func validateESPDiskIndex(diskIndex, numDisks int) error {
	if diskIndex < 0 || diskIndex >= numDisks {
		return fmt.Errorf("resolved ESP disk index %d is out of range for %d disk(s); not treating any disk as the ESP disk", diskIndex, numDisks)
	}
	return nil
}

// DetectESPDiskIndex detects which disk contains the EFI System Partition (ESP)
// Returns the disk index (0-based) or -1 if ESP is not found
func DetectESPDiskIndex(disks []vm.VMDisk) (int, error) {
	// Check if /boot/efi exists using all disks together
	espCheck, err := RunCommandInGuestAllVolumes(disks, "ls", false, "/boot/efi")
	if err != nil || espCheck == "" {
		log.Printf("No /boot/efi found: %v", err)
		return -1, nil
	}

	// Find which device /boot/efi is mounted from by reading /proc/mounts
	mountInfo, err := RunCommandInGuestAllVolumes(disks, "sh", false, "grep '/boot/efi' /proc/mounts || cat /proc/mounts | grep boot")
	if err != nil || mountInfo == "" {
		log.Printf("Could not determine /boot/efi mount point: %v", err)
		return -1, nil
	}

	// Parse /proc/mounts output format: "device mountpoint fstype options dump pass"
	// Example: "/dev/sdc1 /sysroot/boot/efi vfat rw,relatime,..."
	// Note: In guestfish appliance, paths are under /sysroot
	fields := strings.Fields(mountInfo)
	if len(fields) < 2 {
		return -1, fmt.Errorf("unexpected mount output format: %s", mountInfo)
	}

	espDevice := fields[0] // e.g., "/dev/sdc1"
	baseDisk := deriveBaseDisk(espDevice)

	// list-devices excludes the appliance's own disk (unlike the /proc/mounts
	// scan above); reject baseDisk here unless it's actually one of the real disks.
	realDisksStr, listErr := RunCommandInGuestAllVolumes(disks, "list-devices", false)
	if listErr != nil {
		return -1, fmt.Errorf("failed to list real guest devices via list-devices: %w", listErr)
	}
	realDisks := strings.Fields(strings.TrimSpace(realDisksStr))
	if !slices.Contains(realDisks, baseDisk) {
		log.Printf("ESP mount device %s (base %s) is not one of the real attached disks %v; ignoring (likely the libguestfs appliance's own disk)", espDevice, baseDisk, realDisks)
		return -1, nil
	}

	// Map device back to disk index
	deviceIndexStr, err := RunCommandInGuestAllVolumes(disks, "device-index", false, baseDisk)
	if err != nil {
		log.Printf("Failed to get device index for %s: %v", baseDisk, err)
		return -1, err
	}

	deviceIndex := strings.TrimSpace(deviceIndexStr)

	// Convert to integer
	var diskIndex int
	_, err = fmt.Sscanf(deviceIndex, "%d", &diskIndex)
	if err != nil {
		return -1, fmt.Errorf("failed to parse device index '%s': %v", deviceIndex, err)
	}

	if err := validateESPDiskIndex(diskIndex, len(disks)); err != nil {
		return -1, err
	}

	log.Printf("ESP detected on disk %d (%s) at %s", diskIndex, disks[diskIndex].Name, espDevice)

	return diskIndex, nil
}
