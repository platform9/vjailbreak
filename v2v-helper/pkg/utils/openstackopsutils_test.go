package utils

import (
	"testing"

	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/ports"
)

// TestBuildPortCreateOptions verifies that buildPortCreateOptions layers the
// correct OpenStack port extensions onto the base create options:
//   - L2-only networks must carry the {"l2-port": true} binding profile that PCD
//     (and the PCD Veeam Proxy) requires.
//   - Ports without security groups must have port security disabled.
//   - Both extensions must compose when both conditions apply.
func TestBuildPortCreateOptions(t *testing.T) {
	emptyGroups := []string{}
	withGroups := []string{"sg-1"}

	tests := []struct {
		name                string
		createOpts          ports.CreateOpts
		isL2Network         bool
		wantBindingProfile  bool
		wantPortSecurityKey bool // whether "port_security_enabled" should be present
	}{
		{
			name:                "L2 network with security groups => binding profile only",
			createOpts:          ports.CreateOpts{NetworkID: "net-1", SecurityGroups: &withGroups},
			isL2Network:         true,
			wantBindingProfile:  true,
			wantPortSecurityKey: false,
		},
		{
			name:                "non-L2 network with security groups => neither extension",
			createOpts:          ports.CreateOpts{NetworkID: "net-1", SecurityGroups: &withGroups},
			isL2Network:         false,
			wantBindingProfile:  false,
			wantPortSecurityKey: false,
		},
		{
			name:                "non-L2 network with no security groups => port security disabled only",
			createOpts:          ports.CreateOpts{NetworkID: "net-1", SecurityGroups: &emptyGroups},
			isL2Network:         false,
			wantBindingProfile:  false,
			wantPortSecurityKey: true,
		},
		{
			name:                "non-L2 network with nil security groups => port security disabled only",
			createOpts:          ports.CreateOpts{NetworkID: "net-1", SecurityGroups: nil},
			isL2Network:         false,
			wantBindingProfile:  false,
			wantPortSecurityKey: true,
		},
		{
			name:                "L2 network with no security groups => both extensions compose",
			createOpts:          ports.CreateOpts{NetworkID: "net-1", SecurityGroups: &emptyGroups},
			isL2Network:         true,
			wantBindingProfile:  true,
			wantPortSecurityKey: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := buildPortCreateOptions(tt.createOpts, tt.isL2Network)

			body, err := builder.ToPortCreateMap()
			if err != nil {
				t.Fatalf("ToPortCreateMap() returned error: %v", err)
			}

			port, ok := body["port"].(map[string]any)
			if !ok {
				t.Fatalf("expected body[\"port\"] to be map[string]any, got %T", body["port"])
			}

			// Binding profile assertions.
			profile, hasProfile := port["binding:profile"]
			if tt.wantBindingProfile {
				if !hasProfile {
					t.Fatalf("expected binding:profile to be set, got none. port=%v", port)
				}
				profileMap, ok := profile.(map[string]any)
				if !ok {
					t.Fatalf("expected binding:profile to be map[string]any, got %T", profile)
				}
				if v, ok := profileMap[l2PortBindingProfileKey].(bool); !ok || !v {
					t.Fatalf("expected binding:profile[%q] == true, got %v", l2PortBindingProfileKey, profileMap[l2PortBindingProfileKey])
				}
			} else if hasProfile {
				t.Fatalf("did not expect binding:profile to be set, got %v", profile)
			}

			// Port security assertions.
			if _, hasPortSec := port["port_security_enabled"]; hasPortSec != tt.wantPortSecurityKey {
				t.Fatalf("port_security_enabled present = %v, want %v. port=%v", hasPortSec, tt.wantPortSecurityKey, port)
			}
		})
	}
}
