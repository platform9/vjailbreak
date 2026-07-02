package debugbundle

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
)

const testNamespace = "migration-system"

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add client-go scheme: %v", err)
	}
	if err := vjailbreakv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add vjailbreak scheme: %v", err)
	}
	return scheme
}

// cr builds an unstructured vJailbreak CR in the test namespace.
func cr(kind, name string, extra map[string]interface{}) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{}}
	obj.SetAPIVersion(VjailbreakGroupVersion.String())
	obj.SetKind(kind)
	obj.SetName(name)
	obj.SetNamespace(testNamespace)
	for key, val := range extra {
		obj.Object[key] = val
	}
	return obj
}

// coreObj builds an unstructured core/v1 object in the test namespace.
func coreObj(kind, name string, extra map[string]interface{}) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{}}
	obj.SetAPIVersion("v1")
	obj.SetKind(kind)
	obj.SetName(name)
	obj.SetNamespace(testNamespace)
	for key, val := range extra {
		obj.Object[key] = val
	}
	return obj
}

func newFakeClient(t *testing.T, objs ...*unstructured.Unstructured) client.Client {
	t.Helper()
	builder := fake.NewClientBuilder().WithScheme(newTestScheme(t))
	for _, obj := range objs {
		builder = builder.WithObjects(obj)
	}
	return builder.Build()
}

func entryPaths(entries []BundleEntry) map[string]bool {
	paths := map[string]bool{}
	for _, entry := range entries {
		paths[entry.Path] = true
	}
	return paths
}
