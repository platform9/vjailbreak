package utils

import (
	"context"
	"testing"

	"github.com/platform9/vjailbreak/pkg/common/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
)

func testNodeScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := vjailbreakv1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("AddToScheme failed: %v", err)
	}
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatalf("corev1.AddToScheme failed: %v", err)
	}
	return s
}

func configMapWithHostEntries(data string) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.VjailbreakSettingsConfigMapName,
			Namespace: constants.NamespaceMigrationSystem,
		},
		Data: map[string]string{},
	}
	if data != "" {
		cm.Data[constants.AgentHostEntriesKey] = data
	}
	return cm
}

func TestGetAgentHostEntries_KeyPresentValidJSON(t *testing.T) {
	ctx := context.Background()
	s := testNodeScheme(t)
	cm := configMapWithHostEntries(`[{"ip":"1.2.3.4","hostnames":["h1"]}]`)
	fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(cm).Build()

	entries, err := GetAgentHostEntries(ctx, fakeClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].IP != "1.2.3.4" {
		t.Errorf("IP = %q, want %q", entries[0].IP, "1.2.3.4")
	}
	if len(entries[0].Hostnames) != 1 || entries[0].Hostnames[0] != "h1" {
		t.Errorf("Hostnames = %v, want [h1]", entries[0].Hostnames)
	}
}

func TestGetAgentHostEntries_KeyAbsent(t *testing.T) {
	ctx := context.Background()
	s := testNodeScheme(t)
	cm := configMapWithHostEntries("") // key not set
	fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(cm).Build()

	entries, err := GetAgentHostEntries(ctx, fakeClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(entries))
	}
}

func TestGetAgentHostEntries_KeyPresentEmptyString(t *testing.T) {
	ctx := context.Background()
	s := testNodeScheme(t)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.VjailbreakSettingsConfigMapName,
			Namespace: constants.NamespaceMigrationSystem,
		},
		Data: map[string]string{
			constants.AgentHostEntriesKey: "",
		},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(cm).Build()

	entries, err := GetAgentHostEntries(ctx, fakeClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(entries))
	}
}

func TestGetAgentHostEntries_MalformedJSON(t *testing.T) {
	ctx := context.Background()
	s := testNodeScheme(t)
	cm := configMapWithHostEntries(`not-json`)
	fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(cm).Build()

	_, err := GetAgentHostEntries(ctx, fakeClient)
	if err == nil {
		t.Error("expected error for malformed JSON, got nil")
	}
}

func TestGetAgentHostEntries_ConfigMapMissing(t *testing.T) {
	ctx := context.Background()
	s := testNodeScheme(t)
	// no ConfigMap in the fake client
	fakeClient := fake.NewClientBuilder().WithScheme(s).Build()

	_, err := GetAgentHostEntries(ctx, fakeClient)
	if err == nil {
		t.Error("expected error when ConfigMap is missing, got nil")
	}
}
