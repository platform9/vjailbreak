package debugbundle

import (
	"context"
	"fmt"
	"io/fs"
	"strings"

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

// ArchiveFile is one file inside the debug bundle archive. Path is relative
// to the archive root, e.g. kubernetes/migrations/migration-foo.yaml.
type ArchiveFile struct {
	Path string
	Data []byte
}

// Result is a fully assembled debug bundle.
type Result struct {
	// Files are the archive entries: pod-logs/, kubernetes/, debug-logs/
	// and collection-warnings.txt.
	Files []ArchiveFile
	// VMName is the migration's spec.vmName when found, used for the
	// download file name.
	VMName string
	// PodName is the pod whose logs were included (resolved from the
	// migration's spec.podRef when not supplied by the caller).
	PodName string
}

const warningsFileName = "collection-warnings.txt"

// Build assembles the complete debug bundle for a migration as a list of
// archive files:
//
//	pod-logs/<pod>.log             pod stdout/stderr
//	kubernetes/<plural>/<name>.yaml one file per related resource
//	debug-logs/...                  log files from /var/log/pf9
//	collection-warnings.txt         anything that could not be collected
//
// Failures in any part degrade to a warning entry so the caller always
// receives the files that could be collected.
func Build(ctx context.Context, deps Deps, namespace, migrationName, podName string) Result {
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

	files := make([]ArchiveFile, 0, len(entries)+2)

	if podName == "" {
		warnings = append(warnings, "No pod found for this migration — pod logs omitted")
	} else {
		logs, err := FetchPodLogs(ctx, deps.Clientset, namespace, podName)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("Failed to fetch pod logs: %v", err))
		} else {
			files = append(files, ArchiveFile{Path: "pod-logs/" + podName + ".log", Data: []byte(logs)})
		}
	}

	for _, entry := range entries {
		files = append(files, ArchiveFile{Path: entry.Path, Data: []byte(renderObjectYAML(entry.Object))})
	}

	if deps.LogsFS == nil {
		warnings = append(warnings, "Debug logs directory is not available — /var/log/pf9 logs omitted")
	} else {
		debugFiles, debugWarnings := CollectDebugFiles(deps.LogsFS, migrationName)
		files = append(files, debugFiles...)
		warnings = append(warnings, debugWarnings...)
	}

	if len(warnings) > 0 {
		files = append(files, ArchiveFile{
			Path: warningsFileName,
			Data: []byte(strings.Join(warnings, "\n") + "\n"),
		})
	}

	return Result{Files: files, VMName: vmName, PodName: podName}
}
