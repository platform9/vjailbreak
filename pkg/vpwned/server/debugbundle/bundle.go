package debugbundle

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"time"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Deps holds the external dependencies needed to build a debug bundle.
type Deps struct {
	// Client reads vJailbreak CRs, ConfigMaps and Pods.
	Client client.Client
	// Clientset streams pod logs.
	Clientset kubernetes.Interface
	// LogsFS is the debug logs directory (/var/log/pf9 in production).
	// May be nil when the directory is unavailable.
	LogsFS fs.FS
}

const warningsFileName = "collection-warnings.txt"

type Plan struct {
	deps      Deps
	namespace string
	entries   []BundleEntry
	warnings  []string

	// VMName is the migration's spec.vmName when found, used for the
	// download file name.
	VMName string
	// PodName is the pod whose logs will be included (resolved from the
	// migration's spec.podRef when not supplied by the caller).
	PodName string
	// MigrationName is the resolved migration CR name.
	MigrationName string
}

func PlanBundle(ctx context.Context, deps Deps, namespace, migrationName, podName string) *Plan {
	entries, warnings := CollectResources(ctx, deps.Client, namespace, migrationName, podName)

	var vmName string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Path, "kubernetes/migrations/") {
			vmName = nestedString(entry.Object.Object, "spec", "vmName")
			if migrationName == "" {
				migrationName = entry.Object.GetName()
			}
			if podName == "" {
				podName = nestedString(entry.Object.Object, "spec", "podRef")
			}
			break
		}
	}

	return &Plan{
		deps:          deps,
		namespace:     namespace,
		entries:       entries,
		warnings:      warnings,
		VMName:        vmName,
		PodName:       podName,
		MigrationName: migrationName,
	}
}

// WriteTarGz streams the bundle as a gzip-compressed tar to out, placing
// every entry under baseDir.
func (p *Plan) WriteTarGz(ctx context.Context, baseDir string, out io.Writer) error {
	gzWriter := gzip.NewWriter(out)
	tarWriter := tar.NewWriter(gzWriter)
	now := time.Now().UTC()
	warnings := append([]string{}, p.warnings...)

	writeBytes := func(path string, data []byte) error {
		header := &tar.Header{
			Name:    baseDir + "/" + path,
			Mode:    0o644,
			Size:    int64(len(data)),
			ModTime: now,
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write tar header for %s: %w", path, err)
		}
		if _, err := tarWriter.Write(data); err != nil {
			return fmt.Errorf("failed to write tar entry %s: %w", path, err)
		}
		return nil
	}

	// streamFile copies exactly size bytes from r into the tar entry,
	// padding with newlines if the source shrinks mid-copy so the archive
	// stays structurally valid.
	streamFile := func(path string, size int64, modTime time.Time, r io.Reader) error {
		header := &tar.Header{
			Name:    baseDir + "/" + path,
			Mode:    0o644,
			Size:    size,
			ModTime: modTime,
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write tar header for %s: %w", path, err)
		}
		written, err := io.CopyN(tarWriter, r, size)
		if err != nil && err != io.EOF {
			return fmt.Errorf("failed to stream tar entry %s: %w", path, err)
		}
		if written < size {
			warnings = append(warnings, fmt.Sprintf("%s shrank while streaming — padded %d bytes", path, size-written))
			if _, err := io.CopyN(tarWriter, newlineReader{}, size-written); err != nil {
				return fmt.Errorf("failed to pad tar entry %s: %w", path, err)
			}
		}
		return nil
	}

	// Pod logs: spooled to a temp file first because the tar header needs
	// the size up front and the log stream length is unknown.
	if p.PodName == "" {
		warnings = append(warnings, "No pod found for this migration — pod logs omitted")
	} else if spool, size, err := spoolPodLogs(ctx, p.deps.Clientset, p.namespace, p.PodName); err != nil {
		warnings = append(warnings, fmt.Sprintf("Failed to fetch pod logs: %v", err))
	} else {
		streamErr := streamFile("pod-logs/"+p.PodName+".log", size, now, spool)
		spool.Close()
		os.Remove(spool.Name())
		if streamErr != nil {
			return streamErr
		}
	}

	// Resource YAMLs are small; render in memory.
	for _, entry := range p.entries {
		if err := writeBytes(entry.Path, []byte(renderObjectYAML(entry.Object))); err != nil {
			return err
		}
	}

	// Debug log files: streamed straight from the logs directory.
	if p.deps.LogsFS == nil {
		warnings = append(warnings, "Debug logs directory is not available — /var/log/pf9 logs omitted")
	} else {
		paths, listWarnings := ListDebugLogPaths(p.deps.LogsFS, p.MigrationName)
		warnings = append(warnings, listWarnings...)
		for _, path := range paths {
			info, err := fs.Stat(p.deps.LogsFS, path)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("Failed to stat debug log %s: %v", path, err))
				continue
			}
			file, err := p.deps.LogsFS.Open(path)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("Failed to open debug log %s: %v", path, err))
				continue
			}
			streamErr := streamFile("debug-logs/"+path, info.Size(), info.ModTime(), file)
			file.Close()
			if streamErr != nil {
				return streamErr
			}
		}
	}

	// Warnings last so streaming failures above are included.
	if len(warnings) > 0 {
		if err := writeBytes(warningsFileName, []byte(strings.Join(warnings, "\n")+"\n")); err != nil {
			return err
		}
	}

	if err := tarWriter.Close(); err != nil {
		return fmt.Errorf("failed to finalize tar: %w", err)
	}
	if err := gzWriter.Close(); err != nil {
		return fmt.Errorf("failed to finalize gzip: %w", err)
	}
	return nil
}

// newlineReader yields an endless stream of newline bytes, used to pad tar
// entries whose source file shrank between stat and copy.
type newlineReader struct{}

func (newlineReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = '\n'
	}
	return len(p), nil
}
