// Package debugbundle collects a migration debug bundle: the related
// vJailbreak custom resources, the v2v-helper pod object, migration
// ConfigMaps, pod stdout/stderr logs and the debug log files written to
// /var/log/pf9. It is the backend replacement for the UI-side bundle
// collection added in PR #1929.
package debugbundle

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// VjailbreakGroupVersion is the API group/version of all vJailbreak CRDs.
var VjailbreakGroupVersion = schema.GroupVersion{Group: "vjailbreak.k8s.pf9.io", Version: "v1alpha1"}

// relatedCRDKinds maps the CRD plural (used in bundle file paths) to the
// resource Kind. Mirrors RELATED_CRD_PLURALS in the UI bundle collector.
var relatedCRDKinds = map[string]string{
	"arraycreds":            "ArrayCreds",
	"arraycredsmappings":    "ArrayCredsMapping",
	"bmconfigs":             "BMConfig",
	"clustermigrations":     "ClusterMigration",
	"esximigrations":        "ESXIMigration",
	"esxisshcreds":          "ESXiSSHCreds",
	"migrationplans":        "MigrationPlan",
	"migrations":            "Migration",
	"migrationtemplates":    "MigrationTemplate",
	"networkmappings":       "NetworkMapping",
	"openstackcreds":        "OpenstackCreds",
	"pcdclusters":           "PCDCluster",
	"pcdhosts":              "PCDHost",
	"proxyvms":              "ProxyVM",
	"rdmdisks":              "RDMDisk",
	"rollingmigrationplans": "RollingMigrationPlan",
	"storagemappings":       "StorageMapping",
	"vjailbreaknodes":       "VjailbreakNode",
	"vmwareclusters":        "VMwareCluster",
	"vmwarecreds":           "VMwareCreds",
	"vmwarehosts":           "VMwareHost",
	"vmwaremachines":        "VMwareMachine",
	"volumeimageprofiles":   "VolumeImageProfile",
}

// BundleEntry is a single object included in the resource bundle.
type BundleEntry struct {
	// Path is the virtual file path shown in the bundle, e.g.
	// kubernetes/migrations/migration-foo.yaml
	Path   string
	Object *unstructured.Unstructured
}

// stringSet mirrors the Set<string> helpers used by the UI collector:
// values are trimmed and empty strings are never stored.
type stringSet map[string]struct{}

func newStringSet() stringSet { return stringSet{} }

func (s stringSet) add(input string) {
	trimmed := strings.TrimSpace(input)
	if trimmed != "" {
		s[trimmed] = struct{}{}
	}
}

func (s stringSet) addAll(inputs []string) {
	for _, input := range inputs {
		s.add(input)
	}
}

func (s stringSet) has(input string) bool {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return false
	}
	_, ok := s[trimmed]
	return ok
}

func (s stringSet) isEmpty() bool { return len(s) == 0 }

// nestedString reads a nested string field, tolerating missing paths and
// wrong types, and trims the result (parity with the UI `value(field(...))`).
func nestedString(obj map[string]interface{}, path ...string) string {
	val, found, err := unstructured.NestedString(obj, path...)
	if !found || err != nil {
		return ""
	}
	return strings.TrimSpace(val)
}

// nestedStringSlice reads a nested field that holds a list of strings.
// Non-string items are skipped.
func nestedStringSlice(obj map[string]interface{}, path ...string) []string {
	items, found, err := unstructured.NestedSlice(obj, path...)
	if !found || err != nil {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if str, ok := item.(string); ok {
			out = append(out, str)
		}
	}
	return out
}

// nestedSlice reads a nested field that holds a list of arbitrary values.
func nestedSlice(obj map[string]interface{}, path ...string) []interface{} {
	items, found, err := unstructured.NestedSlice(obj, path...)
	if !found || err != nil {
		return nil
	}
	return items
}

// isOwnedBy reports whether any owner reference of obj matches a name or
// UID in owners.
func isOwnedBy(obj *unstructured.Unstructured, owners stringSet) bool {
	for _, owner := range obj.GetOwnerReferences() {
		if owners.has(string(owner.UID)) || owners.has(owner.Name) {
			return true
		}
	}
	return false
}
