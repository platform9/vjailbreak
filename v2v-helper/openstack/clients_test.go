package openstack

import (
	"context"
	"testing"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	ctrlfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func clientsTestScheme(t *testing.T) *k8sruntime.Scheme {
	t.Helper()
	s := k8sruntime.NewScheme()
	if err := vjailbreakv1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("AddToScheme failed: %v", err)
	}
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatalf("corev1.AddToScheme failed: %v", err)
	}
	return s
}

// TestGetInstanceUUIDForPod_AgentMatchesByOpenstackName is a regression test
// for issue #2343: an agent's k8s Node name is its Status.OpenstackName (e.g.
// "<master-name>-vjailbreak-agent-x9k2p1"), not its VjailbreakNode CR name
// (e.g. "vjailbreak-agent-x9k2p1"). The lookup must match on OpenstackName.
func TestGetInstanceUUIDForPod_AgentMatchesByOpenstackName(t *testing.T) {
	ctx := context.Background()
	s := clientsTestScheme(t)

	const podName = "agent-pod"
	const nodeName = "vjb-appliance-01-vjailbreak-agent-x9k2p1" // real k8s node name == OpenstackName
	const crName = "vjailbreak-agent-x9k2p1"                    // CR name != node name
	const wantUUID = "agent-uuid-1"

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: constants.NamespaceMigrationSystem},
		Spec:       corev1.PodSpec{NodeName: nodeName},
	}
	agentNode := &vjailbreakv1alpha1.VjailbreakNode{
		ObjectMeta: metav1.ObjectMeta{Name: crName, Namespace: constants.NamespaceMigrationSystem},
		Status: vjailbreakv1alpha1.VjailbreakNodeStatus{
			OpenstackName: nodeName,
			OpenstackUUID: wantUUID,
		},
	}
	// A VjailbreakNode whose CR name happens to equal nodeName but whose real
	// OpenstackName differs must NOT be matched - proves the lookup isn't
	// keying off .Name.
	decoyNode := &vjailbreakv1alpha1.VjailbreakNode{
		ObjectMeta: metav1.ObjectMeta{Name: nodeName, Namespace: constants.NamespaceMigrationSystem},
		Status: vjailbreakv1alpha1.VjailbreakNodeStatus{
			OpenstackName: "some-other-real-name",
			OpenstackUUID: "decoy-uuid",
		},
	}

	fakeClient := ctrlfake.NewClientBuilder().
		WithScheme(s).
		WithObjects(pod, agentNode, decoyNode).
		Build()

	got, err := getInstanceUUIDForPod(ctx, fakeClient, podName)
	if err != nil {
		t.Fatalf("getInstanceUUIDForPod returned unexpected error: %v", err)
	}
	if got != wantUUID {
		t.Errorf("getInstanceUUIDForPod() = %q, want %q", got, wantUUID)
	}
}

// TestGetInstanceUUIDForPod_MasterMatchesOwnOpenstackName covers the common
// case where the master's OS hostname (its k8s node name) mirrors its real
// OpenStack server name - the lookup must match the master's own record, not
// an unrelated agent's, even though neither name literally contains
// "vjailbreak-agent-" as a clean prefix/non-prefix signal.
func TestGetInstanceUUIDForPod_MasterMatchesOwnOpenstackName(t *testing.T) {
	ctx := context.Background()
	s := clientsTestScheme(t)

	const podName = "master-pod"
	const nodeName = "vjb-appliance-01"
	const wantUUID = "master-uuid-1"

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: constants.NamespaceMigrationSystem},
		Spec:       corev1.PodSpec{NodeName: nodeName},
	}
	masterNode := &vjailbreakv1alpha1.VjailbreakNode{
		ObjectMeta: metav1.ObjectMeta{Name: "vjailbreak-master", Namespace: constants.NamespaceMigrationSystem},
		Spec:       vjailbreakv1alpha1.VjailbreakNodeSpec{NodeRole: constants.NodeRoleMaster},
		Status: vjailbreakv1alpha1.VjailbreakNodeStatus{
			OpenstackName: nodeName,
			OpenstackUUID: wantUUID,
		},
	}
	// This agent's OpenstackName is "<master>-vjailbreak-agent-<cr>" and must
	// never be matched for a pod scheduled on the master's own node.
	agentNode := &vjailbreakv1alpha1.VjailbreakNode{
		ObjectMeta: metav1.ObjectMeta{Name: "vjailbreak-agent-x9k2p1", Namespace: constants.NamespaceMigrationSystem},
		Spec:       vjailbreakv1alpha1.VjailbreakNodeSpec{NodeRole: "worker"},
		Status: vjailbreakv1alpha1.VjailbreakNodeStatus{
			OpenstackName: "vjb-appliance-01-vjailbreak-agent-x9k2p1",
			OpenstackUUID: "agent-uuid-should-not-be-returned",
		},
	}

	fakeClient := ctrlfake.NewClientBuilder().
		WithScheme(s).
		WithObjects(pod, masterNode, agentNode).
		Build()

	got, err := getInstanceUUIDForPod(ctx, fakeClient, podName)
	if err != nil {
		t.Fatalf("getInstanceUUIDForPod returned unexpected error: %v", err)
	}
	if got != wantUUID {
		t.Errorf("getInstanceUUIDForPod() = %q, want %q", got, wantUUID)
	}
}

// TestGetInstanceUUIDForPod_MasterHostnameMismatchReturnsError documents the
// deliberate degraded-but-safe behavior when the master's OS hostname does
// not match its raw OpenStack server name (e.g. nova/cloud-init sanitized the
// hostname from a name containing spaces/uppercase). The function must return
// an error rather than a wrong UUID - GetCurrentInstanceUUID's caller then
// falls back to the OpenStack metadata service, which is always correct for
// the master.
func TestGetInstanceUUIDForPod_MasterHostnameMismatchReturnsError(t *testing.T) {
	ctx := context.Background()
	s := clientsTestScheme(t)

	const podName = "master-pod"
	const nodeName = "my-vjb-appliance" // sanitized hostname

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: constants.NamespaceMigrationSystem},
		Spec:       corev1.PodSpec{NodeName: nodeName},
	}
	masterNode := &vjailbreakv1alpha1.VjailbreakNode{
		ObjectMeta: metav1.ObjectMeta{Name: "vjailbreak-master", Namespace: constants.NamespaceMigrationSystem},
		Spec:       vjailbreakv1alpha1.VjailbreakNodeSpec{NodeRole: constants.NodeRoleMaster},
		Status: vjailbreakv1alpha1.VjailbreakNodeStatus{
			OpenstackName: "My VJB Appliance", // raw nova name, differs from hostname above
			OpenstackUUID: "master-uuid-1",
		},
	}

	fakeClient := ctrlfake.NewClientBuilder().
		WithScheme(s).
		WithObjects(pod, masterNode).
		Build()

	if _, err := getInstanceUUIDForPod(ctx, fakeClient, podName); err == nil {
		t.Error("expected an error when no VjailbreakNode's OpenstackName matches the node name")
	}
}

func TestGetInstanceUUIDForPod_AgentNotFound(t *testing.T) {
	ctx := context.Background()
	s := clientsTestScheme(t)

	const podName = "agent-pod"
	const nodeName = "vjb-appliance-01-vjailbreak-agent-missing"

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: constants.NamespaceMigrationSystem},
		Spec:       corev1.PodSpec{NodeName: nodeName},
	}

	fakeClient := ctrlfake.NewClientBuilder().
		WithScheme(s).
		WithObjects(pod).
		Build()

	if _, err := getInstanceUUIDForPod(ctx, fakeClient, podName); err == nil {
		t.Error("expected an error when no matching agent VjailbreakNode exists")
	}
}

func TestGetInstanceUUIDForPod_PodNotScheduled(t *testing.T) {
	ctx := context.Background()
	s := clientsTestScheme(t)

	const podName = "unscheduled-pod"
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: constants.NamespaceMigrationSystem},
		// NodeName intentionally empty - pod not yet scheduled.
	}

	fakeClient := ctrlfake.NewClientBuilder().
		WithScheme(s).
		WithObjects(pod).
		Build()

	if _, err := getInstanceUUIDForPod(ctx, fakeClient, podName); err == nil {
		t.Error("expected an error when pod has no node name assigned")
	}
}
