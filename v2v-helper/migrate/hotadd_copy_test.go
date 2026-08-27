// Copyright © 2025 The vjailbreak authors

package migrate

import (
	"testing"

	"github.com/vmware/govmomi/object"
	govmomitypes "github.com/vmware/govmomi/vim25/types"
)

// pvscsiController builds a minimal VMware Paravirtual SCSI controller with
// the given device key -- enough for findPreferredDiskController's type
// assertion and Key/UnitNumber bookkeeping, nothing vCenter-specific.
func pvscsiController(key int32) *govmomitypes.ParaVirtualSCSIController {
	return &govmomitypes.ParaVirtualSCSIController{
		VirtualSCSIController: govmomitypes.VirtualSCSIController{
			VirtualController: govmomitypes.VirtualController{
				VirtualDevice: govmomitypes.VirtualDevice{Key: key},
			},
		},
	}
}

// lsiLogicController builds a minimal LSI Logic Parallel SCSI controller --
// the "found a non-PVSCSI controller instead" case.
func lsiLogicController(key int32) *govmomitypes.VirtualLsiLogicController {
	return &govmomitypes.VirtualLsiLogicController{
		VirtualSCSIController: govmomitypes.VirtualSCSIController{
			VirtualController: govmomitypes.VirtualController{
				VirtualDevice: govmomitypes.VirtualDevice{Key: key},
			},
		},
	}
}

// diskOn builds a VirtualDisk occupying the given unit number on controllerKey.
func diskOn(controllerKey, unit int32) *govmomitypes.VirtualDisk {
	u := unit
	return &govmomitypes.VirtualDisk{
		VirtualDevice: govmomitypes.VirtualDevice{
			ControllerKey: controllerKey,
			UnitNumber:    &u,
		},
	}
}

// fillDisks returns one VirtualDisk per unit 0..maxSCSIControllerUnits-1 on
// controllerKey, skipping unit 7 (reserved for the controller itself) -- i.e.
// enough disks to make that controller report as full.
func fillDisks(controllerKey int32) []govmomitypes.BaseVirtualDevice {
	var disks []govmomitypes.BaseVirtualDevice
	for unit := int32(0); unit < maxSCSIControllerUnits; unit++ {
		if unit == 7 {
			continue
		}
		disks = append(disks, diskOn(controllerKey, unit))
	}
	return disks
}

// devs flattens a mix of individual devices and device slices (e.g. from
// fillDisks) into a single object.VirtualDeviceList, so each test case below
// can be written as a flat, readable list of what's on the Proxy VM.
func devs(items ...interface{}) object.VirtualDeviceList {
	var out []govmomitypes.BaseVirtualDevice
	for _, item := range items {
		switch v := item.(type) {
		case govmomitypes.BaseVirtualDevice:
			out = append(out, v)
		case []govmomitypes.BaseVirtualDevice:
			out = append(out, v...)
		}
	}
	return object.VirtualDeviceList(out)
}

func TestFindPreferredDiskController(t *testing.T) {
	tests := []struct {
		name       string
		devices    object.VirtualDeviceList
		wantKey    int32
		wantPVSCSI bool
	}{
		{
			name:       "PVSCSI with room is chosen",
			devices:    devs(pvscsiController(1000), diskOn(1000, 0)),
			wantKey:    1000,
			wantPVSCSI: true,
		},
		{
			name:       "full PVSCSI is skipped, falls back to the other controller",
			devices:    devs(pvscsiController(1000), lsiLogicController(1001), fillDisks(1000)),
			wantKey:    1001,
			wantPVSCSI: false,
		},
		{
			name:       "first PVSCSI full, second PVSCSI free -> second is chosen",
			devices:    devs(pvscsiController(1000), pvscsiController(1001), fillDisks(1000), diskOn(1001, 0)),
			wantKey:    1001,
			wantPVSCSI: true,
		},
		{
			name:       "no PVSCSI controller falls back to FindDiskController",
			devices:    devs(lsiLogicController(1000)),
			wantKey:    1000,
			wantPVSCSI: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := findPreferredDiskController(tt.devices)
			if err != nil {
				t.Fatalf("findPreferredDiskController() error = %v", err)
			}
			gotDev, ok := got.(govmomitypes.BaseVirtualDevice)
			if !ok {
				t.Fatalf("returned controller %T does not implement BaseVirtualDevice", got)
			}
			if key := gotDev.GetVirtualDevice().Key; key != tt.wantKey {
				t.Errorf("got controller key %d, want %d", key, tt.wantKey)
			}
			if _, isPVSCSI := got.(*govmomitypes.ParaVirtualSCSIController); isPVSCSI != tt.wantPVSCSI {
				t.Errorf("got PVSCSI = %v, want %v (returned type %T)", isPVSCSI, tt.wantPVSCSI, got)
			}
		})
	}
}
