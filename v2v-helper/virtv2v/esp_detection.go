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
	log.Println("Detecting EFI System Partition (ESP) disk index")

	// First, check if /boot/efi exists using all disks together
	espCheck, err := RunCommandInGuestAllVolumes(disks, "ls", false, "-la", "/boot/efi")
	if err != nil || espCheck == "" {
		log.Printf("No /boot/efi found: %v", err)
		return -1, nil
	}

	log.Printf("/boot/efi exists, checking which disk contains it")

	// Find which device /boot/efi is mounted from
	mountInfo, err := RunCommandInGuestAllVolumes(disks, "sh", false, "-c", "mount | grep '/boot/efi'")
	if err != nil || mountInfo == "" {
		log.Printf("Could not determine /boot/efi mount point: %v", err)
		return -1, nil
	}

	log.Printf("Mount info for /boot/efi: %s", strings.TrimSpace(mountInfo))

	// Parse mount output: "/dev/sdc1 on /boot/efi type vfat ..."
	// Extract the device name (e.g., /dev/sdc1)
	fields := strings.Fields(mountInfo)
	if len(fields) < 1 {
		return -1, fmt.Errorf("unexpected mount output format: %s", mountInfo)
	}

	espDevice := fields[0] // e.g., "/dev/sdc1"
	log.Printf("ESP device: %s", espDevice)

	// Extract base disk name (e.g., /dev/sdc from /dev/sdc1)
	// Handle both /dev/sdX and /dev/vdX naming
	baseDisk := espDevice
	if len(espDevice) > 0 {
		// Remove partition number: /dev/sdc1 -> /dev/sdc, /dev/vda2 -> /dev/vda
		baseDisk = strings.TrimRight(espDevice, "0123456789")
		// Also handle /dev/nvme0n1p1 -> /dev/nvme0n1
		if strings.Contains(baseDisk, "nvme") && strings.HasSuffix(baseDisk, "p") {
			baseDisk = strings.TrimSuffix(baseDisk, "p")
		}
	}

	log.Printf("ESP base disk: %s", baseDisk)

	// Now map this device back to disk index
	// Use device-index command to find which disk this corresponds to
	deviceIndexStr, err := RunCommandInGuestAllVolumes(disks, "device-index", false, baseDisk)
	if err != nil {
		log.Printf("Failed to get device index for %s: %v", baseDisk, err)
		return -1, err
	}

	deviceIndex := strings.TrimSpace(deviceIndexStr)
	log.Printf("Device index for %s: %s", baseDisk, deviceIndex)

	// Convert to integer
	var diskIndex int
	_, err = fmt.Sscanf(deviceIndex, "%d", &diskIndex)
	if err != nil {
		return -1, fmt.Errorf("failed to parse device index '%s': %v", deviceIndex, err)
	}

	log.Printf("ESP found on disk index: %d (disk name: %s)", diskIndex, disks[diskIndex].Name)

	// Verify by checking for EFI bootloaders
	bootloaderCheck, _ := RunCommandInGuestAllVolumes(disks, "sh", false, "-c", "find /boot/efi -name '*.efi' 2>/dev/null | head -5")
	if bootloaderCheck != "" {
		log.Printf("EFI bootloaders found: %s", strings.TrimSpace(bootloaderCheck))
	}

	return diskIndex, nil
}
