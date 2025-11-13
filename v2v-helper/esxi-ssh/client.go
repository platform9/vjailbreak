// Copyright © 2024 The vjailbreak authors

package esxissh

import (
	"context"
	"fmt"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

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
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	addr := net.JoinHostPort(hostname, "22")
	dialer := &net.Dialer{}
	netConn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to connect to SSH server: %w", err)
	}

	if deadline, ok := ctx.Deadline(); ok {
		_ = netConn.SetDeadline(deadline)
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
		return "", fmt.Errorf("command timeout after %v: %w", c.commandTimeout, ctx.Err())
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
