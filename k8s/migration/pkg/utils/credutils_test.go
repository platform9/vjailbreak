// Copyright 2024. Licensed under the Apache License, Version 2.0.
//
// Tests for ExtractVirtualNICs — verifies that the correct MOR type (NetworkType)
// is captured for each VMware network backing variant so the property collector
// lookup uses the right type instead of the hardcoded "Network" fallback.
//
// Run with:
//
//	cd k8s/migration && go test ./pkg/utils/... -v -run TestExtractVirtualNICs
package utils

import (
	"strings"
	"testing"

	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// makeVMWithNICs constructs a minimal mo.VirtualMachine containing the given
// virtual devices (NICs). This avoids needing a live vCenter connection.
func makeVMWithNICs(devices ...types.BaseVirtualDevice) *mo.VirtualMachine {
	return &mo.VirtualMachine{
		Config: &types.VirtualMachineConfigInfo{
			Hardware: types.VirtualHardware{
				Device: devices,
			},
		},
	}
}

// makeVmxnet3 builds a VirtualVmxnet3 with the supplied backing.
func makeVmxnet3(mac string, backing types.BaseVirtualDeviceBackingInfo) *types.VirtualVmxnet3 {
	card := &types.VirtualVmxnet3{}
	card.MacAddress = mac
	card.Backing = backing
	return card
}

func TestExtractVirtualNICs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		device          types.BaseVirtualDevice
		wantNetwork     string
		wantNetworkType string
		wantMAC         string
	}{
		{
			name: "standard network backing sets NetworkType=Network",
			device: makeVmxnet3("aa:bb:cc:dd:ee:01", &types.VirtualEthernetCardNetworkBackingInfo{
				Network: &types.ManagedObjectReference{
					Type:  "Network",
					Value: "network-100",
				},
			}),
			wantNetwork:     "network-100",
			wantNetworkType: "Network",
			wantMAC:         "aa:bb:cc:dd:ee:01",
		},
		{
			name: "DVS portgroup backing sets NetworkType=DistributedVirtualPortgroup",
			device: makeVmxnet3("aa:bb:cc:dd:ee:02", &types.VirtualEthernetCardDistributedVirtualPortBackingInfo{
				Port: types.DistributedVirtualSwitchPortConnection{
					PortgroupKey: "dvportgroup-1234",
				},
			}),
			wantNetwork:     "dvportgroup-1234",
			wantNetworkType: "DistributedVirtualPortgroup",
			wantMAC:         "aa:bb:cc:dd:ee:02",
		},
		{
			name: "opaque network backing sets NetworkType=OpaqueNetwork",
			device: makeVmxnet3("aa:bb:cc:dd:ee:03", &types.VirtualEthernetCardOpaqueNetworkBackingInfo{
				OpaqueNetworkId: "nsx-opaque-net-42",
			}),
			wantNetwork:     "nsx-opaque-net-42",
			wantNetworkType: "OpaqueNetwork",
			wantMAC:         "aa:bb:cc:dd:ee:03",
		},
		{
			name: "standard network backing with nil Network ref yields empty network value",
			device: makeVmxnet3("aa:bb:cc:dd:ee:04", &types.VirtualEthernetCardNetworkBackingInfo{
				Network: nil,
			}),
			wantNetwork:     "",
			wantNetworkType: "Network",
			wantMAC:         "aa:bb:cc:dd:ee:04",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			vm := makeVMWithNICs(tc.device)
			nics, err := ExtractVirtualNICs(vm)

			if err != nil {
				t.Fatalf("ExtractVirtualNICs returned unexpected error: %v", err)
			}
			if len(nics) != 1 {
				t.Fatalf("expected 1 NIC, got %d", len(nics))
			}

			got := nics[0]
			if got.Network != tc.wantNetwork {
				t.Errorf("NIC.Network = %q, want %q", got.Network, tc.wantNetwork)
			}
			if got.NetworkType != tc.wantNetworkType {
				t.Errorf("NIC.NetworkType = %q, want %q", got.NetworkType, tc.wantNetworkType)
			}
			if got.MAC != strings.ToLower(tc.wantMAC) {
				t.Errorf("NIC.MAC = %q, want %q", got.MAC, strings.ToLower(tc.wantMAC))
			}
			if got.Index != 0 {
				t.Errorf("NIC.Index = %d, want 0 (first NIC)", got.Index)
			}
		})
	}
}

// TestExtractVirtualNICs_MultipleNICs verifies correct index assignment and
// that mixed backing types in the same VM are all handled correctly.
func TestExtractVirtualNICs_MultipleNICs(t *testing.T) {
	t.Parallel()

	vm := makeVMWithNICs(
		makeVmxnet3("aa:bb:cc:dd:ee:01", &types.VirtualEthernetCardNetworkBackingInfo{
			Network: &types.ManagedObjectReference{Type: "Network", Value: "network-100"},
		}),
		makeVmxnet3("aa:bb:cc:dd:ee:02", &types.VirtualEthernetCardDistributedVirtualPortBackingInfo{
			Port: types.DistributedVirtualSwitchPortConnection{PortgroupKey: "dvportgroup-200"},
		}),
		makeVmxnet3("aa:bb:cc:dd:ee:03", &types.VirtualEthernetCardOpaqueNetworkBackingInfo{
			OpaqueNetworkId: "opaque-300",
		}),
	)

	nics, err := ExtractVirtualNICs(vm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nics) != 3 {
		t.Fatalf("expected 3 NICs, got %d", len(nics))
	}

	want := []struct {
		network     string
		networkType string
		index       int
	}{
		{"network-100", "Network", 0},
		{"dvportgroup-200", "DistributedVirtualPortgroup", 1},
		{"opaque-300", "OpaqueNetwork", 2},
	}

	for i, w := range want {
		if nics[i].Network != w.network {
			t.Errorf("nics[%d].Network = %q, want %q", i, nics[i].Network, w.network)
		}
		if nics[i].NetworkType != w.networkType {
			t.Errorf("nics[%d].NetworkType = %q, want %q", i, nics[i].NetworkType, w.networkType)
		}
		if nics[i].Index != w.index {
			t.Errorf("nics[%d].Index = %d, want %d", i, nics[i].Index, w.index)
		}
	}
}

// TestExtractVirtualNICs_EmptyVM verifies that a VM with no devices returns
// an empty NIC list without error.
func TestExtractVirtualNICs_EmptyVM(t *testing.T) {
	t.Parallel()

	vm := makeVMWithNICs()
	nics, err := ExtractVirtualNICs(vm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nics) != 0 {
		t.Errorf("expected 0 NICs for empty VM, got %d", len(nics))
	}
}
