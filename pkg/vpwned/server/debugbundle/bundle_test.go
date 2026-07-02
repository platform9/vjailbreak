package debugbundle

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestBuildAssemblesAllSections(t *testing.T) {
	ctrlClient := newFakeClient(t, testMigrationGraph()...)
	clientset := k8sfake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "v2v-helper-testvm", Namespace: testNamespace},
	})

	deps := Deps{Client: ctrlClient, Clientset: clientset, LogsFS: testLogsFS()}
	// podName intentionally empty — must be resolved from spec.podRef.
	result := Build(context.Background(), deps, testNamespace, "migration-testvm", "")

	if result.VMName != "testvm" {
		t.Errorf("expected VMName=testvm, got %q", result.VMName)
	}
	if result.PodName != "v2v-helper-testvm" {
		t.Errorf("expected PodName resolved from spec.podRef, got %q", result.PodName)
	}

	for _, section := range []string{
		"STDOUT/STDERR LOGS (pod)",
		"RELATED KUBERNETES RESOURCES",
		"DEBUG LOGS FROM /var/log/pf9",
	} {
		if !strings.Contains(result.Content, section) {
			t.Errorf("expected section %q in bundle:\n%s", section, result.Content)
		}
	}

	// The fake clientset serves a fixed log body for any pod.
	if !strings.Contains(result.Content, "fake logs") {
		t.Errorf("expected pod logs section content, got:\n%s", result.Content)
	}
	if !strings.Contains(result.Content, "FILE: kubernetes/migrations/migration-testvm.yaml") {
		t.Errorf("expected migration YAML in bundle:\n%s", result.Content)
	}
	// migrationName filter matches the debug log fixture prefix.
	if !strings.Contains(result.Content, "root log line") {
		t.Errorf("expected debug file logs in bundle:\n%s", result.Content)
	}
}

func TestBuildWithoutLogsFS(t *testing.T) {
	ctrlClient := newFakeClient(t, testMigrationGraph()...)
	clientset := k8sfake.NewSimpleClientset()

	deps := Deps{Client: ctrlClient, Clientset: clientset}
	result := Build(context.Background(), deps, testNamespace, "migration-testvm", "v2v-helper-testvm")

	if !strings.Contains(result.Content, "[Debug logs directory is not available]") {
		t.Errorf("expected missing-logs note, got:\n%s", result.Content)
	}
}

func TestBuildMigrationMissingStillReturnsBundle(t *testing.T) {
	ctrlClient := newFakeClient(t)
	clientset := k8sfake.NewSimpleClientset()

	deps := Deps{Client: ctrlClient, Clientset: clientset, LogsFS: testLogsFS()}
	result := Build(context.Background(), deps, testNamespace, "missing-migration", "")

	if !strings.Contains(result.Content, "Migration resource not found") {
		t.Errorf("expected not-found warning in bundle:\n%s", result.Content)
	}
	if !strings.Contains(result.Content, "[No pod found for this migration]") {
		t.Errorf("expected no-pod note, got:\n%s", result.Content)
	}
}
