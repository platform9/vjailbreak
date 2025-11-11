// Copyright © 2024 The vjailbreak authors

package esxissh

import (
	"context"
	"io"
)

//go:generate mockgen -source=operations.go -destination=operations_mock.go -package=esxissh

// ESXiOperations defines the interface for ESXi operations via SSH
type ESXiOperations interface {
	// Connection management
	Connect() error
	Disconnect() error
	IsConnected() bool
	TestConnection() error

	// Datastore operations
	ListDatastores() ([]DatastoreInfo, error)
	GetDatastoreInfo(datastoreName string) (*DatastoreInfo, error)

	// VM operations
	ListVMs() ([]VMInfo, error)
	GetVMInfo(vmName string) (*VMInfo, error)
	GetVMDisks(vmPath string) ([]DiskInfo, error)

	// Disk operations
	GetDiskInfo(diskPath string) (*DiskInfo, error)
	StreamDisk(ctx context.Context, diskPath string, writer io.Writer, options *TransferOptions) error
	CloneDisk(ctx context.Context, sourcePath, destPath string, options *TransferOptions) error

	// Utility operations
	ExecuteCommand(command string) (string, error)
}

// Ensure Client implements ESXiOperations
var _ ESXiOperations = (*Client)(nil)
