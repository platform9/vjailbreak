// Copyright © 2024 The vjailbreak authors

package virtv2v

import (
	"bufio"
	"bytes"
	"context"
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
	"github.com/platform9/vjailbreak/v2v-helper/pkg/constants"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/utils"
	"github.com/platform9/vjailbreak/v2v-helper/vm"
)

//go:generate mockgen -source=../virtv2v/virtv2vops.go -destination=../virtv2v/virtv2vops_mock.go -package=virtv2v

type VirtV2VOperations interface {
	RetainAlphanumeric(input string) string
	GetPartitions(disk string) ([]string, error)
	NTFSFix(path string) error
	ConvertDisk(ctx context.Context, path, ostype, virtiowindriver string, firstbootscripts []string, useSingleDisk bool, diskPath string) error
	AddWildcardNetplan(path string) error
	GetOsRelease(path string) (string, error)
	AddFirstBootScript(firstbootscript, firstbootscriptname string) error
	AddUdevRules(disks []vm.VMDisk, useSingleDisk bool, diskPath string, interfaces []string, macs []string) error
	GetNetworkInterfaceNames(path string) ([]string, error)
	IsRHELFamily(osRelease string) (bool, error)
	GetOsReleaseAllVolumes(disks []vm.VMDisk) (string, error)
}

// AddNetplanConfig uploads a provided netplan YAML into the guest at /etc/netplan/50-vj.yaml
func AddNetplanConfig(disks []vm.VMDisk, useSingleDisk bool, diskPath string, netplanYAML string) error {
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
	if useSingleDisk {
		command := `upload /home/fedora/50-vj.yaml /etc/netplan/50-vj.yaml`
		ans, err = RunCommandInGuest(diskPath, command, true)
	} else {
		command := "upload"
		ans, err = RunCommandInGuestAllVolumes(disks, command, true, "/home/fedora/50-vj.yaml", "/etc/netplan/50-vj.yaml")
	}
	if err != nil {
		fmt.Printf("failed to run command (%s): %v: %s\n", "upload", err, strings.TrimSpace(ans))
		return err
	}
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
	log.Printf("Partitions: %v", partitions)
	for _, partition := range partitions {
		if partition == path {
			continue
		}
		cmd := exec.Command("ntfsfix", partition)
		log.Printf("Executing %s", cmd.String())

		// Use the debug logging with proper file cleanup
		err := utils.RunCommandWithLogFile(cmd)
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

func ConvertDisk(ctx context.Context, xmlFile, path, ostype, virtiowindriver string, firstbootscripts []string, useSingleDisk bool, diskPath string) error {
	// Step 1: Handle Windows driver injection
	if strings.ToLower(ostype) == constants.OSFamilyWindows {
		filePath := "/home/fedora/virtio-win/virtio-win.iso"

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
	args := []string{"-v", "--no-fstrim", "--firstboot", "/home/fedora/scripts/user_firstboot.sh"}
	for _, script := range firstbootscripts {
		args = append(args, "--firstboot", fmt.Sprintf("/home/fedora/%s.sh", script))
	}
	if useSingleDisk {
		args = append(args, "-i", "disk", diskPath)
	} else {
		args = append(args, "-i", "libvirtxml", xmlFile, "--root", path)
	}

	start := time.Now()
	// Step 5: Run virt-v2v-in-place

	cmd := exec.CommandContext(ctx, "virt-v2v-in-place", args...)
	log.Printf("Executing %s", cmd.String())

	// Use the debug logging with proper file cleanup
	err := utils.RunCommandWithLogFile(cmd)
	duration := time.Since(start)

	if err != nil {
		return fmt.Errorf("failed to run virt-v2v-in-place: %s", err)
	}
	log.Printf("virt-v2v-in-place conversion took: %s", duration)
	return nil
}

func GetOsRelease(path string) (string, error) {
	os.Setenv("LIBGUESTFS_BACKEND", "direct")

	releaseFiles := []string{
		"/etc/os-release",
		"/etc/redhat-release",
		"/etc/SuSE-release", // SLES 11
	}

	runGuestfishCat := func(imgPath, file string) (string, error) {
		cmd := exec.Command("guestfish", "--ro", "-a", imgPath, "-i")
		cmd.Stdin = strings.NewReader(fmt.Sprintf("cat %s", file))
		log.Printf("Executing %s with input: cat %s", cmd.String(), file)

		out, err := cmd.CombinedOutput()
		if err != nil {
			return strings.ToLower(string(out)), err
		}
		return strings.ToLower(string(out)), nil
	}

	var errs []string
	for _, file := range releaseFiles {
		out, err := runGuestfishCat(path, file)
		if err == nil {
			return out, nil
		}

		errStr := strings.TrimSpace(out)
		errs = append(errs, errStr)

		// If it's not a "no such file" error, stop immediately
		if !strings.Contains(strings.ToLower(errStr), "no such file or directory") {
			break
		}
	}

	return "", fmt.Errorf("failed to get OS release from %v: %v",
		strings.Join(releaseFiles, ", "), strings.Join(errs, " | "))
}

// GetInterfaceNamesFromDisk reads the source VM's disk and extracts interface names from ifcfg files
// Matches by IP address since the same MAC will have the same IP on the migrated VM
func GetInterfaceNamesFromDisk(disks []vm.VMDisk, useSingleDisk bool, diskPath string, ipPerMac map[string][]vm.IpEntry) (map[string]string, error) {
	log.Println("[GetInterfaceNamesFromDisk] Starting...")
	macToInterface := make(map[string]string)
	
	// Build a reverse map: IP -> MAC for quick lookup
	ipToMac := make(map[string]string)
	for mac, ips := range ipPerMac {
		if len(ips) > 0 {
			ipToMac[ips[0].IP] = strings.ToLower(mac)
			log.Printf("[GetInterfaceNamesFromDisk] IP->MAC mapping: %s -> %s", ips[0].IP, strings.ToLower(mac))
		}
	}
	
	// Determine which directory to use (RHEL vs SUSE)
	var scriptsDir string
	var lsOut string
	var err error
	
	// Try RHEL path first
	if useSingleDisk {
		lsOut, err = RunCommandInGuest(diskPath, "ls /etc/sysconfig/network-scripts", false)
	} else {
		lsOut, err = RunCommandInGuestAllVolumes(disks, "ls", false, "/etc/sysconfig/network-scripts")
	}
	
	if err != nil {
		log.Println("[GetInterfaceNamesFromDisk] RHEL path not found, trying SUSE path...")
		// Try SUSE path
		if useSingleDisk {
			lsOut, err = RunCommandInGuest(diskPath, "ls /etc/sysconfig/network", false)
		} else {
			lsOut, err = RunCommandInGuestAllVolumes(disks, "ls", false, "/etc/sysconfig/network")
		}
		if err != nil {
			log.Printf("[GetInterfaceNamesFromDisk] Warning: Could not list network config directory: %v", err)
			return macToInterface, nil
		}
		scriptsDir = "/etc/sysconfig/network"
		log.Printf("[GetInterfaceNamesFromDisk] Using SUSE directory: %s", scriptsDir)
	} else {
		scriptsDir = "/etc/sysconfig/network-scripts"
		log.Printf("[GetInterfaceNamesFromDisk] Using RHEL directory: %s", scriptsDir)
	}
	
	files := strings.Split(strings.TrimSpace(lsOut), "\n")
	log.Printf("[GetInterfaceNamesFromDisk] Found %d files in directory", len(files))
	for _, file := range files {
		file = strings.TrimSpace(file)
		// Only process ifcfg-* files, skip ifcfg-lo
		if !strings.HasPrefix(file, "ifcfg-") || file == "ifcfg-lo" {
			continue
		}
		
		// Extract interface name from filename (e.g., ifcfg-ens35 -> ens35)
		interfaceName := strings.TrimPrefix(file, "ifcfg-")
		log.Printf("[GetInterfaceNamesFromDisk] Processing file: %s (interface: %s)", file, interfaceName)
		
		// Read the file content
		var content string
		filePath := fmt.Sprintf("%s/%s", scriptsDir, file)
		if useSingleDisk {
			content, err = RunCommandInGuest(diskPath, fmt.Sprintf("cat %s", filePath), false)
		} else {
			content, err = RunCommandInGuestAllVolumes(disks, "cat", false, filePath)
		}
		if err != nil {
			log.Printf("Warning: Could not read %s: %v", file, err)
			continue
		}
		
		// Try to find MAC address for this interface
		var foundMac string
		lines := strings.Split(content, "\n")
		
		// Method 1: Direct HWADDR in the file
		log.Printf("[GetInterfaceNamesFromDisk] Trying Method 1: Looking for HWADDR in %s", file)
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "HWADDR=") {
				mac := strings.TrimPrefix(line, "HWADDR=")
				mac = strings.Trim(mac, "\"'")
				foundMac = strings.ToLower(strings.TrimSpace(mac))
				if foundMac != "" {
					log.Printf("[GetInterfaceNamesFromDisk] ✓ Method 1 SUCCESS: Found HWADDR=%s -> %s", foundMac, interfaceName)
					break
				}
			}
		}
		if foundMac == "" {
			log.Printf("[GetInterfaceNamesFromDisk] ✗ Method 1 FAILED: No HWADDR found in %s", file)
		}
		
		// Method 2: Match by IP address (works for static IPs)
		// Since same MAC gets same IP, we can use ipPerMac to find the MAC
		if foundMac == "" {
			log.Printf("[GetInterfaceNamesFromDisk] Trying Method 2: Looking for IPADDR in %s", file)
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "IPADDR=") {
					ip := strings.TrimPrefix(line, "IPADDR=")
					ip = strings.Trim(ip, "\"'")
					ip = strings.TrimSpace(ip)
					log.Printf("[GetInterfaceNamesFromDisk] Found IPADDR=%s in %s", ip, file)
					if ip != "" {
						if mac, found := ipToMac[ip]; found {
							foundMac = mac
							log.Printf("[GetInterfaceNamesFromDisk] ✓ Method 2 SUCCESS: Matched IP %s to MAC %s -> %s", ip, foundMac, interfaceName)
							break
						} else {
							log.Printf("[GetInterfaceNamesFromDisk] ✗ Method 2 FAILED: IP %s not found in ipToMac lookup", ip)
						}
					}
				}
			}
			if foundMac == "" {
				log.Printf("[GetInterfaceNamesFromDisk] ✗ Method 2 FAILED: No IPADDR found or no MAC match for %s", file)
			}
		}
		
		// Store the mapping if we found a MAC
		if foundMac != "" {
			macToInterface[foundMac] = interfaceName
			log.Printf("[GetInterfaceNamesFromDisk] ✓ FINAL: Stored mapping %s -> %s", foundMac, interfaceName)
		} else {
			log.Printf("[GetInterfaceNamesFromDisk] ✗ FINAL: Could not find MAC for interface %s (likely DHCP without HWADDR)", interfaceName)
		}
	}
	
	log.Printf("[GetInterfaceNamesFromDisk] Completed with %d mappings", len(macToInterface))
	return macToInterface, nil
}

// GetInterfaceNamesFromNMKeyfiles reads NetworkManager keyfile configs (.nmconnection)
// Used for RHEL 9, CentOS Stream 9, Rocky 9, etc. where ifcfg files are deprecated
func GetInterfaceNamesFromNMKeyfiles(disks []vm.VMDisk, useSingleDisk bool, diskPath string, ipPerMac map[string][]vm.IpEntry) (map[string]string, error) {
	macToInterface := make(map[string]string)
	
	// Build a reverse map: IP -> MAC for quick lookup
	ipToMac := make(map[string]string)
	for mac, ips := range ipPerMac {
		if len(ips) > 0 {
			ipToMac[ips[0].IP] = strings.ToLower(mac)
		}
	}
	
	// List .nmconnection files
	var lsOut string
	var err error
	if useSingleDisk {
		lsOut, err = RunCommandInGuest(diskPath, "ls /etc/NetworkManager/system-connections", false)
	} else {
		lsOut, err = RunCommandInGuestAllVolumes(disks, "ls", false, "/etc/NetworkManager/system-connections")
	}
	if err != nil {
		log.Printf("Info: NetworkManager keyfiles directory not found (not RHEL 9): %v", err)
		return macToInterface, nil // Not an error, just not present
	}
	
	files := strings.Split(strings.TrimSpace(lsOut), "\n")
	for _, file := range files {
		file = strings.TrimSpace(file)
		if !strings.HasSuffix(file, ".nmconnection") {
			continue
		}
		
		// Read file content
		var content string
		filePath := fmt.Sprintf("/etc/NetworkManager/system-connections/%s", file)
		if useSingleDisk {
			content, err = RunCommandInGuest(diskPath, fmt.Sprintf("cat %s", filePath), false)
		} else {
			content, err = RunCommandInGuestAllVolumes(disks, "cat", false, filePath)
		}
		if err != nil {
			log.Printf("Warning: Could not read %s: %v", file, err)
			continue
		}
		
		// Parse INI-style format
		var interfaceName string
		var ipAddress string
		var hasMac bool
		
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			
			// Get interface name
			if strings.HasPrefix(line, "interface-name=") {
				interfaceName = strings.TrimPrefix(line, "interface-name=")
			}
			
			// Check if MAC already exists
			if strings.HasPrefix(line, "mac-address=") {
				hasMac = true
			}
			
			// Get IP address (format: address1=10.9.7.182/24,10.9.7.1)
			if strings.HasPrefix(line, "address1=") {
				addr := strings.TrimPrefix(line, "address1=")
				// Split by comma (gateway part), take first part
				parts := strings.Split(addr, ",")
				if len(parts) > 0 {
					// Split by / (CIDR), take IP
					ipParts := strings.Split(parts[0], "/")
					if len(ipParts) > 0 {
						ipAddress = strings.TrimSpace(ipParts[0])
					}
				}
			}
		}
		
		// Match IP to MAC
		if interfaceName != "" && ipAddress != "" {
			if mac, found := ipToMac[ipAddress]; found {
				macToInterface[mac] = interfaceName
				if hasMac {
					log.Printf("Found interface mapping from keyfile (has MAC): %s -> %s (IP: %s)", mac, interfaceName, ipAddress)
				} else {
					log.Printf("Found interface mapping from keyfile (no MAC): %s -> %s (IP: %s)", mac, interfaceName, ipAddress)
				}
			}
		} else if interfaceName != "" {
			log.Printf("Info: Keyfile %s has interface %s but no static IP", file, interfaceName)
		}
	}
	
	return macToInterface, nil
}

// AddHWADDRToStaticInterfaces adds HWADDR to static IP ifcfg files
// This allows NetworkManager to match configs by MAC even if interface names change
func AddHWADDRToStaticInterfaces(disks []vm.VMDisk, useSingleDisk bool, diskPath string, ipPerMac map[string][]vm.IpEntry) error {
	// Build IP -> MAC map
	ipToMac := make(map[string]string)
	for mac, ips := range ipPerMac {
		if len(ips) > 0 {
			ipToMac[ips[0].IP] = mac
		}
	}
	
	// List all ifcfg files
	var lsOut string
	var err error
	if useSingleDisk {
		lsOut, err = RunCommandInGuest(diskPath, "ls /etc/sysconfig/network-scripts", false)
	} else {
		lsOut, err = RunCommandInGuestAllVolumes(disks, "ls", false, "/etc/sysconfig/network-scripts")
	}
	if err != nil {
		log.Printf("Warning: Could not list network-scripts directory: %v", err)
		return nil
	}
	
	files := strings.Split(strings.TrimSpace(lsOut), "\n")
	for _, file := range files {
		file = strings.TrimSpace(file)
		if !strings.HasPrefix(file, "ifcfg-") || file == "ifcfg-lo" {
			continue
		}
		
		// Read the file
		var content string
		filePath := fmt.Sprintf("/etc/sysconfig/network-scripts/%s", file)
		if useSingleDisk {
			content, err = RunCommandInGuest(diskPath, fmt.Sprintf("cat %s", filePath), false)
		} else {
			content, err = RunCommandInGuestAllVolumes(disks, "cat", false, filePath)
		}
		if err != nil {
			continue
		}
		
		// Check if it has static IP and no HWADDR
		hasHWADDR := false
		hasIPADDR := false
		var staticIP string
		
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "HWADDR=") {
				hasHWADDR = true
			}
			if strings.HasPrefix(line, "IPADDR=") {
				hasIPADDR = true
				ip := strings.TrimPrefix(line, "IPADDR=")
				staticIP = strings.Trim(ip, "\"'")
				staticIP = strings.TrimSpace(staticIP)
			}
		}
		
		// Only add HWADDR to static IP interfaces that don't have it
		if hasIPADDR && !hasHWADDR && staticIP != "" {
			if mac, found := ipToMac[staticIP]; found {
				// Add HWADDR to the file
				newContent := content
				if !strings.HasSuffix(newContent, "\n") {
					newContent += "\n"
				}
				newContent += fmt.Sprintf("HWADDR=%s\n", mac)
				
				// Write to temp file
				tempFile := fmt.Sprintf("/home/fedora/%s", file)
				f, err := os.Create(tempFile)
				if err != nil {
					log.Printf("Warning: Could not create temp file for %s: %v", file, err)
					continue
				}
				if _, err := f.WriteString(newContent); err != nil {
					f.Close()
					log.Printf("Warning: Could not write temp file for %s: %v", file, err)
					continue
				}
				f.Close()
				
				// Upload back to the disk
				var ans string
				if useSingleDisk {
					ans, err = RunCommandInGuest(diskPath, fmt.Sprintf("upload %s %s", tempFile, filePath), true)
				} else {
					ans, err = RunCommandInGuestAllVolumes(disks, "upload", true, tempFile, filePath)
				}
				if err != nil {
					log.Printf("Warning: Could not upload %s: %v: %s", file, err, strings.TrimSpace(ans))
				} else {
					log.Printf("Added HWADDR=%s to %s", mac, file)
				}
			}
		}
	}
	
	return nil
}

// AddMACToNMKeyfiles adds MAC addresses to NetworkManager keyfile configs
// This allows NetworkManager to match configs by MAC even if interface names change (RHEL 9+)
func AddMACToNMKeyfiles(disks []vm.VMDisk, useSingleDisk bool, diskPath string, ipPerMac map[string][]vm.IpEntry) error {
	// Build IP -> MAC map
	ipToMac := make(map[string]string)
	for mac, ips := range ipPerMac {
		if len(ips) > 0 {
			ipToMac[ips[0].IP] = mac
		}
	}
	
	// List .nmconnection files
	var lsOut string
	var err error
	if useSingleDisk {
		lsOut, err = RunCommandInGuest(diskPath, "ls /etc/NetworkManager/system-connections", false)
	} else {
		lsOut, err = RunCommandInGuestAllVolumes(disks, "ls", false, "/etc/NetworkManager/system-connections")
	}
	if err != nil {
		log.Printf("Info: NetworkManager keyfiles directory not found (not RHEL 9): %v", err)
		return nil // Not an error
	}
	
	files := strings.Split(strings.TrimSpace(lsOut), "\n")
	for _, file := range files {
		file = strings.TrimSpace(file)
		if !strings.HasSuffix(file, ".nmconnection") {
			continue
		}
		
		// Read the file
		var content string
		filePath := fmt.Sprintf("/etc/NetworkManager/system-connections/%s", file)
		if useSingleDisk {
			content, err = RunCommandInGuest(diskPath, fmt.Sprintf("cat %s", filePath), false)
		} else {
			content, err = RunCommandInGuestAllVolumes(disks, "cat", false, filePath)
		}
		if err != nil {
			continue
		}
		
		// Check if it has static IP and no MAC
		hasMac := false
		hasStaticIP := false
		var staticIP string
		var inEthernetSection bool
		
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			
			// Track sections
			if strings.HasPrefix(line, "[ethernet]") {
				inEthernetSection = true
			} else if strings.HasPrefix(line, "[") {
				inEthernetSection = false
			}
			
			// Check for existing MAC
			if inEthernetSection && strings.HasPrefix(line, "mac-address=") {
				hasMac = true
			}
			
			// Get static IP
			if strings.HasPrefix(line, "address1=") {
				hasStaticIP = true
				addr := strings.TrimPrefix(line, "address1=")
				parts := strings.Split(addr, ",")
				if len(parts) > 0 {
					ipParts := strings.Split(parts[0], "/")
					if len(ipParts) > 0 {
						staticIP = strings.TrimSpace(ipParts[0])
					}
				}
			}
		}
		
		// Only add MAC to static IP connections that don't have it
		if hasStaticIP && !hasMac && staticIP != "" {
			if mac, found := ipToMac[staticIP]; found {
				// Add MAC to the [ethernet] section
				newContent := ""
				inEthernetSection = false
				macAdded := false
				
				for _, line := range lines {
					newContent += line + "\n"
					
					// Track when we enter [ethernet] section
					if strings.TrimSpace(line) == "[ethernet]" {
						inEthernetSection = true
					} else if strings.HasPrefix(strings.TrimSpace(line), "[") && strings.TrimSpace(line) != "[ethernet]" {
						// Entering a different section
						if inEthernetSection && !macAdded {
							// We're leaving ethernet section without adding MAC, add it now
							newContent = strings.TrimSuffix(newContent, line+"\n")
							newContent += fmt.Sprintf("mac-address=%s\n", mac)
							newContent += line + "\n"
							macAdded = true
						}
						inEthernetSection = false
					}
					
					// If we're in ethernet section and haven't added MAC yet, add after first line
					if inEthernetSection && !macAdded && strings.TrimSpace(line) == "[ethernet]" {
						newContent += fmt.Sprintf("mac-address=%s\n", mac)
						macAdded = true
					}
				}
				
				// If [ethernet] section doesn't exist, create it
				if !macAdded {
					newContent += "\n[ethernet]\n"
					newContent += fmt.Sprintf("mac-address=%s\n", mac)
					macAdded = true
				}
				
				if macAdded {
					// Write to temp file
					tempFile := fmt.Sprintf("/home/fedora/%s", file)
					f, err := os.Create(tempFile)
					if err != nil {
						log.Printf("Warning: Could not create temp file for %s: %v", file, err)
						continue
					}
					if _, err := f.WriteString(newContent); err != nil {
						f.Close()
						log.Printf("Warning: Could not write temp file for %s: %v", file, err)
						continue
					}
					f.Close()
					
					// Upload back to the disk
					var ans string
					if useSingleDisk {
						ans, err = RunCommandInGuest(diskPath, fmt.Sprintf("upload %s %s", tempFile, filePath), true)
					} else {
						ans, err = RunCommandInGuestAllVolumes(disks, "upload", true, tempFile, filePath)
					}
					if err != nil {
						log.Printf("Warning: Could not upload %s: %v: %s", file, err, strings.TrimSpace(ans))
					} else {
						log.Printf("Added mac-address=%s to %s", mac, file)
					}
				}
			}
		}
	}
	
	return nil
}

func InjectMacToIps(disks []vm.VMDisk, useSingleDisk bool, diskPath string, guestNetworks []vjailbreakv1alpha1.GuestNetwork, gatewayIP map[string]string, ipPerMac map[string][]vm.IpEntry) error {
	log.Println("=== Starting InjectMacToIps ===")
	log.Printf("Total MACs to process: %d", len(ipPerMac))
	for mac, ips := range ipPerMac {
		if len(ips) > 0 {
			log.Printf("Input: MAC=%s IP=%s", mac, ips[0].IP)
		}
	}
	
	// Add wildcard to netplan
	macToIPs := ipPerMac
	macToIPsFile := "/home/fedora/macToIP"
	f, err := os.Create(macToIPsFile)
	if err != nil {
		return err
	}
	defer f.Close()
	
	// Add HWADDR to static IP ifcfg files (RHEL 7/8) to prevent network breaking if interface names change
	log.Println("Step 1: Adding HWADDR to static IP ifcfg files...")
	if err := AddHWADDRToStaticInterfaces(disks, useSingleDisk, diskPath, ipPerMac); err != nil {
		log.Printf("Warning: Could not add HWADDR to ifcfg files: %v", err)
	} else {
		log.Println("Successfully added HWADDR to static IP ifcfg files")
	}
	
	// Add MAC to NetworkManager keyfiles (RHEL 9+) to prevent network breaking if interface names change
	log.Println("Step 2: Adding MAC to NetworkManager keyfiles...")
	if err := AddMACToNMKeyfiles(disks, useSingleDisk, diskPath, ipPerMac); err != nil {
		log.Printf("Warning: Could not add MAC to keyfiles: %v", err)
	} else {
		log.Println("Successfully added MAC to NetworkManager keyfiles")
	}
	
	// Get interface names from ifcfg files (RHEL 7/8)
	log.Println("Step 3: Getting interface names from ifcfg files...")
	macToDevice, err := GetInterfaceNamesFromDisk(disks, useSingleDisk, diskPath, ipPerMac)
	if err != nil {
		log.Printf("Warning: Could not get interface names from disk: %v", err)
		macToDevice = make(map[string]string)
	} else {
		log.Printf("Found %d interface mappings from ifcfg files", len(macToDevice))
		for mac, dev := range macToDevice {
			log.Printf("  ifcfg mapping: %s -> %s", mac, dev)
		}
	}
	
	// Also get interface names from NetworkManager keyfiles (RHEL 9+) and merge
	log.Println("Step 4: Getting interface names from NetworkManager keyfiles...")
	nmMacToDevice, err := GetInterfaceNamesFromNMKeyfiles(disks, useSingleDisk, diskPath, ipPerMac)
	if err != nil {
		log.Printf("Warning: Could not get interface names from keyfiles: %v", err)
	} else {
		log.Printf("Found %d interface mappings from keyfiles", len(nmMacToDevice))
		for mac, dev := range nmMacToDevice {
			log.Printf("  keyfile mapping: %s -> %s", mac, dev)
		}
		// Merge keyfile results into macToDevice (keyfiles take precedence if both exist)
		for mac, dev := range nmMacToDevice {
			if _, exists := macToDevice[mac]; !exists {
				macToDevice[mac] = dev
				log.Printf("Added interface from keyfile: %s -> %s", mac, dev)
			} else {
				log.Printf("Interface %s already mapped from ifcfg, skipping keyfile entry", mac)
			}
		}
	}
	
	// Write macToIP file with device names if available
	log.Println("Step 5: Writing macToIP file...")
	for mac, ips := range macToIPs {
		if len(ips) > 0 {
			// Include device name if available
			deviceName := macToDevice[strings.ToLower(mac)]
			var writeErr error
			if deviceName != "" {
				log.Printf("Writing to macToIP: %s:ip:%s:dev:%s", mac, ips[0].IP, deviceName)
				_, writeErr = fmt.Fprintf(f, "%s:ip:%s:dev:%s\n", mac, ips[0].IP, deviceName)
			} else {
				log.Printf("Writing to macToIP (no device): %s:ip:%s", mac, ips[0].IP)
				_, writeErr = fmt.Fprintf(f, "%s:ip:%s\n", mac, ips[0].IP)
			}
			if writeErr != nil {
				return writeErr
			}
		}
	}

	// Construct YAML
	log.Println("Step 6: Uploading macToIP file to guest disk...")
	log.Printf("macToIP file location: %s", macToIPsFile)
	// Upload it to the disk
	os.Setenv("LIBGUESTFS_BACKEND", "direct")
	var ans string
	if useSingleDisk {
		command := "upload /home/fedora/macToIP /etc/macToIP"
		ans, err = RunCommandInGuest(diskPath, command, true)
	} else {
		command := "upload"
		ans, err = RunCommandInGuestAllVolumes(disks, command, true, "/home/fedora/macToIP", "/etc/macToIP")
	}
	if err != nil {
		log.Printf("ERROR: Failed to upload macToIP file: %v: %s", err, strings.TrimSpace(ans))
		return fmt.Errorf("failed to upload macToIP file: %w: %s", err, strings.TrimSpace(ans))
	}
	log.Println("Successfully uploaded macToIP file to /etc/macToIP")
	log.Println("=== Completed InjectMacToIps ===")
	return nil
}

func AddWildcardNetplan(disks []vm.VMDisk, useSingleDisk bool, diskPath string, guestNetworks []vjailbreakv1alpha1.GuestNetwork, gatewayIP map[string]string, ipPerMac map[string][]vm.IpEntry) error {
	// Add wildcard to netplan
	macToIPs := ipPerMac
	macToDNS := make(map[string][]string)
	if len(guestNetworks) > 0 {
		for _, gn := range guestNetworks {
			if strings.Contains(gn.IP, ":") { // skip IPv6 here
				continue
			}
			if len(gn.DNS) > 0 {
				macToDNS[gn.MAC] = gn.DNS
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
		id := fmt.Sprintf("vj%d", idx)
		b.WriteString(fmt.Sprintf("    %s:\n", id))
		b.WriteString("      match:\n")
		b.WriteString(fmt.Sprintf("        macaddress: %s\n", mac))
		b.WriteString("      dhcp4: false\n")
		b.WriteString("      addresses:\n")
		for _, e := range entries {
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
		idx++
	}
	if !routesAdded {
		log.Println("WARNING: No gateway found")
	}
	netplanYAML := b.String()
	log.Printf("NETPLAN YAML : %s", netplanYAML)
	// Create the netplan file
	err := os.WriteFile("/home/fedora/99-wildcard.network", []byte(netplanYAML), 0644)
	if err != nil {
		return fmt.Errorf("failed to create netplan file: %w", err)
	}
	log.Println("Created local netplan file")
	log.Println("Uploading netplan file to disk")
	// Upload it to the disk
	os.Setenv("LIBGUESTFS_BACKEND", "direct")
	var ans string
	if useSingleDisk {
		command := "mv /etc/netplan /etc/netplan-bkp"
		ans, err = RunCommandInGuest(diskPath, command, true)
		if err != nil {
			return fmt.Errorf("failed to run command (%s): %w: %s", command, err, strings.TrimSpace(ans))
		}
		command = "mkdir /etc/netplan"
		ans, err = RunCommandInGuest(diskPath, command, true)
		if err != nil {
			return fmt.Errorf("failed to run command (%s): %w: %s", command, err, strings.TrimSpace(ans))
		}
		command = "upload /home/fedora/99-wildcard.network /etc/netplan/99-wildcard.yaml"
		ans, err = RunCommandInGuest(diskPath, command, true)
	} else {
		command := "mv"
		ans, err = RunCommandInGuestAllVolumes(disks, command, true, "/etc/netplan", "/etc/netplan-bkp")
		if err != nil {
			return fmt.Errorf("failed to run command (%s): %w: %s", command, err, strings.TrimSpace(ans))
		}
		command = "mkdir"
		ans, err = RunCommandInGuestAllVolumes(disks, command, true, "/etc/netplan")
		if err != nil {
			return fmt.Errorf("failed to run command (%s): %w: %s", command, err, strings.TrimSpace(ans))
		}
		command = "upload"
		ans, err = RunCommandInGuestAllVolumes(disks, command, true, "/home/fedora/99-wildcard.network", "/etc/netplan/99-wildcard.yaml")
	}
	if err != nil {
		log.Printf("failed to upload netplan file: %v: %s", err, strings.TrimSpace(ans))
		return fmt.Errorf("failed to upload netplan file: %w: %s", err, strings.TrimSpace(ans))
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

// Runs command inside temporary qemu-kvm that virt-v2v creates
func RunCommandInGuest(path string, command string, write bool) (string, error) {
	os.Setenv("LIBGUESTFS_BACKEND", "direct")
	option := "--ro"
	if write {
		option = "--rw"
	}
	cmd := exec.Command(
		"guestfish",
		option,
		"-a",
		path,
		"-i")
	cmd.Stdin = strings.NewReader(command)
	log.Printf("Executing %s", cmd.String()+" "+command)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to run command (%s): %v: %s", command, err, strings.TrimSpace(string(out)))
	}
	return strings.ToLower(strings.TrimSpace(string(out))), nil
}

// Runs command inside temporary qemu-kvm that virt-v2v creates
func CheckForLVM(disks []vm.VMDisk) (string, error) {
	os.Setenv("LIBGUESTFS_BACKEND", "direct")

	// Get the installed os info
	command := "inspect-os"
	osPath, err := RunCommandInGuestAllVolumes(disks, command, false)
	if err != nil {
		return "", fmt.Errorf("failed to run command (%s): %v: %s", command, err, strings.TrimSpace(osPath))
	}

	// Get the lvs list
	command = "lvs"
	lvsStr, err := RunCommandInGuestAllVolumes(disks, command, false)
	if err != nil {
		return "", fmt.Errorf("failed to run command (%s): %v: %s", command, err, strings.TrimSpace(lvsStr))
	}

	lvs := strings.Split(string(lvsStr), "\n")
	if slices.Contains(lvs, strings.TrimSpace(string(osPath))) {
		return string(strings.TrimSpace(string(osPath))), nil
	}

	return "", fmt.Errorf("LVM not found: %v, %d", lvs, len(lvs))
}

func prepareGuestfishCommand(disks []vm.VMDisk, command string, write bool, args ...string) *exec.Cmd {
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
	cmd.Args = append(cmd.Args, "-i", "--", command)
	cmd.Args = append(cmd.Args, args...)
	return cmd
}

func RunCommandInGuestAllVolumes(disks []vm.VMDisk, command string, write bool, args ...string) (string, error) {
	os.Setenv("LIBGUESTFS_BACKEND", "direct")
	cmd := prepareGuestfishCommand(disks, command, write, args...)
	log.Printf("Executing %s", cmd.String())
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to run command (%s): %v: %s", command, err, strings.TrimSpace(string(out)))
	}
	return strings.ToLower(string(out)), nil
}

// GetDeviceNumberFromPartition returns the device index for a given partition name
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

func GetBootableVolumeIndex(disks []vm.VMDisk) (int, error) {
	command := "list-partitions"
	partitionsStr, err := RunCommandInGuestAllVolumes(disks, command, false)
	if err != nil {
		return -1, fmt.Errorf("failed to run command (%s): %v: %s", command, err, strings.TrimSpace(partitionsStr))
	}

	partitions := strings.Split(strings.TrimSpace(partitionsStr), "\n")
	for _, partition := range partitions {
		deviceNum, err := GetDeviceNumberFromPartition(disks, partition)
		if err == nil {
			return deviceNum, nil
		}
		// Continue to next partition if this one is not bootable or has an error
	}
	return -1, errors.New("bootable volume not found")
}

func AddUdevRules(disks []vm.VMDisk, useSingleDisk bool, diskPath string, interfaces []string, macs []string) error {

	if len(interfaces) != len(macs) {
		return fmt.Errorf("mismatch between number of interfaces and MACs")
	}
	var ans string

	// Create the udev rules content
	var udevRules strings.Builder
	for i, iface := range interfaces {
		udevRules.WriteString(fmt.Sprintf("SUBSYSTEM==\"net\", ACTION==\"add\", ATTR{address}==\"%s\", NAME=\"%s\"\n", macs[i], iface))
		log.Printf("Adding udev rule: %s", udevRules.String())
	}

	err := os.WriteFile("/home/fedora/70-persistent-net.rules", []byte(udevRules.String()), 0644)
	if err != nil {
		return fmt.Errorf("failed to create udev rules file: %s", err)
	}
	log.Println("Uploading udev rules file to disk")
	// Upload it to the disk
	os.Setenv("LIBGUESTFS_BACKEND", "direct")
	if useSingleDisk {
		command := `upload /home/fedora/70-persistent-net.rules /etc/udev/rules.d/70-persistent-net.rules`
		ans, err = RunCommandInGuest(diskPath, command, true)
	} else {
		command := "upload"
		ans, err = RunCommandInGuestAllVolumes(disks, command, true, "/home/fedora/70-persistent-net.rules", "/etc/udev/rules.d/70-persistent-net.rules")
	}
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

func GetInterfaceNames(path string) ([]string, error) {
	cmd := "ls /etc/sysconfig/network-scripts | grep '^ifcfg-'"
	lsOut, err := RunCommandInGuest(path, cmd, false)
	if err != nil {
		return nil, err
	}
	interfaces := []string{}
	// Parse the output
	// Split by newline and trim spaces
	// Ignore 'ifcfg-lo' as it is the loopback interface
	files := strings.Split(strings.TrimSpace(lsOut), "\n")
	for _, file := range files {
		if file == "ifcfg-lo" {
			continue
		}
		content, err := RunCommandInGuest(path, fmt.Sprintf("cat /etc/sysconfig/network-scripts/%s", file), false)
		if err != nil {
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

func GetOsReleaseAllVolumes(disks []vm.VMDisk) (string, error) {
	// Attempt /etc/os-release first
	osRelease, err := RunCommandInGuestAllVolumes(disks, "cat", false, "/etc/os-release")
	if err == nil {
		return osRelease, nil
	}
	log.Printf("Failed to get /etc/os-release: %v", err)
	// Fallback if file is missing
	if strings.Contains(err.Error(), "No such file or directory") {
		fallbackOutput, fallbackErr := RunCommandInGuestAllVolumes(disks, "cat", false, "/etc/redhat-release")
		if fallbackErr != nil {
			return "", fmt.Errorf("failed to get OS release: primary (/etc/os-release): %v, fallback (/etc/redhat-release): %v", err, fallbackErr)
		}
		return fallbackOutput, nil
	}

	// Return original error if not a missing file issue
	return "", err
}

// RunMountPersistenceScript runs the generate-mount-persistence.sh script with --force-uuid option
// during guest inspection phase for Linux migrations
func RunMountPersistenceScript(disks []vm.VMDisk, useSingleDisk bool, diskPath string) error {
	os.Setenv("LIBGUESTFS_BACKEND", "direct")

	// Script should be available in the container at /home/fedora/
	scriptPath := "/home/fedora/generate-mount-persistence.sh"

	// Check if script exists in the container
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return fmt.Errorf("generate-mount-persistence.sh script not found at %s", scriptPath)
	}

	log.Printf("Running generate-mount-persistence.sh with --force-uuid option")

	// Upload the script to the guest VM
	var uploadErr error
	var uploadOutput string

	if useSingleDisk {
		command := fmt.Sprintf("upload %s /tmp/generate-mount-persistence.sh", scriptPath)
		uploadOutput, uploadErr = RunCommandInGuest(diskPath, command, true)
	} else {
		command := "upload"
		uploadOutput, uploadErr = RunCommandInGuestAllVolumes(disks, command, true, scriptPath, "/tmp/generate-mount-persistence.sh")
	}

	if uploadErr != nil {
		return fmt.Errorf("failed to upload generate-mount-persistence.sh: %v: %s", uploadErr, strings.TrimSpace(uploadOutput))
	}

	log.Printf("Successfully uploaded generate-mount-persistence.sh to guest")

	// Make the script executable
	var chmodErr error
	var chmodOutput string

	if useSingleDisk {
		command := "chmod 0755 /tmp/generate-mount-persistence.sh"
		chmodOutput, chmodErr = RunCommandInGuest(diskPath, command, true)
	} else {
		command := "chmod"
		chmodOutput, chmodErr = RunCommandInGuestAllVolumes(disks, command, true, "0755", "/tmp/generate-mount-persistence.sh")
	}

	if chmodErr != nil {
		return fmt.Errorf("failed to make script executable: %v: %s", chmodErr, strings.TrimSpace(chmodOutput))
	}

	log.Printf("Made generate-mount-persistence.sh executable")

	// Run the script with --force-uuid
	var runErr error
	var runOutput string

	if useSingleDisk {
		command := "sh /tmp/generate-mount-persistence.sh --force-uuid"
		runOutput, runErr = RunCommandInGuest(diskPath, command, true)
	} else {
		command := "sh"
		runOutput, runErr = RunCommandInGuestAllVolumes(disks, command, true, "/tmp/generate-mount-persistence.sh --force-uuid")
	}

	if runErr != nil {
		log.Printf("Warning: generate-mount-persistence.sh execution failed: %v: %s", runErr, strings.TrimSpace(runOutput))
		// Don't return error, just log warning as this is not critical
		return nil
	}

	log.Printf("Successfully executed generate-mount-persistence.sh with --force-uuid")
	log.Printf("Script output: %s", strings.TrimSpace(runOutput))

	return nil
}

func RunGetBootablePartitionScript(disks []vm.VMDisk) (string, error) {
	os.Setenv("LIBGUESTFS_BACKEND", "direct")

	// Script should be available in the container at /home/fedora/
	scriptPath := "/home/fedora/get-bootable-partition.sh"

	// Check if script exists in the container
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return "", fmt.Errorf("get-bootable-partition.sh script not found at %s", scriptPath)
	}

	// Upload the script to the guest VM
	var uploadErr error
	var uploadOutput string

	command := "upload"
	uploadOutput, uploadErr = RunCommandInGuestAllVolumes(disks, command, true, scriptPath, "/tmp/get-bootable-partition.sh")

	if uploadErr != nil {
		return "", fmt.Errorf("failed to upload get-bootable-partition.sh: %v: %s", uploadErr, strings.TrimSpace(uploadOutput))
	}

	log.Printf("Successfully uploaded get-bootable-partition.sh to guest")

	// Make the script executable
	var chmodErr error
	var chmodOutput string

	command = "chmod"
	chmodOutput, chmodErr = RunCommandInGuestAllVolumes(disks, command, true, "0755", "/tmp/get-bootable-partition.sh")

	if chmodErr != nil {
		return "", fmt.Errorf("failed to make script executable: %v: %s", chmodErr, strings.TrimSpace(chmodOutput))
	}

	log.Printf("Made get-bootable-partition.sh executable")

	// Run the script
	var runErr error
	var runOutput string

	command = "sh"
	runOutput, runErr = RunCommandInGuestAllVolumes(disks, command, true, "/tmp/get-bootable-partition.sh")

	if runErr != nil {
		return "", fmt.Errorf("failed to run get-bootable-partition.sh: %v: %s", runErr, strings.TrimSpace(runOutput))
	}

	log.Printf("Successfully executed get-bootable-partition.sh")
	log.Printf("Script output: %s", strings.TrimSpace(runOutput))

	return strings.TrimSpace(runOutput), nil
}

// RunNetworkPersistence mounts the disk locally and runs the network persistence script
func RunNetworkPersistence(disks []vm.VMDisk, useSingleDisk bool, diskPath string, ostype string, isNetplan bool) error {
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

	// Construct the guestmount command
	args := []string{"-i", "--rw"}

	if useSingleDisk {
		args = append(args, "-a", diskPath)
	} else {
		for _, disk := range disks {
			args = append(args, "-a", disk.Path)
		}
	}
	args = append(args, mountPoint)

	log.Printf("Mounting disk to %s using guestmount...", mountPoint)
	mountCmd := exec.Command("guestmount", args...)
	if out, err := mountCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("guestmount failed: %v, output: %s", err, string(out))
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
