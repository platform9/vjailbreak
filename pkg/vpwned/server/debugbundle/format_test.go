package debugbundle

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestRenderObjectYAMLStripsManagedFields(t *testing.T) {
	obj := coreObj("ConfigMap", "sample", nil)
	if err := unstructured.SetNestedSlice(obj.Object, []interface{}{
		map[string]interface{}{"manager": "kubectl"},
	}, "metadata", "managedFields"); err != nil {
		t.Fatalf("failed to set managedFields: %v", err)
	}

	output := renderObjectYAML(obj)

	if strings.Contains(output, "managedFields") {
		t.Errorf("managedFields must be stripped from output:\n%s", output)
	}
	if !strings.Contains(output, "name: sample") {
		t.Errorf("expected object name in YAML, got:\n%s", output)
	}
	if !strings.Contains(output, "kind: ConfigMap") {
		t.Errorf("expected kind in YAML, got:\n%s", output)
	}
}
