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

		if strings.HasPrefix(line, "(vim.Datastore.Summary)") && strings.Contains(line, "{") {
			// New datastore entry - save previous one if exists
			// Only process if it's a real datastore entry (contains opening brace)
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
		// Updated regex to handle paths with spaces
		vmxPattern := regexp.MustCompile(`\[([^\]]+)\]\s+(.+\.vmx)`)
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

	diskInfo := &DiskInfo{
		Path: diskPath,
		Name: filepath.Base(diskPath),
	}

	// Read VMDK descriptor to find the actual disk file and type
	descriptor, err := c.ExecuteCommand(fmt.Sprintf("cat %s 2>/dev/null || true", diskPath))
	if err == nil && descriptor != "" {
		// Parse descriptor for createType
		lowerDesc := strings.ToLower(descriptor)
		if strings.Contains(lowerDesc, "createtype") {
			if strings.Contains(lowerDesc, "monolithicsparse") || strings.Contains(lowerDesc, "thin") {
				diskInfo.ProvisionType = "thin"
			} else if strings.Contains(lowerDesc, "monolithicflat") {
				diskInfo.ProvisionType = "thick"
			} else if strings.Contains(lowerDesc, "eagerzeroedthick") {
				diskInfo.ProvisionType = "eagerzeroedthick"
			}
		}

		// Find the -flat.vmdk file reference to get actual size
		// Look for lines like: RW 8388608 VMFS "disk-flat.vmdk"
		for _, line := range strings.Split(descriptor, "\n") {
			if strings.Contains(line, "-flat.vmdk") || strings.Contains(line, "VMFS") {
				// Extract the extent size (in sectors)
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					if sectors, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
						diskInfo.SizeBytes = sectors * 512 // 512 bytes per sector
						break
					}
				}
			}
		}
	}

	// If we still don't have a size, try to get the -flat.vmdk file size directly
	if diskInfo.SizeBytes == 0 {
		// Try to find the corresponding -flat.vmdk file
		flatPath := strings.Replace(diskPath, ".vmdk", "-flat.vmdk", 1)
		output, err := c.ExecuteCommand(fmt.Sprintf("ls -l %s 2>/dev/null || true", flatPath))
		if err == nil && output != "" {
			// Parse ls output: -rw-------    1 root     root     107374182400 Dec  6 10:30 disk-flat.vmdk
			fields := strings.Fields(output)
			if len(fields) >= 5 {
				if size, err := strconv.ParseInt(fields[4], 10, 64); err == nil {
					diskInfo.SizeBytes = size
				}
			}
		}
	}

	return diskInfo, nil
}

// ListStorageDevices returns all storage devices/LUNs visible to the ESXi host
func (c *Client) ListStorageDevices() ([]StorageDeviceInfo, error) {
	if c.sshClient == nil {
		return nil, fmt.Errorf("not connected to ESXi host")
	}

	// Use esxcli to list storage devices
	output, err := c.ExecuteCommand("esxcli storage core device list")
	if err != nil {
		return nil, fmt.Errorf("failed to list storage devices: %w", err)
	}

	devices := []StorageDeviceInfo{}
	lines := strings.Split(output, "\n")

	var currentDevice *StorageDeviceInfo
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			// Empty line marks end of device entry
			if currentDevice != nil && currentDevice.DeviceID != "" {
				devices = append(devices, *currentDevice)
				currentDevice = nil
			}
			continue
		}

		// New device starts with a device ID line
		if !strings.Contains(line, ":") && strings.HasPrefix(line, "naa.") || strings.HasPrefix(line, "t10.") || strings.HasPrefix(line, "mpx.") {
			// Save previous device if exists
			if currentDevice != nil && currentDevice.DeviceID != "" {
				devices = append(devices, *currentDevice)
			}
			currentDevice = &StorageDeviceInfo{
				DeviceID: line,
			}
			continue
		}

		// Parse device properties
		if currentDevice != nil && strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			switch key {
			case "Display Name":
				currentDevice.DisplayName = value
			case "Size":
				// Parse size like "107374 MB" or "1000 GB"
				sizeFields := strings.Fields(value)
				if len(sizeFields) >= 2 {
					if size, err := strconv.ParseInt(sizeFields[0], 10, 64); err == nil {
						unit := sizeFields[1]
						switch unit {
						case "MB":
							currentDevice.Size = size * 1024 * 1024
						case "GB":
							currentDevice.Size = size * 1024 * 1024 * 1024
						case "TB":
							currentDevice.Size = size * 1024 * 1024 * 1024 * 1024
						}
					}
				}
			case "Device Type":
				currentDevice.DeviceType = value
			case "Vendor":
				currentDevice.Vendor = value
			case "Model":
				currentDevice.Model = value
			case "Is Local":
				currentDevice.IsLocal = (value == "true")
			case "Is SSD":
				currentDevice.IsSSD = (value == "true")
			case "Devfs Path":
				currentDevice.DevfsPath = value
			}
		}
	}

	// Add last device if exists
	if currentDevice != nil && currentDevice.DeviceID != "" {
		devices = append(devices, *currentDevice)
	}

	return devices, nil
}
