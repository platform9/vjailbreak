// Copyright © 2024 The vjailbreak authors

package reporter

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/platform9/vjailbreak/pkg/common/constants"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type ReporterOps interface {
	IsRunningInPod() bool
	GetKubernetesClient() error
	GetPodName() error
	GetPodNamespace() error
	CreateKubernetesEvent(ctx context.Context, eventType, reason, message string) error
	UpdatePodEvents(ch <-chan string)
	GetCutoverLabel() (string, error)
	WatchPodLabels(ctx context.Context, ch chan<- string) error
}

type Reporter struct {
	PodName      string
	PodNamespace string
	Pod          *corev1.Pod
	Clientset    *kubernetes.Clientset
}

func IsRunningInPod() bool {
	if _, ok := os.LookupEnv("KUBERNETES_SERVICE_HOST"); !ok {
		return false
	}
	if _, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount"); err != nil {
		return false
	}
	return true
}

func (r *Reporter) GetKubernetesClient() error {
	config, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("failed to get in-cluster config: %v", err)
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %v", err)
	}

	r.Clientset = clientset
	return nil
}

func (r *Reporter) GetPodName() error {
	if podName, ok := os.LookupEnv("POD_NAME"); !ok {
		return fmt.Errorf("failed to get pod name")
	} else {
		r.PodName = podName
	}
	return nil
}

func (r *Reporter) GetPodNamespace() error {
	if podNamespace, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		r.PodNamespace = string(podNamespace)
	} else {
		return fmt.Errorf("failed to get pod namespace: %v", err)
	}
	return nil
}

func (r *Reporter) GetPod() error {
	pod, err := r.Clientset.CoreV1().Pods(r.PodNamespace).Get(context.TODO(), r.PodName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get pod: %v", err)
	}
	r.Pod = pod
	return nil
}

func NewReporter() (*Reporter, error) {
	r := &Reporter{}
	if IsRunningInPod() {
		utils.PrintLog("Running in pod")
		if err := r.GetPodName(); err != nil {
			return nil, err
		}
		if err := r.GetPodNamespace(); err != nil {
			return nil, err
		}
		if err := r.GetKubernetesClient(); err != nil {
			return nil, err
		}
		if err := r.GetPod(); err != nil {
			return nil, err
		}
	} else {
		utils.PrintLog("Not running in pod")
		os.Exit(1)
	}
	return r, nil
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func (r *Reporter) CreateKubernetesEvent(ctx context.Context, eventType, reason, message string) error {
	currtime := metav1.Now()
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:              fmt.Sprintf("%s.%s.%s", r.PodName, r.PodNamespace, generateRandomString(10)),
			Namespace:         r.PodNamespace,
			CreationTimestamp: currtime,
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Pod",
			Name:      r.PodName,
			Namespace: r.PodNamespace,
			UID:       types.UID(r.Pod.UID),
		},
		FirstTimestamp: currtime,
		LastTimestamp:  currtime,
		Reason:         reason,
		Message:        message,
		Source: corev1.EventSource{
			Component: "Migration",
		},
		Type: eventType,
	}
	_, err := r.Clientset.CoreV1().Events(r.PodNamespace).Create(ctx, event, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create kubernetes event: %v", err)
	}
	return nil
}

func (r *Reporter) UpdateProgress(progress string) error {
	condition := corev1.PodCondition{
		Type:               "Progressing",
		Status:             corev1.ConditionTrue,
		Reason:             "Migration",
		Message:            progress,
		LastTransitionTime: metav1.Now(),
	}
	// Remove any existing Progressing condition
	for i, cond := range r.Pod.Status.Conditions {
		if cond.Type == "Progressing" {
			r.Pod.Status.Conditions = append(r.Pod.Status.Conditions[:i], r.Pod.Status.Conditions[i+1:]...)
			break
		}
	}

	// Add the new condition
	r.Pod.Status.Conditions = append(r.Pod.Status.Conditions, condition)

	// Update the Pod status
	_, err := r.Clientset.CoreV1().Pods(r.PodNamespace).UpdateStatus(context.TODO(), r.Pod, metav1.UpdateOptions{})
	if err != nil {
		if err := r.GetPod(); err != nil {
			return fmt.Errorf("failed to get pod: %v", err)
		}
		_, err := r.Clientset.CoreV1().Pods(r.PodNamespace).UpdateStatus(context.TODO(), r.Pod, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update pod status: %v", err)
		}
	}
	if err := r.GetPod(); err != nil {
		return fmt.Errorf("failed to get pod: %v", err)
	}

	return nil
}

func (r *Reporter) UpdatePodEvents(ctx context.Context, ch <-chan string, ackChan chan<- struct{}) {
	go func() {
		for {
			select {
			case msg, ok := <-ch:
				if !ok {
					// Channel closed, exit the goroutine
					return
				}
				if err := r.UpdateProgress(msg); err != nil {
					utils.PrintLog(err.Error())
				}
				// Skip creating events for progress updates and most periodic sync messages
				// But allow periodic sync warning messages and success messages through
				shouldCreateEvent := !strings.Contains(msg, "Progress:")
				if strings.Contains(msg, "Periodic") {
					// Only create events for warning messages and sync completion
					shouldCreateEvent = strings.Contains(msg, "WARNING") || strings.Contains(msg, "completed successfully")
				}
				if shouldCreateEvent {
					eventType := corev1.EventTypeNormal
					if strings.Contains(msg, "WARNING") {
						eventType = corev1.EventTypeWarning
					}
					if err := r.CreateKubernetesEvent(ctx, eventType, "Migration", msg); err != nil {
						utils.PrintLog(err.Error())
					}
				}
				// Sending acknowledgment that the message has been processed
				// If no one is expecting the ack, it will be dropped
				select {
				case ackChan <- struct{}{}:
					// Acknowledgment sent successfully
				default:
					// No one expecting ack
				}

			case <-ctx.Done():
				// Context cancelled, exit the goroutine
				return
			}
		}
	}()
}

func (r *Reporter) GetCutoverLabel() (string, error) {
	if err := r.GetPod(); err != nil {
		return "", fmt.Errorf("failed to get pod: %v", err)
	}
	if cutover, ok := r.Pod.Labels["startCutover"]; ok {
		return cutover, nil
	}
	return "", fmt.Errorf("failed to get cutover label")
}

// WatchLDMBootStatusLabel is a parallel watcher to WatchPodLabels for the
// ldmBootStatus label. It is a second watch on the same pod rather than an
// extension of WatchPodLabels, because widening that channel to carry which
// label changed would ripple into the existing cutover wait loop. The cost is
// one extra watch; the benefit is that the cutover path is untouched.
func (r *Reporter) WatchLDMBootStatusLabel(ctx context.Context, ch chan<- string) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				fmt.Printf("Info: Context canceled while watching %s for pod %s\n", constants.LDMBootStatusLabel, r.PodName)
				return
			default:
				// ResourceVersion is deliberately left unset. For a watch that means
				// "Get State and Start at Most Recent", which begins with synthetic
				// ADDED events describing the pod as it exists now. That is what
				// makes the reconnect below safe: an answer set while this watch was
				// down arrives as the first event of the next one, so no separate
				// bootstrap read is needed. Setting a ResourceVersion would opt into
				// "Start at Exact", which suppresses those synthetic events and
				// would genuinely lose an answer set during the gap.
				timeoutSeconds := int64(172800)
				watcher, err := r.Clientset.CoreV1().Pods(r.PodNamespace).Watch(ctx, metav1.ListOptions{
					FieldSelector:  fmt.Sprintf("metadata.name=%s", r.PodName),
					TimeoutSeconds: &timeoutSeconds,
				})
				if err != nil {
					fmt.Printf("Error: Failed to watch %s for pod %s: %v\n", constants.LDMBootStatusLabel, r.PodName, err)
					time.Sleep(5 * time.Second)
					continue
				}

				// Reset per reconnect on purpose, so the synthetic ADDED event
				// re-delivers an answer that arrived while the watch was down.
				last := ""
				for event := range watcher.ResultChan() {
					pod, ok := event.Object.(*corev1.Pod)
					if !ok {
						continue
					}
					status := pod.Labels[constants.LDMBootStatusLabel]
					if status == "" || status == last {
						continue
					}
					fmt.Printf("Info: %s changed for pod %s: %q -> %q\n", constants.LDMBootStatusLabel, r.PodName, last, status)
					// Same shutdown race as the cutover watcher: this goroutine
					// outlives main, so drop a late event rather than panicking.
					select {
					case <-ctx.Done():
						watcher.Stop()
						return
					case ch <- status:
						last = status
					}
				}
				watcher.Stop()
				time.Sleep(5 * time.Second)
			}
		}
	}()
}

func (r *Reporter) WatchPodLabels(ctx context.Context, ch chan<- string) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				fmt.Printf("Error: Context canceled for pod %s: %v\n", r.PodName, ctx.Err())
				return
			default:
				fmt.Printf("Info: Starting watch for pod %s in namespace %s\n", r.PodName, r.PodNamespace)
				timeoutSeconds := int64(172800)
				watch, err := r.Clientset.CoreV1().Pods(r.PodNamespace).Watch(ctx, metav1.ListOptions{
					FieldSelector:  fmt.Sprintf("metadata.name=%s", r.PodName),
					TimeoutSeconds: &timeoutSeconds,
				})
				if err != nil {
					fmt.Printf("Error: Failed to start watch for pod %s: %v\n", r.PodName, err)
					time.Sleep(5 * time.Second)
					continue
				}
				fmt.Printf("Info: Watch established for pod %s with timeout %d seconds\n", r.PodName, timeoutSeconds)
				// Stopped explicitly rather than with defer: this goroutine never
				// returns, so a deferred Stop would never run and would leak one
				// watch per reconnect - once every timeoutSeconds for as long as
				// the admin takes to trigger cutover.
				originalStartCutover := "no"
				fmt.Printf("Info: Entering event loop for pod %s\n", r.PodName)
				for event := range watch.ResultChan() {
					pod, ok := event.Object.(*corev1.Pod)
					if !ok {
						fmt.Printf("Error: Received non-pod event for pod %s: %v\n", r.PodName, event.Object)
						continue
					}
					if cutover, ok := pod.Labels["startCutover"]; ok {
						if cutover != originalStartCutover {
							fmt.Printf("Info: Label changed for pod %s: %s -> %s\n", r.PodName, originalStartCutover, cutover)
							// Never send blindly. This goroutine outlives main, so a
							// bare send races with shutdown; selecting on ctx.Done()
							// means a late event is dropped instead of panicking.
							select {
							case <-ctx.Done():
								watch.Stop()
								return
							case ch <- cutover:
								fmt.Printf("Info: Sent label %s for pod %s to channel\n", cutover, r.PodName)
								originalStartCutover = cutover
							}
						}
					}
				}
				watch.Stop()
				fmt.Printf("Info: Watch channel closed for pod %s after ~%d seconds, retrying...\n", r.PodName, timeoutSeconds)
				time.Sleep(5 * time.Second)
			}
		}
	}()
}
