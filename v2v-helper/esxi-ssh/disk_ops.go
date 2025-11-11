// Copyright © 2024 The vjailbreak authors

package esxissh

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ListDatastores returns all datastores on the ESXi host
func (c *Client) ListDatastores() ([]DatastoreInfo, error) {
	if !c.connected {
		return nil, fmt.Errorf("not connected to ESXi host")
	}

	// Use esxcli to list datastores
	output, err := c.ExecuteCommand("esxcli storage filesystem list")
	if err != nil {
		return nil, fmt.Errorf("failed to list datastores: %w", err)
	}

	datastores := []DatastoreInfo{}
	lines := strings.Split(output, "\n")

	// Skip header lines
	for i, line := range lines {
		if i < 2 || strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		// Parse datastore info
		// Format: Mount Point  Volume Name  UUID  Mounted  Type  Size  Free
		datastore := DatastoreInfo{
			Path: fields[0],
			Name: fields[1],
			UUID: fields[2],
			Type: fields[4],
		}

		// Parse size and free space if available
		if len(fields) >= 6 {
			if size, err := parseSize(fields[5]); err == nil {
				datastore.Capacity = size
			}
		}
		if len(fields) >= 7 {
			if free, err := parseSize(fields[6]); err == nil {
				datastore.FreeSpace = free
			}
		}

		datastores = append(datastores, datastore)
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
	if !c.connected {
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

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		// Format: Vmid  Name  File  Guest OS  Version  Annotation
		vm := VMInfo{
			ID:      fields[0],
			Name:    fields[1],
			VMXPath: fields[2],
		}

		// Extract datastore and path from VMX path
		// Format: [datastore1] vm-name/vm-name.vmx
		if matches := regexp.MustCompile(`\[([^\]]+)\]\s+(.+)`).FindStringSubmatch(vm.VMXPath); len(matches) == 3 {
			vm.Datastore = matches[1]
			vm.Path = filepath.Dir(matches[2])
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
	if !c.connected {
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

		// Extract filename (last field)
		filename := fields[len(fields)-1]
		diskPath := filepath.Join(vmPath, filename)

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
	if !c.connected {
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

// StreamDisk streams a VMDK disk to a writer using vmkfstools
func (c *Client) StreamDisk(ctx context.Context, diskPath string, writer io.Writer, options *TransferOptions) error {
	if !c.connected {
		return fmt.Errorf("not connected to ESXi host")
	}

	if options == nil {
		options = DefaultTransferOptions()
	}

	// Get disk info for progress tracking
	diskInfo, err := c.GetDiskInfo(diskPath)
	if err != nil {
		return fmt.Errorf("failed to get disk info: %w", err)
	}

	// Use vmkfstools to clone to stdout, then stream
	// vmkfstools -i source -d thin /dev/stdout | compress | ssh transfer
	command := fmt.Sprintf("vmkfstools -i %s -d thin /dev/stdout", diskPath)

	if options.UseCompression {
		command = fmt.Sprintf("%s | gzip", command)
	}

	session, err := c.sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	// Get stdout pipe
	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	// Start command
	if err := session.Start(command); err != nil {
		return fmt.Errorf("failed to start disk streaming: %w", err)
	}

	// Track progress
	progress := &TransferProgress{
		DiskPath:   diskPath,
		TotalBytes: diskInfo.SizeBytes,
		StartTime:  time.Now(),
	}

	// Copy data with progress tracking
	buf := make([]byte, options.BufferSize)
	totalRead := int64(0)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			n, err := stdout.Read(buf)
			if n > 0 {
				if _, werr := writer.Write(buf[:n]); werr != nil {
					return fmt.Errorf("failed to write data: %w", werr)
				}

				totalRead += int64(n)
				progress.TransferredBytes = totalRead
				progress.LastUpdateTime = time.Now()
				progress.Percentage = float64(totalRead) / float64(diskInfo.SizeBytes) * 100.0

				elapsed := time.Since(progress.StartTime).Seconds()
				if elapsed > 0 {
					progress.BytesPerSecond = float64(totalRead) / elapsed
					remaining := diskInfo.SizeBytes - totalRead
					progress.EstimatedTimeLeft = time.Duration(float64(remaining)/progress.BytesPerSecond) * time.Second
				}

				// Send progress update
				if options.ProgressChan != nil && totalRead%int64(options.ChunkSize) == 0 {
					options.ProgressChan <- *progress
				}
			}

			if err == io.EOF {
				// Send final progress
				if options.ProgressChan != nil {
					options.ProgressChan <- *progress
				}
				break
			}

			if err != nil {
				return fmt.Errorf("failed to read disk data: %w", err)
			}
		}

		if totalRead >= diskInfo.SizeBytes {
			break
		}
	}

	// Wait for command to complete
	if err := session.Wait(); err != nil {
		return fmt.Errorf("disk streaming command failed: %w", err)
	}

	return nil
}

// CloneDisk clones a VMDK to another location using vmkfstools
func (c *Client) CloneDisk(ctx context.Context, sourcePath, destPath string, options *TransferOptions) error {
	if !c.connected {
		return fmt.Errorf("not connected to ESXi host")
	}

	if options == nil {
		options = DefaultTransferOptions()
	}

	// Get source disk info
	diskInfo, err := c.GetDiskInfo(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to get source disk info: %w", err)
	}

	// Build vmkfstools command
	command := fmt.Sprintf("vmkfstools -i %s -d thin %s", sourcePath, destPath)

	// Create progress tracker
	progress := &TransferProgress{
		DiskPath:   sourcePath,
		TotalBytes: diskInfo.SizeBytes,
		StartTime:  time.Now(),
	}

	// Execute command with progress monitoring
	outputChan := make(chan string, 100)
	errChan := make(chan error, 1)

	go func() {
		errChan <- c.ExecuteCommandWithProgress(command, outputChan)
	}()

	// Monitor progress
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errChan:
			if err != nil {
				return fmt.Errorf("clone command failed: %w", err)
			}
			// Command completed successfully
			if options.ProgressChan != nil {
				progress.TransferredBytes = diskInfo.SizeBytes
				progress.Percentage = 100.0
				progress.LastUpdateTime = time.Now()
				options.ProgressChan <- *progress
			}
			return nil
		case output := <-outputChan:
			// Parse vmkfstools progress output
			// Format: "Destination disk format: VMFS thin-provisioned\nCloning disk 'source'...\nClone: 45% done."
			if strings.Contains(output, "% done") {
				if matches := regexp.MustCompile(`(\d+)%`).FindStringSubmatch(output); len(matches) > 1 {
					if percentage, err := strconv.ParseFloat(matches[1], 64); err == nil {
						progress.Percentage = percentage
						progress.TransferredBytes = int64(percentage / 100.0 * float64(diskInfo.SizeBytes))
						progress.LastUpdateTime = time.Now()

						elapsed := time.Since(progress.StartTime).Seconds()
						if elapsed > 0 {
							progress.BytesPerSecond = float64(progress.TransferredBytes) / elapsed
							remaining := diskInfo.SizeBytes - progress.TransferredBytes
							if progress.BytesPerSecond > 0 {
								progress.EstimatedTimeLeft = time.Duration(float64(remaining)/progress.BytesPerSecond) * time.Second
							}
						}

						if options.ProgressChan != nil {
							options.ProgressChan <- *progress
						}
					}
				}
			}
		case <-ticker.C:
			// Periodic check - could query dest file size
			continue
		}
	}
}

// parseSize parses a size string like "1.5GB" to bytes
func parseSize(sizeStr string) (int64, error) {
	sizeStr = strings.TrimSpace(sizeStr)

	// Extract number and unit
	var value float64
	var unit string

	scanner := bufio.NewScanner(strings.NewReader(sizeStr))
	scanner.Split(bufio.ScanWords)

	if scanner.Scan() {
		value, _ = strconv.ParseFloat(scanner.Text(), 64)
	}
	if scanner.Scan() {
		unit = strings.ToUpper(scanner.Text())
	}

	// Convert to bytes
	multiplier := int64(1)
	switch unit {
	case "KB", "K":
		multiplier = 1024
	case "MB", "M":
		multiplier = 1024 * 1024
	case "GB", "G":
		multiplier = 1024 * 1024 * 1024
	case "TB", "T":
		multiplier = 1024 * 1024 * 1024 * 1024
	}

	return int64(value * float64(multiplier)), nil
}
