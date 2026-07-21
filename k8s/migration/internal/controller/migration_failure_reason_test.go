package controller

import (
	"testing"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestExtractFailureReason(t *testing.T) {
	tests := []struct {
		name     string
		phase    vjailbreakv1alpha1.VMMigrationPhase
		messages []string // newest first, matching GetEventsSorted ordering
		expected string
	}{
		{
			name:  "virt-v2v free space error without cleanup path",
			phase: vjailbreakv1alpha1.VMMigrationPhaseFailed,
			messages: []string{
				"Failed to migrate VM: failed to convert disks: failed to run virt-v2v: " +
					"failed to run virt-v2v-in-place: exit status 1: virt-v2v-in-place: error: not enough free space " +
					"for conversion on filesystem '/corefiles'.  0.0 MB free < 10 MB needed. ",
			},
			expected: "Failed to migrate VM: failed to convert disks: failed to run virt-v2v: " +
				"failed to run virt-v2v-in-place: exit status 1: virt-v2v-in-place: error: not enough free space " +
				"for conversion on filesystem '/corefiles'.  0.0 MB free < 10 MB needed",
		},
		{
			name:  "cleanup boilerplate is stripped",
			phase: vjailbreakv1alpha1.VMMigrationPhaseFailed,
			messages: []string{
				"failed to convert volumes: failed to run virt-v2v: exit status 1: some root cause. Trying to perform cleanup",
			},
			expected: "failed to convert volumes: failed to run virt-v2v: exit status 1: some root cause",
		},
		{
			name:  "not failed phase clears the reason",
			phase: vjailbreakv1alpha1.VMMigrationPhaseCopying,
			messages: []string{
				"Failed to migrate VM: some old failure from a previous attempt",
			},
			expected: "",
		},
		{
			name:     "failed phase but no matching failure event leaves reason empty",
			phase:    vjailbreakv1alpha1.VMMigrationPhaseFailed,
			messages: []string{"Copying disk 0, Completed: 50%"},
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &MigrationReconciler{}

			migration := &vjailbreakv1alpha1.Migration{}
			migration.Status.Phase = tc.phase
			// Pre-populate to verify the function clears it on non-failed phases.
			migration.Status.FailureReason = "stale-reason-from-earlier-reconcile"

			events := &corev1.EventList{}
			now := metav1.Now()
			for _, msg := range tc.messages {
				events.Items = append(events.Items, corev1.Event{
					Message:        msg,
					FirstTimestamp: now,
					LastTimestamp:  now,
				})
			}

			r.ExtractFailureReason(migration, events)

			if migration.Status.FailureReason != tc.expected {
				t.Errorf("ExtractFailureReason() = %q, want %q", migration.Status.FailureReason, tc.expected)
			}
		})
	}
}
