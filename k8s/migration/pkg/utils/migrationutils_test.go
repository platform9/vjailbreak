package utils

import (
	"testing"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeEvent(reason, message string) corev1.Event {
	return corev1.Event{
		Reason:        reason,
		Message:       message,
		LastTimestamp: metav1.Now(),
	}
}

func makeMigration() *vjailbreakv1alpha1.Migration {
	return &vjailbreakv1alpha1.Migration{}
}

func TestCreateFailedCondition(t *testing.T) {
	tests := []struct {
		name           string
		events         []corev1.Event
		wantConditions int
		wantFailed     bool
	}{
		{
			name: "cleanup event sets Failed condition",
			events: []corev1.Event{
				makeEvent(constants.MigrationReason, "failed to create target instance: timeout. Trying to perform cleanup"),
			},
			wantConditions: 1,
			wantFailed:     true,
		},
		{
			name: "bare EventMessageMigrationFailed sets Failed condition",
			events: []corev1.Event{
				makeEvent(constants.MigrationReason, constants.EventMessageMigrationFailed),
			},
			wantConditions: 1,
			wantFailed:     true,
		},
		{
			name: "lowercase 'failed to' no longer triggers Failed condition",
			events: []corev1.Event{
				makeEvent(constants.MigrationReason, "VM created despite CreateTargetInstance error (failed to create VM: context deadline exceeded), skipping cleanup"),
			},
			wantConditions: 0,
			wantFailed:     false,
		},
		{
			name: "capital 'Failed to' warning does not trigger Failed condition",
			events: []corev1.Event{
				makeEvent(constants.MigrationReason, "Warning: Failed to disconnect source VM network interfaces: timeout"),
			},
			wantConditions: 0,
			wantFailed:     false,
		},
		{
			name: "wrong reason ignored",
			events: []corev1.Event{
				makeEvent("SomeOtherReason", constants.EventMessageMigrationFailed),
			},
			wantConditions: 0,
			wantFailed:     false,
		},
		{
			name:           "empty event list",
			events:         []corev1.Event{},
			wantConditions: 0,
			wantFailed:     false,
		},
		{
			name: "only last cleanup event wins (multiple cleanup events)",
			events: []corev1.Event{
				makeEvent(constants.MigrationReason, "error A. Trying to perform cleanup"),
				makeEvent(constants.MigrationReason, "error B. Trying to perform cleanup"),
			},
			wantConditions: 1,
			wantFailed:     true,
		},
		{
			name: "mix of matching and non-matching events",
			events: []corev1.Event{
				makeEvent(constants.MigrationReason, "VM created successfully: ID: abc-123"),
				makeEvent(constants.MigrationReason, "Warning: Failed to disconnect source VM: timeout"),
				makeEvent(constants.MigrationReason, "failed to create instance. Trying to perform cleanup"),
			},
			wantConditions: 1,
			wantFailed:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			migration := makeMigration()
			eventList := &corev1.EventList{Items: tt.events}

			got := CreateFailedCondition(migration, eventList)

			if len(got) != tt.wantConditions {
				t.Errorf("got %d conditions, want %d", len(got), tt.wantConditions)
			}

			hasFailed := false
			for _, c := range got {
				if c.Type == constants.MigrationConditionTypeFailed {
					hasFailed = true
					break
				}
			}
			if hasFailed != tt.wantFailed {
				t.Errorf("hasFailed = %v, want %v", hasFailed, tt.wantFailed)
			}
		})
	}
}

func TestCreateSucceededCondition(t *testing.T) {
	tests := []struct {
		name         string
		events       []corev1.Event
		wantMigrated bool
	}{
		{
			name: "VM created successfully event sets Migrated condition",
			events: []corev1.Event{
				makeEvent(constants.MigrationReason, "VM created successfully: ID: abc-123"),
			},
			wantMigrated: true,
		},
		{
			name: "recovery path message also sets Migrated condition",
			events: []corev1.Event{
				makeEvent(constants.MigrationReason, "VM created successfully: ID: abc-123 (recovered from timeout)"),
			},
			wantMigrated: true,
		},
		{
			name: "wrong reason ignored",
			events: []corev1.Event{
				makeEvent("Other", "VM created successfully: ID: abc-123"),
			},
			wantMigrated: false,
		},
		{
			name: "cleanup event does not set Migrated condition",
			events: []corev1.Event{
				makeEvent(constants.MigrationReason, "failed to create instance. Trying to perform cleanup"),
			},
			wantMigrated: false,
		},
		{
			name:         "empty events",
			events:       []corev1.Event{},
			wantMigrated: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			migration := makeMigration()
			eventList := &corev1.EventList{Items: tt.events}

			got := CreateSucceededCondition(migration, eventList)

			hasMigrated := false
			for _, c := range got {
				if c.Type == constants.MigrationConditionTypeMigrated {
					hasMigrated = true
					break
				}
			}
			if hasMigrated != tt.wantMigrated {
				t.Errorf("hasMigrated = %v, want %v", hasMigrated, tt.wantMigrated)
			}
		})
	}
}

func TestCreateFailedCondition_StripsCleanupBoilerplate(t *testing.T) {
	tests := []struct {
		name            string
		eventMessage    string
		expectedMessage string
	}{
		{
			name: "virt-v2v free space error strips trailing period and cleanup boilerplate",
			eventMessage: "Failed to migrate VM: failed to convert disks: failed to run virt-v2v: " +
				"failed to run virt-v2v-in-place: exit status 1: virt-v2v-in-place: error: not enough free space " +
				"for conversion on filesystem '/corefiles'.  0.0 MB free < 10 MB needed.. Trying to perform cleanup",
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
			name:            "message with cleanup suffix but no trailing period before it is cleaned",
			eventMessage:    "Failed to migrate VM: failed to run nbdcopy: exit status 1: nbdkit: vddk[1]: error: some vddk error. Trying to perform cleanup",
			expectedMessage: "Failed to migrate VM: failed to run nbdcopy: exit status 1: nbdkit: vddk[1]: error: some vddk error",
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

func TestCreateDataCopiedCondition(t *testing.T) {
	tests := []struct {
		name          string
		events        []corev1.Event
		wantCondition bool
	}{
		{
			name: "DataCopied event creates DataCopied condition",
			events: []corev1.Event{
				makeEvent(constants.MigrationReason, constants.EventMessageDataCopied),
			},
			wantCondition: true,
		},
		{
			name: "wrong reason ignored",
			events: []corev1.Event{
				makeEvent("OtherReason", constants.EventMessageDataCopied),
			},
			wantCondition: false,
		},
		{
			name:          "empty events produces no condition",
			events:        []corev1.Event{},
			wantCondition: false,
		},
		{
			name: "unrelated event ignored",
			events: []corev1.Event{
				makeEvent(constants.MigrationReason, "VM created successfully"),
			},
			wantCondition: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			migration := makeMigration()
			eventList := &corev1.EventList{Items: tt.events}

			got := CreateDataCopiedCondition(migration, eventList)

			hasCondition := false
			for _, c := range got {
				if c.Type == constants.MigrationConditionTypeDataCopied {
					hasCondition = true
					break
				}
			}
			if hasCondition != tt.wantCondition {
				t.Errorf("hasDataCopiedCondition = %v, want %v", hasCondition, tt.wantCondition)
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
