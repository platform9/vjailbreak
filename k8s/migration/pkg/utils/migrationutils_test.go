package utils

import (
	"strings"
	"testing"
	"time"

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
			// This exact message is logged via utils.PrintLog, not migobj.logMessage, so it
			// never actually reaches eventList in production — but matching is otherwise
			// correct: lowercase "failed to" is a real failure signal (e.g. virt-v2v/nbd
			// root-cause messages).
			name: "lowercase 'failed to' triggers Failed condition",
			events: []corev1.Event{
				makeEvent(constants.MigrationReason, "VM created despite CreateTargetInstance error (failed to create VM: context deadline exceeded), skipping cleanup"),
			},
			wantConditions: 1,
			wantFailed:     true,
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
		{
			// v2v-helper's top-level error handler (main.go) reports
			// terminal failures as "Failed to migrate VM: <err>." without ever routing through
			// migobj.cleanup(), so this never contains "Trying to perform cleanup". Requiring
			// that phrase made these genuine failures invisible on the migrations page and details page.
			name: "top-level 'Failed to migrate VM' event sets Failed condition",
			events: []corev1.Event{
				makeEvent(constants.MigrationReason, "Failed to migrate VM: failed to convert disks: some error. "),
			},
			wantConditions: 1,
			wantFailed:     true,
		},
		{
			// failures during early setup (before MigrateVM even
			// starts, e.g. vCenter/OpenStack connection issues) never call cleanup() either.
			name: "early setup 'Failed to' event sets Failed condition",
			events: []corev1.Event{
				makeEvent(constants.MigrationReason, "Failed to validate vCenter connection: dial tcp: timeout"),
			},
			wantConditions: 1,
			wantFailed:     true,
		},
		{
			// Non-fatal mid-migration warnings (migrate.go's get-bootable-partition.sh /
			// generate-mount-persistence.sh warnings) must still not be mistaken for failures.
			name: "warning-prefixed 'Failed to' from a mid-migration script does not trigger Failed condition",
			events: []corev1.Event{
				makeEvent(constants.MigrationReason, "Warning: Failed to run get-bootable-partition.sh: exit status 1"),
			},
			wantConditions: 0,
			wantFailed:     false,
		},
		{
			// The reporter raises an event to Warning severity only on uppercase
			// "WARNING", so non-fatal messages are worded that way. The exemption has
			// to match it or they would be read as failures - and these interpolate
			// wrapped errors, which almost always contain "failed to".
			name: "uppercase WARNING prefix is exempt just like Warning:",
			events: []corev1.Event{
				makeEvent(constants.MigrationReason,
					"WARNING: failed to delete the virtio probe volume abc-123: request failed"),
			},
			wantConditions: 0,
			wantFailed:     false,
		},
		{
			// The exemption is a prefix, not a substring: a real failure that merely
			// mentions a warning later in the text must still be caught.
			name: "WARNING appearing mid-message does not exempt a real failure",
			events: []corev1.Event{
				makeEvent(constants.MigrationReason,
					"failed to create target instance (see WARNING above). Trying to perform cleanup"),
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

func TestCreatePodRunningCondition(t *testing.T) {
	startedAt := metav1.Now()

	tests := []struct {
		name          string
		pod           *corev1.Pod
		wantCondition bool
	}{
		{
			name:          "pod with StartTime creates PodRunning condition",
			pod:           &corev1.Pod{Status: corev1.PodStatus{StartTime: &startedAt}},
			wantCondition: true,
		},
		{
			name:          "pod without StartTime (still scheduled, not running) produces no condition",
			pod:           &corev1.Pod{Status: corev1.PodStatus{StartTime: nil}},
			wantCondition: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			migration := makeMigration()

			got := CreatePodRunningCondition(migration, tt.pod)

			hasCondition := false
			for _, c := range got {
				if c.Type == constants.MigrationConditionTypePodRunning {
					hasCondition = true
					if tt.pod.Status.StartTime != nil && !c.LastTransitionTime.Time.Equal(tt.pod.Status.StartTime.Time) {
						t.Errorf("LastTransitionTime = %v, want %v", c.LastTransitionTime.Time, tt.pod.Status.StartTime.Time)
					}
					break
				}
			}
			if hasCondition != tt.wantCondition {
				t.Errorf("hasPodRunningCondition = %v, want %v", hasCondition, tt.wantCondition)
			}
		})
	}
}

// TestCreateSingleEventCondition covers createSingleEventCondition directly - the shared
// helper CreateCutoverTriggeredCondition and CreateSucceededCondition both delegate to - so
// the append-vs-update and first-match-wins behavior only needs testing once instead of being
// duplicated across every function that uses it.
func TestCreateSingleEventCondition(t *testing.T) {
	matchesFoo := func(message string) bool { return message == "foo happened" }

	t.Run("no matching event leaves conditions unchanged", func(t *testing.T) {
		eventList := &corev1.EventList{Items: []corev1.Event{
			makeEvent(constants.MigrationReason, "something else"),
		}}

		got := createSingleEventCondition(nil, eventList, matchesFoo, "FooCondition", "foo done")

		if len(got) != 0 {
			t.Errorf("got %d conditions, want 0", len(got))
		}
	})

	t.Run("wrong reason ignored even if message matches", func(t *testing.T) {
		eventList := &corev1.EventList{Items: []corev1.Event{
			makeEvent("OtherReason", "foo happened"),
		}}

		got := createSingleEventCondition(nil, eventList, matchesFoo, "FooCondition", "foo done")

		if len(got) != 0 {
			t.Errorf("got %d conditions, want 0", len(got))
		}
	})

	t.Run("matching event appends a new condition with the fixed message and type", func(t *testing.T) {
		ts := metav1.Now()
		eventList := &corev1.EventList{Items: []corev1.Event{
			{Reason: constants.MigrationReason, Message: "foo happened", LastTimestamp: ts},
		}}

		got := createSingleEventCondition(nil, eventList, matchesFoo, "FooCondition", "foo done")

		if len(got) != 1 {
			t.Fatalf("got %d conditions, want 1", len(got))
		}
		if got[0].Type != "FooCondition" || got[0].Message != "foo done" || got[0].Reason != constants.MigrationReason {
			t.Errorf("condition = %+v, want type=FooCondition message='foo done' reason=%s", got[0], constants.MigrationReason)
		}
		if !got[0].LastTransitionTime.Time.Equal(ts.Time) {
			t.Errorf("LastTransitionTime = %v, want %v", got[0].LastTransitionTime.Time, ts.Time)
		}
	})

	t.Run("matching event updates an existing condition of the same type in place, not duplicated", func(t *testing.T) {
		existing := []corev1.PodCondition{
			{Type: "FooCondition", Reason: constants.MigrationReason, Message: "stale", LastTransitionTime: metav1.Now()},
		}
		newTs := metav1.NewTime(metav1.Now().Add(time.Minute))
		eventList := &corev1.EventList{Items: []corev1.Event{
			{Reason: constants.MigrationReason, Message: "foo happened", LastTimestamp: newTs},
		}}

		got := createSingleEventCondition(existing, eventList, matchesFoo, "FooCondition", "foo done")

		if len(got) != 1 {
			t.Fatalf("got %d conditions, want 1 (updated in place, not appended)", len(got))
		}
		if got[0].Message != "foo done" {
			t.Errorf("Message = %q, want %q", got[0].Message, "foo done")
		}
		if !got[0].LastTransitionTime.Time.Equal(newTs.Time) {
			t.Errorf("LastTransitionTime not updated: got %v, want %v", got[0].LastTransitionTime.Time, newTs.Time)
		}
	})

	t.Run("only the first matching event is used, matching the break-on-first-match behavior", func(t *testing.T) {
		ts1 := metav1.Now()
		ts2 := metav1.NewTime(ts1.Add(time.Hour))
		eventList := &corev1.EventList{Items: []corev1.Event{
			{Reason: constants.MigrationReason, Message: "foo happened", LastTimestamp: ts1},
			{Reason: constants.MigrationReason, Message: "foo happened", LastTimestamp: ts2},
		}}

		got := createSingleEventCondition(nil, eventList, matchesFoo, "FooCondition", "foo done")

		if len(got) != 1 {
			t.Fatalf("got %d conditions, want 1", len(got))
		}
		if !got[0].LastTransitionTime.Time.Equal(ts1.Time) {
			t.Errorf("LastTransitionTime = %v, want the FIRST matching event's time %v", got[0].LastTransitionTime.Time, ts1.Time)
		}
	})
}

func TestCreateCutoverTriggeredCondition(t *testing.T) {
	tests := []struct {
		name          string
		events        []corev1.Event
		wantCondition bool
	}{
		{
			name: "'Admin cutover triggered' creates CutoverTriggered condition",
			events: []corev1.Event{
				makeEvent(constants.MigrationReason, "Admin cutover triggered"),
			},
			wantCondition: true,
		},
		{
			name: "'Admin cutover triggered during wait' variant also matches",
			events: []corev1.Event{
				makeEvent(constants.MigrationReason, "Admin cutover triggered during wait"),
			},
			wantCondition: true,
		},
		{
			name: "wrong reason ignored",
			events: []corev1.Event{
				makeEvent("OtherReason", "Admin cutover triggered"),
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
				makeEvent(constants.MigrationReason, constants.EventMessageWaitingForAdminCutOver),
			},
			wantCondition: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			migration := makeMigration()
			eventList := &corev1.EventList{Items: tt.events}

			got := CreateCutoverTriggeredCondition(migration, eventList)

			hasCondition := false
			for _, c := range got {
				if c.Type == constants.MigrationConditionTypeCutoverTriggered {
					hasCondition = true
					break
				}
			}
			if hasCondition != tt.wantCondition {
				t.Errorf("hasCutoverTriggeredCondition = %v, want %v", hasCondition, tt.wantCondition)
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

// The SATA build is not a finished migration - the operator still has to answer the
// boot gate - so its event must not be readable as terminal. If these two ever
// overlap, a reconcile landing before the gate event exists resolves the phase to
// Succeeded and nothing wakes the controller to correct it.
func TestLDMSATAMessageIsNotTerminal(t *testing.T) {
	if strings.Contains(constants.EventMessageLDMGuestCreatedOnSATA, constants.EventMessageMigrationSucessful) {
		t.Fatalf("%q must not contain %q",
			constants.EventMessageLDMGuestCreatedOnSATA, constants.EventMessageMigrationSucessful)
	}

	// And it must not light up the Migrated condition either.
	migration := makeMigration()
	eventList := &corev1.EventList{Items: []corev1.Event{
		makeEvent(constants.MigrationReason, constants.EventMessageLDMGuestCreatedOnSATA+": ID: abc-123"),
	}}
	for _, c := range CreateSucceededCondition(migration, eventList) {
		if c.Type == constants.MigrationConditionTypeMigrated {
			t.Error("the SATA build set the Migrated condition before the gate was answered")
		}
	}
}

func TestLDMHeldPhase(t *testing.T) {
	at := func(message string, offset time.Duration) corev1.Event {
		e := makeEvent(constants.MigrationReason, message)
		e.CreationTimestamp = metav1.NewTime(time.Now().Add(offset))
		return e
	}

	t.Run("no gate event means this is not an LDM migration", func(t *testing.T) {
		events := []corev1.Event{at(constants.EventMessageMigrationSucessful, 0)}
		if _, held := LDMHeldPhase(events, ""); held {
			t.Error("held the phase for a migration that never reached the gate")
		}
	})

	t.Run("unanswered gate holds regardless of event order", func(t *testing.T) {
		// The two events land in the same second and sort.Slice is not stable, so
		// both orders must give the same answer. This is the bug that made the
		// phase come out Succeeded on some reconciles and correct on others.
		success := at(constants.EventMessageMigrationSucessful, 0)
		gate := at(constants.EventMessageWaitingForLDMBootSuccess, 0)

		for _, events := range [][]corev1.Event{{success, gate}, {gate, success}} {
			phase, held := LDMHeldPhase(events, "")
			if !held || phase != vjailbreakv1alpha1.VMMigrationPhaseWaitingForLDMBootSuccess {
				t.Errorf("want WaitingForLDMBootSuccess held, got %q held=%v", phase, held)
			}
		}
	})

	t.Run("finish releases immediately", func(t *testing.T) {
		events := []corev1.Event{
			at(constants.EventMessageMigrationSucessful, 0),
			at(constants.EventMessageWaitingForLDMBootSuccess, 0),
		}
		if _, held := LDMHeldPhase(events, constants.LDMBootStatusFinish); held {
			t.Error("held the phase after the operator chose to stay on SATA")
		}
	})

	t.Run("aged out gate event does not hold the phase", func(t *testing.T) {
		// The gate has no time limit, so a wait longer than the API server's event
		// TTL (1h by default) leaves no gate event to match. The gate must release
		// on absence rather than hold; the finish path re-emits the terminal event
		// so the controller still has something newer to resolve against.
		events := []corev1.Event{at(constants.EventMessageMigrationSucessful, 0)}
		if _, held := LDMHeldPhase(events, constants.LDMBootStatusFinish); held {
			t.Error("held the phase after the gate event had expired")
		}
	})

	t.Run("success holds while the rebuild is running", func(t *testing.T) {
		// The only success event so far is the older one from the SATA build; it
		// must not be mistaken for the rebuilt VM reporting in.
		events := []corev1.Event{
			at(constants.EventMessageMigrationSucessful, -10*time.Minute),
			at(constants.EventMessageWaitingForLDMBootSuccess, -10*time.Minute),
			at(constants.EventMessagePromotingLDMGuest, -1*time.Minute),
		}
		phase, held := LDMHeldPhase(events, constants.LDMBootStatusSuccess)
		if !held {
			t.Fatal("released the phase while the VM was still being rebuilt")
		}
		// Must not report the gate here: nothing is waiting on the operator, and the
		// UI would show a stale "action required" for the whole rebuild.
		if phase != vjailbreakv1alpha1.VMMigrationPhasePromotingToVirtio {
			t.Errorf("want PromotingToVirtio during the rebuild, got %q", phase)
		}
	})

	t.Run("success releases once the rebuilt VM reports success", func(t *testing.T) {
		events := []corev1.Event{
			at(constants.EventMessageMigrationSucessful, -10*time.Minute),
			at(constants.EventMessageWaitingForLDMBootSuccess, -10*time.Minute),
			at(constants.EventMessagePromotingLDMGuest, -5*time.Minute),
			at(constants.EventMessageMigrationSucessful, 0),
		}
		if _, held := LDMHeldPhase(events, constants.LDMBootStatusSuccess); held {
			t.Error("held the phase after the rebuild completed")
		}
	})
}

func TestCopyMethodRequiresVDDK(t *testing.T) {
	tests := []struct {
		name              string
		storageCopyMethod string
		want              bool
	}{
		{
			name:              "empty defaults to the normal path, which drives nbdkit's vddk plugin",
			storageCopyMethod: "",
			want:              true,
		},
		{
			name:              "normal needs VDDK",
			storageCopyMethod: "normal",
			want:              true,
		},
		{
			name:              "StorageAcceleratedCopy clones via vmkfstools on the ESXi host",
			storageCopyMethod: constants.StorageCopyMethod,
			want:              false,
		},
		{
			name:              "HotAdd serves with qemu-nbd on the proxy VM",
			storageCopyMethod: constants.HotAddCopyMethod,
			want:              false,
		},
		{
			name:              "unrecognised method falls back to requiring VDDK",
			storageCopyMethod: "SomeFutureMethod",
			want:              true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CopyMethodRequiresVDDK(tt.storageCopyMethod); got != tt.want {
				t.Errorf("CopyMethodRequiresVDDK(%q) = %v, want %v", tt.storageCopyMethod, got, tt.want)
			}
		})
	}
}
