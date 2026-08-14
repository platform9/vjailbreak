/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = ginkgo.Describe("Migration Controller", func() {
	ginkgo.Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		migration := &vjailbreakv1alpha1.Migration{}

		ginkgo.BeforeEach(func() {
			ginkgo.By("creating the custom resource for the Kind Migration")
			err := k8sClient.Get(ctx, typeNamespacedName, migration)
			if err != nil && errors.IsNotFound(err) {
				resource := &vjailbreakv1alpha1.Migration{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					// TODO(user): Specify other spec details if needed.
				}
				gomega.Expect(k8sClient.Create(ctx, resource)).To(gomega.Succeed())
			}
		})

		ginkgo.AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &vjailbreakv1alpha1.Migration{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			ginkgo.By("Cleanup the specific resource instance Migration")
			gomega.Expect(k8sClient.Delete(ctx, resource)).To(gomega.Succeed())
		})

		ginkgo.It("should successfully reconcile the resource", func() {
			ginkgo.By("Reconciling the created resource")
			controllerReconciler := &MigrationReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})

func TestIsPodRunningOrTerminal(t *testing.T) {
	tests := []struct {
		name  string
		phase corev1.PodPhase
		want  bool
	}{
		{name: "pending pod is not yet running or terminal", phase: corev1.PodPending, want: false},
		{name: "unknown pod is not yet running or terminal", phase: corev1.PodUnknown, want: false},
		{name: "running pod counts", phase: corev1.PodRunning, want: true},
		{name: "failed pod counts (terminal)", phase: corev1.PodFailed, want: true},
		{name: "succeeded pod counts (terminal)", phase: corev1.PodSucceeded, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := &corev1.Pod{Status: corev1.PodStatus{Phase: tt.phase}}
			if got := isPodRunningOrTerminal(pod); got != tt.want {
				t.Errorf("isPodRunningOrTerminal(phase=%s) = %v, want %v", tt.phase, got, tt.want)
			}
		})
	}
}

func TestLDMGateNeedsTerminalFallback(t *testing.T) {
	tests := []struct {
		name     string
		podPhase corev1.PodPhase
		phase    vjailbreakv1alpha1.VMMigrationPhase
		want     bool
	}{
		{
			// The reported bug: success events aged out while the operator decided.
			name:     "succeeded pod still pinned at the gate needs the fallback",
			podPhase: corev1.PodSucceeded,
			phase:    vjailbreakv1alpha1.VMMigrationPhaseWaitingForLDMBootSuccess,
			want:     true,
		},
		{
			name:     "running pod at the gate must not be advanced",
			podPhase: corev1.PodRunning,
			phase:    vjailbreakv1alpha1.VMMigrationPhaseWaitingForLDMBootSuccess,
			want:     false,
		},
		{
			// "Rollback" exits non-zero and must never be reported as successful.
			name:     "failed pod at the gate must not be advanced",
			podPhase: corev1.PodFailed,
			phase:    vjailbreakv1alpha1.VMMigrationPhaseWaitingForLDMBootSuccess,
			want:     false,
		},
		{
			// The rebuild has the same exposure: its events can age out while the VM
			// is stopped, deleted and recreated.
			name:     "succeeded pod still promoting to virtio needs the fallback",
			podPhase: corev1.PodSucceeded,
			phase:    vjailbreakv1alpha1.VMMigrationPhasePromotingToVirtio,
			want:     true,
		},
		{
			name:     "running pod promoting to virtio must not be advanced",
			podPhase: corev1.PodRunning,
			phase:    vjailbreakv1alpha1.VMMigrationPhasePromotingToVirtio,
			want:     false,
		},
		{
			name:     "succeeded pod in an unrelated phase is left alone",
			podPhase: corev1.PodSucceeded,
			phase:    vjailbreakv1alpha1.VMMigrationPhaseConvertingDisk,
			want:     false,
		},
		{
			name:     "already succeeded is not re-advanced",
			podPhase: corev1.PodSucceeded,
			phase:    vjailbreakv1alpha1.VMMigrationPhaseSucceeded,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := &corev1.Pod{Status: corev1.PodStatus{Phase: tt.podPhase}}
			if got := ldmGateNeedsTerminalFallback(pod, tt.phase); got != tt.want {
				t.Errorf("ldmGateNeedsTerminalFallback(pod=%s, phase=%s) = %v, want %v",
					tt.podPhase, tt.phase, got, tt.want)
			}
		})
	}
}

func TestIsMigrationAppEvent(t *testing.T) {
	tests := []struct {
		name   string
		reason string
		want   bool
	}{
		{name: "v2v-helper's own event is accepted", reason: "Migration", want: true},
		{
			// A kubelet image-pull-retry event's Reason is "Failed",
			// not "Migration" — even though its Message text can coincidentally contain
			// "Failed to pull image...", which SetupMigrationPhase's keyword matching would
			// otherwise mistake for a real migration failure.
			name:   "kubelet image-pull-failure event is rejected",
			reason: "Failed",
			want:   false,
		},
		{name: "kubelet scheduling-backoff event is rejected", reason: "FailedScheduling", want: false},
		{name: "kubelet image-pull-backoff event is rejected", reason: "BackOff", want: false},
		{name: "empty reason is rejected", reason: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := corev1.Event{Reason: tt.reason}
			if got := isMigrationAppEvent(event); got != tt.want {
				t.Errorf("isMigrationAppEvent(reason=%q) = %v, want %v", tt.reason, got, tt.want)
			}
		})
	}
}
