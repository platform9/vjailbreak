// Copyright © 2024 The vjailbreak authors

package esxissh

import (
	"context"
	"encoding/xml"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"k8s.io/klog/v2"
)

// SSHOperation represents the type of SSH operation
type SSHOperation string

const (
	SSHOperationClone   SSHOperation = "clone"
	SSHOperationStatus  SSHOperation = "status"
	SSHOperationCleanup SSHOperation = "cleanup"
)

// VmkfstoolsTask represents the result of a vmkfstools operation
type VmkfstoolsTask struct {
	TaskId   string `json:"taskId"`
	Pid      int    `json:"pid"`
	ExitCode string `json:"exitCode"`
	LastLine string `json:"lastLine"`
	Stderr   string `json:"stdErr"`
}

// XMLResponse represents the XML response structure
type XMLResponse struct {
	XMLName   xml.Name  `xml:"o"`
	Structure Structure `xml:"structure"`
}

// Structure represents the structure element in the XML response
type Structure struct {
	TypeName string  `xml:"typeName,attr"`
	Fields   []Field `xml:"field"`
}

// Field represents a field in the XML response
type Field struct {
	Name   string `xml:"name,attr"`
	String string `xml:"string"`
}

type Client struct {
	hostname       string
	username       string
	sshClient      *ssh.Client
	commandTimeout time.Duration
}

func NewClient() *Client {
	return &Client{
		commandTimeout: 5 * time.Minute, // Increased for long-running operations
	}
}

func NewClientWithTimeout(timeout time.Duration) *Client {
	return &Client{
		commandTimeout: timeout,
	}
}

func (c *Client) SetCommandTimeout(timeout time.Duration) {
	c.commandTimeout = timeout
}

func (c *Client) Connect(ctx context.Context, hostname, username string, privateKey []byte) error {
	c.hostname = hostname
	c.username = username

	signer, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		// WARNING: InsecureIgnoreHostKey bypasses host key verification.
		// This is acceptable for ESXi hosts which typically use self-signed certificates,
		// but in a production environment with higher security requirements, consider
		// implementing proper host key verification using ssh.FixedHostKey().
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         2 * time.Minute, // Increased timeout for ESXi connections
	}

	addr := net.JoinHostPort(hostname, "22")
	dialer := &net.Dialer{}
	netConn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to connect to SSH server: %w", err)
	}

	if deadline, ok := ctx.Deadline(); ok {
		if err := netConn.SetDeadline(deadline); err != nil {
			_ = netConn.Close()
			return fmt.Errorf("failed to set connection deadline: %w", err)
		}
	}

	cc, chans, reqs, err := ssh.NewClientConn(netConn, addr, config)
	if err != nil {
		_ = netConn.Close()
		return fmt.Errorf("failed to establish SSH client connection: %w", err)
	}
	c.sshClient = ssh.NewClient(cc, chans, reqs)
	return nil
}

func (c *Client) Disconnect() error {
	if c.sshClient == nil {
		return nil
	}

	err := c.sshClient.Close()
	c.sshClient = nil

	return err
}

func (c *Client) IsConnected() bool {
	return c.sshClient != nil
}

// ExecuteCommand runs a command on the ESXi host and returns stdout
// Uses the client's configured command timeout
func (c *Client) ExecuteCommand(command string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.commandTimeout)
	defer cancel()
	return c.ExecuteCommandWithContext(ctx, command)
}

// ExecuteCommandWithContext runs a command with a custom context (for timeout control)
func (c *Client) ExecuteCommandWithContext(ctx context.Context, command string) (string, error) {
	if c.sshClient == nil {
		return "", fmt.Errorf("not connected to ESXi host")
	}

	session, err := c.sshClient.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	// Channel to receive command result
	type result struct {
		output []byte
		err    error
	}
	resultChan := make(chan result, 1)

	// Run command in goroutine
	go func() {
		output, err := session.CombinedOutput(command)
		resultChan <- result{output: output, err: err}
	}()

	// Wait for either command completion or context cancellation/timeout
	select {
	case <-ctx.Done():
		// Try to kill the session
		_ = session.Signal(ssh.SIGKILL)
		_ = session.Close()
		return "", fmt.Errorf("command cancelled or timed out: %w", ctx.Err())
	case res := <-resultChan:
		if res.err != nil {
			return string(res.output), fmt.Errorf("command failed: %w", res.err)
		}
		return string(res.output), nil
	}
}

func (c *Client) TestConnection() error {
	if c.sshClient == nil {
		return fmt.Errorf("not connected")
	}

	// Run a simple command to test
	output, err := c.ExecuteCommand("esxcli system version get")
	if err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}

	if output == "" {
		return fmt.Errorf("connection test returned no output")
	}

	return nil
}

// StartVmkfstoolsClone starts a vmkfstools clone operation from source VMDK to target LUN
// This will automatically use XCOPY/VAAI hardware offload if available on the storage array
// The clone runs in the background and returns immediately
func (c *Client) StartVmkfstoolsClone(sourceVMDK, targetLUN string) (*VmkfstoolsTask, error) {
	klog.Infof("Starting vmkfstools clone: source=%s, target=%s", sourceVMDK, targetLUN)

	// First check if source exists
	checkCmd := fmt.Sprintf("ls -l %s 2>&1", sourceVMDK)
	checkOutput, _ := c.ExecuteCommand(checkCmd)
	if !strings.Contains(checkOutput, sourceVMDK) {
		return nil, fmt.Errorf("source VMDK does not exist: %s", sourceVMDK)
	}

	// Create target directory if it doesn't exist
	targetDir := targetLUN[:strings.LastIndex(targetLUN, "/")]
	mkdirCmd := fmt.Sprintf("mkdir -p %s", targetDir)
	_, err := c.ExecuteCommand(mkdirCmd)
	if err != nil {
		klog.Warningf("Failed to create target directory: %v", err)
	} else {
		klog.Infof("Created target directory: %s", targetDir)
	}

	// Create a log file for vmkfstools output so we can debug failures
	logFile := fmt.Sprintf("/tmp/vmkfstools_clone_%d.log", time.Now().Unix())

	// Run vmkfstools in background
	// Use vmkfstools -i (clone/import) which automatically uses XCOPY on VAAI-capable storage
	// -d thin creates thin provisioned disk (can also use eagerzeroedthick, zeroedthick)
	// Capture output to log file for debugging
	command := fmt.Sprintf("vmkfstools -i %s %s -d thin >%s 2>&1 & echo $!", sourceVMDK, targetLUN, logFile)

	output, err := c.ExecuteCommand(command)
	if err != nil {
		return nil, fmt.Errorf("failed to start clone: %w", err)
	}

	pid := strings.TrimSpace(output)
	klog.Infof("Started vmkfstools clone with PID: %s, log: %s", pid, logFile)

	// Return task info with PID so we can check status later
	task := &VmkfstoolsTask{
		TaskId:   fmt.Sprintf("vmkfstools-clone-%s", pid),
		Pid:      0, // Will be parsed from output
		LastLine: fmt.Sprintf("Clone started with PID %s, log: %s", pid, logFile),
	}

	// Try to parse PID
	if pidInt, err := fmt.Sscanf(pid, "%d", &task.Pid); err == nil && pidInt == 1 {
		klog.Infof("Parsed PID: %d", task.Pid)
	}

	return task, nil
}

// StartVmkfstoolsRDMClone starts a vmkfstools clone operation from source VMDK to target raw device (RDM)
// This uses VAAI XCOPY to clone directly to a raw device without creating a datastore
// Command format: vmkfstools -i <source> -d rdm:<target_device> <dummy_vmdk_path>
func (c *Client) StartVmkfstoolsRDMClone(sourceVMDK, targetDevicePath string) (*VmkfstoolsTask, error) {
	klog.Infof("Starting vmkfstools RDM clone: source=%s, target=%s", sourceVMDK, targetDevicePath)

	// First check if source exists
	checkCmd := fmt.Sprintf("ls -l %s 2>&1", sourceVMDK)
	checkOutput, _ := c.ExecuteCommand(checkCmd)
	if !strings.Contains(checkOutput, sourceVMDK) {
		return nil, fmt.Errorf("source VMDK does not exist: %s", sourceVMDK)
	}

	// Verify target device exists
	checkDevCmd := fmt.Sprintf("ls -l %s 2>&1", targetDevicePath)
	checkDevOutput, _ := c.ExecuteCommand(checkDevCmd)
	if !strings.Contains(checkDevOutput, targetDevicePath) {
		return nil, fmt.Errorf("target device does not exist: %s", targetDevicePath)
	}

	// Create a temporary directory for the RDM descriptor file
	tmpDir := fmt.Sprintf("/tmp/vaai-rdm-%d", time.Now().Unix())
	mkdirCmd := fmt.Sprintf("mkdir -p %s", tmpDir)
	_, err := c.ExecuteCommand(mkdirCmd)
	if err != nil {
		klog.Warningf("Failed to create temp directory: %v", err)
	} else {
		klog.Infof("Created temp directory: %s", tmpDir)
	}

	// Create a log file for vmkfstools output
	logFile := fmt.Sprintf("/tmp/vmkfstools_rdm_clone_%d.log", time.Now().Unix())

	// RDM descriptor file path (vmkfstools will create this)
	rdmDescriptor := fmt.Sprintf("%s/rdm-disk.vmdk", tmpDir)

	// Run vmkfstools in background with RDM format
	// vmkfstools -i <source> -d rdm:<device> <rdm_descriptor>
	command := fmt.Sprintf("vmkfstools -i %s -d rdm:%s %s >%s 2>&1 & echo $!",
		sourceVMDK, targetDevicePath, rdmDescriptor, logFile)

	output, err := c.ExecuteCommand(command)
	if err != nil {
		return nil, fmt.Errorf("failed to start RDM clone: %w", err)
	}

	pid := strings.TrimSpace(output)
	klog.Infof("Started vmkfstools RDM clone with PID: %s, log: %s", pid, logFile)

	// Return task info with PID so we can check status later
	task := &VmkfstoolsTask{
		TaskId:   fmt.Sprintf("vmkfstools-rdm-clone-%s", pid),
		Pid:      0, // Will be parsed from output
		LastLine: fmt.Sprintf("RDM clone started with PID %s, log: %s", pid, logFile),
	}

	// Try to parse PID
	if pidInt, err := fmt.Sscanf(pid, "%d", &task.Pid); err == nil && pidInt == 1 {
		klog.Infof("Parsed PID: %d", task.Pid)
	}

	return task, nil
}

// GetCloneLog retrieves the vmkfstools clone log output
func (c *Client) GetCloneLog(pid int) (string, error) {
	if c.sshClient == nil {
		return "", fmt.Errorf("not connected to ESXi host")
	}

	// Try to find the log file for this PID
	// We use a pattern since we don't know the exact timestamp
	cmd := fmt.Sprintf("cat /tmp/vmkfstools_clone_*.log 2>/dev/null | tail -n 100")
	output, _ := c.ExecuteCommand(cmd)

	return output, nil
}

// CheckCloneStatus checks if a clone operation is still running
func (c *Client) CheckCloneStatus(pid int) (bool, error) {
	// Check if process is still running using ps
	checkCmd := fmt.Sprintf("ps -c -p %d 2>/dev/null | grep %d", pid, pid)
	output, _ := c.ExecuteCommand(checkCmd)

	isRunning := strings.Contains(output, fmt.Sprintf("%d", pid))

	return isRunning, nil
}

// VerifyVMDKClone verifies that a cloned VMDK was created successfully
func (c *Client) VerifyVMDKClone(vmdkPath string) error {
	if c.sshClient == nil {
		return fmt.Errorf("not connected to ESXi host")
	}

	// Use test -f to check if file exists (more reliable than ls)
	descCmd := fmt.Sprintf("test -f %s && echo 'exists' || echo 'missing'", vmdkPath)
	descOutput, err := c.ExecuteCommand(descCmd)
	if err != nil || !strings.Contains(descOutput, "exists") {
		// Try with ls as fallback
		lsOutput, _ := c.ExecuteCommand(fmt.Sprintf("ls -l %s 2>&1", vmdkPath))
		return fmt.Errorf("descriptor file not found at %s (output: %s)", vmdkPath, lsOutput)
	}

	// Check flat file exists
	flatFile := strings.Replace(vmdkPath, ".vmdk", "-flat.vmdk", 1)
	flatCmd := fmt.Sprintf("test -f %s && echo 'exists' || echo 'missing'", flatFile)
	flatOutput, err := c.ExecuteCommand(flatCmd)
	if err != nil || !strings.Contains(flatOutput, "exists") {
		// Try with ls as fallback
		lsOutput, _ := c.ExecuteCommand(fmt.Sprintf("ls -l %s 2>&1", flatFile))
		return fmt.Errorf("flat file not found at %s (output: %s)", flatFile, lsOutput)
	}

	klog.Infof("VMDK clone verified: descriptor and flat files exist at %s", vmdkPath)
	return nil
}

// GetVMDKSize returns the size of a VMDK in bytes by reading the descriptor
func (c *Client) GetVMDKSize(vmdkPath string) (int64, error) {
	if c.sshClient == nil {
		return 0, fmt.Errorf("not connected to ESXi host")
	}

	// Read VMDK descriptor to find size
	descriptor, err := c.ExecuteCommand(fmt.Sprintf("cat %s 2>/dev/null", vmdkPath))
	if err != nil || descriptor == "" {
		return 0, fmt.Errorf("failed to read VMDK descriptor: %w", err)
	}

	// Look for lines like: RW 8388608 VMFS "disk-flat.vmdk"
	// The number is in 512-byte sectors
	for _, line := range strings.Split(descriptor, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "-flat.vmdk") && strings.Contains(line, "VMFS") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if sectors, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
					klog.Infof("Found VMDK size in descriptor: %d sectors (%d bytes)", sectors, sectors*512)
					return sectors * 512, nil // Convert sectors to bytes
				}
			}
		}
	}

	return 0, fmt.Errorf("could not find size in VMDK descriptor")
}

// CheckVMDKExists checks if a VMDK already exists at the given path
func (c *Client) CheckVMDKExists(vmdkPath string) (bool, error) {
	if c.sshClient == nil {
		return false, fmt.Errorf("not connected to ESXi host")
	}

	output, _ := c.ExecuteCommand(fmt.Sprintf("ls -l %s 2>/dev/null", vmdkPath))
	return output != "", nil
}

// DeleteVMDKFiles removes a VMDK and its flat file (for cleanup before clone)
func (c *Client) DeleteVMDKFiles(vmdkPath string) error {
	if c.sshClient == nil {
		return fmt.Errorf("not connected to ESXi host")
	}

	// Delete flat file
	flatFile := strings.Replace(vmdkPath, ".vmdk", "-flat.vmdk", 1)
	_, _ = c.ExecuteCommand(fmt.Sprintf("rm -f %s", flatFile))

	// Delete descriptor
	_, err := c.ExecuteCommand(fmt.Sprintf("rm -f %s", vmdkPath))
	if err != nil {
		return fmt.Errorf("failed to delete VMDK files: %w", err)
	}

	klog.Infof("Deleted existing VMDK files at %s", vmdkPath)
	return nil
}

// CheckVMKernelLogsForXCopy checks vmkernel logs for VAAI XCOPY operations
// Returns recent log lines containing XCOPY/VAAI indicators
func (c *Client) CheckVMKernelLogsForXCopy(timeWindowMinutes int) (string, error) {
	if c.sshClient == nil {
		return "", fmt.Errorf("not connected to ESXi host")
	}

	// Search for XCOPY-related keywords in vmkernel logs
	// Common indicators: "XCOPY", "VAAI", "ExtendedCopy", "vmw_ahci"
	cmd := fmt.Sprintf("tail -n 1000 /var/log/vmkernel.log | grep -i -E 'xcopy|vaai|extendedcopy|clone' | tail -n 50")

	output, err := c.ExecuteCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to check vmkernel logs: %w", err)
	}

	return output, nil
}

// CheckStorageIOStats checks storage I/O statistics to verify VAAI usage
func (c *Client) CheckStorageIOStats(deviceName string) (string, error) {
	if c.sshClient == nil {
		return "", fmt.Errorf("not connected to ESXi host")
	}

	// Get VAAI statistics for a device
	cmd := fmt.Sprintf("esxcli storage core device vaai status get -d %s", deviceName)

	output, err := c.ExecuteCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to get VAAI stats: %w", err)
	}

	return output, nil
}

// RunEsxcliCommand runs an esxcli command with XML formatter and returns parsed results
// Returns a slice of maps, where each map represents a row with column name -> value
func (c *Client) RunEsxcliCommand(namespace string, args []string) ([]map[string]string, error) {
	if c.sshClient == nil {
		return nil, fmt.Errorf("not connected to ESXi host")
	}

	// Build command with XML formatter for structured output
	cmd := fmt.Sprintf("esxcli --formatter=xml %s %s", namespace, strings.Join(args, " "))
	klog.Infof("Running esxcli command: %s", cmd)

	output, err := c.ExecuteCommand(cmd)
	if err != nil {
		return nil, fmt.Errorf("esxcli command failed: %w", err)
	}

	return parseEsxcliXMLOutput(output)
}

// EsxcliListResponse represents the XML list response from esxcli
type EsxcliListResponse struct {
	XMLName xml.Name            `xml:"output"`
	List    EsxcliStructureList `xml:"list"`
}

// EsxcliStructureList represents a list of structures
type EsxcliStructureList struct {
	Structures []EsxcliStructure `xml:"structure"`
}

// EsxcliStructure represents a single structure with fields
type EsxcliStructure struct {
	Fields []EsxcliField `xml:"field"`
}

// EsxcliField represents a field in the structure
type EsxcliField struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"string"`
}

// parseEsxcliXMLOutput parses XML output from esxcli --formatter=xml
func parseEsxcliXMLOutput(output string) ([]map[string]string, error) {
	var resp EsxcliListResponse
	if err := xml.Unmarshal([]byte(output), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse esxcli XML output: %w", err)
	}

	var results []map[string]string
	for _, structure := range resp.List.Structures {
		row := make(map[string]string)
		for _, field := range structure.Fields {
			row[field.Name] = field.Value
		}
		results = append(results, row)
	}

	return results, nil
}

// parseTaskResponse parses the XML response from vmkfstools operations
func parseTaskResponse(output string) (*VmkfstoolsTask, error) {
	var xmlResp XMLResponse
	if err := xml.Unmarshal([]byte(output), &xmlResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal XML: %w", err)
	}

	task := &VmkfstoolsTask{}
	for _, field := range xmlResp.Structure.Fields {
		switch field.Name {
		case "taskId":
			task.TaskId = field.String
		case "pid":
			var pid int
			if _, err := fmt.Sscanf(field.String, "%d", &pid); err == nil {
				task.Pid = pid
			}
		case "exitCode":
			task.ExitCode = field.String
		case "lastLine":
			task.LastLine = field.String
		case "stdErr":
			task.Stderr = field.String
		}
	}

	return task, nil
}
