package utils

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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

func TestBuildSchedulerHints(t *testing.T) {
	tests := []struct {
		name          string
		serverGroupID string
		wantNil       bool
		wantGroup     string
	}{
		{
			name:          "empty string returns nil",
			serverGroupID: "",
			wantNil:       true,
		},
		{
			name:          "non-empty ID returns SchedulerHintOpts with Group set",
			serverGroupID: "sg-abc123",
			wantNil:       false,
			wantGroup:     "sg-abc123",
		},
		{
			name:          "UUID-style ID is preserved",
			serverGroupID: "550e8400-e29b-41d4-a716-446655440000",
			wantNil:       false,
			wantGroup:     "550e8400-e29b-41d4-a716-446655440000",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildSchedulerHints(tt.serverGroupID)
			if tt.wantNil {
				if got != nil {
					t.Errorf("buildSchedulerHints(%q) = %v, want nil", tt.serverGroupID, got)
				}
				return
			}
			hints, ok := got.(servers.SchedulerHintOpts)
			if !ok {
				t.Fatalf("buildSchedulerHints(%q) returned %T, want servers.SchedulerHintOpts", tt.serverGroupID, got)
			}
			if hints.Group != tt.wantGroup {
				t.Errorf("SchedulerHintOpts.Group = %q, want %q", hints.Group, tt.wantGroup)
			}
		})
	}
}

func TestVjailbreakNodeServerGroupField(t *testing.T) {
	ctx := context.Background()
	s := testNodeScheme(t)

	tests := []struct {
		name        string
		serverGroup string
	}{
		{name: "empty server group", serverGroup: ""},
		{name: "server group set", serverGroup: "sg-anti-affinity-123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &vjailbreakv1alpha1.VjailbreakNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent-" + tt.name,
					Namespace: constants.NamespaceMigrationSystem,
				},
				Spec: vjailbreakv1alpha1.VjailbreakNodeSpec{
					NodeRole:             "worker",
					OpenstackServerGroup: tt.serverGroup,
				},
			}
			fakeClient := fake.NewClientBuilder().WithScheme(s).Build()

			if err := fakeClient.Create(ctx, node); err != nil {
				t.Fatalf("Create VjailbreakNode: %v", err)
			}

			got := &vjailbreakv1alpha1.VjailbreakNode{}
			if err := fakeClient.Get(ctx, types.NamespacedName{
				Name:      node.Name,
				Namespace: node.Namespace,
			}, got); err != nil {
				t.Fatalf("Get VjailbreakNode: %v", err)
			}

			if got.Spec.OpenstackServerGroup != tt.serverGroup {
				t.Errorf("OpenstackServerGroup = %q, want %q", got.Spec.OpenstackServerGroup, tt.serverGroup)
			}
		})
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

func TestComputeAgentInstanceName(t *testing.T) {
	tests := []struct {
		name                string
		masterOpenstackName string
		agentCRName         string
		want                string
	}{
		{
			name:                "empty masterOpenstackName returns agentCRName as-is",
			masterOpenstackName: "",
			agentCRName:         "vjailbreak-agent-x9k2p1",
			want:                "vjailbreak-agent-x9k2p1",
		},
		{
			name:                "non-empty masterOpenstackName returns joined name",
			masterOpenstackName: "vjb-appliance-01",
			agentCRName:         "vjailbreak-agent-x9k2p1",
			want:                "vjb-appliance-01-vjailbreak-agent-x9k2p1",
		},
		{
			name:                "both empty returns empty string",
			masterOpenstackName: "",
			agentCRName:         "",
			want:                "",
		},
		{
			name:                "masterOpenstackName is sanitized before joining",
			masterOpenstackName: "VJB_Master 01",
			agentCRName:         "vjailbreak-agent-x9k2p1",
			want:                "vjb-master-01-vjailbreak-agent-x9k2p1",
		},
		{
			name:                "masterOpenstackName that sanitizes to empty falls back to agentCRName",
			masterOpenstackName: "___",
			agentCRName:         "vjailbreak-agent-x9k2p1",
			want:                "vjailbreak-agent-x9k2p1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeAgentInstanceName(tt.masterOpenstackName, tt.agentCRName)
			if got != tt.want {
				t.Errorf("ComputeAgentInstanceName(%q, %q) = %q, want %q", tt.masterOpenstackName, tt.agentCRName, got, tt.want)
			}
		})
	}
}

func TestVjailbreakNodeOpenstackNameField(t *testing.T) {
	ctx := context.Background()
	s := testNodeScheme(t)

	tests := []struct {
		name          string
		openstackName string
	}{
		{name: "empty openstack name", openstackName: ""},
		{name: "openstack name set", openstackName: "vjb-appliance-01-vjailbreak-agent-x9k2p1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &vjailbreakv1alpha1.VjailbreakNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent-" + tt.name,
					Namespace: constants.NamespaceMigrationSystem,
				},
				Spec: vjailbreakv1alpha1.VjailbreakNodeSpec{
					NodeRole: "worker",
				},
			}
			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				WithStatusSubresource(&vjailbreakv1alpha1.VjailbreakNode{}).
				Build()

			if err := fakeClient.Create(ctx, node); err != nil {
				t.Fatalf("Create VjailbreakNode: %v", err)
			}

			node.Status.OpenstackName = tt.openstackName
			if err := fakeClient.Status().Update(ctx, node); err != nil {
				t.Fatalf("Status().Update VjailbreakNode: %v", err)
			}

			got := &vjailbreakv1alpha1.VjailbreakNode{}
			if err := fakeClient.Get(ctx, types.NamespacedName{
				Name:      node.Name,
				Namespace: node.Namespace,
			}, got); err != nil {
				t.Fatalf("Get VjailbreakNode: %v", err)
			}

			if got.Status.OpenstackName != tt.openstackName {
				t.Errorf("OpenstackName = %q, want %q", got.Status.OpenstackName, tt.openstackName)
			}
		})
	}
}

// TestReconcileK8sNodeStatus_UsesOpenstackNameNotCRName is a regression test
// for issue #2343: the k8s Node backing an agent is named after its real
// OpenStack/k8s identity (Status.OpenstackName), not the VjailbreakNode CR
// name. ReconcileK8sNodeStatus must look the Node up by OpenstackName.
func TestReconcileK8sNodeStatus_UsesOpenstackNameNotCRName(t *testing.T) {
	ctx := context.Background()
	s := testNodeScheme(t)

	const crName = "vjailbreak-agent-54cf0n"
	const realOpenstackName = "vjb-appliance-01-vjailbreak-agent-54cf0n"

	vjNode := &vjailbreakv1alpha1.VjailbreakNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      crName,
			Namespace: constants.NamespaceMigrationSystem,
		},
		Status: vjailbreakv1alpha1.VjailbreakNodeStatus{
			OpenstackName: realOpenstackName,
		},
	}

	// The real k8s Node is named after the OpenStack identity and is Ready.
	// A Node named after the CR itself does not exist, so if the lookup ever
	// used the CR name it would (wrongly) report NotFound / not-ready.
	readyNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: realOpenstackName,
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(vjNode, readyNode).
		WithStatusSubresource(&vjailbreakv1alpha1.VjailbreakNode{}).
		Build()

	nodeReady, err := ReconcileK8sNodeStatus(ctx, fakeClient, vjNode)
	if err != nil {
		t.Fatalf("ReconcileK8sNodeStatus returned unexpected error: %v", err)
	}
	if !nodeReady {
		t.Error("expected nodeReady=true when the Node named per OpenstackName is Ready")
	}
	if vjNode.Status.Phase != constants.VjailbreakNodePhaseNodeReady {
		t.Errorf("Phase = %q, want %q", vjNode.Status.Phase, constants.VjailbreakNodePhaseNodeReady)
	}
}

func TestListAggregatesFromClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/os-aggregates" {
			t.Fatalf("unexpected request path %q", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected request method %q", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"aggregates": [
				{
					"id": 1,
					"name": "vjb-simple-aggregate",
					"availability_zone": "vjb-simple",
					"hosts": ["host1"],
					"metadata": {"cluster": "vjb-simple", "availability_zone": "vjb-simple"}
				},
				{
					"id": 2,
					"name": "vjb-test-aggregate",
					"availability_zone": "vjb-test",
					"hosts": [],
					"metadata": {"cluster": "vjb-test", "availability_zone": "vjb-test"}
				}
			]
		}`))
	}))
	defer server.Close()

	client := &gophercloud.ServiceClient{
		ProviderClient: &gophercloud.ProviderClient{},
		Endpoint:       server.URL + "/",
	}

	got, err := listAggregatesFromClient(context.Background(), client)
	if err != nil {
		t.Fatalf("listAggregatesFromClient() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d aggregates, want 2", len(got))
	}
	if got[0].Name != "vjb-simple-aggregate" || len(got[0].Hosts) != 1 || got[0].Hosts[0] != "host1" {
		t.Errorf("unexpected first aggregate: %+v", got[0])
	}
	if got[1].Name != "vjb-test-aggregate" || len(got[1].Hosts) != 0 {
		t.Errorf("unexpected second aggregate: %+v", got[1])
	}
	if got[0].Metadata["cluster"] != "vjb-simple" {
		t.Errorf("expected metadata to be extracted, got %+v", got[0].Metadata)
	}
}

func TestListAggregatesFromClient_ErrorPropagates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := &gophercloud.ServiceClient{
		ProviderClient: &gophercloud.ProviderClient{},
		Endpoint:       server.URL + "/",
	}

	_, err := listAggregatesFromClient(context.Background(), client)
	if err == nil {
		t.Fatal("expected error from a 500 response, got nil")
	}
}
