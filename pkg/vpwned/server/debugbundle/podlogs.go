package debugbundle

import (
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// maxPodLogBytes caps the pod log section so a runaway log cannot exhaust
// memory while assembling the bundle (64 MiB).
const maxPodLogBytes = int64(64 << 20)

// FetchPodLogs returns the stdout/stderr logs of the given pod's default
// container, capped at maxPodLogBytes.
func FetchPodLogs(ctx context.Context, clientset kubernetes.Interface, namespace, podName string) (string, error) {
	limit := maxPodLogBytes
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		LimitBytes: &limit,
	})
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to stream logs for pod %s/%s: %w", namespace, podName, err)
	}
	defer stream.Close()

	data, err := io.ReadAll(io.LimitReader(stream, maxPodLogBytes))
	if err != nil {
		return "", fmt.Errorf("failed to read logs for pod %s/%s: %w", namespace, podName, err)
	}
	return string(data), nil
}
