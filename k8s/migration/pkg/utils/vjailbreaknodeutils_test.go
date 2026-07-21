package utils

import (
	"context"
	"testing"

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
