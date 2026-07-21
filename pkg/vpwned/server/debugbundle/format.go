package debugbundle

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// renderObjectYAML renders one Kubernetes object as YAML with the noisy
// managedFields metadata stripped.
func renderObjectYAML(obj *unstructured.Unstructured) string {
	clean := obj.DeepCopy()
	unstructured.RemoveNestedField(clean.Object, "metadata", "managedFields")
	data, err := yaml.Marshal(clean.Object)
	if err != nil {
		return fmt.Sprintf("[failed to render %s/%s as YAML: %v]\n", obj.GetKind(), obj.GetName(), err)
	}
	return string(data)
}
