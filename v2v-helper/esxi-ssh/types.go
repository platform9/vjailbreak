// Copyright © 2024 The vjailbreak authors

package esxissh

import (
	"time"
)

// ESXiCredentials contains authentication information for ESXi host
type ESXiCredentials struct {
	Host     string
	Port     int
	Username string
	Password string
	// Optional: SSH key path for key-based authentication
	SSHKeyPath string
}

// DiskInfo represents a VMDK disk on ESXi
type DiskInfo struct {
	Path          string // Full datastore path, e.g., /vmfs/volumes/datastore1/vm-name/disk.vmdk
	Name          string // Disk filename
	SizeBytes     int64  // Disk size in bytes
	ProvisionType string // thin, thick, etc.
	Datastore     string // Datastore name
}

// VMInfo represents a VM on ESXi
type VMInfo struct {
	Name      string
	ID        string // VM ID from ESXi
	VMXPath   string // Path to .vmx file
	Path      string // VM directory path
	Disks     []DiskInfo
	Datastore string
}

// DatastoreInfo represents a datastore on ESXi
type DatastoreInfo struct {
	Name      string
	Path      string // /vmfs/volumes/...
	UUID      string // Datastore UUID
	Type      string // VMFS, NFS, etc.
	Capacity  int64
	FreeSpace int64
}

// TransferProgress tracks disk transfer progress
type TransferProgress struct {
	DiskPath         string
	TotalBytes       int64
	TransferredBytes int64
	StartTime        time.Time
	LastUpdateTime   time.Time
	Percentage       float64
	BytesPerSecond   float64
	EstimatedTimeLeft time.Duration
}

// TransferOptions configures disk transfer behavior
type TransferOptions struct {
	BufferSize     int    // Buffer size for streaming (default: 64MB)
	ChunkSize      int    // Chunk size for progress reporting (default: 1GB)
	UseCompression bool   // Enable compression during transfer
	VerifyChecksum bool   // Verify checksum after transfer
	ProgressChan   chan<- TransferProgress // Optional channel for progress updates
}

// DefaultTransferOptions returns sensible defaults
func DefaultTransferOptions() *TransferOptions {
	return &TransferOptions{
		BufferSize:     64 * 1024 * 1024, // 64MB
		ChunkSize:      1024 * 1024 * 1024, // 1GB
		UseCompression: false,
		VerifyChecksum: false,
	}
}
