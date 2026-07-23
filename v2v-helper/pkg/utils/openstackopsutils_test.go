package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack/blockstorage/v3/volumes"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/ports"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	"github.com/platform9/vjailbreak/v2v-helper/vm"
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

// TestHotplugMetadata verifies the server metadata built for hotplug
// (flavorless) provisioning.
func TestHotplugMetadata(t *testing.T) {
	tests := []struct {
		name     string
		cpu      int32
		memoryMB int32
		want     map[string]string
	}{
		{
			name:     "typical small VM",
			cpu:      1,
			memoryMB: 2048,
			want: map[string]string{
				"HOTPLUG_CPU":        "1",
				"HOTPLUG_MEMORY":     "2048",
				"HOTPLUG_CPU_MAX":    "2",
				"HOTPLUG_MEMORY_MAX": "4096",
			},
		},
		{
			name:     "larger VM",
			cpu:      8,
			memoryMB: 16384,
			want: map[string]string{
				"HOTPLUG_CPU":        "8",
				"HOTPLUG_MEMORY":     "16384",
				"HOTPLUG_CPU_MAX":    "16",
				"HOTPLUG_MEMORY_MAX": "32768",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HotplugMetadata(tt.cpu, tt.memoryMB)
			if len(got) != len(tt.want) {
				t.Fatalf("HotplugMetadata(%d, %d) has %d keys, want %d: %v", tt.cpu, tt.memoryMB, len(got), len(tt.want), got)
			}
			for k, want := range tt.want {
				if got[k] != want {
					t.Errorf("HotplugMetadata(%d, %d)[%q] = %q, want %q", tt.cpu, tt.memoryMB, k, got[k], want)
				}
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

// newTestNetworkingClient builds a gophercloud ServiceClient pointed at srv
// for use as OpenStackClients.NetworkingClient.
func newTestNetworkingClient(srv *httptest.Server) *gophercloud.ServiceClient {
	return &gophercloud.ServiceClient{
		ProviderClient: &gophercloud.ProviderClient{TokenID: "test-token", Throwaway: true},
		ResourceBase:   srv.URL + "/v2.0/",
	}
}

// TestGetCreateOpts_NilIPEntries_L2NetworkWithNoSubnets reproduces a panic:
// when ipEntries is nil (e.g. preserveIP=false + fallbackToDHCP=true routes
// here to let OpenStack auto-allocate an IP) and the target network is an
// L2-only "simple_network" with zero subnets, GetCreateOpts's nil branch used
// to unconditionally index network.Subnets[0] and crash the migration with an
// index-out-of-range panic. L2 networks are explicitly allowed to have no
// subnets (see the guard in ValidateAndCreatePort), so this is a reachable
// case, not a hypothetical one.
func TestGetCreateOpts_NilIPEntries_L2NetworkWithNoSubnets(t *testing.T) {
	const networkID = "net-l2-no-subnets"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/networks/"+networkID) {
			body := map[string]any{
				"network": map[string]any{
					"id":   networkID,
					"tags": []string{"simple_network"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(body)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := &OpenStackClients{NetworkingClient: newTestNetworkingClient(srv)}
	network := &networks.Network{ID: networkID, Subnets: []string{}}
	gatewayIP := map[string]string{}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("GetCreateOpts panicked on an L2 network with no subnets: %v", r)
		}
	}()

	createOpts, err := client.GetCreateOpts(t.Context(), network, "aa:bb:cc:dd:ee:ff", nil, "test-vm", nil, gatewayIP, nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if createOpts.FixedIPs != nil {
		t.Fatalf("expected no FixedIPs to be set, got: %+v", createOpts.FixedIPs)
	}
	if _, ok := gatewayIP["aa:bb:cc:dd:ee:ff"]; ok {
		t.Fatalf("expected no gateway to be recorded for a subnet-less L2 network, got: %v", gatewayIP)
	}
	if createOpts.Name != "port-test-vm-1" {
		t.Errorf("expected fallback port name %q, got %q", "port-test-vm-1", createOpts.Name)
	}
}

// TestCreatePortWithDHCP_MarksEntriesAsDHCP verifies that the IP(s) it pulls
// back from the created port are marked IpEntry.DHCP=true. These IPs came
// from a live Neutron allocation (the preferred static IP didn't fit the
// target subnet), not a preserved/custom static IP, so guest-config code
// must configure a real DHCP client for them instead of pinning them
// statically (see buildWildcardNetplanYAML in v2v-helper/virtv2v).
func TestCreatePortWithDHCP_MarksEntriesAsDHCP(t *testing.T) {
	const networkID = "net-1"
	const subnetID = "subnet-1"
	const mac = "aa:bb:cc:dd:ee:ff"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/networks/"+networkID):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"network": map[string]any{"id": networkID, "tags": []string{}},
			})
		case strings.Contains(r.URL.Path, "/subnets/"+subnetID):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"subnet": map[string]any{
					"id":         subnetID,
					"cidr":       "192.168.50.0/24",
					"gateway_ip": "192.168.50.1",
				},
			})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/ports"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"port": map[string]any{
					"id":          "port-1",
					"mac_address": mac,
					"fixed_ips": []map[string]any{
						{"ip_address": "192.168.50.77", "subnet_id": subnetID},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := &OpenStackClients{NetworkingClient: newTestNetworkingClient(srv)}
	network := &networks.Network{ID: networkID, Subnets: []string{subnetID}}
	ipPerMac := map[string][]vm.IpEntry{
		mac: {{IP: "10.0.0.5", Prefix: 24}}, // the static IP that didn't fit, about to be replaced
	}
	gatewayIP := map[string]string{}
	createOpts := ports.CreateOpts{Name: "port-test", NetworkID: networkID}

	port, err := client.CreatePortWithDHCP(t.Context(), network, ipPerMac, mac, gatewayIP, createOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if port.ID != "port-1" {
		t.Fatalf("expected port-1, got: %s", port.ID)
	}

	want := []vm.IpEntry{{IP: "192.168.50.77", Prefix: 0, DHCP: true}}
	got := ipPerMac[mac]
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("ipPerMac[mac] = %#v, want %#v (DHCP must be true for a live Neutron allocation)", got, want)
	}
	if gatewayIP[mac] != "192.168.50.1" {
		t.Fatalf("gatewayIP[mac] = %q, want %q", gatewayIP[mac], "192.168.50.1")
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

func TestBuildPortName(t *testing.T) {
	tests := []struct {
		name             string
		vmname           string
		subnetName       string
		index            int
		want             string
		wantLen          int
		wantSuffixIntact bool // whether the "-<subnetName>-<index>"/"-<index>" suffix must survive untruncated
	}{
		{
			name:             "normal case with subnet, first NIC",
			vmname:           "my-vm",
			subnetName:       "prod-subnet",
			index:            1,
			want:             "port-my-vm-prod-subnet-1",
			wantSuffixIntact: true,
		},
		{
			name:             "normal case with subnet, second NIC on same subnet",
			vmname:           "my-vm",
			subnetName:       "prod-subnet",
			index:            2,
			want:             "port-my-vm-prod-subnet-2",
			wantSuffixIntact: true,
		},
		{
			name:             "L2 network no subnet, first NIC",
			vmname:           "my-vm",
			subnetName:       "",
			index:            1,
			want:             "port-my-vm-1",
			wantSuffixIntact: true,
		},
		{
			name:             "L2 network no subnet, second NIC",
			vmname:           "my-vm",
			subnetName:       "",
			index:            2,
			want:             "port-my-vm-2",
			wantSuffixIntact: true,
		},
		{
			name:             "vmname truncated to fit 255 limit",
			vmname:           strings.Repeat("a", 300),
			subnetName:       "sub",
			index:            1,
			wantLen:          constants.NeutronMaxPortNameLen,
			wantSuffixIntact: true,
		},
		{
			name:             "vmname truncated, L2 (no subnet)",
			vmname:           strings.Repeat("a", 300),
			subnetName:       "",
			index:            1,
			wantLen:          constants.NeutronMaxPortNameLen,
			wantSuffixIntact: true,
		},
		{
			name:       "extremely long subnet name exceeds prefix budget",
			vmname:     "vm",
			subnetName: strings.Repeat("s", 300),
			index:      1,
			wantLen:    constants.NeutronMaxPortNameLen,
			// subnet name itself doesn't fit even with an empty vmname, so the whole
			// string is hard-truncated as a last resort and the suffix is not preserved.
			wantSuffixIntact: false,
		},
		{
			name:             "all short names fit without truncation",
			vmname:           "vm",
			subnetName:       "sub",
			index:            1,
			want:             "port-vm-sub-1",
			wantSuffixIntact: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPortName(tt.vmname, tt.subnetName, tt.index)
			if tt.want != "" && got != tt.want {
				t.Errorf("buildPortName(%q, %q, %d) = %q, want %q", tt.vmname, tt.subnetName, tt.index, got, tt.want)
			}
			if tt.wantLen > 0 && len(got) != tt.wantLen {
				t.Errorf("buildPortName(%q, %q, %d) len=%d, want %d", tt.vmname, tt.subnetName, tt.index, len(got), tt.wantLen)
			}
			if len(got) > constants.NeutronMaxPortNameLen {
				t.Errorf("buildPortName result len=%d exceeds %d: %q", len(got), constants.NeutronMaxPortNameLen, got)
			}
			wantSuffix := fmt.Sprintf("-%d", tt.index)
			if tt.subnetName != "" {
				wantSuffix = "-" + tt.subnetName + wantSuffix
			}
			if tt.wantSuffixIntact && !strings.HasSuffix(got, wantSuffix) {
				t.Errorf("buildPortName(%q, %q, %d) = %q, expected suffix %q to be preserved intact for uniqueness", tt.vmname, tt.subnetName, tt.index, got, wantSuffix)
			}
		})
	}
}

// TestGetCreateOpts_MultipleNICsSameSubnet_IndexIncrements verifies the fix
// for the remaining collision case in #2143: a VM with two or more NICs that
// land on the *same* subnet would still get identical port names under plain
// port-<vmname>-<subnet-name> naming. Sharing one subnetPortIndex map across
// the calls (as callers must, one per NIC) makes each subsequent NIC on that
// subnet get a distinct, incrementing suffix instead.
func TestGetCreateOpts_MultipleNICsSameSubnet_IndexIncrements(t *testing.T) {
	const networkID = "net-shared"
	const subnetID = "subnet-shared"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/networks/"+networkID):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"network": map[string]any{"id": networkID, "tags": []string{}},
			})
		case strings.Contains(r.URL.Path, "/subnets/"+subnetID):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"subnet": map[string]any{
					"id":         subnetID,
					"name":       "prod-subnet",
					"cidr":       "10.0.0.0/24",
					"gateway_ip": "10.0.0.1",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := &OpenStackClients{NetworkingClient: newTestNetworkingClient(srv)}
	network := &networks.Network{ID: networkID, Subnets: []string{subnetID}}
	subnetPortIndex := map[string]int{}

	// NIC 1 and NIC 2 both resolve to the same subnet (different IPs, same /24).
	opts1, err := client.GetCreateOpts(t.Context(), network, "aa:bb:cc:dd:ee:01",
		[]vm.IpEntry{{IP: "10.0.0.10", Prefix: 24}}, "test-vm", nil, map[string]string{}, subnetPortIndex)
	if err != nil {
		t.Fatalf("first GetCreateOpts call failed: %v", err)
	}
	opts2, err := client.GetCreateOpts(t.Context(), network, "aa:bb:cc:dd:ee:02",
		[]vm.IpEntry{{IP: "10.0.0.20", Prefix: 24}}, "test-vm", nil, map[string]string{}, subnetPortIndex)
	if err != nil {
		t.Fatalf("second GetCreateOpts call failed: %v", err)
	}

	if want := "port-test-vm-prod-subnet-1"; opts1.Name != want {
		t.Errorf("first NIC port name = %q, want %q", opts1.Name, want)
	}
	if want := "port-test-vm-prod-subnet-2"; opts2.Name != want {
		t.Errorf("second NIC port name = %q, want %q", opts2.Name, want)
	}
	if opts1.Name == opts2.Name {
		t.Fatalf("both NICs on the same subnet got identical port names: %q", opts1.Name)
	}
}
