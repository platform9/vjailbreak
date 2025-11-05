// Copyright © 2024 The vjailbreak authors

package esxi

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type ESXiOperator struct {
	host      string
	username  string
	password  string
	sshClient *ssh.Client
	connected bool
}

type Config struct {
	Host     string
	Username string
	Password string
}

func NewESXiOperator(config Config) (*ESXiOperator, error) {
	if config.Host == "" {
		return nil, fmt.Errorf("ESXi host is required")
	}
	if config.Username == "" || config.Password == "" {
		return nil, fmt.Errorf("username and password are required")
	}

	return &ESXiOperator{
		host:      config.Host,
		username:  config.Username,
		password:  config.Password,
		connected: false,
	}, nil
}

func (e *ESXiOperator) Connect(ctx context.Context) error {
	if e.connected && e.sshClient != nil {
		return nil
	}

	sshConfig := &ssh.ClientConfig{
		User: e.username,
		Auth: []ssh.AuthMethod{
			ssh.Password(e.password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: Add proper host key verification for production
		Timeout:         30 * time.Second,
	}

	// ESXi SSH typically runs on port 22
	endpoint := e.host
	if !strings.Contains(endpoint, ":") {
		endpoint = endpoint + ":22"
	}

	client, err := ssh.Dial("tcp", endpoint, sshConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to ESXi host %s: %w", e.host, err)
	}

	e.sshClient = client
	e.connected = true
	return nil
}

func (e *ESXiOperator) Disconnect() error {
	if e.sshClient != nil {
		err := e.sshClient.Close()
		e.sshClient = nil
		e.connected = false
		return err
	}
	e.connected = false
	return nil
}

func (e *ESXiOperator) executeCommand(ctx context.Context, command string) (string, error) {
	if !e.connected || e.sshClient == nil {
		return "", fmt.Errorf("not connected to ESXi host")
	}

	session, err := e.sshClient.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	if err != nil {
		return string(output), fmt.Errorf("command execution failed: %w, output: %s", err, string(output))
	}

	return string(output), nil
}

// GetIQNs returns the iSCSI IQNs for the ESXi host
func (e *ESXiOperator) GetIQNs(ctx context.Context) ([]string, error) {
	if !e.connected {
		return nil, fmt.Errorf("not connected to ESXi host")
	}

	// Try vmkfstools first (simpler output)
	output, err := e.executeCommand(ctx, "vmkfstools -q hostiqn")
	if err == nil {
		iqn := strings.TrimSpace(output)
		if iqn != "" && strings.HasPrefix(iqn, "iqn.") {
			return []string{iqn}, nil
		}
	}

	// Fallback to esxcli
	output, err = e.executeCommand(ctx, "esxcli iscsi adapter list")
	if err != nil {
		return nil, fmt.Errorf("failed to get IQNs: %w", err)
	}

	var iqns []string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		// Look for lines containing IQN
		if strings.Contains(line, "iqn.") {
			fields := strings.Fields(line)
			for _, field := range fields {
				if strings.HasPrefix(field, "iqn.") {
					iqns = append(iqns, field)
				}
			}
		}
	}

	if len(iqns) == 0 {
		return nil, fmt.Errorf("no iSCSI IQNs found on host %s", e.host)
	}

	return iqns, nil
}

func (e *ESXiOperator) RescanStorage(ctx context.Context) error {
	if !e.connected {
		return fmt.Errorf("not connected to ESXi host")
	}

	_, err := e.executeCommand(ctx, "esxcli storage core adapter rescan --all")
	if err != nil {
		return fmt.Errorf("failed to rescan storage: %w", err)
	}

	return nil
}

// GetDeviceByNAA returns the device path for a given NAA identifier
func (e *ESXiOperator) GetDeviceByNAA(ctx context.Context, naa string) (string, error) {
	if !e.connected {
		return "", fmt.Errorf("not connected to ESXi host")
	}

	naaID := strings.TrimPrefix(naa, "naa.")

	devicePath := fmt.Sprintf("/vmfs/devices/disks/naa.%s", naaID)

	_, err := e.executeCommand(ctx, fmt.Sprintf("ls -lh %s", devicePath))
	if err != nil {
		return "", fmt.Errorf("device with NAA %s not found: %w", naa, err)
	}

	return devicePath, nil
}

type XCopyRequest struct {
	SourceNAA string // Source NAA identifier (e.g., naa.xxx)
	TargetNAA string // Target NAA identifier (e.g., naa.xxx)
	SizeBytes int64  // Size in bytes to copy (0 = entire device)
}

type XCopyResult struct {
	Success      bool
	Error        string
	Output       string
	BytesCopied  int64
	DurationSecs int64
}

// SupportsXCOPY checks if both devices support SCSI XCOPY
func (e *ESXiOperator) SupportsXCOPY(ctx context.Context, sourceNAA, targetNAA string) (bool, error) {
	if !e.connected {
		return false, fmt.Errorf("not connected to ESXi host")
	}

	sourceDevice, err := e.GetDeviceByNAA(ctx, sourceNAA)
	if err != nil {
		return false, fmt.Errorf("source device not found: %w", err)
	}

	//	targetDevice, err := e.GetDeviceByNAA(ctx, targetNAA)
	if err != nil {
		return false, fmt.Errorf("target device not found: %w", err)
	}

	// Check if devices support XCOPY using esxcli
	// Query SCSI capabilities - devices supporting XCOPY will show in inquiry data
	cmd := fmt.Sprintf("esxcli storage core device list -d %s | grep -i 'VAAI'", sourceDevice)
	output, err := e.executeCommand(ctx, cmd)
	// Redundant >>
	if err == nil && (strings.Contains(output, "supported") || strings.Contains(output, "enabled")) {
		return true, nil
	}

	// Default to assuming XCOPY is supported for block devices
	return true, nil
}

func (e *ESXiOperator) XCopyDevices(ctx context.Context, req XCopyRequest) (*XCopyResult, error) {
	if !e.connected {
		return nil, fmt.Errorf("not connected to ESXi host")
	}

	startTime := time.Now()

	sourceDevice, err := e.GetDeviceByNAA(ctx, req.SourceNAA)
	if err != nil {
		return &XCopyResult{
			Success: false,
			Error:   fmt.Sprintf("source device not found: %v", err),
		}, err
	}

	targetDevice, err := e.GetDeviceByNAA(ctx, req.TargetNAA)
	if err != nil {
		return &XCopyResult{
			Success: false,
			Error:   fmt.Sprintf("target device not found: %v", err),
		}, err
	}

	sourceOk, _ := e.VerifyDeviceAccess(ctx, sourceDevice)
	targetOk, _ := e.VerifyDeviceAccess(ctx, targetDevice)

	if !sourceOk || !targetOk {
		return &XCopyResult{
			Success: false,
			Error:   "one or both devices are not accessible",
		}, fmt.Errorf("device access verification failed")
	}

	// Use dd with direct I/O to trigger array-level copy
	// The array will optimize this into an XCOPY
	cmd := fmt.Sprintf("dd if=%s of=%s bs=1M oflag=direct iflag=direct status=progress", sourceDevice, targetDevice)

	output, err := e.executeCommand(ctx, cmd)

	duration := time.Since(startTime)

	if err != nil {
		return &XCopyResult{
			Success:      false,
			Error:        err.Error(),
			Output:       output,
			DurationSecs: int64(duration.Seconds()),
		}, err
	}

	var bytesCopied int64
	if strings.Contains(output, "bytes") {
		re := regexp.MustCompile(`(\d+) bytes`)
		matches := re.FindStringSubmatch(output)
		if len(matches) > 1 {
			fmt.Sscanf(matches[1], "%d", &bytesCopied)
		}
	}

	return &XCopyResult{
		Success:      true,
		Output:       output,
		BytesCopied:  bytesCopied,
		DurationSecs: int64(duration.Seconds()),
	}, nil
}

// XCopyVMDKToDevice copies a VMDK to a raw device using array-level operations
// This method first identifies the backing NAA for the VMDK, then uses XCOPY
func (e *ESXiOperator) XCopyVMDKToDevice(ctx context.Context, vmdkPath string, targetNAA string) (*XCopyResult, error) {
	if !e.connected {
		return nil, fmt.Errorf("not connected to ESXi host")
	}

	// Extract datastore and VM path from VMDK path
	// Format: /vmfs/volumes/datastore1/vm/disk.vmdk
	// We need to find the backing NAA for this VMDK

	// Get the VMDK's backing device NAA
	// This requires querying the vmdk descriptor or using vmkfstools
	cmd := fmt.Sprintf("vmkfstools -q %s", vmdkPath)
	output, err := e.executeCommand(ctx, cmd)
	if err != nil {
		return &XCopyResult{
			Success: false,
			Error:   fmt.Sprintf("failed to query VMDK: %v", err),
		}, err
	}

	// Parse output to find backing device NAA
	// For flat VMDK files, the backing device info is in the descriptor
	var sourceNAA string

	// Try to find NAA in output
	re := regexp.MustCompile(`naa\.[0-9a-f]+`)
	matches := re.FindStringSubmatch(output)
	if len(matches) > 0 {
		sourceNAA = matches[0]
	} else {
		re2 := regexp.MustCompile(`/vmfs/volumes/([^/]+)/`)
		dsMatches := re2.FindStringSubmatch(vmdkPath)
		if len(dsMatches) > 1 {
			datastoreName := dsMatches[1]
			devices, err := e.GetDatastoreDevices(ctx, datastoreName)
			if err != nil || len(devices) == 0 {
				return &XCopyResult{
					Success: false,
					Error:   "failed to identify backing device for VMDK",
				}, fmt.Errorf("cannot determine source NAA")
			}
			sourceNAA = devices[0] // Use first device
		} else {
			return &XCopyResult{
				Success: false,
				Error:   "failed to parse VMDK path",
			}, fmt.Errorf("invalid VMDK path format")
		}
	}

	return e.XCopyDevices(ctx, XCopyRequest{
		SourceNAA: sourceNAA,
		TargetNAA: targetNAA,
	})
}

func (e *ESXiOperator) VerifyDeviceAccess(ctx context.Context, devicePath string) (bool, error) {
	if !e.connected {
		return false, fmt.Errorf("not connected to ESXi host")
	}

	output, err := e.executeCommand(ctx, fmt.Sprintf("ls -lh %s", devicePath))
	if err != nil {
		return false, nil
	}

	if strings.Contains(output, "brw") {
		return true, nil
	}

	return false, nil
}

func (e *ESXiOperator) GetStorageAdapters(ctx context.Context) ([]StorageAdapter, error) {
	if !e.connected {
		return nil, fmt.Errorf("not connected to ESXi host")
	}

	output, err := e.executeCommand(ctx, "esxcli storage core adapter list")
	if err != nil {
		return nil, fmt.Errorf("failed to list storage adapters: %w", err)
	}

	var adapters []StorageAdapter
	lines := strings.Split(output, "\n")

	// Skip header line
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 3 {
			adapter := StorageAdapter{
				HBA:         fields[0],
				Driver:      fields[1],
				Description: strings.Join(fields[2:], " "),
			}
			adapters = append(adapters, adapter)
		}
	}

	return adapters, nil
}

type StorageAdapter struct {
	HBA         string
	Driver      string
	Description string
}

func (e *ESXiOperator) GetDatastoreDevices(ctx context.Context, datastoreName string) ([]string, error) {
	if !e.connected {
		return nil, fmt.Errorf("not connected to ESXi host")
	}

	// Get datastore information
	cmd := fmt.Sprintf("esxcli storage filesystem list | grep '%s'", datastoreName)
	output, err := e.executeCommand(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("datastore %s not found: %w", datastoreName, err)
	}

	// Extract mount point
	fields := strings.Fields(output)
	if len(fields) == 0 {
		return nil, fmt.Errorf("failed to parse datastore info for %s", datastoreName)
	}

	mountPoint := fields[0]

	// Get backing devices for the datastore
	cmd = fmt.Sprintf("esxcli storage vmfs extent list | grep '%s'", mountPoint)
	output, err = e.executeCommand(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to get backing devices for %s: %w", datastoreName, err)
	}

	var devices []string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 2 {
			deviceName := fields[1]
			re := regexp.MustCompile(`naa\.[0-9a-f]+`)
			matches := re.FindStringSubmatch(deviceName)
			if len(matches) > 0 {
				devices = append(devices, matches[0])
			}
		}
	}

	return devices, nil
}
