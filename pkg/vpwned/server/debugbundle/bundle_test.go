package debugbundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"strings"
	"testing"
	"testing/fstest"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

// untarGz extracts an archive into a path→content map for assertions.
func untarGz(t *testing.T, data []byte) map[string]string {
	t.Helper()
	gzReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("failed to open gzip: %v", err)
	}
	defer gzReader.Close()

	out := map[string]string{}
	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("failed to read tar: %v", err)
		}
		content, err := io.ReadAll(tarReader)
		if err != nil {
			t.Fatalf("failed to read tar entry %s: %v", header.Name, err)
		}
		out[header.Name] = string(content)
	}
	return out
}

func buildTarGz(t *testing.T, plan *Plan) map[string]string {
	t.Helper()
	var buf bytes.Buffer
	if err := plan.WriteTarGz(context.Background(), "bundle", &buf); err != nil {
		t.Fatalf("WriteTarGz failed: %v", err)
	}
	return untarGz(t, buf.Bytes())
}

func TestPlanAndWriteTarGzProducesStructure(t *testing.T) {
	ctrlClient := newFakeClient(t, testMigrationGraph()...)
	clientset := k8sfake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "v2v-helper-testvm", Namespace: testNamespace},
	})

	deps := Deps{Client: ctrlClient, Clientset: clientset, LogsFS: testLogsFS()}
	// podName intentionally empty — must be resolved from spec.podRef.
	plan := PlanBundle(context.Background(), deps, testNamespace, "migration-testvm", "")

	if plan.VMName != "testvm" {
		t.Errorf("expected VMName=testvm, got %q", plan.VMName)
	}
	if plan.PodName != "v2v-helper-testvm" {
		t.Errorf("expected PodName resolved from spec.podRef, got %q", plan.PodName)
	}

	entries := buildTarGz(t, plan)

	// The fake clientset serves a fixed log body for any pod.
	if !strings.Contains(entries["bundle/pod-logs/v2v-helper-testvm.log"], "fake logs") {
		t.Errorf("expected pod log entry, got %v", entries)
	}
	if !strings.Contains(entries["bundle/kubernetes/migrations/migration-testvm.yaml"], "name: migration-testvm") {
		t.Errorf("expected migration YAML entry, got %v", entries)
	}
	if entries["bundle/debug-logs/migration-testvm.log"] != "root log line" {
		t.Errorf("expected root debug log entry, got %v", entries)
	}
	if entries["bundle/debug-logs/migration-testvm/migration.001.log"] != "subdir log line" {
		t.Errorf("expected subdir debug log entry, got %v", entries)
	}
	if content, ok := entries["bundle/"+warningsFileName]; ok {
		t.Errorf("expected no warnings entry for a clean build, got %q", content)
	}
}

func TestWriteTarGzNoTruncation(t *testing.T) {
	// A debug log well past the old 32 MiB-style caps must arrive whole.
	largeLog := strings.Repeat("x", 5<<20)
	ctrlClient := newFakeClient(t, testMigrationGraph()...)
	clientset := k8sfake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "v2v-helper-testvm", Namespace: testNamespace},
	})
	deps := Deps{
		Client:    ctrlClient,
		Clientset: clientset,
		LogsFS: fstest.MapFS{
			"migration-testvm.log": {Data: []byte(largeLog)},
		},
	}

	plan := PlanBundle(context.Background(), deps, testNamespace, "migration-testvm", "")
	entries := buildTarGz(t, plan)

	if got := len(entries["bundle/debug-logs/migration-testvm.log"]); got != len(largeLog) {
		t.Errorf("expected full %d-byte debug log, got %d bytes", len(largeLog), got)
	}
	if content, ok := entries["bundle/"+warningsFileName]; ok && strings.Contains(content, "truncated") {
		t.Errorf("no truncation warnings expected, got %q", content)
	}
}

func TestWriteTarGzWithoutLogsFS(t *testing.T) {
	ctrlClient := newFakeClient(t, testMigrationGraph()...)
	clientset := k8sfake.NewSimpleClientset()

	deps := Deps{Client: ctrlClient, Clientset: clientset}
	plan := PlanBundle(context.Background(), deps, testNamespace, "migration-testvm", "v2v-helper-testvm")
	entries := buildTarGz(t, plan)

	if !strings.Contains(entries["bundle/"+warningsFileName], "Debug logs directory is not available") {
		t.Errorf("expected missing-logs warning, got %v", entries)
	}
	for path := range entries {
		if strings.Contains(path, "debug-logs/") {
			t.Errorf("expected no debug-logs entries, got %s", path)
		}
	}
}

func TestPlanBundleMigrationMissing(t *testing.T) {
	ctrlClient := newFakeClient(t)
	clientset := k8sfake.NewSimpleClientset()

	deps := Deps{Client: ctrlClient, Clientset: clientset, LogsFS: testLogsFS()}
	plan := PlanBundle(context.Background(), deps, testNamespace, "missing-migration", "")
	entries := buildTarGz(t, plan)

	warnings := entries["bundle/"+warningsFileName]
	if !strings.Contains(warnings, "Migration resource not found") {
		t.Errorf("expected not-found warning, got %q", warnings)
	}
	if !strings.Contains(warnings, "No pod found for this migration") {
		t.Errorf("expected no-pod warning, got %q", warnings)
	}
}
