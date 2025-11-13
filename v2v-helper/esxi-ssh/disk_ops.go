// Copyright © 2024 The vjailbreak authors

package esxissh

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// ListDatastores returns all datastores on the ESXi host
// Note: Using vim-cmd instead of esxcli as it's more reliable (esxcli can hang on some hosts)
// This is primarily for testing/debugging - production code gets VMDK paths from vSphere API
func (c *Client) ListDatastores() ([]DatastoreInfo, error) {
	if c.sshClient == nil {
		return nil, fmt.Errorf("not connected to ESXi host")
	}

	// Use vim-cmd which is faster and more reliable than esxcli storage filesystem list
	output, err := c.ExecuteCommand("vim-cmd hostsvc/datastore/listsummary")
	if err != nil {
		return nil, fmt.Errorf("failed to list datastores: %w", err)
	}

	// Parse vim-cmd output format:
	// (vim.Datastore.Summary) {
	//    name = "datastore1",
	//    url = "/vmfs/volumes/...",
	//    capacity = 1099511627776,
	//    freeSpace = 549755813888,
	//    type = "VMFS",
	// }
	datastores := []DatastoreInfo{}
	lines := strings.Split(output, "\n")

	var currentDS *DatastoreInfo
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "(vim.Datastore.Summary)") {
			// New datastore entry - save previous one if exists
			if currentDS != nil {
				datastores = append(datastores, *currentDS)
			}
			currentDS = &DatastoreInfo{}
		} else if currentDS != nil {
			// Parse datastore properties
			if strings.HasPrefix(line, "name = ") {
				currentDS.Name = strings.Trim(strings.TrimPrefix(line, "name = "), "\",")
			} else if strings.HasPrefix(line, "url = ") {
				currentDS.Path = strings.Trim(strings.TrimPrefix(line, "url = "), "\",")
			} else if strings.HasPrefix(line, "capacity = ") {
				capacityStr := strings.Trim(strings.TrimPrefix(line, "capacity = "), ",")
				if capacity, err := strconv.ParseInt(capacityStr, 10, 64); err == nil {
					currentDS.Capacity = capacity
				}
			} else if strings.HasPrefix(line, "freeSpace = ") {
				freeStr := strings.Trim(strings.TrimPrefix(line, "freeSpace = "), ",")
				if free, err := strconv.ParseInt(freeStr, 10, 64); err == nil {
					currentDS.FreeSpace = free
				}
			} else if strings.HasPrefix(line, "type = ") {
				currentDS.Type = strings.Trim(strings.TrimPrefix(line, "type = "), "\",")
			}
		}
	}

	// Add last datastore if exists
	if currentDS != nil {
		datastores = append(datastores, *currentDS)
	}

	return datastores, nil
}

// GetDatastoreInfo returns information about a specific datastore
func (c *Client) GetDatastoreInfo(datastoreName string) (*DatastoreInfo, error) {
	datastores, err := c.ListDatastores()
	if err != nil {
		return nil, err
	}

	for _, ds := range datastores {
		if ds.Name == datastoreName {
			return &ds, nil
		}
	}

	return nil, fmt.Errorf("datastore %s not found", datastoreName)
}

// ListVMs returns all VMs on the ESXi host
func (c *Client) ListVMs() ([]VMInfo, error) {
	if c.sshClient == nil {
		return nil, fmt.Errorf("not connected to ESXi host")
	}

	// Use vim-cmd to list VMs
	output, err := c.ExecuteCommand("vim-cmd vmsvc/getallvms")
	if err != nil {
		return nil, fmt.Errorf("failed to list VMs: %w", err)
	}

	vms := []VMInfo{}
	lines := strings.Split(output, "\n")

	// Skip header line
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}

		// Parse line more carefully - fields are: Vmid  Name  File  Guest OS  Version
		// The File field is like: [datastore] vm-folder/vm-name.vmx
		// We need to extract from the full line, not just split by whitespace

		// Extract VM ID (first field)
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		vmID := fields[0]

		// Find the VMX path - it's everything between the VM name and the Guest OS
		// Look for the pattern [datastore] path/to/file.vmx
		vmxPattern := regexp.MustCompile(`\[([^\]]+)\]\s+([^\s]+\.vmx)`)
		matches := vmxPattern.FindStringSubmatch(line)

		if len(matches) < 3 {
			// Couldn't parse VMX path, skip this VM
			continue
		}

		datastore := matches[1]
		vmxFile := matches[2]
		vmxPath := fmt.Sprintf("[%s] %s", datastore, vmxFile)

		// Extract VM name - it's between the ID and the VMX path
		nameStart := strings.Index(line, vmID) + len(vmID)
		nameEnd := strings.Index(line, "["+datastore+"]")
		vmName := strings.TrimSpace(line[nameStart:nameEnd])

		vm := VMInfo{
			ID:        vmID,
			Name:      vmName,
			VMXPath:   vmxPath,
			Datastore: datastore,
			Path:      filepath.Dir(vmxFile),
		}

		vms = append(vms, vm)
	}

	return vms, nil
}

// GetVMInfo returns detailed information about a specific VM
func (c *Client) GetVMInfo(vmName string) (*VMInfo, error) {
	vms, err := c.ListVMs()
	if err != nil {
		return nil, err
	}

	for _, vm := range vms {
		if vm.Name == vmName {
			// Get disk information for this VM
			disks, err := c.GetVMDisks(vm.VMXPath)
			if err != nil {
				return nil, fmt.Errorf("failed to get disks for VM %s: %w", vmName, err)
			}
			vm.Disks = disks
			return &vm, nil
		}
	}

	return nil, fmt.Errorf("VM %s not found", vmName)
}

// GetVMDisks returns all disks for a VM given its VMX path
func (c *Client) GetVMDisks(vmxPath string) ([]DiskInfo, error) {
	if c.sshClient == nil {
		return nil, fmt.Errorf("not connected to ESXi host")
	}

	// Extract datastore and path
	var datastore, vmDir string
	if matches := regexp.MustCompile(`\[([^\]]+)\]\s+(.+)`).FindStringSubmatch(vmxPath); len(matches) == 3 {
		datastore = matches[1]
		vmDir = filepath.Dir(matches[2])
	} else {
		return nil, fmt.Errorf("invalid VMX path format: %s", vmxPath)
	}

	// List files in VM directory to find VMDK files
	vmPath := fmt.Sprintf("/vmfs/volumes/%s/%s", datastore, vmDir)
	output, err := c.ExecuteCommand(fmt.Sprintf("ls -lh %s/*.vmdk 2>/dev/null || true", vmPath))
	if err != nil {
		return nil, fmt.Errorf("failed to list VM disks: %w", err)
	}

	disks := []DiskInfo{}
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Skip descriptor files (flat.vmdk are the actual data files)
		if strings.Contains(line, "-flat.vmdk") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}

		// Extract filename (last field) - this is the full path from ls output
		diskPath := fields[len(fields)-1]
		filename := filepath.Base(diskPath)

		// Get detailed disk info
		diskInfo, err := c.GetDiskInfo(diskPath)
		if err != nil {
			// If we can't get detailed info, create basic info
			diskInfo = &DiskInfo{
				Path:      diskPath,
				Name:      filename,
				Datastore: datastore,
			}
		}

		disks = append(disks, *diskInfo)
	}

	return disks, nil
}

// GetDiskInfo returns detailed information about a specific VMDK
func (c *Client) GetDiskInfo(diskPath string) (*DiskInfo, error) {
	if c.sshClient == nil {
		return nil, fmt.Errorf("not connected to ESXi host")
	}

	// Use vmkfstools to get disk info
	output, err := c.ExecuteCommand(fmt.Sprintf("vmkfstools -q %s", diskPath))
	if err != nil {
		return nil, fmt.Errorf("failed to get disk info: %w", err)
	}

	diskInfo := &DiskInfo{
		Path: diskPath,
		Name: filepath.Base(diskPath),
	}

	// Parse vmkfstools output
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)

		// Extract disk type
		if strings.Contains(line, "Disk Type:") {
			if strings.Contains(line, "thin") {
				diskInfo.ProvisionType = "thin"
			} else if strings.Contains(line, "thick") {
				diskInfo.ProvisionType = "thick"
			} else if strings.Contains(line, "eagerzeroedthick") {
				diskInfo.ProvisionType = "eagerzeroedthick"
			}
		}

		// Extract size
		if strings.Contains(line, "geometry") {
			// Parse geometry line to get size
			if matches := regexp.MustCompile(`(\d+)\s+sectors`).FindStringSubmatch(line); len(matches) > 1 {
				sectors, _ := strconv.ParseInt(matches[1], 10, 64)
				diskInfo.SizeBytes = sectors * 512 // 512 bytes per sector
			}
		}
	}

	// Get actual file size as fallback
	if diskInfo.SizeBytes == 0 {
		sizeOutput, err := c.ExecuteCommand(fmt.Sprintf("stat -c %%s %s 2>/dev/null || stat -f %%z %s", diskPath, diskPath))
		if err == nil {
			if size, err := strconv.ParseInt(strings.TrimSpace(sizeOutput), 10, 64); err == nil {
				diskInfo.SizeBytes = size
			}
		}
	}

	return diskInfo, nil
}
