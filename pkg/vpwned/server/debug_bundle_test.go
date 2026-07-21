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
	"google.golang.org/genproto/googleapis/api/httpbody"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
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

// fakeDebugBundleStream implements api.VailbreakProxy_GetDebugBundleServer,
// capturing streamed chunks and headers.
type fakeDebugBundleStream struct {
	ctx    context.Context
	chunks []*httpbody.HttpBody
	header metadata.MD
}

func (s *fakeDebugBundleStream) Send(body *httpbody.HttpBody) error {
	s.chunks = append(s.chunks, body)
	return nil
}
func (s *fakeDebugBundleStream) SetHeader(md metadata.MD) error {
	s.header = metadata.Join(s.header, md)
	return nil
}
func (s *fakeDebugBundleStream) SendHeader(metadata.MD) error { return nil }
func (s *fakeDebugBundleStream) SetTrailer(metadata.MD)       {}
func (s *fakeDebugBundleStream) Context() context.Context     { return s.ctx }
func (s *fakeDebugBundleStream) SendMsg(interface{}) error    { return nil }
func (s *fakeDebugBundleStream) RecvMsg(interface{}) error    { return nil }

func newFakeStream() *fakeDebugBundleStream {
	return &fakeDebugBundleStream{ctx: context.Background()}
}

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
	err := proxy.GetDebugBundle(&api.GetDebugBundleRequest{Migration: "m1"}, newFakeStream())

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
			err := proxy.GetDebugBundle(tc.req, newFakeStream())
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("expected InvalidArgument, got %v", err)
			}
		})
	}
}

func TestGetDebugBundleStreamsArchive(t *testing.T) {
	original := debugBundleDeps
	debugBundleDeps = newDebugBundleTestDeps(t)
	defer func() { debugBundleDeps = original }()

	proxy := &vjailbreakProxy{}
	stream := newFakeStream()
	if err := proxy.GetDebugBundle(&api.GetDebugBundleRequest{Migration: "migration-testvm"}, stream); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(stream.chunks) == 0 {
		t.Fatal("expected streamed chunks")
	}
	var archive bytes.Buffer
	for _, chunk := range stream.chunks {
		if chunk.GetContentType() != "application/gzip" {
			t.Errorf("unexpected chunk content type %q", chunk.GetContentType())
		}
		archive.Write(chunk.GetData())
	}

	disposition := stream.header.Get(contentDispositionMetadataKey)
	if len(disposition) == 0 || !strings.Contains(disposition[0], ".tar.gz") {
		t.Errorf("expected .tar.gz content-disposition header, got %v", disposition)
	}

	gzReader, err := gzip.NewReader(&archive)
	if err != nil {
		t.Fatalf("streamed data is not valid gzip: %v", err)
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
