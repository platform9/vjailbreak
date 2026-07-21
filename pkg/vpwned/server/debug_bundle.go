package server

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/platform9/vjailbreak/pkg/common/constants"
	api "github.com/platform9/vjailbreak/pkg/vpwned/api/proto/v1/service"
	"github.com/platform9/vjailbreak/pkg/vpwned/server/debugbundle"
	"github.com/sirupsen/logrus"
	"google.golang.org/genproto/googleapis/api/httpbody"
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

const debugBundleChunkBytes = 2 << 20

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
// .tar.gz debug bundle — pod stdout/stderr logs, related Kubernetes resource
// YAMLs and debug logs from /var/log/pf9 — by streaming every source into a
// temporary archive file on disk, then streams that archive back in chunks.
func (p *vjailbreakProxy) GetDebugBundle(req *api.GetDebugBundleRequest, stream api.VailbreakProxy_GetDebugBundleServer) error {
	ctx := stream.Context()

	if debugBundleDeps == nil {
		return status.Error(codes.Unavailable, "debug bundle collection is unavailable outside the cluster")
	}

	migrationName := strings.TrimSpace(req.GetMigration())
	podName := strings.TrimSpace(req.GetPod())
	namespace := strings.TrimSpace(req.GetNamespace())
	if namespace == "" {
		namespace = migrationSystemNamespace
	}
	if namespace != migrationSystemNamespace {
		return status.Errorf(codes.InvalidArgument, "namespace must be %s", migrationSystemNamespace)
	}
	if migrationName == "" && podName == "" {
		return status.Error(codes.InvalidArgument, "either migration or pod is required")
	}

	logrus.Infof("debug bundle requested: migration=%q pod=%q namespace=%s", migrationName, podName, namespace)
	plan := debugbundle.PlanBundle(ctx, *debugBundleDeps, namespace, migrationName, podName)

	baseName := plan.VMName
	if baseName == "" {
		baseName = migrationName
	}
	if baseName == "" {
		baseName = plan.PodName
	}
	if baseName == "" {
		baseName = "logs"
	}
	now := time.Now().UTC()
	baseDir := fmt.Sprintf("%s-debug-bundle-%s", baseName, now.Format("2006-01-02T15-04-05Z"))

	archive, err := os.CreateTemp("", "debug-bundle-*.tar.gz")
	if err != nil {
		return status.Errorf(codes.Internal, "failed to create debug bundle archive file: %v", err)
	}
	defer func() {
		archive.Close()
		os.Remove(archive.Name())
	}()

	if err := plan.WriteTarGz(ctx, baseDir, archive); err != nil {
		return status.Errorf(codes.Internal, "failed to build debug bundle archive: %v", err)
	}
	if _, err := archive.Seek(0, io.SeekStart); err != nil {
		return status.Errorf(codes.Internal, "failed to rewind debug bundle archive: %v", err)
	}

	// The gateway promotes this metadata to a real Content-Disposition
	// header (see forwardDownloadHeaders in server.go). Must be set before
	// the first Send.
	if err := stream.SetHeader(metadata.Pairs(contentDispositionMetadataKey, fmt.Sprintf("attachment; filename=%q", baseDir+".tar.gz"))); err != nil {
		logrus.Debugf("debug bundle: could not set content-disposition header: %v", err)
	}

	buf := make([]byte, debugBundleChunkBytes)
	for {
		n, readErr := archive.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			if err := stream.Send(&httpbody.HttpBody{
				ContentType: "application/gzip",
				Data:        chunk,
			}); err != nil {
				return status.Errorf(codes.Internal, "failed to stream debug bundle chunk: %v", err)
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return status.Errorf(codes.Internal, "failed to read debug bundle archive: %v", readErr)
		}
	}
}
