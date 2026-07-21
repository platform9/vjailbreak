package debugbundle

import (
	"context"
	"fmt"
	"io"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

func spoolPodLogs(ctx context.Context, clientset kubernetes.Interface, namespace, podName string) (*os.File, int64, error) {
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{})
	stream, err := req.Stream(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to stream logs for pod %s/%s: %w", namespace, podName, err)
	}
	defer stream.Close()

	spool, err := os.CreateTemp("", "debug-bundle-podlogs-*.log")
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create pod logs spool file: %w", err)
	}

	size, err := io.Copy(spool, stream)
	if err != nil {
		spool.Close()
		os.Remove(spool.Name())
		return nil, 0, fmt.Errorf("failed to read logs for pod %s/%s: %w", namespace, podName, err)
	}
	if _, err := spool.Seek(0, io.SeekStart); err != nil {
		spool.Close()
		os.Remove(spool.Name())
		return nil, 0, fmt.Errorf("failed to rewind pod logs spool file: %w", err)
	}
	return spool, size, nil
}
