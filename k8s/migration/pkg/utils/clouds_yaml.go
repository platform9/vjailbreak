package utils

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/gophercloud/gophercloud/v2"
	"gopkg.in/yaml.v3"
)

// Sentinel errors returned by ParseCloudsYAML. Callers should map these to the
// corresponding Condition Reason codes when populating OpenstackCreds status.
var (
	// ErrInvalidYAML indicates the credential Secret's clouds.yaml content
	// failed YAML parsing. Maps to ReasonInvalidYAML.
	ErrInvalidYAML = errors.New("clouds.yaml: invalid YAML")

	// ErrAmbiguousCloudName indicates the clouds.yaml contains multiple cloud
	// entries and no cloudName was supplied. The wrapper error message lists
	// the available cloud names. Maps to ReasonAmbiguousCloudName.
	ErrAmbiguousCloudName = errors.New("clouds.yaml: cloudName required when multiple cloud entries are present")

	// ErrCloudNotFound indicates the supplied cloudName does not match any
	// cloud entry in the clouds.yaml.
	ErrCloudNotFound = errors.New("clouds.yaml: cloudName not found")

	// ErrMissingRequiredField indicates a required field (e.g., auth_url) is
	// absent from the selected cloud entry. Maps to ReasonMissingRequiredField.
	ErrMissingRequiredField = errors.New("clouds.yaml: missing required field")

	// ErrUnknownAuthType indicates the cloud entry declares an auth_type that
	// vjailbreak does not currently support. Maps to ReasonUnknownAuthType.
	ErrUnknownAuthType = errors.New("clouds.yaml: unsupported auth_type")

	// ErrCacertPathUnresolvable indicates the cloud entry's cacert field is a
	// filesystem path rather than inline certificate content. Maps to
	// ReasonCacertPathUnresolvable.
	ErrCacertPathUnresolvable = errors.New("clouds.yaml: cacert references a filesystem path; inline content required")
)

// SecretContainsCloudsYAML reports whether the credential Secret's data
// indicates the new clouds.yaml-backed credential format. The function returns
// true if the Secret contains a non-empty value under the "clouds.yaml" key.
// Callers branch on this result to dispatch between the clouds.yaml parser
// and the legacy OS_*-keyed path.
func SecretContainsCloudsYAML(secretData map[string][]byte) bool {
	v, ok := secretData["clouds.yaml"]
	return ok && len(v) > 0
}

// CloudConfig is the parsed representation of a single cloud entry from
// clouds.yaml, mapped into the values vjailbreak's controller needs.
type CloudConfig struct {
	// AuthOptions is ready to pass to openstack.AuthenticatedClient.
	AuthOptions gophercloud.AuthOptions

	// AuthType is the literal auth_type from clouds.yaml (e.g., "v3password",
	// "v3applicationcredential"). Useful for status reporting and operator UX.
	AuthType string

	// RegionName is the OpenStack region for the destination cloud.
	RegionName string

	// Interface is the endpoint interface ("public", "internal", "admin").
	Interface string

	// Microversions maps the gophercloud service name ("compute", "volume",
	// "image", "network", "identity") to the operator-configured microversion
	// from clouds.yaml's *_api_version fields. Empty when not configured.
	Microversions map[string]string

	// Verify is the TLS verification setting from clouds.yaml. nil means
	// "use library default".
	Verify *bool

	// Cacert holds inline certificate content (PEM) when configured.
	Cacert string
}

// supportedAuthTypes lists the auth_type values vjailbreak currently consumes.
// Empty auth_type is permitted and defaults to v3password if username/password
// are present.
var supportedAuthTypes = map[string]struct{}{
	"":                        {},
	"v3password":              {},
	"password":                {},
	"v3applicationcredential": {},
}

// cloudsFile mirrors the subset of clouds.yaml that vjailbreak reads.
type cloudsFile struct {
	Clouds map[string]cloudEntry `yaml:"clouds"`
}

type cloudEntry struct {
	AuthType            string    `yaml:"auth_type"`
	Auth                authBlock `yaml:"auth"`
	RegionName          string    `yaml:"region_name"`
	Interface           string    `yaml:"interface"`
	Verify              *bool     `yaml:"verify"`
	Cacert              string    `yaml:"cacert"`
	IdentityAPIVersion  string    `yaml:"identity_api_version"`
	ComputeAPIVersion   string    `yaml:"compute_api_version"`
	VolumeAPIVersion    string    `yaml:"volume_api_version"`
	ImageAPIVersion     string    `yaml:"image_api_version"`
	NetworkAPIVersion   string    `yaml:"network_api_version"`
}

type authBlock struct {
	AuthURL                     string `yaml:"auth_url"`
	Username                    string `yaml:"username"`
	UserID                      string `yaml:"user_id"`
	Password                    string `yaml:"password"`
	ApplicationCredentialID     string `yaml:"application_credential_id"`
	ApplicationCredentialSecret string `yaml:"application_credential_secret"`
	UserDomainName              string `yaml:"user_domain_name"`
	UserDomainID                string `yaml:"user_domain_id"`
	ProjectName                 string `yaml:"project_name"`
	ProjectID                   string `yaml:"project_id"`
	ProjectDomainName           string `yaml:"project_domain_name"`
	ProjectDomainID             string `yaml:"project_domain_id"`
	DomainName                  string `yaml:"domain_name"`
	DomainID                    string `yaml:"domain_id"`
}

// ParseCloudsYAML reads OpenStack clouds.yaml content from a Kubernetes Secret
// data value, selects the cloud entry by cloudName, and returns the parsed
// configuration.
//
// Selection rules:
//   - If the YAML has exactly one cloud entry, cloudName may be empty.
//   - If the YAML has multiple entries and cloudName is empty, returns
//     ErrAmbiguousCloudName (wrapped with the list of cloud names).
//   - If cloudName is supplied but not present, returns ErrCloudNotFound.
func ParseCloudsYAML(yamlBytes []byte, cloudName string) (*CloudConfig, error) {
	file, err := parseCloudsFile(yamlBytes)
	if err != nil {
		return nil, err
	}

	entry, selectedName, err := selectCloud(file, cloudName)
	if err != nil {
		return nil, err
	}
	_ = selectedName

	if _, ok := supportedAuthTypes[entry.AuthType]; !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownAuthType, entry.AuthType)
	}

	if entry.Auth.AuthURL == "" {
		return nil, fmt.Errorf("%w: auth_url", ErrMissingRequiredField)
	}

	if entry.Cacert != "" && !looksLikeInlineCert(entry.Cacert) {
		return nil, fmt.Errorf("%w: %q", ErrCacertPathUnresolvable, entry.Cacert)
	}

	cfg := &CloudConfig{
		AuthType:      entry.AuthType,
		RegionName:    entry.RegionName,
		Interface:     entry.Interface,
		Verify:        entry.Verify,
		Cacert:        entry.Cacert,
		Microversions: collectMicroversions(entry),
	}
	cfg.AuthOptions = buildAuthOptions(entry)
	return cfg, nil
}

// CloudNames returns the top-level cloud entry names from a clouds.yaml. Used
// to surface available choices in error messages and status conditions when
// cloudName is ambiguous or unrecognized.
func CloudNames(yamlBytes []byte) ([]string, error) {
	file, err := parseCloudsFile(yamlBytes)
	if err != nil {
		return nil, err
	}
	return sortedKeys(file.Clouds), nil
}

func parseCloudsFile(yamlBytes []byte) (*cloudsFile, error) {
	var file cloudsFile
	if err := yaml.Unmarshal(yamlBytes, &file); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidYAML, err)
	}
	if len(file.Clouds) == 0 {
		return nil, fmt.Errorf("%w: no cloud entries under top-level 'clouds' key", ErrInvalidYAML)
	}
	return &file, nil
}

func selectCloud(file *cloudsFile, cloudName string) (cloudEntry, string, error) {
	if cloudName == "" {
		if len(file.Clouds) == 1 {
			for name, entry := range file.Clouds {
				return entry, name, nil
			}
		}
		return cloudEntry{}, "", fmt.Errorf("%w: available cloud names: %s",
			ErrAmbiguousCloudName, strings.Join(sortedKeys(file.Clouds), ", "))
	}
	entry, ok := file.Clouds[cloudName]
	if !ok {
		return cloudEntry{}, "", fmt.Errorf("%w: %q (available: %s)",
			ErrCloudNotFound, cloudName, strings.Join(sortedKeys(file.Clouds), ", "))
	}
	return entry, cloudName, nil
}

func buildAuthOptions(entry cloudEntry) gophercloud.AuthOptions {
	opts := gophercloud.AuthOptions{
		IdentityEndpoint:            entry.Auth.AuthURL,
		Username:                    entry.Auth.Username,
		UserID:                      entry.Auth.UserID,
		Password:                    entry.Auth.Password,
		ApplicationCredentialID:     entry.Auth.ApplicationCredentialID,
		ApplicationCredentialSecret: entry.Auth.ApplicationCredentialSecret,
		DomainName:                  entry.Auth.DomainName,
		DomainID:                    entry.Auth.DomainID,
	}

	// Application Credentials carry scope at creation time; user-side
	// project/domain hints are not forwarded to the auth request.
	if entry.AuthType == "v3applicationcredential" {
		return opts
	}

	opts.TenantName = entry.Auth.ProjectName
	opts.TenantID = entry.Auth.ProjectID
	// User-domain hints fall back to domain hints when present.
	if entry.Auth.UserDomainName != "" {
		opts.DomainName = entry.Auth.UserDomainName
	}
	if entry.Auth.UserDomainID != "" {
		opts.DomainID = entry.Auth.UserDomainID
	}
	return opts
}

func collectMicroversions(entry cloudEntry) map[string]string {
	mvs := map[string]string{}
	if entry.IdentityAPIVersion != "" {
		mvs["identity"] = entry.IdentityAPIVersion
	}
	if entry.ComputeAPIVersion != "" {
		mvs["compute"] = entry.ComputeAPIVersion
	}
	if entry.VolumeAPIVersion != "" {
		mvs["volume"] = entry.VolumeAPIVersion
	}
	if entry.ImageAPIVersion != "" {
		mvs["image"] = entry.ImageAPIVersion
	}
	if entry.NetworkAPIVersion != "" {
		mvs["network"] = entry.NetworkAPIVersion
	}
	return mvs
}

// looksLikeInlineCert returns true when the cacert value appears to be inline
// PEM content rather than a filesystem path. Filesystem paths starting with
// "/" or "~/" would refer to the operator's local environment and are not
// resolvable inside the controller pod.
func looksLikeInlineCert(s string) bool {
	trimmed := strings.TrimSpace(s)
	return strings.HasPrefix(trimmed, "-----BEGIN")
}

func sortedKeys(m map[string]cloudEntry) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
