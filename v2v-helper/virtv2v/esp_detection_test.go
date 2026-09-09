// Copyright © 2024 The vjailbreak authors

package virtv2v

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeriveBaseDisk(t *testing.T) {
	tests := []struct {
		name      string
		espDevice string
		want      string
	}{
		{name: "sata/scsi partition", espDevice: "/dev/sdc1", want: "/dev/sdc"},
		{name: "virtio partition", espDevice: "/dev/vda2", want: "/dev/vda"},
		{name: "double-digit partition number", espDevice: "/dev/sdc12", want: "/dev/sdc"},
		{name: "nvme partition", espDevice: "/dev/nvme0n1p1", want: "/dev/nvme0n1"},
		{name: "nvme double-digit partition", espDevice: "/dev/nvme0n1p12", want: "/dev/nvme0n1"},
		{name: "whole disk, no partition suffix", espDevice: "/dev/sdc", want: "/dev/sdc"},
		{name: "empty input", espDevice: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deriveBaseDisk(tt.espDevice)
			assert.Equal(t, tt.want, got, "deriveBaseDisk(%q)", tt.espDevice)
		})
	}
}

func TestValidateESPDiskIndex(t *testing.T) {
	tests := []struct {
		name      string
		diskIndex int
		numDisks  int
		wantErr   bool
	}{
		{name: "first of one disk", diskIndex: 0, numDisks: 1, wantErr: false},
		{name: "last of several disks", diskIndex: 2, numDisks: 3, wantErr: false},
		{name: "negative index", diskIndex: -1, numDisks: 1, wantErr: true},
		{name: "equal to numDisks (one past the end)", diskIndex: 1, numDisks: 1, wantErr: true},
		{name: "past a multi-disk VM (phantom appliance disk)", diskIndex: 2, numDisks: 2, wantErr: true},
		{name: "zero disks", diskIndex: 0, numDisks: 0, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateESPDiskIndex(tt.diskIndex, tt.numDisks)
			if tt.wantErr {
				assert.Error(t, err, "validateESPDiskIndex(%d, %d)", tt.diskIndex, tt.numDisks)
				return
			}
			assert.NoError(t, err, "validateESPDiskIndex(%d, %d)", tt.diskIndex, tt.numDisks)
		})
	}
}
