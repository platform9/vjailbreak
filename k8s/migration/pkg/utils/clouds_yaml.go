package utils

import (
	"errors"

	"github.com/gophercloud/gophercloud/v2"
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

// ParseCloudsYAML reads OpenStack clouds.yaml content from a Kubernetes Secret
// data value, selects the cloud entry by cloudName, and returns the parsed
// configuration.
//
// Selection rules:
//   - If the YAML has exactly one cloud entry, cloudName may be empty.
//   - If the YAML has multiple entries and cloudName is empty, returns
//     ErrAmbiguousCloudName (wrapped with the list of cloud names).
//   - If cloudName is supplied but not present, returns ErrCloudNotFound.
//
// Stub: returns ErrInvalidYAML so tests in clouds_yaml_test.go fail until
// the parser is implemented in a follow-up commit.
func ParseCloudsYAML(yamlBytes []byte, cloudName string) (*CloudConfig, error) {
	_ = yamlBytes
	_ = cloudName
	return nil, ErrInvalidYAML
}

// CloudNames returns the top-level cloud entry names from a clouds.yaml. Used
// to surface available choices in error messages and status conditions when
// cloudName is ambiguous or unrecognized.
//
// Stub: returns ErrInvalidYAML until the parser is implemented.
func CloudNames(yamlBytes []byte) ([]string, error) {
	_ = yamlBytes
	return nil, ErrInvalidYAML
}
