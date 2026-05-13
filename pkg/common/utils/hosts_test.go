package utils

import (
	"fmt"
	"strings"
	"testing"

	"github.com/platform9/vjailbreak/pkg/common/constants"
)

// TestValidateHostEntry verifies IP and hostname validation rules.
func TestValidateHostEntry(t *testing.T) {
	tests := []struct {
		name    string
		entry   HostEntry
		wantErr bool
	}{
		{
			name:    "valid single hostname",
			entry:   HostEntry{IP: "1.2.3.4", Hostnames: []string{"h1"}},
			wantErr: false,
		},
		{
			name:    "valid multiple hostnames",
			entry:   HostEntry{IP: "::1", Hostnames: []string{"h1", "h2.local"}},
			wantErr: false,
		},
		{
			name:    "valid FQDN hostnames",
			entry:   HostEntry{IP: "192.168.1.10", Hostnames: []string{"vcenter.corp.local", "vcenter"}},
			wantErr: false,
		},
		{
			name:    "empty IP",
			entry:   HostEntry{IP: "", Hostnames: []string{"h1"}},
			wantErr: true,
		},
		{
			name:    "invalid IP",
			entry:   HostEntry{IP: "999.x.y.z", Hostnames: []string{"h1"}},
			wantErr: true,
		},
		{
			name:    "no hostnames",
			entry:   HostEntry{IP: "1.2.3.4", Hostnames: []string{}},
			wantErr: true,
		},
		{
			name:    "nil hostnames",
			entry:   HostEntry{IP: "1.2.3.4", Hostnames: nil},
			wantErr: true,
		},
		{
			name:    "invalid hostname chars",
			entry:   HostEntry{IP: "1.2.3.4", Hostnames: []string{"bad_host!"}},
			wantErr: true,
		},
		{
			name:    "hostname starting with hyphen",
			entry:   HostEntry{IP: "1.2.3.4", Hostnames: []string{"-bad"}},
			wantErr: true,
		},
		{
			name:    "hostname ending with hyphen",
			entry:   HostEntry{IP: "1.2.3.4", Hostnames: []string{"bad-"}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHostEntry(tt.entry)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateHostEntry(%+v) error = %v, wantErr %v", tt.entry, err, tt.wantErr)
			}
		})
	}
}

// TestParseHostEntries verifies JSON deserialization from ConfigMap values.
func TestParseHostEntries(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantLen      int
		wantErr      bool
		checkFirst   bool
		wantFirst    HostEntry
	}{
		{
			name:    "empty string returns empty slice",
			input:   "",
			wantLen: 0,
			wantErr: false,
		},
		{
			name:    "empty array returns empty slice",
			input:   "[]",
			wantLen: 0,
			wantErr: false,
		},
		{
			name:       "valid single entry",
			input:      `[{"ip":"1.2.3.4","hostnames":["h1","h2"]}]`,
			wantLen:    1,
			wantErr:    false,
			checkFirst: true,
			wantFirst:  HostEntry{IP: "1.2.3.4", Hostnames: []string{"h1", "h2"}},
		},
		{
			name:    "valid multiple entries",
			input:   `[{"ip":"1.2.3.4","hostnames":["h1"]},{"ip":"5.6.7.8","hostnames":["h2"]}]`,
			wantLen: 2,
			wantErr: false,
		},
		{
			name:    "malformed JSON returns error",
			input:   `not-json`,
			wantLen: 0,
			wantErr: true,
		},
		{
			name:    "truncated JSON returns error",
			input:   `[{"ip":"1.2.3.4"`,
			wantLen: 0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseHostEntries(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseHostEntries(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if len(got) != tt.wantLen {
				t.Errorf("ParseHostEntries(%q) len = %d, want %d", tt.input, len(got), tt.wantLen)
				return
			}
			if tt.checkFirst {
				if got[0].IP != tt.wantFirst.IP {
					t.Errorf("got[0].IP = %q, want %q", got[0].IP, tt.wantFirst.IP)
				}
				if len(got[0].Hostnames) != len(tt.wantFirst.Hostnames) {
					t.Errorf("got[0].Hostnames len = %d, want %d", len(got[0].Hostnames), len(tt.wantFirst.Hostnames))
				}
			}
		})
	}
}

// TestSerializeParseRoundTrip verifies serialize→parse produces identical entries.
func TestSerializeParseRoundTrip(t *testing.T) {
	entries := []HostEntry{
		{IP: "192.168.1.10", Hostnames: []string{"vcenter.corp.local", "vcenter"}},
		{IP: "192.168.1.101", Hostnames: []string{"esxi01.corp.local"}},
		{IP: "192.168.2.5", Hostnames: []string{"pcd.corp.local", "pcd-api.corp.local"}},
	}

	serialized, err := SerializeHostEntries(entries)
	if err != nil {
		t.Fatalf("SerializeHostEntries failed: %v", err)
	}

	parsed, err := ParseHostEntries(serialized)
	if err != nil {
		t.Fatalf("ParseHostEntries failed: %v", err)
	}

	if len(parsed) != len(entries) {
		t.Fatalf("round-trip len = %d, want %d", len(parsed), len(entries))
	}

	for i, want := range entries {
		got := parsed[i]
		if got.IP != want.IP {
			t.Errorf("[%d] IP = %q, want %q", i, got.IP, want.IP)
		}
		if len(got.Hostnames) != len(want.Hostnames) {
			t.Errorf("[%d] Hostnames len = %d, want %d", i, len(got.Hostnames), len(want.Hostnames))
			continue
		}
		for j, h := range want.Hostnames {
			if got.Hostnames[j] != h {
				t.Errorf("[%d][%d] hostname = %q, want %q", i, j, got.Hostnames[j], h)
			}
		}
	}
}

// TestBuildUserData verifies cloud-init output for various entry combinations.
func TestBuildUserData(t *testing.T) {
	envFile := constants.ENVFileLocation
	masterIP := "192.168.1.100"
	token := "mytoken"

	// baseline: what the old fmt.Sprintf produced
	baseline := fmt.Sprintf(constants.K3sCloudInitScript, envFile, "false", masterIP, token)

	tests := []struct {
		name        string
		entries     []HostEntry
		wantBaseline bool // output must equal baseline
		wantContains []string
	}{
		{
			name:         "nil entries equals baseline",
			entries:      nil,
			wantBaseline: true,
		},
		{
			name:         "empty slice equals baseline",
			entries:      []HostEntry{},
			wantBaseline: true,
		},
		{
			name:    "single entry has echo line",
			entries: []HostEntry{{IP: "1.2.3.4", Hostnames: []string{"h1", "h2"}}},
			wantContains: []string{
				`echo "1.2.3.4 h1 h2" >> /etc/hosts`,
			},
		},
		{
			name: "multiple entries all present in order",
			entries: []HostEntry{
				{IP: "192.168.1.10", Hostnames: []string{"vcenter.corp.local"}},
				{IP: "192.168.1.101", Hostnames: []string{"esxi01.corp.local", "esxi01"}},
				{IP: "192.168.2.5", Hostnames: []string{"pcd.corp.local"}},
			},
			wantContains: []string{
				`echo "192.168.1.10 vcenter.corp.local" >> /etc/hosts`,
				`echo "192.168.1.101 esxi01.corp.local esxi01" >> /etc/hosts`,
				`echo "192.168.2.5 pcd.corp.local" >> /etc/hosts`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildUserData(envFile, masterIP, token, tt.entries)

			if tt.wantBaseline && got != baseline {
				t.Errorf("BuildUserData with no entries:\ngot:\n%s\nwant:\n%s", got, baseline)
				return
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("BuildUserData output missing line %q\nfull output:\n%s", want, got)
				}
			}
		})
	}
}
