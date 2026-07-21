package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func buildDeletableHandlerScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(s))
	utilruntime.Must(vjailbreakv1alpha1.AddToScheme(s))
	return s
}

func makeVjailbreakNode(name, namespace, role, openstackCredsName string, activeMigrations []string) *vjailbreakv1alpha1.VjailbreakNode {
	return &vjailbreakv1alpha1.VjailbreakNode{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: vjailbreakv1alpha1.VjailbreakNodeSpec{
			NodeRole: role,
			OpenstackCreds: corev1.ObjectReference{
				Name:      openstackCredsName,
				Namespace: namespace,
			},
			OpenstackFlavorID: "flavor-1",
			OpenstackImageID:  "image-1",
		},
		Status: vjailbreakv1alpha1.VjailbreakNodeStatus{
			ActiveMigrations: activeMigrations,
		},
	}
}

// handleDeletableWithClient runs HandleCheckOpenstackCredsDeletable with a fake k8s client
// containing the given objects. It swaps openstackCredsDeletableClientFunc for the duration.
func handleDeletableWithClient(t *testing.T, req *http.Request, objs ...runtime.Object) *httptest.ResponseRecorder {
	t.Helper()
	scheme := buildDeletableHandlerScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()

	orig := openstackCredsDeletableClientFunc
	openstackCredsDeletableClientFunc = func() (ctrlclient.Client, error) { return fakeClient, nil }
	defer func() { openstackCredsDeletableClientFunc = orig }()

	w := httptest.NewRecorder()
	HandleCheckOpenstackCredsDeletable(w, req)
	return w
}

func TestHandleCheckOpenstackCredsDeletable_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/vpw/v1/openstackcreds-deletable?name=cred1", nil)
	w := httptest.NewRecorder()
	HandleCheckOpenstackCredsDeletable(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleCheckOpenstackCredsDeletable_MissingName(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/vpw/v1/openstackcreds-deletable", nil)
	w := httptest.NewRecorder()
	HandleCheckOpenstackCredsDeletable(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleCheckOpenstackCredsDeletable_NoNodes(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet,
		"/vpw/v1/openstackcreds-deletable?name=my-creds&namespace=migration-system", nil)
	w := handleDeletableWithClient(t, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp OpenstackCredsDeletableResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.CanDelete {
		t.Error("expected canDelete=true when no nodes exist")
	}
	if resp.AgentNodeCount != 0 {
		t.Errorf("expected agentNodeCount=0, got %d", resp.AgentNodeCount)
	}
	if len(resp.AgentNodeNames) != 0 {
		t.Errorf("expected empty agentNodeNames, got %v", resp.AgentNodeNames)
	}
}

func TestHandleCheckOpenstackCredsDeletable_MasterNodeOnly(t *testing.T) {
	masterNode := makeVjailbreakNode("vjailbreak-master", "migration-system",
		constants.NodeRoleMaster, "my-creds", nil)
	req := httptest.NewRequest(http.MethodGet,
		"/vpw/v1/openstackcreds-deletable?name=my-creds&namespace=migration-system", nil)
	w := handleDeletableWithClient(t, req, masterNode)

	var resp OpenstackCredsDeletableResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if !resp.CanDelete {
		t.Error("master node should not block deletion")
	}
	if resp.AgentNodeCount != 0 {
		t.Errorf("master node should not be counted, got %d", resp.AgentNodeCount)
	}
}

func TestHandleCheckOpenstackCredsDeletable_WithAgentNodes(t *testing.T) {
	ns := "migration-system"
	credName := "my-creds"

	tests := []struct {
		name                    string
		nodes                   []*vjailbreakv1alpha1.VjailbreakNode
		wantCanDelete           bool
		wantAgentCount          int
		wantHasActiveMigrations bool
	}{
		{
			name: "one agent node, no active migrations",
			nodes: []*vjailbreakv1alpha1.VjailbreakNode{
				makeVjailbreakNode("agent-1", ns, "worker", credName, nil),
			},
			wantCanDelete:           false,
			wantAgentCount:          1,
			wantHasActiveMigrations: false,
		},
		{
			name: "two agent nodes, one with active migration",
			nodes: []*vjailbreakv1alpha1.VjailbreakNode{
				makeVjailbreakNode("agent-1", ns, "worker", credName, nil),
				makeVjailbreakNode("agent-2", ns, "worker", credName, []string{"migration-plan-1"}),
			},
			wantCanDelete:           false,
			wantAgentCount:          2,
			wantHasActiveMigrations: true,
		},
		{
			name: "agent node references different creds — should not count",
			nodes: []*vjailbreakv1alpha1.VjailbreakNode{
				makeVjailbreakNode("agent-1", ns, "worker", "other-creds", nil),
			},
			wantCanDelete:           true,
			wantAgentCount:          0,
			wantHasActiveMigrations: false,
		},
		{
			name: "master + agent nodes mixed",
			nodes: []*vjailbreakv1alpha1.VjailbreakNode{
				makeVjailbreakNode("master", ns, constants.NodeRoleMaster, credName, nil),
				makeVjailbreakNode("agent-1", ns, "worker", credName, nil),
			},
			wantCanDelete:           false,
			wantAgentCount:          1,
			wantHasActiveMigrations: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs := make([]runtime.Object, len(tt.nodes))
			for i, n := range tt.nodes {
				objs[i] = n
			}
			req := httptest.NewRequest(http.MethodGet,
				"/vpw/v1/openstackcreds-deletable?name="+credName+"&namespace="+ns, nil)
			w := handleDeletableWithClient(t, req, objs...)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
			}
			var resp OpenstackCredsDeletableResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode: %v", err)
			}
			if resp.CanDelete != tt.wantCanDelete {
				t.Errorf("canDelete: got %v, want %v", resp.CanDelete, tt.wantCanDelete)
			}
			if resp.AgentNodeCount != tt.wantAgentCount {
				t.Errorf("agentNodeCount: got %d, want %d", resp.AgentNodeCount, tt.wantAgentCount)
			}
			if resp.HasActiveMigrations != tt.wantHasActiveMigrations {
				t.Errorf("hasActiveMigrations: got %v, want %v", resp.HasActiveMigrations, tt.wantHasActiveMigrations)
			}
		})
	}
}

func TestHandleCheckOpenstackCredsDeletable_DefaultNamespace(t *testing.T) {
	ns := constants.NamespaceMigrationSystem
	node := makeVjailbreakNode("agent-1", ns, "worker", "my-creds", nil)
	req := httptest.NewRequest(http.MethodGet,
		"/vpw/v1/openstackcreds-deletable?name=my-creds", nil)
	w := handleDeletableWithClient(t, req, node)

	var resp OpenstackCredsDeletableResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp.AgentNodeCount != 1 {
		t.Errorf("expected 1 agent node with default namespace, got %d", resp.AgentNodeCount)
	}
}
