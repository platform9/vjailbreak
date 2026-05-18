package utils

import (
	"errors"
	"sort"
	"strings"
	"testing"
)

const singleEntryYAML = `
clouds:
  destination:
    auth_type: v3password
    auth:
      auth_url: https://keystone.example.com:5000/v3
      username: admin
      password: secret-password
      project_name: migration
      user_domain_name: Default
      project_domain_name: Default
    region_name: RegionOne
    interface: public
`

const multiEntryYAML = `
clouds:
  dc-paris:
    auth_type: v3password
    auth:
      auth_url: https://paris.example.com:5000/v3
      username: paris-admin
      password: paris-secret
      project_name: migration
      user_domain_name: Default
    region_name: paris
    interface: public
  dc-frankfurt:
    auth_type: v3applicationcredential
    auth:
      auth_url: https://frankfurt.example.com:5000/v3
      application_credential_id: appcred-id-123
      application_credential_secret: appcred-secret-xyz
    region_name: frankfurt
    interface: internal
`

const appCredYAML = `
clouds:
  destination:
    auth_type: v3applicationcredential
    auth:
      auth_url: https://keystone.example.com:5000/v3
      application_credential_id: my-app-cred-id
      application_credential_secret: my-app-cred-secret
    region_name: RegionOne
    interface: public
    compute_api_version: "2.95"
    volume_api_version: "3.70"
    image_api_version: "2.16"
`

const cacertPathYAML = `
clouds:
  destination:
    auth_type: v3password
    auth:
      auth_url: https://keystone.example.com:5000/v3
      username: admin
      password: secret
    region_name: RegionOne
    cacert: /etc/ssl/certs/destination-ca.pem
`

const cacertInlineYAML = `
clouds:
  destination:
    auth_type: v3password
    auth:
      auth_url: https://keystone.example.com:5000/v3
      username: admin
      password: secret
    region_name: RegionOne
    cacert: |
      -----BEGIN CERTIFICATE-----
      MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAxxx
      -----END CERTIFICATE-----
`

const missingAuthURLYAML = `
clouds:
  destination:
    auth_type: v3password
    auth:
      username: admin
      password: secret
`

const unknownAuthTypeYAML = `
clouds:
  destination:
    auth_type: v3oidcaccesstoken
    auth:
      auth_url: https://keystone.example.com:5000/v3
      access_token: some-token
`

func TestParseCloudsYAML_SingleEntryEmptyName(t *testing.T) {
	cfg, err := ParseCloudsYAML([]byte(singleEntryYAML), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AuthOptions.IdentityEndpoint != "https://keystone.example.com:5000/v3" {
		t.Errorf("IdentityEndpoint = %q; want keystone URL", cfg.AuthOptions.IdentityEndpoint)
	}
	if cfg.AuthOptions.Username != "admin" {
		t.Errorf("Username = %q; want admin", cfg.AuthOptions.Username)
	}
	if cfg.AuthOptions.Password != "secret-password" {
		t.Errorf("Password not populated")
	}
	if cfg.AuthType != "v3password" {
		t.Errorf("AuthType = %q; want v3password", cfg.AuthType)
	}
	if cfg.RegionName != "RegionOne" {
		t.Errorf("RegionName = %q; want RegionOne", cfg.RegionName)
	}
	if cfg.Interface != "public" {
		t.Errorf("Interface = %q; want public", cfg.Interface)
	}
}

func TestParseCloudsYAML_MultiEntryNamed(t *testing.T) {
	cfg, err := ParseCloudsYAML([]byte(multiEntryYAML), "dc-frankfurt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AuthType != "v3applicationcredential" {
		t.Errorf("AuthType = %q; want v3applicationcredential", cfg.AuthType)
	}
	if cfg.AuthOptions.ApplicationCredentialID != "appcred-id-123" {
		t.Errorf("ApplicationCredentialID = %q; want appcred-id-123", cfg.AuthOptions.ApplicationCredentialID)
	}
	if cfg.AuthOptions.ApplicationCredentialSecret != "appcred-secret-xyz" {
		t.Errorf("ApplicationCredentialSecret not populated")
	}
	if cfg.AuthOptions.Username != "" || cfg.AuthOptions.Password != "" {
		t.Errorf("Username/Password should be empty for v3applicationcredential auth")
	}
	if cfg.RegionName != "frankfurt" {
		t.Errorf("RegionName = %q; want frankfurt", cfg.RegionName)
	}
}

func TestParseCloudsYAML_MultiEntryEmptyNameAmbiguous(t *testing.T) {
	_, err := ParseCloudsYAML([]byte(multiEntryYAML), "")
	if !errors.Is(err, ErrAmbiguousCloudName) {
		t.Fatalf("error = %v; want %v", err, ErrAmbiguousCloudName)
	}
	msg := err.Error()
	if !strings.Contains(msg, "dc-paris") || !strings.Contains(msg, "dc-frankfurt") {
		t.Errorf("error message should list available cloud names; got: %s", msg)
	}
}

func TestParseCloudsYAML_CloudNotFound(t *testing.T) {
	_, err := ParseCloudsYAML([]byte(multiEntryYAML), "dc-london")
	if !errors.Is(err, ErrCloudNotFound) {
		t.Fatalf("error = %v; want %v", err, ErrCloudNotFound)
	}
}

func TestParseCloudsYAML_InvalidYAML(t *testing.T) {
	_, err := ParseCloudsYAML([]byte("{not valid: yaml: at all: :"), "")
	if !errors.Is(err, ErrInvalidYAML) {
		t.Fatalf("error = %v; want %v", err, ErrInvalidYAML)
	}
}

func TestParseCloudsYAML_MissingAuthURL(t *testing.T) {
	_, err := ParseCloudsYAML([]byte(missingAuthURLYAML), "")
	if !errors.Is(err, ErrMissingRequiredField) {
		t.Fatalf("error = %v; want %v", err, ErrMissingRequiredField)
	}
	if !strings.Contains(err.Error(), "auth_url") {
		t.Errorf("error message should mention auth_url; got: %s", err)
	}
}

func TestParseCloudsYAML_UnknownAuthType(t *testing.T) {
	_, err := ParseCloudsYAML([]byte(unknownAuthTypeYAML), "")
	if !errors.Is(err, ErrUnknownAuthType) {
		t.Fatalf("error = %v; want %v", err, ErrUnknownAuthType)
	}
	if !strings.Contains(err.Error(), "v3oidcaccesstoken") {
		t.Errorf("error message should mention the offending auth_type; got: %s", err)
	}
}

func TestParseCloudsYAML_AppCredentialAndMicroversions(t *testing.T) {
	cfg, err := ParseCloudsYAML([]byte(appCredYAML), "destination")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AuthOptions.ApplicationCredentialID != "my-app-cred-id" {
		t.Errorf("ApplicationCredentialID = %q; want my-app-cred-id", cfg.AuthOptions.ApplicationCredentialID)
	}
	want := map[string]string{
		"compute": "2.95",
		"volume":  "3.70",
		"image":   "2.16",
	}
	if len(cfg.Microversions) != len(want) {
		t.Errorf("Microversions size = %d; want %d (got %v)", len(cfg.Microversions), len(want), cfg.Microversions)
	}
	for svc, expected := range want {
		if got := cfg.Microversions[svc]; got != expected {
			t.Errorf("Microversions[%s] = %q; want %q", svc, got, expected)
		}
	}
}

func TestParseCloudsYAML_CacertInlineAccepted(t *testing.T) {
	cfg, err := ParseCloudsYAML([]byte(cacertInlineYAML), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(cfg.Cacert, "BEGIN CERTIFICATE") {
		t.Errorf("Cacert should contain inline PEM content; got: %q", cfg.Cacert)
	}
}

func TestParseCloudsYAML_CacertPathRejected(t *testing.T) {
	_, err := ParseCloudsYAML([]byte(cacertPathYAML), "")
	if !errors.Is(err, ErrCacertPathUnresolvable) {
		t.Fatalf("error = %v; want %v", err, ErrCacertPathUnresolvable)
	}
}

func TestCloudNames_MultiEntry(t *testing.T) {
	names, err := CloudNames([]byte(multiEntryYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sort.Strings(names)
	want := []string{"dc-frankfurt", "dc-paris"}
	if len(names) != len(want) {
		t.Fatalf("got %d names, want %d (%v)", len(names), len(want), names)
	}
	for i, n := range names {
		if n != want[i] {
			t.Errorf("names[%d] = %q; want %q", i, n, want[i])
		}
	}
}

func TestCloudNames_SingleEntry(t *testing.T) {
	names, err := CloudNames([]byte(singleEntryYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 1 || names[0] != "destination" {
		t.Errorf("names = %v; want [destination]", names)
	}
}

func TestCloudNames_InvalidYAML(t *testing.T) {
	_, err := CloudNames([]byte("{not yaml"))
	if !errors.Is(err, ErrInvalidYAML) {
		t.Fatalf("error = %v; want %v", err, ErrInvalidYAML)
	}
}
