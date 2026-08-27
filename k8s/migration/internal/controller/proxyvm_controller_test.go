/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"testing"

	"github.com/vmware/govmomi/object"
	govmomitypes "github.com/vmware/govmomi/vim25/types"
)

// pvscsiController builds a minimal VMware Paravirtual SCSI controller --
// enough for classifyProxyControllers' type assertion, nothing vCenter-specific.
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

func TestClassifyProxyControllers(t *testing.T) {
	tests := []struct {
		name          string
		devices       object.VirtualDeviceList
		wantPresent   bool
		wantOtherType string
	}{
		{
			name:          "PVSCSI present",
			devices:       object.VirtualDeviceList{pvscsiController(1000)},
			wantPresent:   true,
			wantOtherType: "",
		},
		{
			name:          "LSI Logic Parallel only -- reports the other type",
			devices:       object.VirtualDeviceList{lsiLogicController(1000)},
			wantPresent:   false,
			wantOtherType: "*types.VirtualLsiLogicController",
		},
		{
			name:          "PVSCSI alongside a non-PVSCSI controller -- PVSCSI wins",
			devices:       object.VirtualDeviceList{lsiLogicController(1000), pvscsiController(1001)},
			wantPresent:   true,
			wantOtherType: "",
		},
		{
			name:          "no SCSI controllers at all",
			devices:       object.VirtualDeviceList{},
			wantPresent:   false,
			wantOtherType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPresent, gotOtherType := classifyProxyControllers(tt.devices)
			if gotPresent != tt.wantPresent {
				t.Errorf("classifyProxyControllers() present = %v, want %v", gotPresent, tt.wantPresent)
			}
			if gotOtherType != tt.wantOtherType {
				t.Errorf("classifyProxyControllers() otherType = %q, want %q", gotOtherType, tt.wantOtherType)
			}
		})
	}
}
