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
