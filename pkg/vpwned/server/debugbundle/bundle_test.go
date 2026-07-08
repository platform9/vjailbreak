package debugbundle

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func filesByPath(files []ArchiveFile) map[string]string {
	out := make(map[string]string, len(files))
	for _, file := range files {
		out[file.Path] = string(file.Data)
	}
	return out
}

func TestBuildProducesArchiveFiles(t *testing.T) {
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

	files := filesByPath(result.Files)

	// The fake clientset serves a fixed log body for any pod.
	if !strings.Contains(files["pod-logs/v2v-helper-testvm.log"], "fake logs") {
		t.Errorf("expected pod log file, got files %v", files)
	}
	migrationYAML := files["kubernetes/migrations/migration-testvm.yaml"]
	if !strings.Contains(migrationYAML, "name: migration-testvm") {
		t.Errorf("expected migration YAML file, got %q", migrationYAML)
	}
	if !strings.Contains(files["debug-logs/migration-testvm.log"], "root log line") {
		t.Errorf("expected root debug log file, got files %v", files)
	}
	if !strings.Contains(files["debug-logs/migration-testvm/migration.001.log"], "subdir log line") {
		t.Errorf("expected subdir debug log file, got files %v", files)
	}
	if _, ok := files[warningsFileName]; ok {
		t.Errorf("expected no warnings file for a clean build, got %q", files[warningsFileName])
	}
}

func TestBuildWithoutLogsFS(t *testing.T) {
	ctrlClient := newFakeClient(t, testMigrationGraph()...)
	clientset := k8sfake.NewSimpleClientset()

	deps := Deps{Client: ctrlClient, Clientset: clientset}
	result := Build(context.Background(), deps, testNamespace, "migration-testvm", "v2v-helper-testvm")

	files := filesByPath(result.Files)
	if !strings.Contains(files[warningsFileName], "Debug logs directory is not available") {
		t.Errorf("expected missing-logs warning, got %q", files[warningsFileName])
	}
	for path := range files {
		if strings.HasPrefix(path, "debug-logs/") {
			t.Errorf("expected no debug-logs entries, got %s", path)
		}
	}
}

func TestBuildMigrationMissingStillReturnsWarnings(t *testing.T) {
	ctrlClient := newFakeClient(t)
	clientset := k8sfake.NewSimpleClientset()

	deps := Deps{Client: ctrlClient, Clientset: clientset, LogsFS: testLogsFS()}
	result := Build(context.Background(), deps, testNamespace, "missing-migration", "")

	files := filesByPath(result.Files)
	warnings := files[warningsFileName]
	if !strings.Contains(warnings, "Migration resource not found") {
		t.Errorf("expected not-found warning, got %q", warnings)
	}
	if !strings.Contains(warnings, "No pod found for this migration") {
		t.Errorf("expected no-pod warning, got %q", warnings)
	}
}
