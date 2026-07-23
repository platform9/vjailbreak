package utils

import (
	"testing"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreateFailedCondition_StripsCleanupBoilerplate(t *testing.T) {
	tests := []struct {
		name            string
		eventMessage    string
		expectedMessage string
	}{
		{
			name: "virt-v2v free space error without cleanup path",
			eventMessage: "Failed to migrate VM: failed to convert disks: failed to run virt-v2v: " +
				"failed to run virt-v2v-in-place: exit status 1: virt-v2v-in-place: error: not enough free space " +
				"for conversion on filesystem '/corefiles'.  0.0 MB free < 10 MB needed. ",
			expectedMessage: "Failed to migrate VM: failed to convert disks: failed to run virt-v2v: " +
				"failed to run virt-v2v-in-place: exit status 1: virt-v2v-in-place: error: not enough free space " +
				"for conversion on filesystem '/corefiles'.  0.0 MB free < 10 MB needed",
		},
		{
			name:            "cleanup boilerplate is stripped",
			eventMessage:    "failed to convert volumes: failed to run virt-v2v: exit status 1: some root cause. Trying to perform cleanup",
			expectedMessage: "failed to convert volumes: failed to run virt-v2v: exit status 1: some root cause",
		},
		{
			name:            "message without trailing period or cleanup suffix is unchanged",
			eventMessage:    "failed to run nbdcopy: exit status 1: nbdkit: vddk[1]: error: some vddk error",
			expectedMessage: "failed to run nbdcopy: exit status 1: nbdkit: vddk[1]: error: some vddk error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			migration := &vjailbreakv1alpha1.Migration{}
			now := metav1.Now()
			eventList := &corev1.EventList{
				Items: []corev1.Event{
					{
						Reason:         constants.MigrationReason,
						Message:        tc.eventMessage,
						LastTimestamp:  now,
						FirstTimestamp: now,
					},
				},
			}

			conditions := CreateFailedCondition(migration, eventList)

			idx := GetConditonIndex(conditions, constants.MigrationConditionTypeFailed, constants.MigrationReason)
			if idx == -1 {
				t.Fatalf("expected a Failed condition to be created, got none")
			}
			if got := conditions[idx].Message; got != tc.expectedMessage {
				t.Errorf("condition message = %q, want %q", got, tc.expectedMessage)
			}
		})
	}
}

func TestCleanFailureMessage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "strips trailing cleanup boilerplate",
			input:    "some error. Trying to perform cleanup",
			expected: "some error",
		},
		{
			name:     "strips trailing period",
			input:    "some error with trailing period.",
			expected: "some error with trailing period",
		},
		{
			name:     "leaves message without boilerplate untouched",
			input:    "some error without trailing punctuation",
			expected: "some error without trailing punctuation",
		},
		{
			name:     "trims surrounding whitespace",
			input:    "  some error.  ",
			expected: "some error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := cleanFailureMessage(tc.input); got != tc.expected {
				t.Errorf("cleanFailureMessage(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}
