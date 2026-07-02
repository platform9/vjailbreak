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

// Result is a fully assembled debug bundle.
type Result struct {
	// Content is the bundle text (pod logs + resources + debug file logs).
	Content string
	// VMName is the migration's spec.vmName when found, used for the
	// download file name.
	VMName string
	// PodName is the pod whose logs were included (resolved from the
	// migration's spec.podRef when not supplied by the caller).
	PodName string
}

func sectionHeader(title string) string {
	return sectionSeparator + title + "\n" + sectionSeparator + "\n"
}

// Build assembles the complete debug bundle for a migration, in the same
// three-section layout the UI download previously produced:
// pod stdout/stderr logs, related Kubernetes resources, and debug logs
// from /var/log/pf9. Failures in any section degrade to an inline note so
// the caller always receives the sections that could be collected.
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

	var out strings.Builder

	out.WriteString(sectionHeader("STDOUT/STDERR LOGS (pod)"))
	switch {
	case podName == "":
		out.WriteString("[No pod found for this migration]\n")
	default:
		logs, err := FetchPodLogs(ctx, deps.Clientset, namespace, podName)
		if err != nil {
			out.WriteString(fmt.Sprintf("[Failed to fetch pod logs: %v]\n", err))
		} else {
			out.WriteString(logs)
			if !strings.HasSuffix(logs, "\n") {
				out.WriteString("\n")
			}
		}
	}

	out.WriteString("\n")
	out.WriteString(sectionHeader("RELATED KUBERNETES RESOURCES"))
	resourceBundle := FormatYAMLBundle(entries, warnings)
	if strings.TrimSpace(resourceBundle) == "" {
		out.WriteString("[No related Kubernetes resources found]\n")
	} else {
		out.WriteString(resourceBundle)
	}

	out.WriteString("\n")
	out.WriteString(sectionHeader("DEBUG LOGS FROM /var/log/pf9"))
	if deps.LogsFS == nil {
		out.WriteString("[Debug logs directory is not available]\n")
	} else {
		debugLogs := CollectDebugFileLogs(deps.LogsFS, migrationName)
		if strings.TrimSpace(debugLogs) == "" {
			out.WriteString("[No debug log files found]\n")
		} else {
			out.WriteString(debugLogs)
		}
	}

	return Result{Content: out.String(), VMName: vmName, PodName: podName}
}
