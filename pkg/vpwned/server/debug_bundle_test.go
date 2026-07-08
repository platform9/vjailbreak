package server

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"strings"
	"testing"

	api "github.com/platform9/vjailbreak/pkg/vpwned/api/proto/v1/service"
	"github.com/platform9/vjailbreak/pkg/vpwned/server/debugbundle"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrlfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
)

func newDebugBundleTestDeps(t *testing.T) *debugbundle.Deps {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add client-go scheme: %v", err)
	}
	if err := vjailbreakv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add vjailbreak scheme: %v", err)
	}

	migration := &unstructured.Unstructured{}
	migration.SetAPIVersion(debugbundle.VjailbreakGroupVersion.String())
	migration.SetKind("Migration")
	migration.SetName("migration-testvm")
	migration.SetNamespace(migrationSystemNamespace)
	if err := unstructured.SetNestedMap(migration.Object, map[string]interface{}{
		"vmName": "testvm",
		"podRef": "v2v-helper-testvm",
	}, "spec"); err != nil {
		t.Fatalf("failed to set migration spec: %v", err)
	}

	ctrlClient := ctrlfake.NewClientBuilder().WithScheme(scheme).WithObjects(migration).Build()
	clientset := k8sfake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "v2v-helper-testvm", Namespace: migrationSystemNamespace},
	})
	return &debugbundle.Deps{Client: ctrlClient, Clientset: clientset}
}

func TestGetDebugBundleUnavailableOutsideCluster(t *testing.T) {
	original := debugBundleDeps
	debugBundleDeps = nil
	defer func() { debugBundleDeps = original }()

	proxy := &vjailbreakProxy{}
	_, err := proxy.GetDebugBundle(context.Background(), &api.GetDebugBundleRequest{Migration: "m1"})

	if status.Code(err) != codes.Unavailable {
		t.Fatalf("expected Unavailable, got %v", err)
	}
}

func TestGetDebugBundleValidation(t *testing.T) {
	original := debugBundleDeps
	debugBundleDeps = newDebugBundleTestDeps(t)
	defer func() { debugBundleDeps = original }()

	proxy := &vjailbreakProxy{}
	tests := []struct {
		name string
		req  *api.GetDebugBundleRequest
	}{
		{"missing identifiers", &api.GetDebugBundleRequest{}},
		{"foreign namespace", &api.GetDebugBundleRequest{Migration: "m1", Namespace: "kube-system"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := proxy.GetDebugBundle(context.Background(), tc.req)
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("expected InvalidArgument, got %v", err)
			}
		})
	}
}

func TestGetDebugBundleReturnsBundle(t *testing.T) {
	original := debugBundleDeps
	debugBundleDeps = newDebugBundleTestDeps(t)
	defer func() { debugBundleDeps = original }()

	proxy := &vjailbreakProxy{}
	body, err := proxy.GetDebugBundle(context.Background(), &api.GetDebugBundleRequest{Migration: "migration-testvm"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if body.GetContentType() != "application/gzip" {
		t.Errorf("unexpected content type %q", body.GetContentType())
	}

	gzReader, err := gzip.NewReader(bytes.NewReader(body.GetData()))
	if err != nil {
		t.Fatalf("response is not valid gzip: %v", err)
	}
	defer gzReader.Close()

	entries := map[string]string{}
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
		entries[header.Name] = string(content)
	}

	var migrationYAML, podLog string
	for name, content := range entries {
		if strings.HasSuffix(name, "/kubernetes/migrations/migration-testvm.yaml") {
			migrationYAML = content
		}
		if strings.HasSuffix(name, "/pod-logs/v2v-helper-testvm.log") {
			podLog = content
		}
		if !strings.Contains(name, "testvm-debug-bundle-") {
			t.Errorf("entry %s not under the bundle base directory", name)
		}
	}
	if !strings.Contains(migrationYAML, "name: migration-testvm") {
		t.Errorf("expected migration YAML in archive, entries: %v", entries)
	}
	if !strings.Contains(podLog, "fake logs") {
		t.Errorf("expected pod log file in archive, entries: %v", entries)
	}
}
