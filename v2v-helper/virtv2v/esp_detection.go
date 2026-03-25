// Copyright © 2024 The vjailbreak authors

package virtv2v

import (
	"fmt"
	"log"
	"strings"

	"github.com/platform9/vjailbreak/v2v-helper/vm"
)

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

	// Extract base disk name (e.g., /dev/sdc from /dev/sdc1)
	baseDisk := espDevice
	if len(espDevice) > 0 {
		// Remove partition number: /dev/sdc1 -> /dev/sdc, /dev/vda2 -> /dev/vda
		baseDisk = strings.TrimRight(espDevice, "0123456789")
		// Handle /dev/nvme0n1p1 -> /dev/nvme0n1
		if strings.Contains(baseDisk, "nvme") && strings.HasSuffix(baseDisk, "p") {
			baseDisk = strings.TrimSuffix(baseDisk, "p")
		}
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

	log.Printf("ESP detected on disk %d (%s) at %s", diskIndex, disks[diskIndex].Name, espDevice)

	return diskIndex, nil
}
