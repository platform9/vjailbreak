// Copyright © 2024 The vjailbreak authors
package storagearray

import "fmt"

type UnsupportedArrayTypeError struct {
	ArrayType string
}

func (e *UnsupportedArrayTypeError) Error() string {
	return fmt.Sprintf("unsupported storage array type: %s (supported: pure, netapp)", e.ArrayType)
}

type ConnectionError struct {
	ArrayName string
	Endpoint  string
	Err       error
}

func (e *ConnectionError) Error() string {
	return fmt.Sprintf("failed to connect to storage array %s at %s: %v", e.ArrayName, e.Endpoint, e.Err)
}

func (e *ConnectionError) Unwrap() error {
	return e.Err
}

type VolumeNotFoundError struct {
	Identifier string
}

func (e *VolumeNotFoundError) Error() string {
	return fmt.Sprintf("volume not found: %s", e.Identifier)
}

type InitiatorGroupNotFoundError struct {
	Name string
}

func (e *InitiatorGroupNotFoundError) Error() string {
	return fmt.Sprintf("initiator group not found: %s", e.Name)
}

type MappingError struct {
	Volume         string
	InitiatorGroup string
	Operation      string // "map" or "unmap"
	Err            error
}

func (e *MappingError) Error() string {
	return fmt.Sprintf("failed to %s volume %s to/from initiator group %s: %v",
		e.Operation, e.Volume, e.InitiatorGroup, e.Err)
}

func (e *MappingError) Unwrap() error {
	return e.Err
}
