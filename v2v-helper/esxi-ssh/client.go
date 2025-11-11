// Copyright © 2024 The vjailbreak authors

package esxissh

import (
	"fmt"
	"io/ioutil"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

// Client represents an SSH client connected to an ESXi host
type Client struct {
	credentials *ESXiCredentials
	sshClient   *ssh.Client
	connected   bool
}

// NewClient creates a new ESXi SSH client
func NewClient(credentials *ESXiCredentials) *Client {
	return &Client{
		credentials: credentials,
		connected:   false,
	}
}

// Connect establishes an SSH connection to the ESXi host
func (c *Client) Connect() error {
	if c.connected {
		return nil // Already connected
	}

	var authMethods []ssh.AuthMethod

	// Try key-based authentication first if SSH key path is provided
	if c.credentials.SSHKeyPath != "" {
		key, err := ioutil.ReadFile(c.credentials.SSHKeyPath)
		if err != nil {
			return fmt.Errorf("failed to read SSH key: %w", err)
		}

		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return fmt.Errorf("failed to parse SSH key: %w", err)
		}

		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	// Add password authentication
	if c.credentials.Password != "" {
		authMethods = append(authMethods, ssh.Password(c.credentials.Password))
	}

	if len(authMethods) == 0 {
		return fmt.Errorf("no authentication method provided (need password or SSH key)")
	}

	config := &ssh.ClientConfig{
		User:            c.credentials.Username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // ESXi often has self-signed certs
		Timeout:         30 * time.Second,
	}

	// Default port is 22 if not specified
	port := c.credentials.Port
	if port == 0 {
		port = 22
	}

	addr := fmt.Sprintf("%s:%d", c.credentials.Host, port)

	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("failed to connect to ESXi host %s: %w", addr, err)
	}

	c.sshClient = client
	c.connected = true

	return nil
}

// Disconnect closes the SSH connection
func (c *Client) Disconnect() error {
	if !c.connected || c.sshClient == nil {
		return nil
	}

	err := c.sshClient.Close()
	c.connected = false
	c.sshClient = nil

	return err
}

// IsConnected returns whether the client is currently connected
func (c *Client) IsConnected() bool {
	return c.connected && c.sshClient != nil
}

// ExecuteCommand runs a command on the ESXi host and returns stdout
func (c *Client) ExecuteCommand(command string) (string, error) {
	if !c.connected {
		return "", fmt.Errorf("not connected to ESXi host")
	}

	session, err := c.sshClient.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w (output: %s)", err, string(output))
	}

	return string(output), nil
}

// ExecuteCommandWithProgress runs a command and streams output line by line
func (c *Client) ExecuteCommandWithProgress(command string, outputChan chan<- string) error {
	if !c.connected {
		return fmt.Errorf("not connected to ESXi host")
	}

	session, err := c.sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	// Set up pipes for stdout and stderr
	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// Start the command
	if err := session.Start(command); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	// Read output in goroutines
	done := make(chan error, 1)

	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stdout.Read(buf)
			if n > 0 && outputChan != nil {
				outputChan <- string(buf[:n])
			}
			if err != nil {
				break
			}
		}
	}()

	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if n > 0 && outputChan != nil {
				outputChan <- string(buf[:n])
			}
			if err != nil {
				break
			}
		}
	}()

	// Wait for command to finish
	go func() {
		done <- session.Wait()
	}()

	return <-done
}

// OpenTunnel creates an SSH tunnel for data transfer
func (c *Client) OpenTunnel(remoteHost string, remotePort int) (net.Conn, error) {
	if !c.connected {
		return nil, fmt.Errorf("not connected to ESXi host")
	}

	addr := fmt.Sprintf("%s:%d", remoteHost, remotePort)
	conn, err := c.sshClient.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to open tunnel to %s: %w", addr, err)
	}

	return conn, nil
}

// TestConnection verifies the SSH connection is working
func (c *Client) TestConnection() error {
	if !c.connected {
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
