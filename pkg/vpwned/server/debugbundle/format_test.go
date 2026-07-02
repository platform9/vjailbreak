package debugbundle

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestFormatYAMLBundleStripsManagedFieldsAndSorts(t *testing.T) {
	first := coreObj("ConfigMap", "aaa", nil)
	second := coreObj("ConfigMap", "bbb", nil)
	if err := unstructured.SetNestedSlice(second.Object, []interface{}{
		map[string]interface{}{"manager": "kubectl"},
	}, "metadata", "managedFields"); err != nil {
		t.Fatalf("failed to set managedFields: %v", err)
	}

	// Pass entries out of order to verify sorting by path.
	output := FormatYAMLBundle([]BundleEntry{
		{Path: "kubernetes/configmaps/bbb.yaml", Object: second},
		{Path: "kubernetes/configmaps/aaa.yaml", Object: first},
	}, nil)

	if strings.Contains(output, "managedFields") {
		t.Errorf("managedFields must be stripped from output:\n%s", output)
	}
	aaaIdx := strings.Index(output, "FILE: kubernetes/configmaps/aaa.yaml")
	bbbIdx := strings.Index(output, "FILE: kubernetes/configmaps/bbb.yaml")
	if aaaIdx < 0 || bbbIdx < 0 || aaaIdx > bbbIdx {
		t.Errorf("entries must be sorted by path, got:\n%s", output)
	}
	if !strings.Contains(output, "name: aaa") {
		t.Errorf("expected YAML body for aaa, got:\n%s", output)
	}
}

func TestFormatYAMLBundleWarningsSection(t *testing.T) {
	output := FormatYAMLBundle(nil, []string{"warn one", "warn two"})

	if !strings.Contains(output, "FILE: collection-warnings.txt") {
		t.Fatalf("expected warnings section, got:\n%s", output)
	}
	if !strings.Contains(output, "warn one\nwarn two") {
		t.Errorf("expected warnings joined by newline, got:\n%s", output)
	}
}

func TestFormatYAMLBundleEmpty(t *testing.T) {
	if output := FormatYAMLBundle(nil, nil); output != "" {
		t.Errorf("expected empty output, got %q", output)
	}
}
