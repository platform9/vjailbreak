// Copyright © 2024 The vjailbreak authors

package esxissh

import (
	"context"
	"encoding/xml"
	"fmt"
	"net"
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
		commandTimeout: 30 * time.Second,
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
		Timeout:         30 * time.Second,
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
func (c *Client) StartVmkfstoolsClone(sourceVMDK, targetLUN string) (*VmkfstoolsTask, error) {
	klog.Infof("Starting vmkfstools clone: source=%s, target=%s", sourceVMDK, targetLUN)

	// Build the command with operation and arguments
	command := fmt.Sprintf("%s %s %s", SSHOperationClone, sourceVMDK, targetLUN)
	output, err := c.ExecuteCommand(command)
	if err != nil {
		return nil, fmt.Errorf("failed to start clone: %w", err)
	}

	klog.Infof("Received output from script: %s", output)

	// Parse the XML response from the script
	task, err := parseTaskResponse(output)
	if err != nil {
		return nil, fmt.Errorf("failed to parse clone response: %w", err)
	}

	klog.Infof("Started vmkfstools clone task %s with PID %d", task.TaskId, task.Pid)
	return task, nil
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
