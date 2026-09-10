// Copyright © 2024 The vjailbreak authors

package migrate

import (
	"reflect"
	"testing"

	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/ports"
	"github.com/platform9/vjailbreak/v2v-helper/openstack"
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

// TestApplyPreserveMACOverride_RemovesStaleOriginalKey asserts that once the
// entries are moved to the "" placeholder key, the original MAC key is
// deleted rather than left behind holding a stale, unused copy.
func TestApplyPreserveMACOverride_RemovesStaleOriginalKey(t *testing.T) {
	const mac = "aa:bb:cc:dd:ee:ff"
	vminfo := &vm.VMInfo{IPperMac: map[string][]vm.IpEntry{mac: {{IP: "10.0.0.5", Prefix: 24}}}}

	got := applyPreserveMACOverride(vminfo, 0, mac, false)

	if got != "" {
		t.Fatalf("expected placeholder MAC \"\", got %q", got)
	}
	if _, exists := vminfo.IPperMac[mac]; exists {
		t.Fatalf("expected original MAC key %q to be removed, still present: %#v", mac, vminfo.IPperMac[mac])
	}
	want := []vm.IpEntry{{IP: "10.0.0.5", Prefix: 24}}
	if !reflect.DeepEqual(vminfo.IPperMac[""], want) {
		t.Fatalf("IPperMac[\"\"] = %#v, want %#v", vminfo.IPperMac[""], want)
	}
}

// TestSyncIPperMacFromPort covers reconciling vminfo.IPperMac with the
// port OpenStack actually created, for the cases that need it: a MAC that
// changed (preserveMAC=false), an IP that was left nil for OpenStack to
// auto-allocate (preserveIP=false + fallbackToDHCP=true), both at once, and
// the no-op case where nothing needs reconciling.
func TestSyncIPperMacFromPort(t *testing.T) {
	tests := []struct {
		name           string
		placeholderMAC string
		initial        map[string][]vm.IpEntry
		port           *ports.Port
		wantIPperMac   map[string][]vm.IpEntry
	}{
		{
			name:           "preserve everything, no error: already correct, left untouched (prefix preserved)",
			placeholderMAC: "aa:bb:cc:dd:ee:ff",
			initial: map[string][]vm.IpEntry{
				"aa:bb:cc:dd:ee:ff": {{IP: "10.0.0.5", Prefix: 16}},
			},
			port: &ports.Port{
				MACAddress: "aa:bb:cc:dd:ee:ff",
				FixedIPs:   []ports.IP{{IPAddress: "10.0.0.5"}},
			},
			wantIPperMac: map[string][]vm.IpEntry{
				"aa:bb:cc:dd:ee:ff": {{IP: "10.0.0.5", Prefix: 16}},
			},
		},
		{
			name:           "preserveMAC=false, preserveIP=true: rekeyed to real MAC, prefix preserved",
			placeholderMAC: "",
			initial: map[string][]vm.IpEntry{
				"": {{IP: "10.0.0.5", Prefix: 16}},
			},
			port: &ports.Port{
				MACAddress: "fa:16:3e:11:22:33",
				FixedIPs:   []ports.IP{{IPAddress: "10.0.0.5"}},
			},
			wantIPperMac: map[string][]vm.IpEntry{
				"fa:16:3e:11:22:33": {{IP: "10.0.0.5", Prefix: 16}},
			},
		},
		{
			name:           "preserveIP=false, fallbackToDHCP=true, preserveMAC=true: nil filled from port.FixedIPs",
			placeholderMAC: "aa:bb:cc:dd:ee:ff",
			initial: map[string][]vm.IpEntry{
				"aa:bb:cc:dd:ee:ff": nil,
			},
			port: &ports.Port{
				MACAddress: "aa:bb:cc:dd:ee:ff",
				FixedIPs:   []ports.IP{{IPAddress: "192.168.50.77"}},
			},
			wantIPperMac: map[string][]vm.IpEntry{
				"aa:bb:cc:dd:ee:ff": {{IP: "192.168.50.77", Prefix: 0, DHCP: true}},
			},
		},
		{
			name:           "preserveIP=false, fallbackToDHCP=true, preserveMAC=false: rekey and fill together",
			placeholderMAC: "",
			initial: map[string][]vm.IpEntry{
				"": nil,
			},
			port: &ports.Port{
				MACAddress: "fa:16:3e:aa:bb:cc",
				FixedIPs:   []ports.IP{{IPAddress: "192.168.50.77"}},
			},
			wantIPperMac: map[string][]vm.IpEntry{
				"fa:16:3e:aa:bb:cc": {{IP: "192.168.50.77", Prefix: 0, DHCP: true}},
			},
		},
		{
			name:           "preserveIP=false, fallbackToDHCP=true, L2 network: no FixedIPs, stays empty (not nil) so AddWildcardNetplan still skips it",
			placeholderMAC: "",
			initial: map[string][]vm.IpEntry{
				"": nil,
			},
			port: &ports.Port{
				MACAddress: "fa:16:3e:dd:ee:ff",
				FixedIPs:   []ports.IP{},
			},
			wantIPperMac: map[string][]vm.IpEntry{
				"fa:16:3e:dd:ee:ff": {},
			},
		},
		{
			name:           "preserveIP=false, fallbackToDHCP=false: explicit empty slice is left untouched, not treated as nil",
			placeholderMAC: "aa:bb:cc:dd:ee:ff",
			initial: map[string][]vm.IpEntry{
				"aa:bb:cc:dd:ee:ff": {},
			},
			port: &ports.Port{
				MACAddress: "aa:bb:cc:dd:ee:ff",
				FixedIPs:   []ports.IP{},
			},
			wantIPperMac: map[string][]vm.IpEntry{
				"aa:bb:cc:dd:ee:ff": {},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vminfo := &vm.VMInfo{IPperMac: tt.initial}

			syncIPperMacFromPort(vminfo, tt.placeholderMAC, tt.port)

			if len(vminfo.IPperMac) != len(tt.wantIPperMac) {
				t.Fatalf("IPperMac has %d keys, want %d: got %#v", len(vminfo.IPperMac), len(tt.wantIPperMac), vminfo.IPperMac)
			}
			for wantMAC, wantEntries := range tt.wantIPperMac {
				gotEntries, ok := vminfo.IPperMac[wantMAC]
				if !ok {
					t.Fatalf("expected key %q to be present, IPperMac=%#v", wantMAC, vminfo.IPperMac)
				}
				if gotEntries == nil {
					t.Fatalf("IPperMac[%q] is nil, want non-nil %#v (nil vs empty-slice matters downstream)", wantMAC, wantEntries)
				}
				if !reflect.DeepEqual(gotEntries, wantEntries) {
					t.Fatalf("IPperMac[%q] = %#v, want %#v", wantMAC, gotEntries, wantEntries)
				}
			}
		})
	}
}

// TestResolveNICOverride_AllowedAddressPairs covers parsing and validating
// NICOverride.AllowedAddressPairs: a valid pair (with and without a MAC), an
// invalid IP, a duplicate IP, and an override for a different interface index
// (must be ignored).
func TestResolveNICOverride_AllowedAddressPairs(t *testing.T) {
	tests := []struct {
		name      string
		overrides []NICOverride
		idx       int
		want      []openstack.AllowedAddressPair
		wantErr   bool
	}{
		{
			name: "single valid pair, no MAC",
			overrides: []NICOverride{
				{InterfaceIndex: 0, AllowedAddressPairs: []AddressPairs{{IP: "10.0.0.58"}}},
			},
			idx:  0,
			want: []openstack.AllowedAddressPair{{IP: "10.0.0.58"}},
		},
		{
			name: "pair with an explicit MAC",
			overrides: []NICOverride{
				{InterfaceIndex: 0, AllowedAddressPairs: []AddressPairs{{IP: "10.0.0.58", MAC: "00:00:5e:00:01:01"}}},
			},
			idx:  0,
			want: []openstack.AllowedAddressPair{{IP: "10.0.0.58", MAC: "00:00:5e:00:01:01"}},
		},
		{
			name: "override for a different interface is ignored",
			overrides: []NICOverride{
				{InterfaceIndex: 1, AllowedAddressPairs: []AddressPairs{{IP: "10.0.0.58"}}},
			},
			idx:  0,
			want: nil,
		},
		{
			name: "invalid IP is rejected",
			overrides: []NICOverride{
				{InterfaceIndex: 0, AllowedAddressPairs: []AddressPairs{{IP: "not-an-ip"}}},
			},
			idx:     0,
			wantErr: true,
		},
		{
			name: "duplicate IP is rejected",
			overrides: []NICOverride{
				{InterfaceIndex: 0, AllowedAddressPairs: []AddressPairs{{IP: "10.0.0.58"}, {IP: "10.0.0.58"}}},
			},
			idx:     0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveNICOverride(tt.overrides, tt.idx)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got.allowedAddressPairs, tt.want) {
				t.Fatalf("allowedAddressPairs = %#v, want %#v", got.allowedAddressPairs, tt.want)
			}
		})
	}
}

// TestExcludeAllowedAddressPairIPs asserts that an allowed-address-pair IP is
// removed from vminfo.IPperMac[mac] (so it is never also sent as a fixed IP),
// while an unrelated IP on the same NIC is left in place.
func TestExcludeAllowedAddressPairIPs(t *testing.T) {
	const mac = "aa:bb:cc:dd:ee:ff"
	vminfo := &vm.VMInfo{
		IPperMac: map[string][]vm.IpEntry{
			mac: {{IP: "10.102.164.57"}, {IP: "10.102.164.58"}},
		},
	}

	excludeAllowedAddressPairIPs(vminfo, mac, []openstack.AllowedAddressPair{{IP: "10.102.164.58"}})

	want := []vm.IpEntry{{IP: "10.102.164.57"}}
	if !reflect.DeepEqual(vminfo.IPperMac[mac], want) {
		t.Fatalf("IPperMac[mac] = %#v, want %#v", vminfo.IPperMac[mac], want)
	}
}

// TestExcludeAllowedAddressPairIPs_NoPairs asserts that IPperMac is left
// untouched (not even reallocated) when there are no allowed-address-pairs.
func TestExcludeAllowedAddressPairIPs_NoPairs(t *testing.T) {
	const mac = "aa:bb:cc:dd:ee:ff"
	original := []vm.IpEntry{{IP: "10.102.164.57"}}
	vminfo := &vm.VMInfo{IPperMac: map[string][]vm.IpEntry{mac: original}}

	excludeAllowedAddressPairIPs(vminfo, mac, nil)

	if !reflect.DeepEqual(vminfo.IPperMac[mac], original) {
		t.Fatalf("IPperMac[mac] = %#v, want unchanged %#v", vminfo.IPperMac[mac], original)
	}
}

// TestValidateAllowedAddressPairSecurityGroup asserts that a NIC's allowed-
// address-pairs are rejected when no security group is resolved for the
// migration (Neutron requires port security to stay enabled for them), but
// allowed otherwise. Callers are expected to skip this check entirely for a
// NIC on an L2 network (see createPortsForNetworks).
func TestValidateAllowedAddressPairSecurityGroup(t *testing.T) {
	pairs := []openstack.AllowedAddressPair{{IP: "10.0.0.58"}}

	if err := validateAllowedAddressPairSecurityGroup(0, pairs, []string{"sg-1"}); err != nil {
		t.Fatalf("expected no error with a security group present, got: %v", err)
	}
	if err := validateAllowedAddressPairSecurityGroup(0, nil, nil); err != nil {
		t.Fatalf("expected no error with no pairs at all, got: %v", err)
	}
	if err := validateAllowedAddressPairSecurityGroup(0, pairs, nil); err == nil {
		t.Fatalf("expected an error: allowed-address-pairs with no security groups")
	}
}
