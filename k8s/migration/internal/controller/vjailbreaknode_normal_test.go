package controller

import (
	"context"
	"testing"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/scope"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func normalTestScheme(t *testing.T) *runtime.Scheme {
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

// TestReconcileNormal_SetsOpenstackNameBeforeLookup is a regression test for
// the VMCreated-without-VM bug: when a new agent's Status.OpenstackName is
// empty, reconcileNormal must compute and persist it BEFORE calling
// GetOpenstackVMByName. An empty name sent to Nova returns all servers, causing
// the first server's UUID to be mistakenly treated as "VM exists", which skips
// creation and advances the phase to VMCreated with a wrong UUID.
func TestReconcileNormal_SetsOpenstackNameBeforeLookup(t *testing.T) {
	ctx := context.Background()
	s := normalTestScheme(t)

	const agentCRName = "vjailbreak-agent-x9k2p1"
	const masterOpenstackName = "vjb-appliance-01"
	const wantOpenstackName = "vjb-appliance-01-vjailbreak-agent-x9k2p1"

	masterVjNode := &vjailbreakv1alpha1.VjailbreakNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.VjailbreakMasterNodeName,
			Namespace: constants.NamespaceMigrationSystem,
		},
		Spec: vjailbreakv1alpha1.VjailbreakNodeSpec{NodeRole: constants.NodeRoleMaster},
		Status: vjailbreakv1alpha1.VjailbreakNodeStatus{
			OpenstackName: masterOpenstackName,
		},
	}

	agentVjNode := &vjailbreakv1alpha1.VjailbreakNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      agentCRName,
			Namespace: constants.NamespaceMigrationSystem,
		},
		Spec: vjailbreakv1alpha1.VjailbreakNodeSpec{NodeRole: "worker"},
		// Status.OpenstackName intentionally empty — simulates first reconcile.
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(masterVjNode, agentVjNode).
		WithStatusSubresource(&vjailbreakv1alpha1.VjailbreakNode{}).
		Build()

	// Seed the master status (WithObjects can't set status via status subresource).
	masterVjNode.Status.OpenstackName = masterOpenstackName
	if err := fakeClient.Status().Update(ctx, masterVjNode); err != nil {
		t.Fatalf("Status().Update master: %v", err)
	}

	r := &VjailbreakNodeReconciler{Client: fakeClient, Scheme: s}
	nodeScope, err := scope.NewVjailbreakNodeScope(scope.VjailbreakNodeScopeParams{
		Client:         fakeClient,
		VjailbreakNode: agentVjNode,
	})
	if err != nil {
		t.Fatalf("NewVjailbreakNodeScope: %v", err)
	}

	// reconcileNormal will fail later (missing OpenStack creds in test), but
	// Status.OpenstackName must be persisted before that point.
	_, _ = r.reconcileNormal(ctx, nodeScope)

	got := &vjailbreakv1alpha1.VjailbreakNode{}
	if err := fakeClient.Get(ctx, types.NamespacedName{
		Name:      agentCRName,
		Namespace: constants.NamespaceMigrationSystem,
	}, got); err != nil {
		t.Fatalf("Get agent VjailbreakNode: %v", err)
	}

	if got.Status.OpenstackName != wantOpenstackName {
		t.Errorf("Status.OpenstackName = %q, want %q (empty name would match all nova servers)",
			got.Status.OpenstackName, wantOpenstackName)
	}
}
