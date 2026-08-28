// Copyright © 2024 The vjailbreak authors

package virtv2v

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/utils"
	"github.com/platform9/vjailbreak/v2v-helper/vm"
)

//go:generate mockgen -source=../virtv2v/virtv2vops.go -destination=../virtv2v/virtv2vops_mock.go -package=virtv2v

type VirtV2VOperations interface {
	RetainAlphanumeric(input string) string
	GetPartitions(disk string) ([]string, error)
	NTFSFix(path string) error
	ConvertDisk(ctx context.Context, path, ostype, virtiowindriver string, firstbootscripts []string, diskPath string, osRelease string, blockDriver string) error
	AddWildcardNetplan(path string) error
	GetOsRelease(path string) (string, error)
	AddFirstBootScript(firstbootscript, firstbootscriptname string) error
	AddUdevRules(disks []vm.VMDisk, diskPath string, interfaces []string, macs []string) error
	GetNetworkInterfaceNames(path string) ([]string, error)
	IsRHELFamily(osRelease string) (bool, error)
	GetOsReleaseAllVolumes(disks []vm.VMDisk) (string, error)
}
type FirstBootWindows struct {
	Script string
	Async  bool
}

func splitAndFilterUserScripts(content, ostype string) []string {
	if strings.TrimSpace(content) == "" {
		return nil
	}

	blocks := splitUserScriptBlocks(content)
	filtered := make([]string, 0, len(blocks))
	for _, block := range blocks {
		script, target := parseUserScriptBlock(block)
		if script == "" {
			continue
		}
		if !scriptTargetAppliesToOS(target, ostype) {
			continue
		}
		filtered = append(filtered, script)
	}

	return filtered
}

func splitUserScriptBlocks(content string) []string {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	blocks := make([]string, 0)
	current := make([]string, 0)

	flush := func() {
		block := strings.TrimSpace(strings.Join(current, "\n"))
		if block != "" {
			blocks = append(blocks, block)
		}
		current = current[:0]
	}

	for _, line := range lines {
		if strings.TrimSpace(line) == constants.NextScriptDelimiterLine {
			flush()
			continue
		}
		current = append(current, line)
	}
	flush()

	if len(blocks) == 0 {
		only := strings.TrimSpace(content)
		if only != "" {
			return []string{only}
		}
	}

	return blocks
}

func parseUserScriptBlock(block string) (string, string) {
	lines := strings.Split(block, "\n")
	for idx, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		tagLine := strings.ToUpper(trimmed)
		switch {
		case strings.HasPrefix(tagLine, "// "+constants.LinuxTag), strings.HasPrefix(tagLine, "# "+constants.LinuxTag):
			return strings.TrimSpace(strings.Join(append(lines[:idx], lines[idx+1:]...), "\n")), constants.LinuxTag
		case strings.HasPrefix(tagLine, "// "+constants.WindowsTag), strings.HasPrefix(tagLine, "# "+constants.WindowsTag):
			return strings.TrimSpace(strings.Join(append(lines[:idx], lines[idx+1:]...), "\n")), constants.WindowsTag
		default:
			return strings.TrimSpace(block), ""
		}
	}

	return "", ""
}

func scriptTargetAppliesToOS(target, ostype string) bool {
	normalizedOS := strings.ToLower(strings.TrimSpace(ostype))
	isWindows := normalizedOS == constants.OSFamilyWindows || strings.Contains(normalizedOS, "windows")
	isLinux := normalizedOS == constants.OSFamilyLinux || strings.Contains(normalizedOS, "linux")

	switch target {
	case constants.LinuxTag:
		return isLinux
	case constants.WindowsTag:
		return isWindows
	default:
		return true
	}
}

// prepareLinuxUserFirstBootWrapper builds a single Bash wrapper script for Linux guests,
// embedding filtered user post-migration script blocks inline via heredocs.
func prepareLinuxUserFirstBootWrapper(ostype string) (string, error) {
	userScriptPath := "/home/fedora/scripts/user_firstboot.sh"
	userScriptWorkDir := "/tmp/vjailbreak-user-firstboot"
	content, err := os.ReadFile(userScriptPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read user firstboot script: %w", err)
	}

	scripts := splitAndFilterUserScripts(string(content), ostype)
	if len(scripts) == 0 {
		log.Printf("No user post-migration scripts applicable for OS '%s'", ostype)
		return "", nil
	}

	if err := os.MkdirAll(userScriptWorkDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create Linux user script work dir: %w", err)
	}

	var wrapper strings.Builder
	wrapper.WriteString("#!/bin/bash\n")
	wrapper.WriteString("set +e\n")

	for idx, script := range scripts {
		wrapper.WriteString(fmt.Sprintf("echo \"-> running user script part %d\"\n", idx+1))
		heredocMarker := fmt.Sprintf("VJ_USER_SCRIPT_PART_%03d", idx+1)
		wrapper.WriteString(fmt.Sprintf("/bin/bash <<'%s'\n", heredocMarker))
		wrapper.WriteString(script)
		if !strings.HasSuffix(script, "\n") {
			wrapper.WriteString("\n")
		}
		wrapper.WriteString(fmt.Sprintf("%s\n", heredocMarker))
		wrapper.WriteString("rc=$?\n")
		wrapper.WriteString(fmt.Sprintf("if [ $rc -ne 0 ]; then echo \"WARNING: user script part %d failed with exit code $rc, continuing\"; fi\n", idx+1))
	}

	wrapper.WriteString("exit 0\n")

	wrapperPath := fmt.Sprintf("%s/user_firstboot_wrapper.sh", userScriptWorkDir)
	if err := os.WriteFile(wrapperPath, []byte(wrapper.String()), 0755); err != nil {
		return "", fmt.Errorf("failed to write Linux user firstboot wrapper: %w", err)
	}

	return wrapperPath, nil
}

// AddNetplanConfig uploads a provided netplan YAML into the guest at /etc/netplan/50-vj.yaml
func AddNetplanConfig(disks []vm.VMDisk, diskPath string, netplanYAML string) error {
	// Create the netplan file locally
	localPath := "/home/fedora/50-vj.yaml"
	if err := os.WriteFile(localPath, []byte(netplanYAML), 0644); err != nil {
		return fmt.Errorf("failed to create netplan yaml: %s", err)
	}
	log.Println("Created local netplan YAML")
	log.Println("Uploading netplan YAML to disk")
	os.Setenv("LIBGUESTFS_BACKEND", "direct")
	var (
		ans string
		err error
	)
	command := "upload"
	ans, err = RunCommandInGuestAllVolumes(disks, command, true, "/home/fedora/50-vj.yaml", "/etc/netplan/50-vj.yaml")
	if err != nil {
		fmt.Printf("failed to run command (%s): %v: %s\n", "upload", err, strings.TrimSpace(ans))
		return err
	}
	return nil
}

// UploadVirtIOScripts uploads the VirtIO installation scripts into the guest
func UploadVirtIOScripts(disks []vm.VMDisk, diskPath string) error {
	log.Println("Uploading VirtIO installation scripts to guest")

	// Verify the PowerShell script exists in the container
	scriptPath := "/home/fedora/install-virtio-win12.ps1"
	if _, err := os.Stat(scriptPath); err != nil {
		return fmt.Errorf("PowerShell script not found at %s: %w", scriptPath, err)
	}
	log.Printf("Found PowerShell script at %s", scriptPath)

	os.Setenv("LIBGUESTFS_BACKEND", "direct")

	var (
		ans string
		err error
	)

	// Upload PowerShell script to Windows\Temp which always exists
	log.Println("Executing guestfs upload command...")
	command := "upload"
	ans, err = RunCommandInGuestAllVolumes(disks, command, true, "/home/fedora/install-virtio-win12.ps1", "C:\\Windows\\Temp\\install-virtio-win12.ps1")
	if err != nil {
		log.Printf("Upload command failed: %v", err)
		log.Printf("Upload command output: %s", strings.TrimSpace(ans))
		return fmt.Errorf("failed to upload PowerShell script: %w: %s", err, strings.TrimSpace(ans))
	}
	log.Printf("Upload command output: %s", strings.TrimSpace(ans))

	log.Println("Successfully uploaded VirtIO installation scripts")
	return nil
}

func RetainAlphanumeric(input string) string {
	var builder strings.Builder
	for _, char := range input {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			builder.WriteRune(char)
		}
	}
	return builder.String()
}

func IsRHELFamily(osRelease string) bool {
	lowerRelease := strings.ToLower(osRelease)
	return strings.Contains(lowerRelease, "red hat") ||
		strings.Contains(lowerRelease, "rhel") ||
		strings.Contains(lowerRelease, "centos") ||
		strings.Contains(lowerRelease, "rocky") ||
		strings.Contains(lowerRelease, "alma")
}

// IsSUSEFamily returns true when osRelease identifies a SUSE / openSUSE guest.
func IsSUSEFamily(osRelease string) bool {
	lowerRelease := strings.ToLower(osRelease)
	return strings.Contains(lowerRelease, "suse") ||
		strings.Contains(lowerRelease, "sles") ||
		strings.Contains(lowerRelease, "sled") ||
		strings.Contains(lowerRelease, "opensuse")
}

// MountPersistenceScriptArgs picks generate-mount-persistence.sh flags per OS: non-SUSE uses
// --force-uuid; SUSE uses --replace-fstab --os-family=suse to skip the device.map rewrite that breaks GRUB (Error 21).
func MountPersistenceScriptArgs(osRelease string) string {
	if IsSUSEFamily(osRelease) {
		return "--replace-fstab --os-family=suse"
	}
	return "--force-uuid"
}

func GetPartitions(disk string) ([]string, error) {
	// Execute lsblk command to get partition information
	cmd := exec.Command("lsblk", "-no", "NAME", disk)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to execute lsblk: %w", err)
	}

	var partitions []string
	scanner := bufio.NewScanner(&out)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && line != disk {
			partitions = append(partitions, "/dev/"+RetainAlphanumeric(line))
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading lsblk output: %w", err)
	}

	return partitions, nil
}

func NTFSFix(path string) error {
	// Fix NTFS
	partitions, err := GetPartitions(path)
	if err != nil {
		return fmt.Errorf("failed to get partitions: %w", err)
	}
	// add arguments to ensure no dirty disk post migration
	args := []string{"--clear-dirty"}
	log.Printf("Partitions: %v", partitions)
	for _, partition := range partitions {
		if partition == path {
			continue
		}
		cmd := exec.Command("ntfsfix", append(args, partition)...)
		log.Printf("Executing %s", cmd.String())

		// Use the debug logging with proper file cleanup, into the dedicated virtv2v log file
		err := utils.RunCommandWithLogFileCategory(cmd, utils.LogCategoryVirtV2V)
		if err != nil {
			log.Printf("Skipping NTFS fix on %s", partition)
		}
		log.Printf("Fixed NTFS on %s", partition)
	}
	return nil
}

func downloadFile(url, filePath string) error {
	// Get the data from the URL
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download file: %s", err)
	}
	defer resp.Body.Close()

	// Check for successful response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// Create the file
	out, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %s", err)
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write to file: %s", err)
	}
	return nil
}

func CheckForVirtioDrivers() (bool, error) {

	// Before downloading virtio windrivers Check if iso is present in the path
	preDownloadPath := "/home/fedora/virtio-win"

	// Check if path exists
	_, err := os.Stat(preDownloadPath)
	if err != nil {
		return false, fmt.Errorf("failed to check if path exists: %s", err)
	}
	// Check if iso is present in the path
	files, err := os.ReadDir(preDownloadPath)
	if err != nil {
		return false, fmt.Errorf("failed to read directory: %s", err)
	}
	for _, file := range files {
		if file.Name() == "virtio-win.iso" {
			log.Println("Found virtio windrivers")
			return true, nil
		}
	}
	return false, nil
}

// isBareDisk returns true when path is a bare block device with no trailing
// partition digit (e.g. "/dev/sda", "/dev/vda") as opposed to a partition
// ("/dev/sda1") or LVM/device-mapper path ("/dev/vg0/lv_root").
// A bare disk is typically an LVM PV, not a directly-mountable root filesystem,
// so virt-v2v's --root flag should use "first" rather than this path.
func isBareDisk(path string) bool {
	if path == "" {
		return false
	}
	// Must start with /dev/
	if !strings.HasPrefix(path, "/dev/") {
		return false
	}
	// If it contains a slash after /dev/ it is a device-mapper or LVM path → not a bare disk
	rest := path[len("/dev/"):]
	if strings.Contains(rest, "/") {
		return false
	}
	// If the last character is a digit it is a partition (e.g. sda1, vda2)
	if len(rest) == 0 {
		return false
	}
	lastChar := rest[len(rest)-1]
	return lastChar < '0' || lastChar > '9'
}

func ConvertDisk(ctx context.Context, xmlFile, path, ostype, virtiowindriver string, firstbootscripts []string, diskPath string, osRelease string, blockDriver string) error {
	// Step 1: Handle Windows driver injection
	if strings.ToLower(ostype) == constants.OSFamilyWindows {
		filePath := "/home/fedora/virtio-win/virtio-win.iso"

		// Use Windows Server 2012-specific ISO if detected
		if isWindowsServer2012(osRelease) {
			filePath = "/home/fedora/virtio-win/virtio-win-server12.iso"
			log.Printf("Detected Windows Server 2012, using virtio-win-server12.iso")
		} else if osRelease == "" || strings.Contains(strings.ToLower(osRelease), "version unknown") {
			// The default ISO is the rolling stable build, which does not support
			// Server 2012. Falling back to it silently on a failed version probe is
			// how a 2012 guest ends up with drivers it cannot boot from.
			log.Printf("WARNING: Windows version could not be determined (%q); using %s. "+
				"If this guest is Server 2012 or 2012 R2 it will not boot - re-run once "+
				"version detection works.", osRelease, filePath)
		}

		found, err := CheckForVirtioDrivers()
		if err != nil {
			log.Printf("failed to check for virtio drivers: %s", err)
			log.Println("Downloading virtio windrivers instead of using the existing one")
		}
		if found {
			log.Println("Found virtio windrivers")
		} else {
			log.Println("Downloading virtio windrivers")
			err := downloadFile(virtiowindriver, filePath)
			if err != nil {
				return fmt.Errorf("failed to download virtio-win: %s", err)
			}
			log.Println("Downloaded virtio windrivers")
		}
		os.Setenv("VIRTIO_WIN", filePath)
	}

	// Step 2: Set guestfs backend
	os.Setenv("LIBGUESTFS_BACKEND", "direct")

	// Step 3: Prepare virt-v2v args
	args := []string{"-v", "--no-fstrim"}

	if strings.ToLower(ostype) == constants.OSFamilyLinux {
		userWrapperPath, err := prepareLinuxUserFirstBootWrapper(ostype)
		if err != nil {
			log.Printf("Warning: unable to prepare Linux user post-migration scripts; continuing without user scripts: %v", err)
		}
		if userWrapperPath != "" {
			args = append(args, "--firstboot", userWrapperPath)
		}
	}
	for _, script := range firstbootscripts {
		args = append(args, "--firstboot", fmt.Sprintf("/home/fedora/%s.sh", script))
	}
	// For Windows: select which block driver virt-v2v makes boot-critical.
	// Must match the hw_disk_bus/hw_scsi_model set on the volume so the guest
	// boots with the controller whose driver was prepared by virt-v2v.
	if strings.ToLower(ostype) == constants.OSFamilyWindows && blockDriver != "" {
		args = append(args, "--block-driver", blockDriver)
	}

	// Always use libvirtxml mode to convert all disks
	args = append(args, "-i", "libvirtxml", xmlFile)
	if strings.ToLower(ostype) != constants.OSFamilyWindows {
		// --root expects a root *filesystem* device, not a raw disk or LVM PV.
		// get-bootable-partition.sh may return a bare disk name (e.g. /dev/sda) when
		// GRUB's MBR signature is on an LVM PV disk that has no mountable partition.
		// In that case "first" is safer: virt-v2v will auto-detect the root LV via
		// libguestfs inspection (which traverses LVM, RAID, etc. automatically).
		rootArg := path
		if isBareDisk(path) {
			log.Printf("ConvertDisk: --root path %q looks like a bare disk (LVM PV); using 'first' for auto-detection", path)
			rootArg = "first"
		}
		args = append(args, "--root", rootArg)
	}

	start := time.Now()
	// Step 5: Run virt-v2v-in-place

	cmd := exec.CommandContext(ctx, "virt-v2v-in-place", args...)
	log.Printf("Executing %s", cmd.String())

	// Use the debug logging with proper file cleanup, into the dedicated virtv2v log file
	err := utils.RunCommandWithLogFileCategory(cmd, utils.LogCategoryVirtV2V)
	duration := time.Since(start)

	if err != nil {
		// virt-v2v-in-place's own error string is rarely more than "exit
		// status 1". The actionable root cause (e.g. insufficient free
		// disk space, filesystem corruption caught by e2fsck) is only
		// written to the dedicated virtv2v debug log, so pull the most
		// relevant line out of it and fold it into the returned error.
		if reason := virtV2VFailureReasonFromLatestLog(); reason != "" {
			return fmt.Errorf("failed to run virt-v2v-in-place: %s: %s", err, reason)
		}
		return fmt.Errorf("failed to run virt-v2v-in-place: %s", err)
	}
	log.Printf("virt-v2v-in-place conversion took: %s", duration)

	return nil
}

// virtV2VFailureReasonFromLatestLog looks up the debug log file that was just
// written for the current migration's virt-v2v invocation and extracts a
// concise, human-readable root-cause line from it. Returns "" if the
// migration name or log file can't be resolved, or no error line is found -
// in which case callers should fall back to the raw process error.
func virtV2VFailureReasonFromLatestLog() string {
	migrationName, err := utils.GetMigrationObjectName()
	if err != nil {
		return ""
	}

	logPath, err := utils.GetLatestLogFilePath(migrationName, utils.LogCategoryVirtV2V)
	if err != nil {
		return ""
	}

	return utils.ExtractVirtV2VFailureReason(logPath)
}

// osReleaseCatSteps builds one tolerant `cat` per candidate release file,
// tried in one appliance boot instead of up to three.
func osReleaseCatSteps() []guestfishStep {
	steps := make([]guestfishStep, len(constants.OSReleaseCandidateFiles))
	for i, file := range constants.OSReleaseCandidateFiles {
		steps[i] = guestfishStep{Command: "cat", Args: []string{file}, Marker: constants.OSReleaseMarkers[i]}
	}
	return steps
}

// getOsReleaseFromDisks is the shared implementation behind GetOsRelease and
// GetOsReleaseAllVolumes: read all candidate files in one tolerant batch
// (see guestfishStep) and return the first that reads back real content.
func getOsReleaseFromDisks(disks []vm.VMDisk) (string, error) {
	out, err := RunGuestfishScript(disks, false, constants.GuestfishOutputCombinedRaw, osReleaseCatSteps()...)
	if err != nil {
		return "", fmt.Errorf("failed to get OS release: %w", err)
	}
	return pickOsRelease(splitByMarker(out, constants.OSReleaseMarkers))
}

// pickOsRelease is getOsReleaseFromDisks's decision logic, pulled out for
// unit testing: pick the first candidate whose section looks like real
// content rather than a "file does not exist" error or no output at all.
func pickOsRelease(sections map[string]string) (string, error) {
	var errs []string
	for i, file := range constants.OSReleaseCandidateFiles {
		section := sections[constants.OSReleaseMarkers[i]]
		if section != "" && !strings.Contains(strings.ToLower(section), "no such file or directory") {
			return strings.ToLower(section), nil
		}
		if section == "" {
			// No content or error text for this candidate - treat as "not
			// found" (same empty-section convention as splitByMarker).
			section = file + ": no output"
		}
		errs = append(errs, section)
	}

	return "", fmt.Errorf("failed to get OS release from %v: %v",
		strings.Join(constants.OSReleaseCandidateFiles, ", "), strings.Join(errs, " | "))
}

func GetOsRelease(path string) (string, error) {
	os.Setenv("LIBGUESTFS_BACKEND", "direct")
	return getOsReleaseFromDisks([]vm.VMDisk{{Path: path}})
}
func InjectMacToIps(disks []vm.VMDisk, diskPath string, guestNetworks []vjailbreakv1alpha1.GuestNetwork, gatewayIP map[string]string, ipPerMac map[string][]vm.IpEntry, osType string) error {
	// Add wildcard to netplan
	macToIPs := ipPerMac
	// log the macToIPs
	log.Println("Mac to IP map:", macToIPs)
	var macToIPsFile string
	switch osType {
	case constants.OSFamilyLinux:
		macToIPsFile = "/home/fedora/macToIP"
	case constants.OSFamilyWindows:
		macToIPsFile = "/home/fedora/NIC-Recovery/macToIP"
	}
	f, err := os.Create(macToIPsFile)
	if err != nil {
		return err
	}
	defer f.Close()
	for mac, ips := range macToIPs {
		if len(ips) > 0 {
			_, err := fmt.Fprintf(f, "%s:ip:%s\n", mac, ips[0].IP)
			if err != nil {
				return err
			}
		} else if len(ips) == 0 {
			_, err := fmt.Fprintf(f, "%s:ip:%s\n", mac, "")
			if err != nil {
				return err
			}
		}
	}

	// Construct YAML
	log.Println("Created macToIP file with entries")
	if osType == constants.OSFamilyLinux {

		os.Setenv("LIBGUESTFS_BACKEND", "direct")
		var ans string
		command := "upload"
		ans, err = RunCommandInGuestAllVolumes(disks, command, true, "/home/fedora/macToIP", "/etc/macToIP")
		if err != nil {
			log.Printf("failed to upload macToIP file: %v: %s", err, strings.TrimSpace(ans))
			return fmt.Errorf("failed to upload macToIP file: %w: %s", err, strings.TrimSpace(ans))
		}
	}
	return nil
}

func AddWildcardNetplanForL2(disks []vm.VMDisk, diskPath string) error {
	// Upload it to the disk
	os.Setenv("LIBGUESTFS_BACKEND", "direct")
	var ans string
	var err error
	command := "upload"
	ans, err = RunCommandInGuestAllVolumes(disks, command, true, "/home/fedora/99-l2-Netplan.yaml", "/etc/netplan/99-l2-Netplan.yaml")
	if err != nil {
		log.Printf("failed to upload netplan file: %v: %s", err, strings.TrimSpace(ans))
		return fmt.Errorf("failed to upload netplan file: %w: %s", err, strings.TrimSpace(ans))
	}
	return nil

}

// buildWildcardNetplanYAML constructs the netplan YAML written by
// AddWildcardNetplan, split out as a pure function so the DHCP-vs-static
// decision can be unit tested without touching a guest disk.
//
// For each MAC, entries are partitioned into DHCP-sourced ones (IpEntry.DHCP
// == true — a live OpenStack/Neutron auto-allocation, e.g. fallback-to-DHCP
// or a subnet-mismatch DHCP fallback) and static ones (a preserved or
// user-assigned IP). DHCP-sourced entries get `dhcp4: true` so the guest
// actually performs a DHCP handshake — pinning that IP as a static address
// instead would leave some networks unreachable despite Neutron holding the
// right fixed_ip, since port security on some backends ties the IP-to-port
// binding to an observed lease, not just a fixed_ip match. Static entries
// keep the existing `dhcp4: false` + explicit `addresses:` behavior. A MAC
// with both (uncommon, e.g. one preserved IP plus one DHCP-fallback IP on
// the same NIC) gets both stanzas together.
//
// A MAC's `routes:`/`nameservers:` are only written when it has at least one
// static entry — they're only meaningful alongside a static address we
// determined ourselves. A purely DHCP-sourced MAC (no static entries at all)
// gets neither: the DHCP client owns the gateway and DNS entirely, so
// writing our own default route on top would fight with (or go stale
// relative to) whatever the lease actually provides, and any carried-over
// DNS servers come from the source VM's original network, which may not
// even be reachable from the target network.
func buildWildcardNetplanYAML(guestNetworks []vjailbreakv1alpha1.GuestNetwork, gatewayIP map[string]string, ipPerMac map[string][]vm.IpEntry) string {
	macToIPs := ipPerMac
	macToDNS := make(map[string][]string)
	if len(guestNetworks) > 0 {
		for _, gn := range guestNetworks {
			if strings.Contains(gn.IP, ":") { // skip IPv6 here
				continue
			}
			if len(gn.DNS) > 0 {
				// ipPerMac keys are canonical lowercase; match that case
				macToDNS[strings.ToLower(gn.MAC)] = gn.DNS
			}
		}
	}

	// Construct YAML
	var b strings.Builder
	b.WriteString("network:\n")
	b.WriteString("  version: 2\n")
	b.WriteString("  renderer: networkd\n")
	b.WriteString("  ethernets:\n")
	idx := 0
	routesAdded := false
	log.Printf("MAC GATEWAY : %v", gatewayIP)
	for mac, entries := range macToIPs {
		if len(entries) == 0 {
			continue
		}
		var staticEntries []vm.IpEntry
		useDHCP := false
		for _, e := range entries {
			if e.DHCP {
				useDHCP = true
			} else {
				staticEntries = append(staticEntries, e)
			}
		}

		id := fmt.Sprintf("vj%d", idx)
		b.WriteString(fmt.Sprintf("    %s:\n", id))
		b.WriteString("      match:\n")
		b.WriteString(fmt.Sprintf("        macaddress: %s\n", mac))
		if useDHCP {
			b.WriteString("      dhcp4: true\n")
		} else {
			b.WriteString("      dhcp4: false\n")
		}
		// Gateway and DNS are only meaningful alongside a static address we
		// determined ourselves (preserved or custom IP). A MAC with no
		// static entries is purely DHCP-sourced, so the DHCP client owns
		// the gateway and DNS entirely — writing our own routes/nameservers
		// on top would fight with (or go stale relative to) whatever the
		// DHCP lease actually provides, and any carried-over DNS servers
		// here come from the source VM's original network, which may not
		// even be reachable from the target network.
		if len(staticEntries) > 0 {
			b.WriteString("      addresses:\n")
			for _, e := range staticEntries {
				// default prefix to 24 if zero
				prefix := e.Prefix
				if prefix == 0 {
					prefix = 24
				}
				b.WriteString(fmt.Sprintf("        - %s/%d\n", e.IP, prefix))
			}
			if gateway, ok := gatewayIP[mac]; ok && gateway != "" {
				if !routesAdded {
					log.Printf("Writing default routes")
					b.WriteString("      routes:\n")
					b.WriteString("        - to: default\n")
					b.WriteString(fmt.Sprintf("          via: %s\n", gateway))
					routesAdded = true
				}
			}
			if dns, ok := macToDNS[mac]; ok && len(dns) > 0 {
				b.WriteString("      nameservers:\n")
				b.WriteString("        addresses:\n")
				for _, d := range dns {
					b.WriteString(fmt.Sprintf("          - %s\n", d))
				}
			}
		}
		idx++
	}
	if !routesAdded {
		log.Println("WARNING: No gateway found")
	}
	return b.String()
}

// wildcardNetplanSteps builds the mv+mkdir+upload sequence
// AddWildcardNetplan runs in one boot instead of three: back up
// /etc/netplan, recreate it empty, then upload the wildcard config.
func wildcardNetplanSteps() []guestfishStep {
	return []guestfishStep{
		{Command: "mv", Args: []string{"/etc/netplan", "/etc/netplan-bkp"}},
		{Command: "mkdir", Args: []string{"/etc/netplan"}},
		{Command: "upload", Args: []string{"/home/fedora/99-wildcard.network", "/etc/netplan/99-wildcard.yaml"}},
	}
}

func AddWildcardNetplan(disks []vm.VMDisk, diskPath string, guestNetworks []vjailbreakv1alpha1.GuestNetwork, gatewayIP map[string]string, ipPerMac map[string][]vm.IpEntry) error {
	netplanYAML := buildWildcardNetplanYAML(guestNetworks, gatewayIP, ipPerMac)
	log.Printf("NETPLAN YAML : %s", netplanYAML)
	// Create the netplan file
	if err := os.WriteFile("/home/fedora/99-wildcard.network", []byte(netplanYAML), 0644); err != nil {
		return fmt.Errorf("failed to create netplan file: %w", err)
	}
	log.Println("Created local netplan file")
	log.Println("Uploading netplan file to disk")

	// mv, mkdir, upload in one appliance boot instead of three; no step
	// carries a Marker, so a failure aborts the rest, same as the three
	// fail-fast calls this replaces.
	if _, err := RunGuestfishScript(disks, true, constants.GuestfishOutputLowercased, wildcardNetplanSteps()...); err != nil {
		return fmt.Errorf("failed to add wildcard netplan: %w", err)
	}
	return nil
}

func AddFirstBootScript(firstbootscript, firstbootscriptname string) error {
	// Create the firstboot script
	firstbootscriptpath := fmt.Sprintf("/home/fedora/%s.sh", firstbootscriptname)
	err := os.WriteFile(firstbootscriptpath, []byte(firstbootscript), 0644)
	if err != nil {
		return fmt.Errorf("failed to create firstboot script: %s", err)
	}
	log.Printf("Created firstboot script %s", firstbootscriptname)
	return nil
}

// Runs command inside temporary qemu-kvm that virt-v2v creates.
//
// The guest is mounted from an explicitly resolved mount plan rather than with
// guestfish's -i option. See guestfish.go for why: -i aborts on any guest whose
// root filesystem spans more than one device.
func RunCommandInGuest(path string, command string, write bool) (string, error) {
	os.Setenv("LIBGUESTFS_BACKEND", "direct")

	plan, err := resolveMountPlan([]vm.VMDisk{{Path: path}})
	if err != nil {
		return "", fmt.Errorf("failed to run command (%s): %w", command, err)
	}

	option := "--ro"
	if write {
		option = "--rw"
	}
	cmd := exec.Command(
		"guestfish",
		option,
		"-a",
		path)
	// The command text is passed through verbatim - callers already supply a
	// complete guestfish command line here, not a command plus separate args.
	cmd.Stdin = strings.NewReader(mountScript(plan, write) + command + "\n")
	log.Printf("Executing %s", cmd.String()+" "+command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to run command (%s): %v: %s", command, err, strings.TrimSpace(string(out)))
	}
	return strings.ToLower(strings.TrimSpace(string(out))), nil
}

func prepareGuestfishCommand(disks []vm.VMDisk, command string, write bool, args ...string) (*exec.Cmd, error) {
	plan, err := resolveMountPlan(disks)
	if err != nil {
		return nil, err
	}

	option := "--ro"
	if write {
		option = "--rw"
	}
	cmd := exec.Command(
		"guestfish",
		option)

	for _, disk := range disks {
		cmd.Args = append(cmd.Args, "-a", disk.Path)
	}
	// Commands go on stdin rather than after "--" so that the mount preamble can
	// be prepended, and so that a failed non-root mount can be tolerated with
	// guestfish's "-" command prefix.
	cmd.Stdin = strings.NewReader(
		mountScript(plan, write) + guestfishLine(command, args...) + "\n")
	return cmd, nil
}

func RunCommandInGuestAllVolumes(disks []vm.VMDisk, command string, write bool, args ...string) (string, error) {
	os.Setenv("LIBGUESTFS_BACKEND", "direct")
	cmd, err := prepareGuestfishCommand(disks, command, write, args...)
	if err != nil {
		return "", fmt.Errorf("failed to run command (%s): %w", command, err)
	}
	log.Printf("Executing %s -- %s", cmd.String(), guestfishLine(command, args...))
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err = cmd.Run()
	if stderrBuf.Len() > 0 {
		log.Printf("guestfish stderr (%s): %s", command, strings.TrimSpace(stderrBuf.String()))
	}
	if err != nil {
		return "", fmt.Errorf("failed to run command (%s): %v: %s", command, err, strings.TrimSpace(stderrBuf.String()))
	}
	return strings.ToLower(stdoutBuf.String()), nil
}

// IsLDMSystemVolume reports whether the guest's system volume sits on a Windows
// Dynamic Disk, and returns the root it resolved. virt-v2v documents these as
// unsupported but has no guard, so conversion produces an unbootable disk or
// wedges. Only the root is checked - data disks on dynamic disks convert fine.
// Reuses the memoised mount plan, so it costs no extra appliance boot.
func IsLDMSystemVolume(disks []vm.VMDisk) (bool, string, error) {
	plan, err := resolveMountPlan(disks)
	if err != nil {
		return false, "", fmt.Errorf("failed to resolve guest root to check for LDM: %w", err)
	}
	return isLDMDevice(plan.Root), plan.Root, nil
}

// GetDeviceNumberFromPartition returns the device index for a given
// partition name. Kept for single-partition callers; GetBootableVolumeIndex
// has its own batched implementation below (partitionDevNumSteps etc.).
func GetDeviceNumberFromPartition(disks []vm.VMDisk, partition string) (int, error) {
	command := "part-to-dev"
	device, err := RunCommandInGuestAllVolumes(disks, command, false, strings.TrimSpace(partition))
	if err != nil {
		fmt.Printf("failed to run command (%s): %v: %s\n", device, err, strings.TrimSpace(device))
		return -1, err
	}

	command = "part-to-partnum"
	num, err := RunCommandInGuestAllVolumes(disks, command, false, strings.TrimSpace(partition))
	if err != nil {
		fmt.Printf("failed to run command (%s): %v: %s\n", num, err, strings.TrimSpace(num))
		return -1, err
	}

	command = "part-get-bootable"
	bootable, err := RunCommandInGuestAllVolumes(disks, command, false, strings.TrimSpace(device), strings.TrimSpace(num))
	if err != nil {
		fmt.Printf("failed to run command (%s): %v: %s\n", bootable, err, strings.TrimSpace(bootable))
		return -1, err
	}

	if strings.TrimSpace(bootable) == "true" {
		command = "device-index"
		index, err := RunCommandInGuestAllVolumes(disks, command, false, strings.TrimSpace(device))
		if err != nil {
			fmt.Printf("failed to run command (%s): %v: %s\n", index, err, strings.TrimSpace(index))
			return -1, err
		}
		return strconv.Atoi(strings.TrimSpace(index))
	}

	return -1, errors.New("partition is not bootable")
}

// partitionDevNumMarkers returns the i-th partition's two markers for
// partitionDevNumSteps: device, then partition number.
func partitionDevNumMarkers(i int) (dev, num string) {
	return fmt.Sprintf(constants.PartitionDevMarkerTemplate, i), fmt.Sprintf(constants.PartitionNumMarkerTemplate, i)
}

// partitionDevNumSteps builds one tolerant part-to-dev + part-to-partnum per
// partition - boot 2 of GetBootableVolumeIndex's flat 3 (part-get-bootable
// and device-index need this boot's results as args, so can't join it).
func partitionDevNumSteps(partitions []string) []guestfishStep {
	steps := make([]guestfishStep, 0, len(partitions)*2)
	for i, partition := range partitions {
		partition = strings.TrimSpace(partition)
		devMarker, numMarker := partitionDevNumMarkers(i)
		steps = append(steps,
			guestfishStep{Command: "part-to-dev", Args: []string{partition}, Marker: devMarker},
			guestfishStep{Command: "part-to-partnum", Args: []string{partition}, Marker: numMarker},
		)
	}
	return steps
}

// partitionBootIndexMarkers returns the i-th partition's two markers for
// partitionBootIndexSteps: bootable flag, then device index.
func partitionBootIndexMarkers(i int) (bootable, index string) {
	return fmt.Sprintf(constants.PartitionBootMarkerTemplate, i), fmt.Sprintf(constants.PartitionIdxMarkerTemplate, i)
}

// partitionBootIndexSteps builds one tolerant part-get-bootable +
// device-index per partition - boot 3 of GetBootableVolumeIndex's flat 3.
// device-index is fetched for every partition up front so no 4th boot is needed.
func partitionBootIndexSteps(devices, nums []string) []guestfishStep {
	steps := make([]guestfishStep, 0, len(devices)*2)
	for i := range devices {
		bootMarker, idxMarker := partitionBootIndexMarkers(i)
		steps = append(steps,
			guestfishStep{Command: "part-get-bootable", Args: []string{devices[i], nums[i]}, Marker: bootMarker},
			guestfishStep{Command: "device-index", Args: []string{devices[i]}, Marker: idxMarker},
		)
	}
	return steps
}

// GetBootableVolumeIndex finds the device index of the guest's bootable
// partition at a flat 3 appliance boots regardless of partition count
// (list-partitions, then dev+num for all, then bootable+index for all).
func GetBootableVolumeIndex(disks []vm.VMDisk) (int, error) {
	command := "list-partitions"
	partitionsStr, err := RunCommandInGuestAllVolumes(disks, command, false)
	if err != nil {
		return -1, fmt.Errorf("failed to run command (%s): %v: %s", command, err, strings.TrimSpace(partitionsStr))
	}

	var partitions []string
	for _, p := range strings.Split(strings.TrimSpace(partitionsStr), "\n") {
		p = strings.TrimSpace(p)
		if p != "" {
			partitions = append(partitions, p)
		}
	}
	if len(partitions) == 0 {
		return -1, errors.New("bootable volume not found")
	}

	devNumMarkers := make([]string, 0, len(partitions)*2)
	for i := range partitions {
		devMarker, numMarker := partitionDevNumMarkers(i)
		devNumMarkers = append(devNumMarkers, devMarker, numMarker)
	}
	devNumOut, err := RunGuestfishScript(disks, false, constants.GuestfishOutputRaw, partitionDevNumSteps(partitions)...)
	if err != nil {
		return -1, fmt.Errorf("failed to resolve partition devices: %w", err)
	}
	devNumSections := splitByMarker(devNumOut, devNumMarkers)

	devices := make([]string, len(partitions))
	nums := make([]string, len(partitions))
	for i := range partitions {
		devMarker, numMarker := partitionDevNumMarkers(i)
		devices[i] = strings.TrimSpace(devNumSections[devMarker])
		nums[i] = strings.TrimSpace(devNumSections[numMarker])
	}

	bootIdxMarkers := make([]string, 0, len(partitions)*2)
	for i := range partitions {
		bootMarker, idxMarker := partitionBootIndexMarkers(i)
		bootIdxMarkers = append(bootIdxMarkers, bootMarker, idxMarker)
	}
	bootIdxOut, err := RunGuestfishScript(disks, false, constants.GuestfishOutputRaw, partitionBootIndexSteps(devices, nums)...)
	if err != nil {
		return -1, fmt.Errorf("failed to check partition bootability: %w", err)
	}

	return pickBootableIndex(len(partitions), splitByMarker(bootIdxOut, bootIdxMarkers))
}

// pickBootableIndex is GetBootableVolumeIndex's decision logic, pulled out
// for unit testing: return the first of partitionCount partitions (in
// list-partitions order) that is bootable with a usable device index.
func pickBootableIndex(partitionCount int, bootIdxSections map[string]string) (int, error) {
	for i := 0; i < partitionCount; i++ {
		bootMarker, idxMarker := partitionBootIndexMarkers(i)
		if strings.TrimSpace(bootIdxSections[bootMarker]) != "true" {
			continue
		}
		index, err := strconv.Atoi(strings.TrimSpace(bootIdxSections[idxMarker]))
		if err != nil {
			// Bootable but no usable index - try the next partition, same
			// as the original version's device-index call failing.
			continue
		}
		return index, nil
	}
	return -1, errors.New("bootable volume not found")
}

func AddUdevRules(disks []vm.VMDisk, diskPath string, interfaces []string, macs []string) error {

	if len(interfaces) != len(macs) {
		return fmt.Errorf("mismatch between number of interfaces and MACs")
	}
	var ans string

	// Create the udev rules content
	var udevRules strings.Builder
	for i, iface := range interfaces {
		// udev ATTR{address} matches sysfs, which is always lowercase
		udevRules.WriteString(fmt.Sprintf("SUBSYSTEM==\"net\", ACTION==\"add\", ATTR{address}==\"%s\", NAME=\"%s\"\n", vm.CanonicalMAC(macs[i]), iface))
		log.Printf("Adding udev rule: %s", udevRules.String())
	}

	err := os.WriteFile("/home/fedora/70-persistent-net.rules", []byte(udevRules.String()), 0644)
	if err != nil {
		return fmt.Errorf("failed to create udev rules file: %s", err)
	}
	log.Println("Uploading udev rules file to disk")
	// Upload it to the disk
	os.Setenv("LIBGUESTFS_BACKEND", "direct")
	command := "upload"
	ans, err = RunCommandInGuestAllVolumes(disks, command, true, "/home/fedora/70-persistent-net.rules", "/etc/udev/rules.d/70-persistent-net.rules")
	if err != nil {
		fmt.Printf("failed to run command (%s): %v: %s\n", "upload", err, strings.TrimSpace(ans))
		return err
	}
	return nil
}

func GetNetworkInterfaceNames(path string) ([]string, error) {
	// Get the network interface names
	command := "cat /etc/network/interfaces"
	ans, err := RunCommandInGuest(path, command, false)
	if err != nil {
		return nil, fmt.Errorf("failed to run command (%s): %v: %s", command, err, strings.TrimSpace(ans))
	}

	// Parse the output
	lines := strings.Split(ans, "\n")
	var interfaces []string
	for _, line := range lines {
		if strings.HasPrefix(line, "iface") && !strings.Contains(line, "lo") {
			interfaces = append(interfaces, strings.Fields(line)[1])
		}
	}
	return interfaces, nil

}

// interfaceFileMarker is the split marker for the i-th matched ifcfg-* file
// in a GetInterfaceNames batch - see interfaceCatSteps.
func interfaceFileMarker(i int) string {
	return fmt.Sprintf(constants.InterfaceFileMarkerTemplate, i)
}

// interfaceCatSteps builds one tolerant `cat` per matched ifcfg-* file, read
// in one boot after the `ls` that found them, instead of one boot per file.
func interfaceCatSteps(files []string) []guestfishStep {
	steps := make([]guestfishStep, len(files))
	for i, file := range files {
		steps[i] = guestfishStep{
			Command: "cat",
			Args:    []string{"/etc/sysconfig/network-scripts/" + file},
			Marker:  interfaceFileMarker(i),
		}
	}
	return steps
}

func GetInterfaceNames(path string) ([]string, error) {
	cmd := "ls /etc/sysconfig/network-scripts | grep '^ifcfg-'"
	lsOut, err := RunCommandInGuest(path, cmd, false)
	if err != nil {
		return nil, err
	}

	// Parse the output: split by newline, trim spaces, ignore 'ifcfg-lo'
	// (the loopback interface) and any blank line.
	var files []string
	for _, file := range strings.Split(strings.TrimSpace(lsOut), "\n") {
		file = strings.TrimSpace(file)
		if file == "" || file == "ifcfg-lo" {
			continue
		}
		files = append(files, file)
	}

	interfaces := []string{}
	if len(files) == 0 {
		return interfaces, nil
	}

	// Every matched file in one boot instead of one per file - boot 2 of 2.
	out, err := RunGuestfishScript([]vm.VMDisk{{Path: path}}, false, constants.GuestfishOutputCombinedRaw, interfaceCatSteps(files)...)
	if err != nil {
		return nil, fmt.Errorf("failed to read interface config files: %w", err)
	}

	markers := make([]string, len(files))
	for i := range files {
		markers[i] = interfaceFileMarker(i)
	}
	sections := splitByMarker(out, markers)

	for i, file := range files {
		content := sections[interfaceFileMarker(i)]
		if content != "" && strings.Contains(strings.ToLower(content), "no such file or directory") {
			// Tolerant cat failed for this file (removed between ls and cat,
			// or permission denied) - same as the per-file boot this replaces
			// returning a Go error: skip it.
			continue
		}
		// Extract DEVICE or infer from filename
		device := extractKeyValue(content, "DEVICE")
		if device != "" {
			interfaces = append(interfaces, device)
		} else {
			// Fall back to filename if DEVICE not found
			device = strings.TrimPrefix(file, "ifcfg-")
			if device != "" {
				interfaces = append(interfaces, device)
			}
		}
	}

	return interfaces, nil
}

// Helper: Extract key=value from content, trim quotes/spaces
func extractKeyValue(content, key string) string {
	re := regexp.MustCompile(fmt.Sprintf(`(?m)^%s=(.*)$`, key))
	match := re.FindStringSubmatch(content)
	if len(match) > 1 {
		return strings.Trim(strings.Trim(match[1], `"'`), " ")
	}
	return ""
}

// GetOsReleaseAllVolumes tries every candidate release file across all of a
// guest's disks at once - see getOsReleaseFromDisks, which this and
// GetOsRelease share.
func GetOsReleaseAllVolumes(disks []vm.VMDisk) (string, error) {
	return getOsReleaseFromDisks(disks)
}

// GetWindowsVersion detects the Windows version using guestfish inspect commands.
//
// inspect-get-product-name reads data that only inspect-os populates, and that
// state does not survive across guestfish processes - so both have to run in one
// script. Issuing them as two RunCommandInGuestAllVolumes calls always failed with
// "no inspection data", which silently degraded to "Windows (version unknown)" and
// then picked the wrong virtio-win ISO for Server 2012.
func GetWindowsVersion(disks []vm.VMDisk, diskPath string) (string, error) {
	plan, err := resolveMountPlan(disks)
	if err != nil {
		return "", fmt.Errorf("failed to resolve guest mount plan: %w", err)
	}

	out, err := runScript(disks, windowsProductNameScript(plan.Root))
	if err != nil {
		return "", fmt.Errorf("failed to inspect OS: %w", err)
	}

	productName := parseWindowsProductName(out)
	if productName == "" {
		// Not fatal - the caller falls back to the default virtio-win ISO - but it
		// must be visible, because that fallback is wrong for Server 2012.
		log.Printf("WARNING: could not read the Windows product name from %s; "+
			"version-specific behaviour will fall back to defaults", plan.Root)
		return "Windows (version unknown)", nil
	}

	log.Printf("Detected Windows version: %s", productName)
	return strings.ToLower(productName), nil
}

// isWindowsServer2012 reports whether the detected product name is Server 2012 or
// 2012 R2, which need the pinned older virtio-win ISO.
func isWindowsServer2012(osRelease string) bool {
	lower := strings.ToLower(osRelease)
	return strings.Contains(lower, "server 2012") || strings.Contains(lower, "server2012")
}

// windowsProductNameScript asks for the product name of one root, re-running
// inspect-os first so the inspection data exists in this process.
func windowsProductNameScript(root string) string {
	var b strings.Builder
	b.WriteString("run\n")
	// Output lands before the marker and is discarded by parseWindowsProductName.
	b.WriteString("inspect-os\n")
	fmt.Fprintf(&b, "echo %s\n", productNameMarker)
	b.WriteString(guestfishLine("inspect-get-product-name", root) + "\n")
	return b.String()
}

// parseWindowsProductName returns the text after the marker, which is whatever
// inspect-get-product-name printed. Empty if the marker never appeared.
func parseWindowsProductName(out string) string {
	_, after, found := strings.Cut(out, productNameMarker)
	if !found {
		return ""
	}
	return strings.TrimSpace(after)
}

// RunMountPersistenceScript runs the generate-mount-persistence.sh script
// during guest inspection phase for Linux migrations.
//
// For SUSE/SLES GRUB Legacy guests, --replace-fstab is used instead of
// --force-uuid. The --force-uuid flag calls fix_grub_config which rewrites
// device.map from /dev/sdX → /dev/vdX BEFORE virt-v2v runs. When virt-v2v
// subsequently runs inside the guestfish appliance (where disks are /dev/sdX),
// it reads a device.map referencing /dev/vdX paths that do not exist in the
// appliance, causing it to reinstall GRUB stage1 with incorrect embedded drive
// references — manifesting as GRUB Error 21 at boot. Using --replace-fstab
// fixes fstab and udev rules without touching device.map or grub config files,
// allowing virt-v2v to handle GRUB correctly.
// mountPersistenceSteps builds the upload+chmod+sh sequence
// RunMountPersistenceScript runs in one appliance boot instead of three,
// against scriptArgs already resolved by MountPersistenceScriptArgs.
func mountPersistenceSteps(scriptPath, scriptArgs string) []guestfishStep {
	return []guestfishStep{
		{Command: "upload", Args: []string{scriptPath, "/tmp/generate-mount-persistence.sh"}},
		{Command: "chmod", Args: []string{"0755", "/tmp/generate-mount-persistence.sh"}},
		{Command: "sh", Args: []string{"/tmp/generate-mount-persistence.sh " + scriptArgs}},
	}
}

func RunMountPersistenceScript(disks []vm.VMDisk, diskPath string, osRelease string) error {
	// Script should be available in the container at /home/fedora/
	scriptPath := "/home/fedora/generate-mount-persistence.sh"

	// Check if script exists in the container
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return fmt.Errorf("generate-mount-persistence.sh script not found at %s", scriptPath)
	}

	// Pick script args by OS family: SUSE uses --replace-fstab --os-family=suse instead of
	// --force-uuid, skipping the pre-virt-v2v device.map rewrite that breaks SUSE GRUB Legacy.
	scriptArgs := MountPersistenceScriptArgs(osRelease)
	if IsSUSEFamily(osRelease) {
		log.Printf("SUSE guest detected: using --replace-fstab --os-family=suse (script will safely fix GRUB Legacy cmdline if detected, without touching device.map)")
	}

	log.Printf("Running generate-mount-persistence.sh with %s option(s)", scriptArgs)

	// Upload, chmod, and run in one boot instead of three. The only caller
	// (handleLinuxOSDetection) warns-and-continues on any failure either
	// way, so collapsing to one "warn and continue" outcome changes nothing.
	out, err := RunGuestfishScript(disks, true, constants.GuestfishOutputRaw, mountPersistenceSteps(scriptPath, scriptArgs)...)
	if err != nil {
		log.Printf("Warning: generate-mount-persistence.sh did not complete: %v", err)
		// Don't return error, just log warning as this is not critical
		return nil
	}

	log.Printf("Successfully executed generate-mount-persistence.sh with %s", scriptArgs)
	log.Printf("Script output: %s", strings.TrimSpace(out))

	return nil
}

// fixLegacyMkinitrdCheckSteps builds FixLegacyMkinitrd's three existence
// checks (four stat calls, since the dracut check tries two paths) as one
// tolerant batch instead of up to three separate appliance boots.
func fixLegacyMkinitrdCheckSteps() []guestfishStep {
	return []guestfishStep{
		{Command: "stat", Args: []string{"/sbin/mkinitrd"}, Marker: constants.MkinitrdCheckMarker},
		{Command: "stat", Args: []string{"/usr/bin/dracut"}, Marker: constants.DracutUsrBinCheckMarker},
		{Command: "stat", Args: []string{"/sbin/dracut"}, Marker: constants.DracutSbinCheckMarker},
		{Command: "stat", Args: []string{"/sbin/mkinitrd.orig"}, Marker: constants.MkinitrdOrigCheckMarker},
	}
}

// fixLegacyMkinitrdWriteSteps builds the backup+upload+chmod chain as one
// fail-fast script instead of three separate boots: a failed backup aborts
// before upload, a failed upload aborts before chmod, as before.
func fixLegacyMkinitrdWriteSteps() []guestfishStep {
	return []guestfishStep{
		{Command: "cp", Args: []string{"/sbin/mkinitrd", "/sbin/mkinitrd.orig"}},
		{Command: "upload", Args: []string{constants.MkinitrdLVMWrapperPath, "/sbin/mkinitrd"}},
		{Command: "chmod", Args: []string{"0755", "/sbin/mkinitrd"}},
	}
}

// This must be called BEFORE ConvertDisk / virt-v2v-in-place so that the
// patched binary is in place when virt-v2v chroots into the guest.
func FixLegacyMkinitrd(disks []vm.VMDisk) error {
	os.Setenv("LIBGUESTFS_BACKEND", "direct")

	// Boot 1: all four existence checks in one tolerant batch instead of
	// up to three boots. A failed step prints nothing to stdout (not a Go
	// error - see splitByMarker), so "found" means a non-empty section.
	out, err := RunGuestfishScript(disks, false, constants.GuestfishOutputRaw, fixLegacyMkinitrdCheckSteps()...)
	if err != nil {
		return fmt.Errorf("FixLegacyMkinitrd: failed to check guest state: %w", err)
	}
	sections := splitByMarker(out, constants.FixLegacyMkinitrdCheckMarkers)

	// 1. Does /sbin/mkinitrd exist on the guest?
	if sections[constants.MkinitrdCheckMarker] == "" {
		log.Printf("FixLegacyMkinitrd: /sbin/mkinitrd not found on guest, skipping")
		return nil
	}

	// 2. Is dracut absent? (dracut == modern SUSE, no patch needed)
	if sections[constants.DracutUsrBinCheckMarker] != "" {
		log.Printf("FixLegacyMkinitrd: dracut found at /usr/bin/dracut, modern system – skipping")
		return nil
	}
	if sections[constants.DracutSbinCheckMarker] != "" {
		log.Printf("FixLegacyMkinitrd: dracut found at /sbin/dracut, modern system – skipping")
		return nil
	}

	// 3. Already patched by a previous run?
	if sections[constants.MkinitrdOrigCheckMarker] != "" {
		log.Printf("FixLegacyMkinitrd: wrapper already installed (/sbin/mkinitrd.orig present), skipping")
		return nil
	}

	log.Printf("FixLegacyMkinitrd: old mkinitrd detected (no dracut), installing LVM path translation wrapper")

	// Boot 2: backup, upload, chmod - one fail-fast script instead of
	// three. A failure no longer says which step it was - same trade-off
	// as RunGetBootablePartitionScript/RunMountPersistenceScript.
	if _, err := RunGuestfishScript(disks, true, constants.GuestfishOutputLowercased, fixLegacyMkinitrdWriteSteps()...); err != nil {
		return fmt.Errorf("FixLegacyMkinitrd: failed to install wrapper: %w", err)
	}

	log.Printf("FixLegacyMkinitrd: wrapper installed successfully at /sbin/mkinitrd (original at /sbin/mkinitrd.orig)")
	return nil
}

// getBootablePartitionSteps passes realDisks to the script as args, so its
// heuristics never see the appliance's own disk; nil/empty falls back to no args.
func getBootablePartitionSteps(scriptPath string, realDisks []string) []guestfishStep {
	shCommand := "/tmp/get-bootable-partition.sh"
	if len(realDisks) > 0 {
		shCommand = shCommand + " " + strings.Join(realDisks, " ")
	}
	return []guestfishStep{
		{Command: "upload", Args: []string{scriptPath, "/tmp/get-bootable-partition.sh"}},
		{Command: "chmod", Args: []string{"0755", "/tmp/get-bootable-partition.sh"}},
		{Command: "sh", Args: []string{shCommand}},
	}
}

// parseBootDiskResult splits stdout into BOOTDISK_RESULT and trace, falling
// back to raw output if untagged, and rejects a result outside realDisks.
func parseBootDiskResult(out string, realDisks []string) (result, trace string, err error) {
	traceLines := make([]string, 0)
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "BOOTDISK_RESULT:") {
			result = strings.TrimSpace(strings.TrimPrefix(line, "BOOTDISK_RESULT:"))
			continue
		}
		if strings.TrimSpace(line) != "" {
			traceLines = append(traceLines, line)
		}
	}
	trace = strings.TrimSpace(strings.Join(traceLines, "\n"))

	if result == "" {
		result = strings.TrimSpace(out)
	}
	if !slices.Contains(realDisks, result) {
		err = fmt.Errorf("get-bootable-partition.sh returned %q, which is not one of the real attached disks %v; refusing to use it", result, realDisks)
	}
	return result, trace, err
}

func RunGetBootablePartitionScript(disks []vm.VMDisk) (string, error) {
	scriptPath := "/home/fedora/get-bootable-partition.sh"
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return "", fmt.Errorf("get-bootable-partition.sh script not found at %s", scriptPath)
	}

	// list-devices excludes the appliance's own disk (unlike the script's raw
	// scan); pass it to the script so it can never pick that disk as bootable.
	realDisksStr, listErr := RunCommandInGuestAllVolumes(disks, "list-devices", false)
	if listErr != nil {
		return "", fmt.Errorf("failed to list real guest devices via list-devices: %v: %s", listErr, strings.TrimSpace(realDisksStr))
	}
	realDisks := strings.Fields(strings.TrimSpace(realDisksStr))
	if len(realDisks) == 0 {
		return "", errors.New("list-devices returned no attached disks")
	}
	if logErr := utils.PrintLog(fmt.Sprintf("get-bootable-partition.sh: real attached disks per list-devices (appliance disk excluded, -a order): %v", realDisks)); logErr != nil {
		log.Printf("WARNING: failed to persist list-devices to migration log: %v", logErr)
	}

	out, err := RunGuestfishScript(disks, true, constants.GuestfishOutputRaw, getBootablePartitionSteps(scriptPath, realDisks)...)
	if err != nil {
		return "", fmt.Errorf("failed to run get-bootable-partition.sh: %w", err)
	}

	resultLine, trace, parseErr := parseBootDiskResult(out, realDisks)
	if trace == "" {
		trace = "(no debug output captured - either the deployed get-bootable-partition.sh predates step logging, or it produced none)"
	}
	if logErr := utils.PrintLog(fmt.Sprintf("get-bootable-partition.sh trace:\n%s", trace)); logErr != nil {
		log.Printf("WARNING: failed to persist get-bootable-partition.sh trace to migration log: %v", logErr)
	}

	// No tagged line found (older/untagged build); already fell back to raw output.
	if !strings.Contains(out, "BOOTDISK_RESULT:") {
		log.Printf("WARNING: get-bootable-partition.sh output had no BOOTDISK_RESULT: line; falling back to raw output as result: %q", resultLine)
	}

	if logErr := utils.PrintLog(fmt.Sprintf("get-bootable-partition.sh result: %q", resultLine)); logErr != nil {
		log.Printf("WARNING: failed to persist get-bootable-partition.sh result to migration log: %v", logErr)
	}

	if parseErr != nil {
		return "", parseErr
	}

	return resultLine, nil
}

// RunNetworkPersistence mounts the disk locally and runs the network persistence script
func RunNetworkPersistence(disks []vm.VMDisk, diskPath string, ostype string, isNetplan bool) error {
	// Skip this entirely for Windows as it doesn't use these udev rules/bash scripts
	if strings.ToLower(ostype) == constants.OSFamilyWindows {
		log.Println("Skipping offline network persistence for Windows guest")
		return nil
	}

	// Create a temporary directory in the Pod to serve as the mount point
	mountPoint, err := os.MkdirTemp("", "v2v-mount-*")
	if err != nil {
		return fmt.Errorf("failed to create temp mount dir: %w", err)
	}
	defer os.RemoveAll(mountPoint)

	// Construct the guestmount command.
	//
	// guestmount's -i option shares inspect_mount_handle() with guestfish, so it
	// fails identically on a guest whose root filesystem spans several devices.
	// Mount the explicitly resolved plan instead.
	plan, err := resolveMountPlan(disks)
	if err != nil {
		return fmt.Errorf("failed to resolve guest mount plan: %w", err)
	}

	log.Printf("Mounting disk to %s using guestmount...", mountPoint)
	mountCmd := exec.Command("guestmount", guestmountArgs(disks, plan.Mounts, mountPoint)...)
	if out, mountErr := mountCmd.CombinedOutput(); mountErr != nil {
		// Unlike a guestfish script, guestmount cannot tolerate one failed mount,
		// and inspect-get-mountpoints is documented as possibly returning
		// filesystems that are absent or unmountable. Retry with the root alone so
		// that a stale fstab entry cannot break network persistence outright.
		log.Printf("guestmount with the full mount plan failed (%v: %s); retrying with the root filesystem only",
			mountErr, strings.TrimSpace(string(out)))

		if strings.Contains(plan.Root, ":") {
			return fmt.Errorf("cannot mount root %q with guestmount: -m cannot express a mountable", plan.Root)
		}

		mountCmd = exec.Command("guestmount", guestmountArgs(disks, []mountSpec{{Device: plan.Root, MountPoint: "/"}}, mountPoint)...)
		if out, mountErr := mountCmd.CombinedOutput(); mountErr != nil {
			return fmt.Errorf("guestmount failed: %v, output: %s", mountErr, string(out))
		}
	}

	// Unmount even if the script execution fails
	defer func() {
		log.Println("Unmounting disk...")
		unmountCmd := exec.Command("guestunmount", mountPoint)
		if out, err := unmountCmd.CombinedOutput(); err != nil {
			log.Printf("Failed to unmount %s: %v, output: %s", mountPoint, err, string(out))
		}
	}()

	scriptPath := "/home/fedora/generate-udev-mapping.sh"
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return fmt.Errorf("script not found at %s", scriptPath)
	}

	runCmd := exec.Command("bash", scriptPath)

	// Configure environment variables to point the script to the Mount Point
	env := os.Environ()
	env = append(env, fmt.Sprintf("NET_MAPPING_DATA=%s", filepath.Join(mountPoint, "/etc/macToIP")))
	env = append(env, fmt.Sprintf("RHEL_NET_DIR=%s", filepath.Join(mountPoint, "/etc/sysconfig/network-scripts")))
	env = append(env, fmt.Sprintf("SUSE_NET_DIR=%s", filepath.Join(mountPoint, "/etc/sysconfig/network")))
	env = append(env, fmt.Sprintf("NM_CONN_PATH=%s", filepath.Join(mountPoint, "/etc/NetworkManager/system-connections")))
	env = append(env, fmt.Sprintf("NM_RUNTIME_DATA=%s", filepath.Join(mountPoint, "/var/lib/NetworkManager")))
	env = append(env, fmt.Sprintf("DHCP_LEASE_PATH=%s", filepath.Join(mountPoint, "/var/lib/dhclient")))
	env = append(env, fmt.Sprintf("DEBIAN_IF_DIR=%s", filepath.Join(mountPoint, "/etc/network/interfaces")))
	env = append(env, fmt.Sprintf("SYSTEMD_NET_PATH=%s", filepath.Join(mountPoint, "/run/systemd/network")))
	env = append(env, fmt.Sprintf("UDEV_OUTPUT_TARGET=%s", filepath.Join(mountPoint, "/etc/udev/rules.d/70-persistent-net.rules")))
	env = append(env, fmt.Sprintf("NETPLAN_EXT_CONF=%s", filepath.Join(mountPoint, "/etc/netplan/99-netcfg.yaml")))
	env = append(env, fmt.Sprintf("NETPLAN_BASE_DIR=%s", mountPoint))
	env = append(env, fmt.Sprintf("USE_NETPLAN_LOGIC=%t", isNetplan))
	runCmd.Env = env

	log.Println("Executing network persistence script")
	output, err := runCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("network persistence script failed: %w, output: %s", err, string(output))
	}
	log.Printf("Network persistence script output: %s", string(output))

	return nil
}

func RunOfflineVMwareCleanup(diskPath string) error {
	const scriptPath = "/home/fedora/offline-vmware-cleanup.sh"

	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		log.Printf("WARNING: offline-vmware-cleanup.sh not found at %s; skipping offline VMware Tools removal", scriptPath)
		return nil
	}

	log.Printf("Running offline VMware Tools cleanup on %s", diskPath)
	os.Setenv("LIBGUESTFS_BACKEND", "direct")

	cmd := exec.Command("bash", scriptPath, diskPath)
	out, err := cmd.CombinedOutput()
	log.Printf("offline-vmware-cleanup output:\n%s", strings.TrimSpace(string(out)))
	if err != nil {
		log.Printf("WARNING: offline VMware Tools cleanup failed: %v", err)
	} else {
		log.Printf("Offline VMware Tools cleanup completed successfully")
	}
	return nil
}

func InjectRestorationScript(disks []vm.VMDisk, diskPath string) error {
	os.Setenv("LIBGUESTFS_BACKEND", "direct")

	var ans string
	var err error
	command := "copy-in"
	ans, err = RunCommandInGuestAllVolumes(disks, command, true, "/home/fedora/NIC-Recovery", "/")
	if err != nil {
		fmt.Printf("failed to run command (%s): %v: %s\n", "copy-in", err, strings.TrimSpace(ans))
		return err
	}
	return nil
}

func InjectFirstBootScriptsFromStore(disks []vm.VMDisk, diskPath string, firstbootwinscripts []FirstBootWindows) error {
	log.Println("Collecting Firstboot Scripts to Inject")
	var ans string
	var err error
	var scriptDir string = "/home/fedora/firstboot"
	if _, err := os.Stat(scriptDir); os.IsNotExist(err) {
		log.Printf("Creating directory %s", scriptDir)

		cpCmd := exec.Command("mkdir", scriptDir)
		if err := cpCmd.Run(); err != nil {
			return fmt.Errorf("failed to create directory %s: %v", scriptDir, err)
		}
	}
	scriptsMetadata := []FirstBootWindows{}
	for idx, script := range firstbootwinscripts {
		log.Printf("Injecting Firstboot Script: %s", script.Script)

		srcPath := fmt.Sprintf("/home/fedora/store/%s", script.Script)
		dstPath := fmt.Sprintf("/home/fedora/firstboot/%d-%s", idx, script.Script)
		if idx > 0 {
			scriptsMetadata = append(scriptsMetadata, FirstBootWindows{Script: fmt.Sprintf("%d-%s", idx, script.Script), Async: script.Async})
		}
		cpCmd := exec.Command("cp", srcPath, dstPath)
		if err := cpCmd.Run(); err != nil {
			return fmt.Errorf("failed to copy firstboot script %s: %v", script.Script, err)
		}
	}
	// Write scripts metadata to JSON file
	metadataPath := "/home/fedora/firstboot/scripts.json"
	metadataJSON, err := json.Marshal(scriptsMetadata)
	log.Printf("Writing scripts metadata to %v", metadataJSON)
	if err != nil {
		return fmt.Errorf("failed to marshal scripts metadata: %v", err)
	}
	if err := os.WriteFile(metadataPath, metadataJSON, 0644); err != nil {
		return fmt.Errorf("failed to write scripts metadata to %s: %v", metadataPath, err)
	}
	log.Printf("Wrote scripts metadata to %s", metadataPath)
	os.Setenv("LIBGUESTFS_BACKEND", "direct")

	command := "copy-in"
	ans, err = RunCommandInGuestAllVolumes(disks, command, true, "/home/fedora/firstboot", "/")
	if err != nil {
		fmt.Printf("failed to run command (%s): %v: %s\n", "copy-in", err, strings.TrimSpace(ans))
		return err
	}
	return nil
}

// PushWindowsFirstBoot creates OS-filtered user script parts in the store directory for Windows
func PushWindowsFirstBoot(ostype string) ([]string, error) {
	srcPath := "/home/fedora/scripts/user_firstboot.sh"

	// Check if source file exists
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("source file not found at %s: %w", srcPath, err)
	}

	// Read the source file content
	content, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read source file %s: %w", srcPath, err)
	}

	scripts := splitAndFilterUserScripts(string(content), ostype)
	if len(scripts) == 0 {
		log.Printf("No Windows user post-migration scripts to inject for OS '%s'", ostype)
		return nil, nil
	}

	// Ensure destination directory exists
	if err := os.MkdirAll("/home/fedora/store", 0755); err != nil {
		return nil, fmt.Errorf("failed to create destination directory: %w", err)
	}

	scriptNames := make([]string, 0, len(scripts))
	for idx, script := range scripts {
		dstPath := fmt.Sprintf("/home/fedora/store/user_firstboot_part_%03d.ps1", idx+1)
		if err := os.WriteFile(dstPath, []byte(script+"\n"), 0644); err != nil {
			return nil, fmt.Errorf("failed to write destination file %s: %w", dstPath, err)
		}

		scriptName := fmt.Sprintf("user_firstboot_part_%03d.ps1", idx+1)
		scriptNames = append(scriptNames, scriptName)
		log.Printf("Prepared Windows user firstboot script part %d at %s", idx+1, dstPath)
	}

	return scriptNames, nil
}
