package utils

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// testSchemeWithCoreTypes returns a scheme with both standard k8s types and vjailbreak CRDs.
func testSchemeWithCoreTypes(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add clientgo scheme: %v", err)
	}
	if err := vjailbreakv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add vjailbreak scheme: %v", err)
	}
	return scheme
}

func makeMasterNode(name, ip string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				constants.K8sMasterNodeAnnotation: "",
			},
			Annotations: map[string]string{
				constants.InternalIPAnnotation: ip,
			},
		},
	}
}

func makeWorkerNode(name, ip string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				constants.InternalIPAnnotation: ip,
			},
		},
	}
}

func makeReadyNode(name string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}
}

// ─── Pure function tests ──────────────────────────────────────────────────────

func TestIsMasterNode(t *testing.T) {
	tests := []struct {
		name string
		node *corev1.Node
		want bool
	}{
		{
			name: "node with master label is master",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{constants.K8sMasterNodeAnnotation: ""},
				},
			},
			want: true,
		},
		{
			name: "node without master label is not master",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"some-other-label": "value"},
				},
			},
			want: false,
		},
		{
			name: "node with no labels is not master",
			node: &corev1.Node{ObjectMeta: metav1.ObjectMeta{}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsMasterNode(tt.node); got != tt.want {
				t.Errorf("IsMasterNode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetNodeInternalIP(t *testing.T) {
	tests := []struct {
		name string
		node *corev1.Node
		want string
	}{
		{
			name: "returns IP from annotation",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{constants.InternalIPAnnotation: "10.0.0.1"},
				},
			},
			want: "10.0.0.1",
		},
		{
			name: "returns empty string when annotation absent",
			node: &corev1.Node{ObjectMeta: metav1.ObjectMeta{}},
			want: "",
		},
		{
			name: "returns empty string when annotation has empty value",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{constants.InternalIPAnnotation: ""},
				},
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetNodeInternalIP(tt.node); got != tt.want {
				t.Errorf("GetNodeInternalIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsNodeReady(t *testing.T) {
	tests := []struct {
		name string
		node *corev1.Node
		want bool
	}{
		{
			name: "node with Ready=True is ready",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
					},
				},
			},
			want: true,
		},
		{
			name: "node with Ready=False is not ready",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
					},
				},
			},
			want: false,
		},
		{
			name: "node with Ready=Unknown is not ready",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionUnknown},
					},
				},
			},
			want: false,
		},
		{
			name: "node with no conditions is not ready",
			node: &corev1.Node{},
			want: false,
		},
		{
			name: "node with other conditions but no Ready is not ready",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionFalse},
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNodeReady(tt.node); got != tt.want {
				t.Errorf("IsNodeReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ─── buildHostsWriteFilesEntry ───────────────────────────────────────────────

func TestBuildHostsWriteFilesEntry(t *testing.T) {
	tests := []struct {
		name         string
		hostsContent string
		wantEmpty    bool
		wantContains []string
		wantAbsent   []string
	}{
		{
			name:      "empty file returns empty string",
			wantEmpty: true,
		},
		{
			name: "only standard loopback entries returns empty string",
			hostsContent: `# /etc/hosts
127.0.0.1 localhost
127.0.1.1 vjailbreak
::1 localhost ip6-localhost ip6-loopback
fe00::0 ip6-localnet
ff00::0 ip6-mcastprefix
ff02::1 ip6-allnodes
ff02::2 ip6-allrouters
`,
			wantEmpty: true,
		},
		{
			name: "custom entries are included",
			hostsContent: `127.0.0.1 localhost
192.168.1.100 vcenter.example.com
192.168.1.101 esxi01.example.com
`,
			wantContains: []string{"vcenter.example.com", "esxi01.example.com", "/etc/hosts", "append: true"},
			wantAbsent:   []string{"127.0.0.1"},
		},
		{
			name: "comment lines are excluded",
			hostsContent: `# This is a comment
# Another comment
10.0.0.5 openstack.internal
`,
			wantContains: []string{"openstack.internal"},
			wantAbsent:   []string{"# This is a comment", "# Another comment"},
		},
		{
			name: "mixed standard and custom entries",
			hostsContent: `127.0.0.1 localhost
::1 localhost
192.168.50.10 vcenter.corp.local
10.10.10.10 esxi-host1.corp.local
10.10.10.11 esxi-host2.corp.local
`,
			wantContains: []string{"vcenter.corp.local", "esxi-host1.corp.local", "esxi-host2.corp.local"},
			wantAbsent:   []string{"127.0.0.1", "::1"},
		},
		{
			name: "blank lines are ignored",
			hostsContent: `
127.0.0.1 localhost

10.0.0.1 custom-host.example.com

`,
			wantContains: []string{"custom-host.example.com"},
		},
		{
			name: "output is valid cloud-init write_files YAML fragment",
			hostsContent: `192.168.1.1 myhost.local
`,
			wantContains: []string{"- path: /etc/hosts", "append: true", "content: |", "    192.168.1.1 myhost.local"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildHostsWriteFilesEntry([]byte(tt.hostsContent))
			if tt.wantEmpty {
				if got != "" {
					t.Errorf("buildHostsWriteFilesEntry() = %q, want empty", got)
				}
				return
			}
			if got == "" {
				t.Fatalf("buildHostsWriteFilesEntry() returned empty, expected content")
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("output missing %q\ngot:\n%s", want, got)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("output should not contain %q\ngot:\n%s", absent, got)
				}
			}
		})
	}
}

// ─── K8s fake-client tests ────────────────────────────────────────────────────

func TestGetAllk8sNodes(t *testing.T) {
	ctx := context.Background()
	scheme := testSchemeWithCoreTypes(t)

	t.Run("returns all nodes", func(t *testing.T) {
		master := makeMasterNode("master", "10.0.0.1")
		worker := makeWorkerNode("worker-1", "10.0.0.2")
		k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(master, worker).Build()

		nodes, err := GetAllk8sNodes(ctx, k8sClient)
		if err != nil {
			t.Fatalf("GetAllk8sNodes() error = %v", err)
		}
		if len(nodes.Items) != 2 {
			t.Errorf("GetAllk8sNodes() returned %d nodes, want 2", len(nodes.Items))
		}
	})

	t.Run("returns empty list when no nodes", func(t *testing.T) {
		k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		nodes, err := GetAllk8sNodes(ctx, k8sClient)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(nodes.Items) != 0 {
			t.Errorf("expected 0 nodes, got %d", len(nodes.Items))
		}
	})
}

func TestGetMasterK8sNode(t *testing.T) {
	ctx := context.Background()
	scheme := testSchemeWithCoreTypes(t)

	t.Run("finds master node among mixed nodes", func(t *testing.T) {
		master := makeMasterNode("master-node", "10.0.0.1")
		worker := makeWorkerNode("worker-1", "10.0.0.2")
		k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(master, worker).Build()

		node, err := GetMasterK8sNode(ctx, k8sClient)
		if err != nil {
			t.Fatalf("GetMasterK8sNode() error = %v", err)
		}
		if node.Name != "master-node" {
			t.Errorf("GetMasterK8sNode() name = %q, want %q", node.Name, "master-node")
		}
	})

	t.Run("returns error when only worker nodes exist", func(t *testing.T) {
		worker := makeWorkerNode("worker-1", "10.0.0.2")
		k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(worker).Build()

		_, err := GetMasterK8sNode(ctx, k8sClient)
		if err == nil {
			t.Error("GetMasterK8sNode() expected error, got nil")
		}
	})

	t.Run("returns error when cluster is empty", func(t *testing.T) {
		k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		_, err := GetMasterK8sNode(ctx, k8sClient)
		if err == nil {
			t.Error("GetMasterK8sNode() expected error, got nil")
		}
	})
}

func TestGetOpenstackCredsVjailbreakNode(t *testing.T) {
	ctx := context.Background()
	scheme := testSchemeWithCoreTypes(t)

	namedCreds := &vjailbreakv1alpha1.OpenstackCreds{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-creds",
			Namespace: constants.NamespaceMigrationSystem,
		},
	}
	fallbackCreds := &vjailbreakv1alpha1.OpenstackCreds{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fallback-creds",
			Namespace: constants.NamespaceMigrationSystem,
		},
	}

	t.Run("returns creds by name when found", func(t *testing.T) {
		vjNode := &vjailbreakv1alpha1.VjailbreakNode{
			ObjectMeta: metav1.ObjectMeta{Name: "test-node", Namespace: constants.NamespaceMigrationSystem},
			Spec:       vjailbreakv1alpha1.VjailbreakNodeSpec{OpenstackCreds: corev1.ObjectReference{Name: "my-creds"}},
		}
		k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(namedCreds, vjNode).Build()

		creds, err := GetOpenstackCredsVjailbreakNode(ctx, k8sClient, vjNode)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if creds.Name != "my-creds" {
			t.Errorf("got creds %q, want %q", creds.Name, "my-creds")
		}
	})

	t.Run("falls back to first available creds when named creds not found", func(t *testing.T) {
		vjNode := &vjailbreakv1alpha1.VjailbreakNode{
			ObjectMeta: metav1.ObjectMeta{Name: "test-node", Namespace: constants.NamespaceMigrationSystem},
			Spec:       vjailbreakv1alpha1.VjailbreakNodeSpec{OpenstackCreds: corev1.ObjectReference{Name: "nonexistent"}},
		}
		k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(fallbackCreds, vjNode).Build()

		creds, err := GetOpenstackCredsVjailbreakNode(ctx, k8sClient, vjNode)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if creds.Name != "fallback-creds" {
			t.Errorf("got creds %q, want %q", creds.Name, "fallback-creds")
		}
	})

	t.Run("returns error when no creds exist at all", func(t *testing.T) {
		vjNode := &vjailbreakv1alpha1.VjailbreakNode{
			ObjectMeta: metav1.ObjectMeta{Name: "test-node", Namespace: constants.NamespaceMigrationSystem},
			Spec:       vjailbreakv1alpha1.VjailbreakNodeSpec{OpenstackCreds: corev1.ObjectReference{Name: "nonexistent"}},
		}
		k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(vjNode).Build()

		_, err := GetOpenstackCredsVjailbreakNode(ctx, k8sClient, vjNode)
		if err == nil {
			t.Error("expected error when no creds found, got nil")
		}
	})
}

func TestGetImageID(t *testing.T) {
	ctx := context.Background()
	scheme := testSchemeWithCoreTypes(t)

	t.Run("returns image ID from master vjailbreaknode", func(t *testing.T) {
		masterVjNode := &vjailbreakv1alpha1.VjailbreakNode{
			ObjectMeta: metav1.ObjectMeta{
				Name:      constants.VjailbreakMasterNodeName,
				Namespace: constants.NamespaceMigrationSystem,
			},
			Spec: vjailbreakv1alpha1.VjailbreakNodeSpec{
				OpenstackImageID: "test-image-abc123",
			},
		}
		k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(masterVjNode).Build()

		imageID, err := GetImageID(ctx, k8sClient)
		if err != nil {
			t.Fatalf("GetImageID() error = %v", err)
		}
		if imageID != "test-image-abc123" {
			t.Errorf("GetImageID() = %q, want %q", imageID, "test-image-abc123")
		}
	})

	t.Run("returns empty string when master has no image ID", func(t *testing.T) {
		masterVjNode := &vjailbreakv1alpha1.VjailbreakNode{
			ObjectMeta: metav1.ObjectMeta{
				Name:      constants.VjailbreakMasterNodeName,
				Namespace: constants.NamespaceMigrationSystem,
			},
		}
		k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(masterVjNode).Build()

		imageID, err := GetImageID(ctx, k8sClient)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if imageID != "" {
			t.Errorf("GetImageID() = %q, want empty string", imageID)
		}
	})

	t.Run("returns error when master vjailbreaknode not found", func(t *testing.T) {
		k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		_, err := GetImageID(ctx, k8sClient)
		if err == nil {
			t.Error("GetImageID() expected error, got nil")
		}
	})
}

func TestGetActiveMigrations(t *testing.T) {
	ctx := context.Background()
	scheme := testSchemeWithCoreTypes(t)

	makeM := func(name, agent string, phase vjailbreakv1alpha1.VMMigrationPhase) *vjailbreakv1alpha1.Migration {
		return &vjailbreakv1alpha1.Migration{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: constants.NamespaceMigrationSystem,
			},
			Status: vjailbreakv1alpha1.MigrationStatus{
				AgentName: agent,
				Phase:     phase,
			},
		}
	}

	tests := []struct {
		name       string
		nodeName   string
		migrations []*vjailbreakv1alpha1.Migration
		wantCount  int
		wantNames  []string
	}{
		{
			name:     "returns only active migrations for the given node",
			nodeName: "agent-1",
			migrations: []*vjailbreakv1alpha1.Migration{
				makeM("mig-active", "agent-1", vjailbreakv1alpha1.VMMigrationPhaseCopying),
				makeM("mig-done", "agent-1", vjailbreakv1alpha1.VMMigrationPhaseSucceeded),
				makeM("mig-failed", "agent-1", vjailbreakv1alpha1.VMMigrationPhaseFailed),
				makeM("mig-other-node", "agent-2", vjailbreakv1alpha1.VMMigrationPhaseCopying),
			},
			wantCount: 1,
			wantNames: []string{"mig-active"},
		},
		{
			name:     "pending and unknown phases are excluded",
			nodeName: "agent-1",
			migrations: []*vjailbreakv1alpha1.Migration{
				makeM("mig-1", "agent-1", vjailbreakv1alpha1.VMMigrationPhasePending),
				makeM("mig-2", "agent-1", vjailbreakv1alpha1.VMMigrationPhaseUnknown),
			},
			wantCount: 0,
		},
		{
			name:       "returns empty when no migrations exist",
			nodeName:   "agent-1",
			migrations: []*vjailbreakv1alpha1.Migration{},
			wantCount:  0,
		},
		{
			name:     "converting disk phase counts as active",
			nodeName: "agent-1",
			migrations: []*vjailbreakv1alpha1.Migration{
				makeM("mig-converting", "agent-1", vjailbreakv1alpha1.VMMigrationPhaseConvertingDisk),
			},
			wantCount: 1,
			wantNames: []string{"mig-converting"},
		},
		{
			name:     "multiple active migrations on same node all returned",
			nodeName: "agent-1",
			migrations: []*vjailbreakv1alpha1.Migration{
				makeM("mig-1", "agent-1", vjailbreakv1alpha1.VMMigrationPhaseCopying),
				makeM("mig-2", "agent-1", vjailbreakv1alpha1.VMMigrationPhaseConvertingDisk),
			},
			wantCount: 2,
			wantNames: []string{"mig-1", "mig-2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtimeObjs := make([]runtime.Object, len(tt.migrations))
			for i, m := range tt.migrations {
				runtimeObjs[i] = m
			}
			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(runtimeObjs...).
				Build()

			active, err := GetActiveMigrations(ctx, tt.nodeName, k8sClient)
			if err != nil {
				t.Fatalf("GetActiveMigrations() error = %v", err)
			}
			if len(active) != tt.wantCount {
				t.Errorf("GetActiveMigrations() count = %d, want %d (got %v)", len(active), tt.wantCount, active)
			}
			for _, wantName := range tt.wantNames {
				found := false
				for _, gotName := range active {
					if gotName == wantName {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("GetActiveMigrations() missing %q in result %v", wantName, active)
				}
			}
		})
	}
}

func TestGetNodeByName(t *testing.T) {
	ctx := context.Background()
	scheme := testSchemeWithCoreTypes(t)

	t.Run("returns node when found", func(t *testing.T) {
		node := makeWorkerNode("worker-1", "10.0.0.5")
		k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node).Build()

		got, err := GetNodeByName(ctx, k8sClient, "worker-1")
		if err != nil {
			t.Fatalf("GetNodeByName() error = %v", err)
		}
		if got.Name != "worker-1" {
			t.Errorf("GetNodeByName() name = %q, want %q", got.Name, "worker-1")
		}
	})

	t.Run("returns error when node not found", func(t *testing.T) {
		k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		_, err := GetNodeByName(ctx, k8sClient, "nonexistent")
		if err == nil {
			t.Error("GetNodeByName() expected error, got nil")
		}
	})
}

func TestDeleteNodeByName(t *testing.T) {
	ctx := context.Background()
	scheme := testSchemeWithCoreTypes(t)

	t.Run("deletes existing node", func(t *testing.T) {
		node := makeWorkerNode("worker-1", "10.0.0.5")
		k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node).Build()

		if err := DeleteNodeByName(ctx, k8sClient, "worker-1"); err != nil {
			t.Fatalf("DeleteNodeByName() error = %v", err)
		}
		_, err := GetNodeByName(ctx, k8sClient, "worker-1")
		if err == nil {
			t.Error("node still exists after DeleteNodeByName()")
		}
	})

	t.Run("no error when node does not exist", func(t *testing.T) {
		k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		if err := DeleteNodeByName(ctx, k8sClient, "nonexistent"); err != nil {
			t.Errorf("DeleteNodeByName() unexpected error for missing node: %v", err)
		}
	})
}

func TestReconcileK8sNodeStatus(t *testing.T) {
	ctx := context.Background()
	scheme := testSchemeWithCoreTypes(t)

	makeVjNode := func(name string) *vjailbreakv1alpha1.VjailbreakNode {
		return &vjailbreakv1alpha1.VjailbreakNode{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: constants.NamespaceMigrationSystem,
			},
		}
	}

	t.Run("returns true and sets NodeReady phase when node is ready", func(t *testing.T) {
		vjNode := makeVjNode("agent-1")
		k8sNode := makeReadyNode("agent-1")
		k8sClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(k8sNode, vjNode).
			WithStatusSubresource(&vjailbreakv1alpha1.VjailbreakNode{}).
			Build()

		ready, err := ReconcileK8sNodeStatus(ctx, k8sClient, vjNode)
		if err != nil {
			t.Fatalf("ReconcileK8sNodeStatus() error = %v", err)
		}
		if !ready {
			t.Error("ReconcileK8sNodeStatus() = false, want true for ready node")
		}
		if vjNode.Status.Phase != constants.VjailbreakNodePhaseNodeReady {
			t.Errorf("phase = %q, want %q", vjNode.Status.Phase, constants.VjailbreakNodePhaseNodeReady)
		}
	})

	t.Run("returns false and sets VMCreated phase when node exists but not ready", func(t *testing.T) {
		vjNode := makeVjNode("agent-2")
		k8sNode := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "agent-2"},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{
					{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
				},
			},
		}
		k8sClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(k8sNode, vjNode).
			WithStatusSubresource(&vjailbreakv1alpha1.VjailbreakNode{}).
			Build()

		ready, err := ReconcileK8sNodeStatus(ctx, k8sClient, vjNode)
		if err != nil {
			t.Fatalf("ReconcileK8sNodeStatus() error = %v", err)
		}
		if ready {
			t.Error("ReconcileK8sNodeStatus() = true, want false for not-ready node")
		}
		if vjNode.Status.Phase != constants.VjailbreakNodePhaseVMCreated {
			t.Errorf("phase = %q, want %q", vjNode.Status.Phase, constants.VjailbreakNodePhaseVMCreated)
		}
	})

	t.Run("returns false when k8s node not yet joined", func(t *testing.T) {
		vjNode := makeVjNode("agent-3")
		k8sClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(vjNode).
			WithStatusSubresource(&vjailbreakv1alpha1.VjailbreakNode{}).
			Build()

		ready, err := ReconcileK8sNodeStatus(ctx, k8sClient, vjNode)
		if err != nil {
			t.Fatalf("ReconcileK8sNodeStatus() error = %v", err)
		}
		if ready {
			t.Error("ReconcileK8sNodeStatus() = true, want false when node not joined")
		}
	})

	t.Run("populates VMIP from k8s node annotation when not already set", func(t *testing.T) {
		vjNode := makeVjNode("agent-4")
		k8sNode := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "agent-4",
				Annotations: map[string]string{constants.InternalIPAnnotation: "192.168.1.50"},
			},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{
					{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
				},
			},
		}
		k8sClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(k8sNode, vjNode).
			WithStatusSubresource(&vjailbreakv1alpha1.VjailbreakNode{}).
			Build()

		_, err := ReconcileK8sNodeStatus(ctx, k8sClient, vjNode)
		if err != nil {
			t.Fatalf("ReconcileK8sNodeStatus() error = %v", err)
		}
		if vjNode.Status.VMIP != "192.168.1.50" {
			t.Errorf("VMIP = %q, want %q", vjNode.Status.VMIP, "192.168.1.50")
		}
	})
}

func TestCheckAndCreateMasterNodeEntry(t *testing.T) {
	ctx := context.Background()
	scheme := testSchemeWithCoreTypes(t)

	masterNN := types.NamespacedName{Namespace: constants.NamespaceMigrationSystem, Name: constants.VjailbreakMasterNodeName}

	t.Run("creates master vjailbreaknode when it does not exist", func(t *testing.T) {
		master := makeMasterNode("master", "10.0.0.1")
		k8sClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(master).
			WithStatusSubresource(&vjailbreakv1alpha1.VjailbreakNode{}).
			Build()

		if err := CheckAndCreateMasterNodeEntry(ctx, k8sClient, true, "test-uuid"); err != nil {
			t.Fatalf("CheckAndCreateMasterNodeEntry() error = %v", err)
		}

		vjNode := &vjailbreakv1alpha1.VjailbreakNode{}
		if err := k8sClient.Get(ctx, masterNN, vjNode); err != nil {
			t.Fatalf("VjailbreakNode not created: %v", err)
		}
		if vjNode.Status.OpenstackUUID != "test-uuid" {
			t.Errorf("OpenstackUUID = %q, want %q", vjNode.Status.OpenstackUUID, "test-uuid")
		}
	})

	t.Run("returns nil immediately when master already has OpenstackUUID", func(t *testing.T) {
		master := makeMasterNode("master", "10.0.0.1")
		existingVjNode := &vjailbreakv1alpha1.VjailbreakNode{
			ObjectMeta: metav1.ObjectMeta{
				Name:      constants.VjailbreakMasterNodeName,
				Namespace: constants.NamespaceMigrationSystem,
			},
			Status: vjailbreakv1alpha1.VjailbreakNodeStatus{
				OpenstackUUID: "already-set-uuid",
			},
		}
		k8sClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(master, existingVjNode).
			WithStatusSubresource(&vjailbreakv1alpha1.VjailbreakNode{}).
			Build()

		if err := CheckAndCreateMasterNodeEntry(ctx, k8sClient, true, "new-uuid"); err != nil {
			t.Fatalf("CheckAndCreateMasterNodeEntry() error = %v", err)
		}
	})

	t.Run("uses fake-openstackuuid in local mode when no uuid provided", func(t *testing.T) {
		master := makeMasterNode("master", "10.0.0.1")
		k8sClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(master).
			WithStatusSubresource(&vjailbreakv1alpha1.VjailbreakNode{}).
			Build()

		if err := CheckAndCreateMasterNodeEntry(ctx, k8sClient, true, ""); err != nil {
			t.Fatalf("CheckAndCreateMasterNodeEntry() error = %v", err)
		}

		vjNode := &vjailbreakv1alpha1.VjailbreakNode{}
		if err := k8sClient.Get(ctx, masterNN, vjNode); err != nil {
			t.Fatalf("VjailbreakNode not found: %v", err)
		}
		if vjNode.Status.OpenstackUUID != "fake-openstackuuid" {
			t.Errorf("OpenstackUUID = %q, want %q", vjNode.Status.OpenstackUUID, "fake-openstackuuid")
		}
	})
}

// ─── ReadFileContent ──────────────────────────────────────────────────────────

func TestReadFileContent(t *testing.T) {
	t.Run("reads existing file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.txt")
		want := []byte("hello world\n")
		if err := os.WriteFile(path, want, 0o644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		got, err := ReadFileContent(path)
		if err != nil {
			t.Fatalf("ReadFileContent() error = %v", err)
		}
		if string(got) != string(want) {
			t.Errorf("ReadFileContent() = %q, want %q", got, want)
		}
	})

	t.Run("returns error for non-existent file", func(t *testing.T) {
		_, err := ReadFileContent("/nonexistent/path/file.txt")
		if err == nil {
			t.Error("ReadFileContent() expected error for missing file, got nil")
		}
	})

	t.Run("returns error for relative path", func(t *testing.T) {
		_, err := ReadFileContent("relative/path/file.txt")
		if err == nil {
			t.Error("ReadFileContent() expected error for relative path, got nil")
		}
	})

	t.Run("reads empty file without error", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "empty.txt")
		if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
			t.Fatalf("failed to create empty file: %v", err)
		}

		got, err := ReadFileContent(path)
		if err != nil {
			t.Fatalf("ReadFileContent() error = %v", err)
		}
		if len(got) != 0 {
			t.Errorf("ReadFileContent() returned %d bytes for empty file", len(got))
		}
	})
}
