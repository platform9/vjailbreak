// Copyright © 2024 The vjailbreak authors

package migrate

import (
	"reflect"
	"testing"

	"github.com/platform9/vjailbreak/v2v-helper/vm"
)

// TestApplyPreserveIPOverride_FallbackToDHCP covers preserveIP=false with no
// custom IP: fallbackToDHCP should pick nil (auto-allocate) vs an empty slice
// (no fixed IPs).
func TestApplyPreserveIPOverride_FallbackToDHCP(t *testing.T) {
	const mac = "aa:bb:cc:dd:ee:ff"

	tests := []struct {
		name           string
		override       nicOverride
		fallbackToDHCP bool
		detectedIPs    []string
		wantIPperMac   []vm.IpEntry
		wantIsNil      bool
	}{
		{
			name:           "preserveIP=false, empty IP, dhcp=false",
			override:       nicOverride{preserveIP: false},
			fallbackToDHCP: false,
			detectedIPs:    []string{"10.0.0.5"},
			wantIPperMac:   []vm.IpEntry{},
			wantIsNil:      false,
		},
		{
			name:           "preserveIP=false, empty IP, dhcp=true",
			override:       nicOverride{preserveIP: false},
			fallbackToDHCP: true,
			detectedIPs:    []string{"10.0.0.5"},
			wantIPperMac:   nil,
			wantIsNil:      true,
		},
		{
			name:           "preserveIP=false, custom IP, dhcp=true",
			override:       nicOverride{preserveIP: false, userAssignedIP: []string{"192.168.1.50"}},
			fallbackToDHCP: true,
			detectedIPs:    []string{"10.0.0.5"},
			wantIPperMac:   []vm.IpEntry{{IP: "192.168.1.50", Prefix: 0}},
			wantIsNil:      false,
		},
		{
			name:           "preserveIP=false, custom IP, dhcp=false",
			override:       nicOverride{preserveIP: false, userAssignedIP: []string{"192.168.1.50"}},
			fallbackToDHCP: false,
			detectedIPs:    []string{"10.0.0.5"},
			wantIPperMac:   []vm.IpEntry{{IP: "192.168.1.50", Prefix: 0}},
			wantIsNil:      false,
		},
		{
			name:           "preserveIP=true: untouched",
			override:       nicOverride{preserveIP: true},
			fallbackToDHCP: true,
			detectedIPs:    []string{"10.0.0.5"},
			wantIPperMac:   []vm.IpEntry{{IP: "10.0.0.5", Prefix: 24}},
			wantIsNil:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vminfo := &vm.VMInfo{
				IPperMac: map[string][]vm.IpEntry{
					mac: {{IP: "10.0.0.5", Prefix: 24}},
				},
			}

			applyPreserveIPOverride(vminfo, 0, mac, tt.override, tt.detectedIPs, tt.fallbackToDHCP)

			got := vminfo.IPperMac[mac]
			if tt.wantIsNil && got != nil {
				t.Fatalf("expected IPperMac[mac] to be nil, got %#v", got)
			}
			if !tt.wantIsNil && got == nil {
				t.Fatalf("expected IPperMac[mac] to be non-nil, got nil")
			}
			if !reflect.DeepEqual(got, tt.wantIPperMac) {
				t.Fatalf("IPperMac[mac] = %#v, want %#v", got, tt.wantIPperMac)
			}
		})
	}
}

// TestApplyPreserveIPOverride_EmptySliceVsNilDistinctionMatters asserts nil
// and an empty slice aren't treated as interchangeable, since GetCreateOpts
// branches on that distinction.
func TestApplyPreserveIPOverride_EmptySliceVsNilDistinctionMatters(t *testing.T) {
	const mac = "11:22:33:44:55:66"

	vminfoDHCPOff := &vm.VMInfo{IPperMac: map[string][]vm.IpEntry{mac: {{IP: "1.2.3.4"}}}}
	applyPreserveIPOverride(vminfoDHCPOff, 0, mac, nicOverride{preserveIP: false}, nil, false)
	if vminfoDHCPOff.IPperMac[mac] == nil {
		t.Fatalf("fallbackToDHCP=false must produce a non-nil empty slice, got nil")
	}
	if len(vminfoDHCPOff.IPperMac[mac]) != 0 {
		t.Fatalf("fallbackToDHCP=false must produce an empty slice, got %#v", vminfoDHCPOff.IPperMac[mac])
	}

	vminfoDHCPOn := &vm.VMInfo{IPperMac: map[string][]vm.IpEntry{mac: {{IP: "1.2.3.4"}}}}
	applyPreserveIPOverride(vminfoDHCPOn, 0, mac, nicOverride{preserveIP: false}, nil, true)
	if vminfoDHCPOn.IPperMac[mac] != nil {
		t.Fatalf("fallbackToDHCP=true must produce a nil slice, got %#v", vminfoDHCPOn.IPperMac[mac])
	}
}
