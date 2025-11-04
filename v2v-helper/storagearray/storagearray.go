// Copyright © 2024 The vjailbreak authors
package storagearray

import (
	"context"
)

// Implementations will have to exist for different storage vendors (Pure, NetApp, etc.)
type StorageOperator interface {
	// Connect establishes a connection to the storage array management endpoint
	Connect(ctx context.Context) error
	// Disconnect closes the connection to the storage array
	Disconnect() error
	// GetVersion returns the storage array software version
	GetVersion(ctx context.Context) (string, error)
	CreateInitiatorGroup(ctx context.Context, name string, initiators []string) (string, error)
	DeleteInitiatorGroup(ctx context.Context, name string) error
	MapVolumeToInitiatorGroup(ctx context.Context, volumeIdentifier string, initiatorGroupName string) error
	UnmapVolumeFromInitiatorGroup(ctx context.Context, volumeIdentifier string, initiatorGroupName string) error
	GetVolumeMappings(ctx context.Context, volumeIdentifier string) ([]string, error)
	GetVolumeByName(ctx context.Context, volumeName string) (*VolumeInfo, error)
	GetVolumeByNAA(ctx context.Context, naa string) (*VolumeInfo, error)
}

type VolumeInfo struct {
	Name string
	// NAA is the SCSI NAA identifier (e.g., naa.624a9370...)
	NAA string
	// Serial is the volume serial number
	Serial    string
	SizeBytes int64
	Created   string
	ArrayName string
	ArrayType string
}

// Config contains configuration for connecting to a storage array
type Config struct {
	// ArrayType is the storage array vendor type (pure, netapp, etc.)
	ArrayType string
	// ArrayName is a friendly name for the array
	ArrayName string
	// ManagementEndpoint is the IP or hostname of the array management interface
	ManagementEndpoint string
	Username           string
	Password           string
	// ISCSITargets is the list of iSCSI target IP addresses
	ISCSITargets []string
	// ISCSIPort is the iSCSI port (default: 3260)
	ISCSIPort int
}

func NewStorageOperator(config Config) (StorageOperator, error) {
	switch config.ArrayType {
	// case "pure":
	// 	return NewPureOperator(config)
	// case "netapp":
	// 	return NewNetAppOperator(config)
	default:
		return nil, &UnsupportedArrayTypeError{ArrayType: config.ArrayType}
	}
}
