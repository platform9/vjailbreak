package controller

import (
	"context"
	"testing"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func reprovisionTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := vjailbreakv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add scheme: %v", err)
	}
	return scheme
}

func TestReprovisionAllowed_NoMigrations(t *testing.T) {
	if !reprovisionAllowed([]string{}) {
		t.Error("reprovisionAllowed([]) should return true")
	}
}

func TestReprovisionAllowed_NilMigrations(t *testing.T) {
	if !reprovisionAllowed(nil) {
		t.Error("reprovisionAllowed(nil) should return true")
	}
}

func TestReprovisionAllowed_WithActiveMigrations(t *testing.T) {
	if reprovisionAllowed([]string{"migration-1"}) {
		t.Error("reprovisionAllowed([migration-1]) should return false")
	}
}

func TestReconcileReprovision_NoAnnotation(t *testing.T) {
	ctx := context.Background()
	scheme := reprovisionTestScheme(t)

	node := &vjailbreakv1alpha1.VjailbreakNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "worker-1",
			Namespace: "default",
		},
		Status: vjailbreakv1alpha1.VjailbreakNodeStatus{
			OpenstackUUID: "uuid-abc",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node).
		WithStatusSubresource(&vjailbreakv1alpha1.VjailbreakNode{}).
		Build()

	r := &VjailbreakNodeReconciler{Client: fakeClient, Scheme: scheme}
	reprovisioned, err := r.reconcileReprovision(ctx, node)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reprovisioned {
		t.Error("expected reprovisioned=false when no annotation")
	}
	// UUID must remain unchanged
	updated := &vjailbreakv1alpha1.VjailbreakNode{}
	if err := fakeClient.Get(ctx, types.NamespacedName{Name: "worker-1", Namespace: "default"}, updated); err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if updated.Status.OpenstackUUID != "uuid-abc" {
		t.Errorf("OpenstackUUID changed unexpectedly: got %q", updated.Status.OpenstackUUID)
	}
}

func TestReconcileReprovision_RequestedWithActiveMigrations(t *testing.T) {
	ctx := context.Background()
	scheme := reprovisionTestScheme(t)

	node := &vjailbreakv1alpha1.VjailbreakNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "worker-1",
			Namespace: "default",
			Annotations: map[string]string{
				reprovisionAnnotation: reprovisionRequested,
			},
		},
		Status: vjailbreakv1alpha1.VjailbreakNodeStatus{
			OpenstackUUID:    "uuid-abc",
			ActiveMigrations: []string{"m-1"},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node).
		WithStatusSubresource(&vjailbreakv1alpha1.VjailbreakNode{}).
		Build()

	r := &VjailbreakNodeReconciler{Client: fakeClient, Scheme: scheme}
	reprovisioned, err := r.reconcileReprovision(ctx, node)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reprovisioned {
		t.Error("expected reprovisioned=false when active migrations block reprovision")
	}

	updated := &vjailbreakv1alpha1.VjailbreakNode{}
	if err := fakeClient.Get(ctx, types.NamespacedName{Name: "worker-1", Namespace: "default"}, updated); err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if updated.Annotations[reprovisionAnnotation] != reprovisionBlocked {
		t.Errorf("annotation = %q, want %q", updated.Annotations[reprovisionAnnotation], reprovisionBlocked)
	}
	if updated.Status.OpenstackUUID != "uuid-abc" {
		t.Errorf("OpenstackUUID changed unexpectedly: got %q", updated.Status.OpenstackUUID)
	}
}

func TestReconcileReprovision_RequestedIdleNode(t *testing.T) {
	ctx := context.Background()
	scheme := reprovisionTestScheme(t)

	node := &vjailbreakv1alpha1.VjailbreakNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "worker-1",
			Namespace: "default",
			Annotations: map[string]string{
				reprovisionAnnotation: reprovisionRequested,
			},
		},
		Status: vjailbreakv1alpha1.VjailbreakNodeStatus{
			OpenstackUUID:    "uuid-abc",
			ActiveMigrations: []string{},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node).
		WithStatusSubresource(&vjailbreakv1alpha1.VjailbreakNode{}).
		Build()

	r := &VjailbreakNodeReconciler{Client: fakeClient, Scheme: scheme}
	reprovisioned, err := r.reconcileReprovision(ctx, node)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reprovisioned {
		t.Error("expected reprovisioned=true for idle node with reprovision annotation")
	}

	updated := &vjailbreakv1alpha1.VjailbreakNode{}
	if err := fakeClient.Get(ctx, types.NamespacedName{Name: "worker-1", Namespace: "default"}, updated); err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if _, hasAnnotation := updated.Annotations[reprovisionAnnotation]; hasAnnotation {
		t.Errorf("reprovision annotation should be removed after reprovisioning")
	}
	if updated.Status.OpenstackUUID != "" {
		t.Errorf("OpenstackUUID should be cleared, got %q", updated.Status.OpenstackUUID)
	}
	if updated.Status.Phase != "" {
		t.Errorf("Phase should be reset, got %q", updated.Status.Phase)
	}
}
