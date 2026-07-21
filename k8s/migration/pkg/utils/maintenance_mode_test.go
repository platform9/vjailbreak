package utils

import (
	"testing"

	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

func TestCheckVMForMaintenanceMode(t *testing.T) {
	tests := []struct {
		name    string
		vm      mo.VirtualMachine
		wantMsg string // empty = no block
	}{
		{
			name: "powered on VM - not blocked",
			vm: mo.VirtualMachine{
				ManagedEntity: mo.ManagedEntity{Name: "vm1"},
				Runtime: types.VirtualMachineRuntimeInfo{
					PowerState: types.VirtualMachinePowerStatePoweredOn,
					Host:       &types.ManagedObjectReference{Type: "HostSystem", Value: "host-1"},
				},
				Config: &types.VirtualMachineConfigInfo{},
			},
			wantMsg: "",
		},
		{
			name: "powered off VM - blocked",
			vm: mo.VirtualMachine{
				ManagedEntity: mo.ManagedEntity{Name: "vm-off"},
				Runtime: types.VirtualMachineRuntimeInfo{
					PowerState: types.VirtualMachinePowerStatePoweredOff,
				},
			},
			wantMsg: "vm-off (powered off)",
		},
		{
			name: "suspended VM - blocked",
			vm: mo.VirtualMachine{
				ManagedEntity: mo.ManagedEntity{Name: "vm-suspended"},
				Runtime: types.VirtualMachineRuntimeInfo{
					PowerState: types.VirtualMachinePowerStateSuspended,
					Host:       &types.ManagedObjectReference{Type: "HostSystem", Value: "host-1"},
				},
				Config: &types.VirtualMachineConfigInfo{},
			},
			wantMsg: "vm-suspended (suspended state)",
		},
		{
			name: "VM with no host info - blocked",
			vm: mo.VirtualMachine{
				ManagedEntity: mo.ManagedEntity{Name: "vm-nohost"},
				Runtime: types.VirtualMachineRuntimeInfo{
					PowerState: types.VirtualMachinePowerStatePoweredOn,
					Host:       nil,
				},
			},
			wantMsg: "vm-nohost (no host information)",
		},
		{
			name: "VM with nil config - blocked",
			vm: mo.VirtualMachine{
				ManagedEntity: mo.ManagedEntity{Name: "vm-noconfig"},
				Runtime: types.VirtualMachineRuntimeInfo{
					PowerState: types.VirtualMachinePowerStatePoweredOn,
					Host:       &types.ManagedObjectReference{Type: "HostSystem", Value: "host-1"},
				},
				Config: nil,
			},
			wantMsg: "vm-noconfig (no configuration)",
		},
		{
			name: "VM with pending question - blocked",
			vm: mo.VirtualMachine{
				ManagedEntity: mo.ManagedEntity{Name: "vm-question"},
				Runtime: types.VirtualMachineRuntimeInfo{
					PowerState: types.VirtualMachinePowerStatePoweredOn,
					Host:       &types.ManagedObjectReference{Type: "HostSystem", Value: "host-1"},
					Question:   &types.VirtualMachineQuestionInfo{},
				},
				Config: &types.VirtualMachineConfigInfo{},
			},
			wantMsg: "vm-question (pending question)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckVMForMaintenanceMode(tt.vm)
			if got != tt.wantMsg {
				t.Errorf("CheckVMForMaintenanceMode() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

// TestCanEnterMaintenanceMode_SkipsMigrationVMs verifies that VMs listed in
// MigrationVMNames are not treated as "blocked" even when they are powered off.
// This covers cold migration where migration plan VMs are expected to be powered off.
func TestCanEnterMaintenanceMode_SkipsMigrationVMs(t *testing.T) {
	poweredOffVM := mo.VirtualMachine{
		ManagedEntity: mo.ManagedEntity{Name: "ubuntu-2"},
		Runtime: types.VirtualMachineRuntimeInfo{
			PowerState: types.VirtualMachinePowerStatePoweredOff,
		},
	}
	otherBlockedVM := mo.VirtualMachine{
		ManagedEntity: mo.ManagedEntity{Name: "other-vm"},
		Runtime: types.VirtualMachineRuntimeInfo{
			PowerState: types.VirtualMachinePowerStatePoweredOff,
		},
	}

	tests := []struct {
		name             string
		vms              []mo.VirtualMachine
		migrationVMNames map[string]struct{}
		wantBlocked      bool
	}{
		{
			name:             "migration VM powered off - skipped, no block",
			vms:              []mo.VirtualMachine{poweredOffVM},
			migrationVMNames: map[string]struct{}{"ubuntu-2": {}},
			wantBlocked:      false,
		},
		{
			name:             "non-migration VM powered off - blocked",
			vms:              []mo.VirtualMachine{otherBlockedVM},
			migrationVMNames: map[string]struct{}{"ubuntu-2": {}},
			wantBlocked:      true,
		},
		{
			name:             "migration VM + other blocked VM - other still blocks",
			vms:              []mo.VirtualMachine{poweredOffVM, otherBlockedVM},
			migrationVMNames: map[string]struct{}{"ubuntu-2": {}},
			wantBlocked:      true,
		},
		{
			name:             "nil migrationVMNames - powered off VM blocks",
			vms:              []mo.VirtualMachine{poweredOffVM},
			migrationVMNames: nil,
			wantBlocked:      true,
		},
		{
			name:             "no VMs - not blocked",
			vms:              []mo.VirtualMachine{},
			migrationVMNames: map[string]struct{}{"ubuntu-2": {}},
			wantBlocked:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blockedVMs := make([]string, 0)
			for _, vm := range tt.vms {
				if _, isMigrationVM := tt.migrationVMNames[vm.Name]; isMigrationVM {
					continue
				}
				if reason := CheckVMForMaintenanceMode(vm); reason != "" {
					blockedVMs = append(blockedVMs, reason)
				}
			}
			gotBlocked := len(blockedVMs) > 0
			if gotBlocked != tt.wantBlocked {
				t.Errorf("blocked=%v (VMs: %v), want blocked=%v", gotBlocked, blockedVMs, tt.wantBlocked)
			}
		})
	}
}
