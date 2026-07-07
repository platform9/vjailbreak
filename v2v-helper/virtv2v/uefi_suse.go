// Copyright © 2024 The vjailbreak authors

package virtv2v

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/platform9/vjailbreak/v2v-helper/vm"
)

// grub2EFITarPath is the bundled GRUB 2 x86_64-efi module tree for SLES 11 SP4.
// The tarball is packed with the x86_64-efi/ prefix, so extracting it under
// /EFI/BOOT/ yields /EFI/BOOT/x86_64-efi/{*.mod,grub.efi,...}.
const grub2EFITarPath = "/home/fedora/grub2-x86_64-efi-sles11sp4.tar.gz"

// ── Phase 1: Detection + grub.cfg generation ─────────────────────────────────

// IsSLES11SP4 returns true when osRelease content (from /etc/SuSE-release)
// identifies the guest as SUSE Linux Enterprise Server 11 SP4.
// /etc/SuSE-release on SLES 11 SP4 looks like (after lower-casing):
//
//	suse linux enterprise server 11 (x86_64)
//	version = 11
//	patchlevel = 4
func IsSLES11SP4(osRelease string) bool {
	lower := strings.ToLower(osRelease)
	return strings.Contains(lower, "suse linux enterprise server 11") &&
		strings.Contains(lower, "patchlevel = 4")
}

// parseSLESMenuLst parses /boot/grub/menu.lst and returns the kernel version,
// kernel argument string, and initrd filename from the first non-failsafe entry.
//
// Example entry in menu.lst:
//
//	title SUSE Linux Enterprise Server 11 SP4 - 3.0.101-63
//	    root (hd3,0)
//	    kernel /vmlinuz-3.0.101-63-default root=/dev/vg_root/root_point splash=silent ...
//	    initrd /initrd-3.0.101-63-default
func parseSLESMenuLst(menuLst string) (kernelVer, kernelArgs, initrdName string, err error) {
	inFailsafe := false
	for _, rawLine := range strings.Split(menuLst, "\n") {
		line := strings.TrimSpace(rawLine)

		if strings.HasPrefix(strings.ToLower(line), "title") {
			inFailsafe = strings.Contains(strings.ToLower(line), "failsafe")
			continue
		}
		if inFailsafe {
			continue
		}

		// kernel /vmlinuz-VERSION [args...]
		if strings.HasPrefix(line, "kernel ") {
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}
			// parts[1] is e.g. /vmlinuz-3.0.101-63-default
			kernelBase := filepath.Base(parts[1])
			kernelVer = strings.TrimPrefix(kernelBase, "vmlinuz-")
			if len(parts) > 2 {
				kernelArgs = strings.Join(parts[2:], " ")
			}
		}

		// initrd /initrd-VERSION
		if strings.HasPrefix(line, "initrd ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				initrdName = filepath.Base(parts[1])
			}
		}

		if kernelVer != "" && initrdName != "" {
			return
		}
	}
	if kernelVer == "" {
		err = fmt.Errorf("no non-failsafe kernel entry found in menu.lst")
	}
	return
}

// buildGrubEFIConfig produces a grub.cfg that loads kernel and initrd directly
// from the ESP root — no search command, because GRUB EFI on KVM/OpenStack can
// only see the disk it booted from (the ESP).
//
//   - kernelVer  e.g. "3.0.101-63-default"
//   - initrdName e.g. "initrd-3.0.101-63-default"
//   - kernelArgs kernel command line extracted from the original menu.lst
func buildGrubEFIConfig(kernelVer, initrdName, kernelArgs string) string {
	failsafeArgs := buildFailsafeArgs(kernelArgs)
	return fmt.Sprintf(`set timeout=5
set default=0

menuentry "SLES 11 SP4" {
    linuxefi /vmlinuz-%s %s
    initrdefi /%s
}

menuentry "SLES 11 SP4 Failsafe" {
    linuxefi /vmlinuz-%s %s
    initrdefi /%s
}
`, kernelVer, kernelArgs, initrdName,
		kernelVer, failsafeArgs, initrdName)
}

// buildFailsafeArgs appends the standard SLES 11 failsafe kernel options to
// the normal args, skipping any that are already present.
func buildFailsafeArgs(normalArgs string) string {
	extras := []string{
		"ide=nodma", "apm=off", "noresume", "edd=off",
		"powersaved=off", "nohz=off", "highres=off",
		"processor.max_cstate=1", "nomodeset", "x11failsafe",
	}
	present := make(map[string]bool)
	for _, tok := range strings.Fields(normalArgs) {
		present[strings.SplitN(tok, "=", 2)[0]] = true
	}
	var toAdd []string
	for _, e := range extras {
		key := strings.SplitN(e, "=", 2)[0]
		if !present[key] {
			toAdd = append(toAdd, e)
		}
	}
	if len(toAdd) == 0 {
		return normalArgs
	}
	return normalArgs + " " + strings.Join(toAdd, " ")
}

// ── Phase 2: ESP disk formatting ─────────────────────────────────────────────

// FormatESPDisk partitions a blank Cinder volume as GPT with one FAT32 EFI
// System Partition and returns the partition device name and its filesystem UUID.
// The disk must already be attached to the pod (i.e. disk.Path is populated).
func FormatESPDisk(espDiskPath string) (partition, uuid string, err error) {
	log.Printf("FormatESPDisk: partitioning and formatting %s as GPT/FAT32", espDiskPath)

	// Create GPT table + one FAT32 partition spanning the disk.
	// Use sectors: start at 2048 (1 MiB) and end at -2048 (leave room for
	// backup GPT at the end).
	script := `run
part-init /dev/sda gpt
part-add /dev/sda primary 2048 -2048
mkfs vfat /dev/sda1
set-gpt-type /dev/sda 1 C12A7328-F81F-11D2-BA4B-00A0C93EC93B
`
	if _, err = runESPGuestfish(espDiskPath, true, script); err != nil {
		return "", "", fmt.Errorf("FormatESPDisk: partition/format failed: %v", err)
	}

	// Detect the partition name (always /dev/sda1 for a fresh single-partition disk).
	partition, err = getESPPartition(espDiskPath)
	if err != nil {
		return "", "", fmt.Errorf("FormatESPDisk: could not detect partition: %v", err)
	}

	// Retrieve the FAT32 filesystem UUID assigned by mkfs.vfat.
	uuid, err = getESPUUID(espDiskPath, partition)
	if err != nil {
		return "", "", fmt.Errorf("FormatESPDisk: could not get UUID: %v", err)
	}

	log.Printf("FormatESPDisk: ready — partition=%s UUID=%s", partition, uuid)
	return partition, uuid, nil
}

// runESPGuestfish runs a multi-step guestfish script against a single ESP disk
// without the -i flag (the ESP has no inspectable OS, only a FAT32 partition).
func runESPGuestfish(espDiskPath string, write bool, script string) (string, error) {
	os.Setenv("LIBGUESTFS_BACKEND", "direct")
	option := "--ro"
	if write {
		option = "--rw"
	}
	cmd := exec.Command("guestfish", option, "-a", espDiskPath)
	cmd.Stdin = strings.NewReader(script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	log.Printf("runESPGuestfish: executing on %s (write=%v)", espDiskPath, write)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("guestfish ESP script failed: %v\nstderr: %s",
			err, strings.TrimSpace(stderr.String()))
	}
	if stderr.Len() > 0 {
		log.Printf("runESPGuestfish stderr: %s", strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// getESPPartition returns the device path of the first partition on the ESP disk.
func getESPPartition(espDiskPath string) (string, error) {
	out, err := runESPGuestfish(espDiskPath, false, "run\nlist-partitions\n")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line, nil
		}
	}
	return "", fmt.Errorf("no partitions found on ESP disk %s", espDiskPath)
}

// getESPUUID returns the filesystem UUID of the given ESP FAT32 partition.
func getESPUUID(espDiskPath, partition string) (string, error) {
	script := fmt.Sprintf("run\nvfs-uuid %s\n", partition)
	out, err := runESPGuestfish(espDiskPath, false, script)
	if err != nil {
		return "", err
	}
	uuid := strings.TrimSpace(out)
	if uuid == "" {
		return "", fmt.Errorf("empty UUID returned for %s on %s", partition, espDiskPath)
	}
	return uuid, nil
}

// ── Phase 3: ESP population ───────────────────────────────────────────────────

// PopulateESP extracts the bundled GRUB 2 EFI module tree onto the ESP,
// installs BOOTX64.EFI, and writes grub.cfg to both expected locations.
//
//   - espDiskPath   block device path of the ESP Cinder volume
//   - grubCfgPath   path to a grub.cfg file on the host to upload
func PopulateESP(espDiskPath, grubCfgPath string) error {
	if _, err := os.Stat(grub2EFITarPath); err != nil {
		return fmt.Errorf("PopulateESP: GRUB2 EFI tarball not found at %s: %v", grub2EFITarPath, err)
	}

	// Stage the tarball in /tmp before invoking guestfish.  When guestfish
	// starts the libguestfs supermin appliance in direct backend mode, the
	// appliance's guestfs daemon and the guestfish client process may not be
	// able to resolve paths outside /tmp in certain pod security contexts.
	// Staging ensures the file is always reachable by the guestfish client.
	stagedTar, err := stageFileForGuestfish(grub2EFITarPath, "grub2-efi-*.tar.gz")
	if err != nil {
		return fmt.Errorf("PopulateESP: cannot stage GRUB2 EFI tarball: %v", err)
	}
	defer os.Remove(stagedTar)

	partition, err := getESPPartition(espDiskPath)
	if err != nil {
		return fmt.Errorf("PopulateESP: could not detect ESP partition: %v", err)
	}

	// tar-in extracts the tarball (x86_64-efi/ prefix) under /EFI/BOOT/,
	// giving /EFI/BOOT/x86_64-efi/*.mod and /EFI/BOOT/x86_64-efi/grub.efi.
	script := fmt.Sprintf(`run
mount %s /
mkdir-p /EFI/BOOT/x86_64-efi
mkdir-p /boot/grub2
tar-in %s /EFI/BOOT compress:gzip
cp /EFI/BOOT/x86_64-efi/grub.efi /EFI/BOOT/BOOTX64.EFI
upload %s /EFI/BOOT/grub.cfg
upload %s /boot/grub2/grub.cfg
`,
		partition,
		stagedTar,
		grubCfgPath,
		grubCfgPath,
	)
	if _, err := runESPGuestfish(espDiskPath, true, script); err != nil {
		return fmt.Errorf("PopulateESP: failed: %v", err)
	}
	log.Printf("PopulateESP: GRUB2 EFI files installed on %s", espDiskPath)
	return nil
}

// ── Phase 4: OS disk fixups ───────────────────────────────────────────────────

// SetupOSDiskForUEFI makes the three pre-conversion changes needed on the OS
// disks so that virt-v2v and the resulting VM work correctly:
//
//  1. Appends a /boot/efi fstab entry (with the ESP's UUID) so guestfish -i
//     mounts the ESP and virt-v2v can find it.
//  2. Injects a no-op /usr/sbin/grub2-mkconfig stub so virt-v2v does not abort
//     when it detects a GRUB 2 EFI config but cannot find the binary.
//  3. Appends INITRD_MODULES="virtio virtio_pci virtio_blk" to
//     /etc/sysconfig/kernel so that mkinitrd (called internally by virt-v2v)
//     includes the virtio block driver in the rebuilt initrd.
func SetupOSDiskForUEFI(osdisks []vm.VMDisk, espUUID string) error {
	if err := addEFIFstabEntry(osdisks, espUUID); err != nil {
		return err
	}
	if err := injectGrub2MkconfigStub(osdisks); err != nil {
		return err
	}
	if err := addVirtioToInitrdModules(osdisks); err != nil {
		return err
	}
	return nil
}

func addEFIFstabEntry(osdisks []vm.VMDisk, espUUID string) error {
	fstab, err := RunCommandInGuestAllVolumes(osdisks, "cat", false, "/etc/fstab")
	if err != nil {
		return fmt.Errorf("addEFIFstabEntry: cannot read /etc/fstab: %v", err)
	}
	if strings.Contains(fstab, "/boot/efi") {
		log.Printf("addEFIFstabEntry: /boot/efi already in fstab, skipping")
		return nil
	}
	entry := fmt.Sprintf("UUID=%s\t/boot/efi\tvfat\tdefaults\t0\t0\n", espUUID)
	if out, err := RunCommandInGuestAllVolumes(osdisks, "write-append", true,
		"/etc/fstab", entry); err != nil {
		return fmt.Errorf("addEFIFstabEntry: failed to update fstab: %v: %s", err, out)
	}
	log.Printf("addEFIFstabEntry: added /boot/efi (UUID=%s) to fstab", espUUID)
	return nil
}

func injectGrub2MkconfigStub(osdisks []vm.VMDisk) error {
	// If a real binary is already present (non-stub), leave it alone.
	existing, _ := RunCommandInGuestAllVolumes(osdisks, "cat", false, "/usr/sbin/grub2-mkconfig")
	if strings.TrimSpace(existing) != "" && !strings.Contains(existing, "exit 0") {
		log.Printf("injectGrub2MkconfigStub: real grub2-mkconfig already present, skipping")
		return nil
	}

	stub := "#!/bin/bash\nexit 0\n"
	f, err := os.CreateTemp("", "grub2-mkconfig-stub-*")
	if err != nil {
		return fmt.Errorf("injectGrub2MkconfigStub: cannot create temp file: %v", err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(stub); err != nil {
		return err
	}
	f.Close()

	if out, err := RunCommandInGuestAllVolumes(osdisks, "upload", true,
		f.Name(), "/usr/sbin/grub2-mkconfig"); err != nil {
		return fmt.Errorf("injectGrub2MkconfigStub: upload failed: %v: %s", err, out)
	}
	if out, err := RunCommandInGuestAllVolumes(osdisks, "chmod", true,
		"0755", "/usr/sbin/grub2-mkconfig"); err != nil {
		return fmt.Errorf("injectGrub2MkconfigStub: chmod failed: %v: %s", err, out)
	}
	log.Printf("injectGrub2MkconfigStub: stub installed at /usr/sbin/grub2-mkconfig")
	return nil
}

func addVirtioToInitrdModules(osdisks []vm.VMDisk) error {
	sysconf, err := RunCommandInGuestAllVolumes(osdisks, "cat", false, "/etc/sysconfig/kernel")
	if err != nil {
		return fmt.Errorf("addVirtioToInitrdModules: cannot read /etc/sysconfig/kernel: %v", err)
	}
	if strings.Contains(sysconf, "virtio_blk") {
		log.Printf("addVirtioToInitrdModules: virtio_blk already present, skipping")
		return nil
	}
	// Appending overrides any earlier INITRD_MODULES line when the file is
	// sourced as a shell script.
	line := "INITRD_MODULES=\"virtio virtio_pci virtio_blk\"\n"
	if out, err := RunCommandInGuestAllVolumes(osdisks, "write-append", true,
		"/etc/sysconfig/kernel", line); err != nil {
		return fmt.Errorf("addVirtioToInitrdModules: update failed: %v: %s", err, out)
	}
	log.Printf("addVirtioToInitrdModules: virtio_blk added to INITRD_MODULES")
	return nil
}

// ── Phase 5: Post-conversion kernel+initrd copy ───────────────────────────────

// CopyKernelInitrdToESP copies the latest kernel and initrd from /boot (on the
// OS disk) to the ESP root so GRUB 2 EFI can load them at boot time.
// Call after ConvertDisk — virt-v2v will have rebuilt the initrd with virtio
// drivers by then.
// It also rewrites grub.cfg on the ESP to use the exact filenames found.
func CopyKernelInitrdToESP(disks []vm.VMDisk, espDiskIndex int) error {
	log.Printf("CopyKernelInitrdToESP: copying kernel+initrd from /boot to ESP (disk %d)", espDiskIndex)

	espDiskPath := disks[espDiskIndex].Path
	osdisks := disksExcludingIndex(disks, espDiskIndex)

	// Find the latest non-rescue kernel.
	kernelOut, err := RunCommandInGuestAllVolumes(osdisks, "sh", false,
		"ls /boot/vmlinuz-*-default 2>/dev/null | grep -v rescue | sort -V | tail -1")
	if err != nil || strings.TrimSpace(kernelOut) == "" {
		return fmt.Errorf("CopyKernelInitrdToESP: no kernel found in /boot: %v", err)
	}
	kernelPath := strings.TrimSpace(kernelOut)
	kernelBase := filepath.Base(kernelPath)
	kernelVer := strings.TrimPrefix(kernelBase, "vmlinuz-")

	// Find the matching initrd.
	initrdOut, err := RunCommandInGuestAllVolumes(osdisks, "sh", false,
		fmt.Sprintf("ls /boot/initrd-%s 2>/dev/null | head -1", kernelVer))
	if err != nil || strings.TrimSpace(initrdOut) == "" {
		// Fallback: any initrd matching the version prefix (minus -default).
		prefix := strings.TrimSuffix(kernelVer, "-default")
		initrdOut, err = RunCommandInGuestAllVolumes(osdisks, "sh", false,
			fmt.Sprintf("ls /boot/initrd-*%s* 2>/dev/null | grep -v rescue | sort -V | tail -1", prefix))
		if err != nil || strings.TrimSpace(initrdOut) == "" {
			return fmt.Errorf("CopyKernelInitrdToESP: no initrd found for %s: %v", kernelVer, err)
		}
	}
	initrdPath := strings.TrimSpace(initrdOut)
	initrdBase := filepath.Base(initrdPath)
	log.Printf("CopyKernelInitrdToESP: kernel=%s initrd=%s", kernelBase, initrdBase)

	// Read the existing grub.cfg kernel args from the ESP (written in phase 3).
	espPartition, err := getESPPartition(espDiskPath)
	if err != nil {
		return fmt.Errorf("CopyKernelInitrdToESP: cannot detect ESP partition: %v", err)
	}
	kernelArgs := readGrubCfgKernelArgs(espDiskPath, espPartition)

	// Download kernel + initrd to host temp files.
	kernelTmp, err := os.CreateTemp("", "vmlinuz-*")
	if err != nil {
		return err
	}
	kernelTmp.Close()
	defer os.Remove(kernelTmp.Name())

	initrdTmp, err := os.CreateTemp("", "initrd-*")
	if err != nil {
		return err
	}
	initrdTmp.Close()
	defer os.Remove(initrdTmp.Name())

	if out, err := RunCommandInGuestAllVolumes(osdisks, "download", false,
		kernelPath, kernelTmp.Name()); err != nil {
		return fmt.Errorf("CopyKernelInitrdToESP: kernel download failed: %v: %s", err, out)
	}
	if out, err := RunCommandInGuestAllVolumes(osdisks, "download", false,
		initrdPath, initrdTmp.Name()); err != nil {
		return fmt.Errorf("CopyKernelInitrdToESP: initrd download failed: %v: %s", err, out)
	}

	// Rebuild grub.cfg with actual filenames, preserving kernel args.
	grubCfg := buildGrubEFIConfig(kernelVer, initrdBase, kernelArgs)
	grubCfgTmp, err := os.CreateTemp("", "grub-*.cfg")
	if err != nil {
		return err
	}
	defer os.Remove(grubCfgTmp.Name())
	if _, err := grubCfgTmp.WriteString(grubCfg); err != nil {
		return err
	}
	grubCfgTmp.Close()

	// Upload kernel, initrd, updated grub.cfg to ESP.
	script := fmt.Sprintf(`run
mount %s /
upload %s /%s
upload %s /%s
upload %s /EFI/BOOT/grub.cfg
upload %s /boot/grub2/grub.cfg
`,
		espPartition,
		kernelTmp.Name(), kernelBase,
		initrdTmp.Name(), initrdBase,
		grubCfgTmp.Name(),
		grubCfgTmp.Name(),
	)
	if _, err := runESPGuestfish(espDiskPath, true, script); err != nil {
		return fmt.Errorf("CopyKernelInitrdToESP: upload to ESP failed: %v", err)
	}

	log.Printf("CopyKernelInitrdToESP: done — %s and %s on ESP, grub.cfg updated", kernelBase, initrdBase)
	return nil
}

// readGrubCfgKernelArgs reads the linuxefi kernel args from the existing
// grub.cfg on the ESP.  Returns empty string on any error (caller falls back).
func readGrubCfgKernelArgs(espDiskPath, partition string) string {
	script := fmt.Sprintf("run\nmount %s /\ncat /EFI/BOOT/grub.cfg\n", partition)
	out, err := runESPGuestfish(espDiskPath, false, script)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "linuxefi ") {
			parts := strings.Fields(line)
			// parts[0]="linuxefi" parts[1]="/vmlinuz-VERSION" parts[2:]="args..."
			if len(parts) > 2 {
				return strings.Join(parts[2:], " ")
			}
			return ""
		}
	}
	return ""
}

// ── Orchestration helper (called from migrate.go) ────────────────────────────

// SetupLegacySUSEPreConversion is the single entry-point called by migrate.go
// after FormatESPDisk.  It:
//  1. Reads /boot/grub/menu.lst from the OS disks to extract the kernel version
//     and command-line arguments (root=, splash=, etc.) exactly as configured on
//     the source VM — no assumptions are made about the layout.
//  2. Builds grub.cfg from those values and populates the ESP.
//  3. Applies the three OS-disk fixups needed before virt-v2v runs:
//     fstab /boot/efi entry, grub2-mkconfig no-op stub, INITRD_MODULES virtio.
//
// Parameters:
//
//	espDiskPath – block device path of the freshly formatted ESP Cinder volume.
//	osdisks     – all OS VMDisk entries (ESP disk not included).
//	espUUID     – UUID of the FAT32 filesystem on the ESP (returned by FormatESPDisk).
func SetupLegacySUSEPreConversion(espDiskPath string, osdisks []vm.VMDisk, espUUID string) error {
	menuLst, err := RunCommandInGuestAllVolumes(osdisks, "cat", false, "/boot/grub/menu.lst")
	if err != nil || strings.TrimSpace(menuLst) == "" {
		return fmt.Errorf("SetupLegacySUSEPreConversion: failed to read /boot/grub/menu.lst: %v", err)
	}
	kernelVer, kernelArgs, initrdName, err := parseSLESMenuLst(menuLst)
	if err != nil {
		return fmt.Errorf("SetupLegacySUSEPreConversion: failed to parse menu.lst: %v", err)
	}
	log.Printf("SetupLegacySUSEPreConversion: kernel=%s initrd=%s args=%q", kernelVer, initrdName, kernelArgs)

	grubCfg := buildGrubEFIConfig(kernelVer, initrdName, kernelArgs)
	grubCfgFile, err := os.CreateTemp("", "grub-*.cfg")
	if err != nil {
		return fmt.Errorf("SetupLegacySUSEPreConversion: cannot create temp grub.cfg: %v", err)
	}
	defer os.Remove(grubCfgFile.Name())
	if _, err := grubCfgFile.WriteString(grubCfg); err != nil {
		return err
	}
	grubCfgFile.Close()

	if err := PopulateESP(espDiskPath, grubCfgFile.Name()); err != nil {
		return fmt.Errorf("SetupLegacySUSEPreConversion: ESP population failed: %v", err)
	}
	log.Printf("SetupLegacySUSEPreConversion: ESP populated with GRUB2 EFI files")

	if err := SetupOSDiskForUEFI(osdisks, espUUID); err != nil {
		return fmt.Errorf("SetupLegacySUSEPreConversion: OS disk fixup failed: %v", err)
	}
	log.Printf("SetupLegacySUSEPreConversion: fstab, grub2-mkconfig stub, INITRD_MODULES updated")
	return nil
}

// ── internal helpers ──────────────────────────────────────────────────────────

// disksExcludingIndex returns a copy of disks with the element at index omitted.
func disksExcludingIndex(disks []vm.VMDisk, index int) []vm.VMDisk {
	result := make([]vm.VMDisk, 0, len(disks)-1)
	for i, d := range disks {
		if i != index {
			result = append(result, d)
		}
	}
	return result
}

// stageFileForGuestfish copies src to a new temp file in the default temp
// directory (/tmp) and returns its path.  The caller is responsible for
// removing the file when done (defer os.Remove).
//
// guestfish tar-in and upload commands resolve paths relative to the process
// that runs guestfish.  In certain pod security contexts the guestfish client
// process cannot access paths outside /tmp (e.g. /home/fedora/), so we always
// stage files there before referencing them from a guestfish script.
func stageFileForGuestfish(src, pattern string) (string, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", fmt.Errorf("stageFileForGuestfish: open %s: %v", src, err)
	}
	defer in.Close()

	out, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", fmt.Errorf("stageFileForGuestfish: create temp: %v", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		os.Remove(out.Name())
		return "", fmt.Errorf("stageFileForGuestfish: copy %s: %v", src, err)
	}
	return out.Name(), nil
}
