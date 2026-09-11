package controller

import (
	"context"
	"testing"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/scope"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func deleteTestScheme(t *testing.T) *runtime.Scheme {
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

// TestReconcileDelete_DeletesK8sNodeByOpenstackName is a regression test for
// issue #2343: the k8s Node object backing a VjailbreakNode is named after its
// real OpenStack/k8s identity (Status.OpenstackName), which for agent nodes
// differs from the VjailbreakNode CR's own name. reconcileDelete must delete
// the Node by Status.OpenstackName, not by the CR name.
func TestReconcileDelete_DeletesK8sNodeByOpenstackName(t *testing.T) {
	ctx := context.Background()
	s := deleteTestScheme(t)

	const crName = "vjailbreak-agent-54cf0n"
	const realOpenstackName = "vjb-appliance-01-vjailbreak-agent-54cf0n"

	vjNode := &vjailbreakv1alpha1.VjailbreakNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      crName,
			Namespace: "migration-system",
		},
		Spec: vjailbreakv1alpha1.VjailbreakNodeSpec{
			NodeRole: "worker",
		},
		Status: vjailbreakv1alpha1.VjailbreakNodeStatus{
			OpenstackName: realOpenstackName,
		},
	}

	// The real k8s Node is named after the OpenStack identity, not the CR name.
	k8sNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: realOpenstackName,
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(vjNode, k8sNode).
		WithStatusSubresource(&vjailbreakv1alpha1.VjailbreakNode{}).
		Build()

	r := &VjailbreakNodeReconciler{Client: fakeClient, Scheme: s}
	nodeScope, err := scope.NewVjailbreakNodeScope(scope.VjailbreakNodeScopeParams{
		Client:         fakeClient,
		VjailbreakNode: vjNode,
	})
	if err != nil {
		t.Fatalf("NewVjailbreakNodeScope failed: %v", err)
	}

	if _, err := r.reconcileDelete(ctx, nodeScope); err != nil {
		t.Fatalf("reconcileDelete returned unexpected error: %v", err)
	}

	// The Node named after the real OpenStack identity must be gone.
	gotNode := &corev1.Node{}
	nodeErr := fakeClient.Get(ctx, types.NamespacedName{Name: realOpenstackName}, gotNode)
	if !errors.IsNotFound(nodeErr) {
		t.Errorf("expected Node %q to be deleted, get returned: %v", realOpenstackName, nodeErr)
	}

	// A lookup by the CR name must never have been attempted/matched - there is
	// no such Node, so this is really asserting reconcileDelete didn't silently
	// no-op against the wrong name and leave the real Node behind.
	if crName == realOpenstackName {
		t.Fatalf("test setup invalid: crName must differ from realOpenstackName")
	}
}

// TestReconcileDelete_MasterNodeSkipsCleanup ensures the master-node early
// return path is untouched by the OpenstackName fix.
func TestReconcileDelete_MasterNodeSkipsCleanup(t *testing.T) {
	ctx := context.Background()
	s := deleteTestScheme(t)

	vjNode := &vjailbreakv1alpha1.VjailbreakNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vjailbreak-master",
			Namespace: "migration-system",
		},
		Spec: vjailbreakv1alpha1.VjailbreakNodeSpec{
			NodeRole: "master",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(vjNode).
		WithStatusSubresource(&vjailbreakv1alpha1.VjailbreakNode{}).
		Build()

	r := &VjailbreakNodeReconciler{Client: fakeClient, Scheme: s}
	nodeScope, err := scope.NewVjailbreakNodeScope(scope.VjailbreakNodeScopeParams{
		Client:         fakeClient,
		VjailbreakNode: vjNode,
	})
	if err != nil {
		t.Fatalf("NewVjailbreakNodeScope failed: %v", err)
	}

	if _, err := r.reconcileDelete(ctx, nodeScope); err != nil {
		t.Fatalf("reconcileDelete returned unexpected error: %v", err)
	}
}
