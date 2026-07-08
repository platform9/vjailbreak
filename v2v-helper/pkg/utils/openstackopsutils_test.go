package utils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack/blockstorage/v3/volumes"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/ports"
)

// TestIsHotplugFlavor verifies that hotplug intent is inferred solely from the
// assigned flavor's shape: a flavor with 0 vCPUs and 0 RAM is a hotplug base
// flavor; anything else is a regular flavor.
func TestIsHotplugFlavor(t *testing.T) {
	tests := []struct {
		name   string
		flavor *flavors.Flavor
		want   bool
	}{
		{
			name:   "hotplug base flavor (0 vCPU, 0 RAM)",
			flavor: &flavors.Flavor{ID: "0-0-x", Name: "hotplug-base", VCPUs: 0, RAM: 0},
			want:   true,
		},
		{
			name:   "regular flavor",
			flavor: &flavors.Flavor{ID: "m1.small", Name: "m1.small", VCPUs: 2, RAM: 2048},
			want:   false,
		},
		{
			name:   "zero vCPUs but non-zero RAM",
			flavor: &flavors.Flavor{ID: "weird-1", VCPUs: 0, RAM: 512},
			want:   false,
		},
		{
			name:   "non-zero vCPUs but zero RAM",
			flavor: &flavors.Flavor{ID: "weird-2", VCPUs: 1, RAM: 0},
			want:   false,
		},
		{
			name:   "nil flavor",
			flavor: nil,
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsHotplugFlavor(tt.flavor); got != tt.want {
				t.Errorf("IsHotplugFlavor(%+v) = %v, want %v", tt.flavor, got, tt.want)
			}
		})
	}
}

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

// newTestBlockStorageClient builds a gophercloud ServiceClient pointed at srv,
// optionally applying a custom http.Client (pass nil for default).
func newTestBlockStorageClient(srv *httptest.Server, httpClient *http.Client) *gophercloud.ServiceClient {
	provider := &gophercloud.ProviderClient{
		TokenID:   "test-token",
		Throwaway: true,
	}
	if httpClient != nil {
		provider.HTTPClient = *httpClient
	}
	return &gophercloud.ServiceClient{
		ProviderClient: provider,
		ResourceBase:   srv.URL + "/",
	}
}

func TestSetVolumeBootable_SuccessOnFirstTry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/action") {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := &OpenStackClients{BlockStorageClient: newTestBlockStorageClient(srv, nil)}
	vol := &volumes.Volume{ID: "vol-001"}

	if err := client.SetVolumeBootable(t.Context(), vol); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

// TestSetVolumeBootable_ErrorThenAlreadyBootable reproduces issue #1872:
// SetBootable returns an error (e.g. timeout) but Cinder already committed the
// change server-side. The function must detect bootable=true via GetVolume and
// return nil instead of failing the migration.
func TestSetVolumeBootable_ErrorThenAlreadyBootable(t *testing.T) {
	// POST /action: deliberate slow response so the client HTTP timeout fires.
	// GET /volumes/{id}: fast response confirming the volume is already bootable.
	const actionDelayMs = 100
	const clientTimeoutMs = 10

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/action") {
			// Sleep longer than the client timeout to force "Client.Timeout exceeded".
			time.Sleep(actionDelayMs * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			return
		}
		// GET /volumes/{id} — responds immediately with bootable=true.
		if r.Method == http.MethodGet {
			vol := map[string]any{
				"volume": map[string]any{
					"id":       "vol-timeout",
					"bootable": "true",
					"status":   "available",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(vol)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	// Tight client timeout so SetBootable triggers "Client.Timeout exceeded".
	// GetVolume uses the same client but the server responds instantly for GET.
	tightClient := &http.Client{Timeout: clientTimeoutMs * time.Millisecond}

	orig := setBootableRetryInterval
	setBootableRetryInterval = 0
	defer func() { setBootableRetryInterval = orig }()

	client := &OpenStackClients{BlockStorageClient: newTestBlockStorageClient(srv, tightClient)}
	vol := &volumes.Volume{ID: "vol-timeout"}

	if err := client.SetVolumeBootable(t.Context(), vol); err != nil {
		t.Fatalf("expected success (volume already bootable), got: %v", err)
	}
}

func TestSetVolumeBootable_NonTimeoutErrorReturnsError(t *testing.T) {
	// Non-timeout errors (e.g. 403 Forbidden) must not trigger the GetVolume
	// verification shortcut — they should exhaust retries and return an error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/action") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	orig := setBootableRetryInterval
	setBootableRetryInterval = 0
	defer func() { setBootableRetryInterval = orig }()

	client := &OpenStackClients{BlockStorageClient: newTestBlockStorageClient(srv, nil)}
	vol := &volumes.Volume{ID: "vol-forbidden"}

	err := client.SetVolumeBootable(t.Context(), vol)
	if err == nil {
		t.Fatal("expected error for consistent 403 responses, got nil")
	}
}

func TestSetVolumeBootable_ErrorButNotBootableReturnsError(t *testing.T) {
	// SetBootable times out AND GetVolume shows bootable=false — must return error.
	const actionDelayMs = 100
	const clientTimeoutMs = 10

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/action") {
			time.Sleep(actionDelayMs * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == http.MethodGet {
			vol := map[string]any{
				"volume": map[string]any{
					"id":       "vol-notbooted",
					"bootable": "false",
					"status":   "available",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(vol)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	tightClient := &http.Client{Timeout: clientTimeoutMs * time.Millisecond}

	orig := setBootableRetryInterval
	setBootableRetryInterval = 0
	defer func() { setBootableRetryInterval = orig }()

	client := &OpenStackClients{BlockStorageClient: newTestBlockStorageClient(srv, tightClient)}
	vol := &volumes.Volume{ID: "vol-notbooted"}

	err := client.SetVolumeBootable(t.Context(), vol)
	if err == nil {
		t.Fatal("expected error when GetVolume confirms not bootable, got nil")
	}
}
