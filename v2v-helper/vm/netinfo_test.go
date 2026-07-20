package vm

import (
	"reflect"
	"testing"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
)

func TestCanonicalMAC(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "already lowercase", in: "00:50:56:9d:47:74", want: "00:50:56:9d:47:74"},
		{name: "uppercase", in: "00:50:56:9D:47:74", want: "00:50:56:9d:47:74"},
		{name: "mixed case with whitespace", in: " 00:50:56:9D:47:74 ", want: "00:50:56:9d:47:74"},
		{name: "empty", in: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanonicalMAC(tt.in); got != tt.want {
				t.Errorf("CanonicalMAC(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCollectIPsPerMac(t *testing.T) {
	tests := []struct {
		name              string
		macs              []string
		guestNetworks     []vjailbreakv1alpha1.GuestNetwork
		networkInterfaces []vjailbreakv1alpha1.NIC
		want              map[string][]IpEntry
	}{
		{
			// Regression for #2115: vCenter reports the MAC uppercase while the
			// VMwareMachine CR stores it lowercase; the IP must still be found.
			name: "uppercase vCenter MAC matches lowercase CR MAC",
			macs: []string{"00:50:56:9D:47:74"},
			guestNetworks: []vjailbreakv1alpha1.GuestNetwork{
				{MAC: "00:50:56:9d:47:74", IP: "146.122.44.210", PrefixLength: 24},
				{MAC: "00:50:56:9d:47:74", IP: "fe80::250:56ff:fe9d:4774", PrefixLength: 64},
			},
			want: map[string][]IpEntry{
				"00:50:56:9d:47:74": {{IP: "146.122.44.210", Prefix: 24}},
			},
		},
		{
			name: "lowercase vCenter MAC matches uppercase CR MAC",
			macs: []string{"00:50:56:9d:47:74"},
			guestNetworks: []vjailbreakv1alpha1.GuestNetwork{
				{MAC: "00:50:56:9D:47:74", IP: "10.0.0.5", PrefixLength: 24},
			},
			want: map[string][]IpEntry{
				"00:50:56:9d:47:74": {{IP: "10.0.0.5", Prefix: 24}},
			},
		},
		{
			name: "IPv6-only guest network yields empty non-nil entry",
			macs: []string{"00:50:56:9d:47:74"},
			guestNetworks: []vjailbreakv1alpha1.GuestNetwork{
				{MAC: "00:50:56:9d:47:74", IP: "fe80::1", PrefixLength: 64},
			},
			want: map[string][]IpEntry{
				"00:50:56:9d:47:74": {},
			},
		},
		{
			name: "no matching guest network leaves MAC absent",
			macs: []string{"00:50:56:9d:47:74"},
			guestNetworks: []vjailbreakv1alpha1.GuestNetwork{
				{MAC: "aa:bb:cc:dd:ee:ff", IP: "10.0.0.9", PrefixLength: 24},
			},
			want: map[string][]IpEntry{},
		},
		{
			name: "network interfaces fallback with case mismatch",
			macs: []string{"00:50:56:9D:47:74"},
			networkInterfaces: []vjailbreakv1alpha1.NIC{
				{MAC: "00:50:56:9d:47:74", IPAddress: []string{"146.122.44.210", "fe80::1"}},
			},
			want: map[string][]IpEntry{
				"00:50:56:9d:47:74": {{IP: "146.122.44.210", Prefix: 0}},
			},
		},
		{
			name: "guest networks take precedence over network interfaces",
			macs: []string{"00:50:56:9d:47:74"},
			guestNetworks: []vjailbreakv1alpha1.GuestNetwork{
				{MAC: "00:50:56:9d:47:74", IP: "10.1.1.1", PrefixLength: 16},
			},
			networkInterfaces: []vjailbreakv1alpha1.NIC{
				{MAC: "00:50:56:9d:47:74", IPAddress: []string{"10.2.2.2"}},
			},
			want: map[string][]IpEntry{
				"00:50:56:9d:47:74": {{IP: "10.1.1.1", Prefix: 16}},
			},
		},
		{
			name: "multiple NICs each keyed canonically",
			macs: []string{"00:50:56:9D:47:74", "00:50:56:AA:BB:CC"},
			guestNetworks: []vjailbreakv1alpha1.GuestNetwork{
				{MAC: "00:50:56:9d:47:74", IP: "10.0.0.1", PrefixLength: 24},
				{MAC: "00:50:56:aa:bb:cc", IP: "10.0.1.1", PrefixLength: 24},
			},
			want: map[string][]IpEntry{
				"00:50:56:9d:47:74": {{IP: "10.0.0.1", Prefix: 24}},
				"00:50:56:aa:bb:cc": {{IP: "10.0.1.1", Prefix: 24}},
			},
		},
		{
			name: "nil guest networks and nil interfaces",
			macs: []string{"00:50:56:9d:47:74"},
			want: map[string][]IpEntry{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CollectIPsPerMac(tt.macs, tt.guestNetworks, tt.networkInterfaces)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CollectIPsPerMac() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

// TestCollectIPsPerMacLookupByCanonicalVCenterMAC pins the invariant that a
// lookup keyed by the canonicalized vCenter MAC (as done in
// migrate.createPortsForNetworks) always finds the collected IPs.
func TestCollectIPsPerMacLookupByCanonicalVCenterMAC(t *testing.T) {
	vcenterMAC := "00:50:56:9D:47:74" // vCenter may preserve manual-MAC case
	macs := []string{CanonicalMAC(vcenterMAC)}
	got := CollectIPsPerMac(macs, []vjailbreakv1alpha1.GuestNetwork{
		{MAC: "00:50:56:9d:47:74", IP: "146.122.44.210", PrefixLength: 24},
	}, nil)

	entries, ok := got[macs[0]]
	if !ok {
		t.Fatalf("lookup by canonical vCenter MAC %q missed; keys: %v", macs[0], got)
	}
	if len(entries) != 1 || entries[0].IP != "146.122.44.210" {
		t.Errorf("entries = %#v, want single entry 146.122.44.210", entries)
	}
}
