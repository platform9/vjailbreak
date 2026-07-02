package server

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/platform9/vjailbreak/pkg/common/constants"
	api "github.com/platform9/vjailbreak/pkg/vpwned/api/proto/v1/service"
	"github.com/platform9/vjailbreak/pkg/vpwned/server/debugbundle"
	"github.com/sirupsen/logrus"
	"google.golang.org/genproto/googleapis/api/httpbody"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// debugBundleDeps is initialized once at server startup by InitDebugBundle.
// It stays nil outside a cluster, in which case GetDebugBundle returns
// codes.Unavailable.
var debugBundleDeps *debugbundle.Deps

// InitDebugBundle wires the kubernetes clients and debug logs directory used
// by the GetDebugBundle API. Call once during server startup; returns an
// error when not running inside a cluster.
func InitDebugBundle() error {
	config, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("in-cluster config unavailable: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}
	ctrlClient, err := CreateInClusterClient()
	if err != nil {
		return fmt.Errorf("failed to create controller-runtime client: %w", err)
	}
	deps := &debugbundle.Deps{
		Client:    ctrlClient,
		Clientset: clientset,
	}
	if info, statErr := os.Stat(constants.LogsDir); statErr == nil && info.IsDir() {
		deps.LogsFS = os.DirFS(constants.LogsDir)
	} else {
		logrus.Warnf("debug bundle: %s not available, bundles will omit debug file logs: %v", constants.LogsDir, statErr)
	}
	debugBundleDeps = deps
	return nil
}

// GetDebugBundle implements VailbreakProxy.GetDebugBundle. It assembles a
// plain-text debug bundle — pod stdout/stderr logs, related Kubernetes
// resource YAMLs and debug logs from /var/log/pf9 — and returns it as an
// HttpBody so the REST gateway serves it as a file download.
func (p *vjailbreakProxy) GetDebugBundle(ctx context.Context, req *api.GetDebugBundleRequest) (*httpbody.HttpBody, error) {
	if debugBundleDeps == nil {
		return nil, status.Error(codes.Unavailable, "debug bundle collection is unavailable outside the cluster")
	}

	migrationName := strings.TrimSpace(req.GetMigration())
	podName := strings.TrimSpace(req.GetPod())
	namespace := strings.TrimSpace(req.GetNamespace())
	if namespace == "" {
		namespace = migrationSystemNamespace
	}
	if namespace != migrationSystemNamespace {
		return nil, status.Errorf(codes.InvalidArgument, "namespace must be %s", migrationSystemNamespace)
	}
	if migrationName == "" && podName == "" {
		return nil, status.Error(codes.InvalidArgument, "either migration or pod is required")
	}

	logrus.Infof("debug bundle requested: migration=%q pod=%q namespace=%s", migrationName, podName, namespace)
	result := debugbundle.Build(ctx, *debugBundleDeps, namespace, migrationName, podName)

	baseName := result.VMName
	if baseName == "" {
		baseName = migrationName
	}
	if baseName == "" {
		baseName = result.PodName
	}
	if baseName == "" {
		baseName = "logs"
	}
	timestamp := time.Now().UTC().Format("2006-01-02T15-04-05Z")
	fileName := fmt.Sprintf("%s-pod-%s.txt", baseName, timestamp)

	// The gateway promotes this metadata to a real Content-Disposition
	// header (see forwardDownloadHeaders in server.go). SetHeader fails on
	// plain (non-transport) contexts, e.g. in unit tests — safe to ignore.
	if err := grpc.SetHeader(ctx, metadata.Pairs(contentDispositionMetadataKey, fmt.Sprintf("attachment; filename=%q", fileName))); err != nil {
		logrus.Debugf("debug bundle: could not set content-disposition header: %v", err)
	}

	return &httpbody.HttpBody{
		ContentType: "text/plain; charset=utf-8",
		Data:        []byte(result.Content),
	}, nil
}
