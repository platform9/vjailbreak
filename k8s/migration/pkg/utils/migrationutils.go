// Package utils provides utility functions for VMware to OpenStack migration operations.
// It includes functions for managing migration status conditions, tracking migration progress,
// handling events related to migrations, and other migration lifecycle management functions.
// These utilities support the core migration process between VMware and OpenStack environments,
// including validation, data copying, migration execution, and failure handling.
package utils

import (
	"slices"
	"sort"
	"strings"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MigrationUtils defines the interface for migration utility functions.
type MigrationUtils interface {
	// CreateValidatedCondition creates a validated condition for the migration.
	CreateValidatedCondition(migration *vjailbreakv1alpha1.Migration, eventList *corev1.EventList) []corev1.PodCondition

	// CreateDataCopyCondition creates a data copy condition for the migration.
	CreateDataCopyCondition(migration *vjailbreakv1alpha1.Migration, eventList *corev1.EventList) []corev1.PodCondition

	// CreateMigratingCondition creates a migrated condition for the migration.
	CreateMigratingCondition(migration *vjailbreakv1alpha1.Migration, eventList *corev1.EventList) []corev1.PodCondition

	// SetCutoverLabel sets the cutover label based on the initiateCutover flag.
	SetCutoverLabel(initiateCutover bool, currentLabel string) string

	// SplitEventStringOnComma splits a string by comma and returns a slice of substrings.
	SplitEventStringOnComma(input string) (string, string)

	// GetSatusConditions returns the status conditions of the migration.
	GetSatusConditions(migration *vjailbreakv1alpha1.Migration) []corev1.PodCondition

	// GetConditonIndex returns the index of the condition in the conditions slice.
	GetConditonIndex(conditions []corev1.PodCondition, conditionType corev1.PodConditionType, reasons ...string) int

	// GeneratePodCondition generates a pod condition.
	GeneratePodCondition(conditionType corev1.PodConditionType,
		status corev1.ConditionStatus,
		reason, message string,
		timestamp metav1.Time) *corev1.PodCondition

	// SortConditionsByLastTransitionTime sorts conditions by LastTransitionTime.
	SortConditionsByLastTransitionTime(conditions []corev1.PodCondition)
}

// CreateValidatedCondition creates a validated condition for a migration
func CreateValidatedCondition(migration *vjailbreakv1alpha1.Migration, eventList *corev1.EventList) []corev1.PodCondition {
	existingConditions := migration.Status.Conditions
	for i := 0; i < len(eventList.Items); i++ {
		if eventList.Items[i].Reason != constants.MigrationReason || eventList.Items[i].Message != "Creating volumes in OpenStack" {
			continue
		}

		idx := GetConditonIndex(existingConditions, constants.MigrationConditionTypeValidated, constants.MigrationReason)
		statuscondition := GeneratePodCondition(constants.MigrationConditionTypeValidated,
			corev1.ConditionTrue,
			constants.MigrationReason,
			"Migration validated successfully",
			eventList.Items[i].LastTimestamp)

		if idx == -1 {
			existingConditions = append(existingConditions, *statuscondition)
		} else {
			existingConditions[idx] = *statuscondition
		}
		break
	}
	return existingConditions
}

// CreateDataCopyCondition creates a data copy condition for a migration
func CreateDataCopyCondition(migration *vjailbreakv1alpha1.Migration, eventList *corev1.EventList) []corev1.PodCondition {
	existingConditions := migration.Status.Conditions
	for i := 0; i < len(eventList.Items); i++ {
		if eventList.Items[i].Reason != constants.MigrationReason || !strings.Contains(eventList.Items[i].Message, "Copying disk") {
			continue
		}
		reason, message := SplitEventStringOnComma(eventList.Items[i].Message)
		idx := GetConditonIndex(existingConditions, constants.MigrationConditionTypeDataCopy, reason)
		statuscondition := GeneratePodCondition(constants.MigrationConditionTypeDataCopy,
			corev1.ConditionTrue,
			reason,
			message,
			eventList.Items[i].LastTimestamp)

		if idx == -1 {
			existingConditions = append(existingConditions, *statuscondition)
		} else {
			existingConditions[idx] = *statuscondition
		}
		break
	}
	return existingConditions
}

// CreateMigratingCondition creates a migrating condition for a migration
func CreateMigratingCondition(migration *vjailbreakv1alpha1.Migration, eventList *corev1.EventList) []corev1.PodCondition {
	existingConditions := migration.Status.Conditions
	for i := 0; i < len(eventList.Items); i++ {
		if eventList.Items[i].Reason != constants.MigrationReason || eventList.Items[i].Message != "Converting disk" {
			continue
		}

		idx := GetConditonIndex(existingConditions, constants.MigrationConditionTypeMigrating, constants.MigrationReason)
		statuscondition := GeneratePodCondition(constants.MigrationConditionTypeMigrating,
			corev1.ConditionTrue,
			constants.MigrationReason,
			"Migrating VM from VMware to OpenStack",
			eventList.Items[i].LastTimestamp)

		if idx == -1 {
			existingConditions = append(existingConditions, *statuscondition)
		} else {
			existingConditions[idx] = *statuscondition
		}
	}
	return existingConditions
}

// CreatePodRunningCondition creates a condition marking the instant the migration pod
// actually started running, read directly from the pod's own Status.StartTime (a native
// Kubernetes field) rather than from an event.
func CreatePodRunningCondition(migration *vjailbreakv1alpha1.Migration, pod *corev1.Pod) []corev1.PodCondition {
	existingConditions := migration.Status.Conditions
	if pod.Status.StartTime == nil {
		return existingConditions
	}

	idx := GetConditonIndex(existingConditions, constants.MigrationConditionTypePodRunning, constants.MigrationReason)
	statuscondition := GeneratePodCondition(constants.MigrationConditionTypePodRunning,
		corev1.ConditionTrue,
		constants.MigrationReason,
		"Migration started running",
		*pod.Status.StartTime)

	if idx == -1 {
		existingConditions = append(existingConditions, *statuscondition)
	} else {
		existingConditions[idx] = *statuscondition
	}
	return existingConditions
}

// createSingleEventCondition scans eventList for the first event matching reason/messageCheck
// and stamps it as a single fixed-message condition on the migration - the shape shared by
// CreateCutoverTriggeredCondition and CreateSucceededCondition (only the message-match test,
// condition type, and stored message text vary between them).
func createSingleEventCondition(
	existingConditions []corev1.PodCondition,
	eventList *corev1.EventList,
	messageMatches func(message string) bool,
	conditionType corev1.PodConditionType,
	conditionMessage string,
) []corev1.PodCondition {
	for i := 0; i < len(eventList.Items); i++ {
		if eventList.Items[i].Reason != constants.MigrationReason || !messageMatches(eventList.Items[i].Message) {
			continue
		}

		idx := GetConditonIndex(existingConditions, conditionType, constants.MigrationReason)
		statuscondition := GeneratePodCondition(conditionType,
			corev1.ConditionTrue,
			constants.MigrationReason,
			conditionMessage,
			eventList.Items[i].LastTimestamp)

		if idx == -1 {
			existingConditions = append(existingConditions, *statuscondition)
		} else {
			existingConditions[idx] = *statuscondition
		}
		break
	}
	return existingConditions
}

// CreateCutoverTriggeredCondition creates a condition marking the instant an admin actually
// triggered cutover for a migration (as opposed to 'AwaitingAdminCutOver', which only marks
// that the migration is waiting for that to happen, however long it takes).
func CreateCutoverTriggeredCondition(migration *vjailbreakv1alpha1.Migration, eventList *corev1.EventList) []corev1.PodCondition {
	return createSingleEventCondition(migration.Status.Conditions, eventList,
		func(message string) bool { return strings.HasPrefix(message, "Admin cutover triggered") },
		constants.MigrationConditionTypeCutoverTriggered,
		"Admin cutover triggered")
}

// isFailureEventMessage reports whether an event message represents a genuine terminal
// migration failure, matched case-insensitively. Warning messages are excluded since they
// don't represent a real failure.
func isFailureEventMessage(msg string) bool {
	trimmed := strings.TrimSpace(msg)
	if strings.HasPrefix(trimmed, constants.EventMessageWarningPrefix) {
		return false
	}
	lower := strings.ToLower(trimmed)
	return strings.Contains(lower, strings.ToLower(constants.EventMessageMigrationFailed)) ||
		strings.Contains(lower, strings.ToLower(constants.EventMessageFailed))
}

// CreateFailedCondition creates or updates a failed condition for a migration based on events.
// It analyzes event logs to identify failure reasons and updates the migration's status conditions accordingly.
func CreateFailedCondition(migration *vjailbreakv1alpha1.Migration, eventList *corev1.EventList) []corev1.PodCondition {
	existingConditions := migration.Status.Conditions
	for i := 0; i < len(eventList.Items); i++ {
		if eventList.Items[i].Reason != constants.MigrationReason || !isFailureEventMessage(eventList.Items[i].Message) {
			continue
		}

		idx := GetConditonIndex(existingConditions, constants.MigrationConditionTypeFailed, constants.MigrationReason)
		statuscondition := GeneratePodCondition(constants.MigrationConditionTypeFailed,
			corev1.ConditionTrue,
			constants.MigrationReason,
			cleanFailureMessage(eventList.Items[i].Message),
			eventList.Items[i].LastTimestamp)

		if idx == -1 {
			existingConditions = append(existingConditions, *statuscondition)
		} else {
			existingConditions[idx] = *statuscondition
		}
	}
	return existingConditions
}

// cleanFailureMessage strips the generic "Trying to perform cleanup" boilerplate
// that migobj.cleanup() appends to the actual v2v-helper error (see
// constants.EventMessageMigrationFailed), so the Failed condition's message -
// and therefore what the UI shows in its progress tooltip and failure banner -
// is just the concise, actionable root cause rather than the full wrapped
// error chain plus cleanup-status prose.
func cleanFailureMessage(msg string) string {
	msg = strings.TrimSpace(msg)

	if idx := strings.Index(msg, ". "+constants.EventMessageMigrationFailed); idx != -1 {
		msg = msg[:idx]
	} else {
		msg = strings.TrimSuffix(msg, constants.EventMessageMigrationFailed)
	}
	msg = strings.TrimSpace(msg)
	msg = strings.TrimSuffix(msg, ".")
	return strings.TrimSpace(msg)
}

// CreateSucceededCondition creates or updates a succeeded condition for a migration based on events.
func CreateSucceededCondition(migration *vjailbreakv1alpha1.Migration, eventList *corev1.EventList) []corev1.PodCondition {
	return createSingleEventCondition(migration.Status.Conditions, eventList,
		func(message string) bool { return strings.Contains(message, "VM created successfully") },
		constants.MigrationConditionTypeMigrated,
		"VM successfully migrated from VMware to OpenStack")
}

// SetCutoverLabel sets the cutover label for a migration
func SetCutoverLabel(initiateCutover bool, currentLabel string) string {
	// If initiateCutover is true, return the current label
	if initiateCutover {
		return currentLabel
	}
	// If initiateCutover is false, set the label to "yes" (User should not be able to change it)
	return constants.StartCutOverYes
}

// SetLDMBootStatusLabel publishes the operator's gate answer to the pod label the
// helper watches. Write-once: the answer is acted on immediately, so a later
// change has nothing left to affect. Unknown values are ignored rather than
// published, so a typo cannot resolve the gate.
func SetLDMBootStatusLabel(ldmBootStatus, currentLabel string) string {
	if currentLabel != "" {
		return currentLabel
	}
	switch ldmBootStatus {
	case constants.LDMBootStatusSuccess, constants.LDMBootStatusFinish, constants.LDMBootStatusFailed:
		return ldmBootStatus
	default:
		return currentLabel
	}
}

// LDMGateHoldsPhase reports whether a migration is at the LDM boot gate, or still
// rebuilding after the operator answered, and so is not Succeeded yet. Decided
// from the label, not event order: the gate event and "VM created successfully"
// land in the same second and sort.Slice is not stable. Only the "still
// promoting" case compares timestamps, which is safe - promotion takes minutes.
func LDMGateHoldsPhase(events []corev1.Event, ldmBootStatus string) bool {
	newest := func(marker string) (metav1.Time, bool) {
		var found metav1.Time
		var ok bool
		for i := range events {
			if !strings.Contains(events[i].Message, marker) {
				continue
			}
			if !ok || found.Before(&events[i].CreationTimestamp) {
				found, ok = events[i].CreationTimestamp, true
			}
		}
		return found, ok
	}

	if _, gateSeen := newest(constants.EventMessageWaitingForLDMBootSuccess); !gateSeen {
		return false
	}

	// Unanswered: definitively still waiting, whatever order the events arrived in.
	if ldmBootStatus == "" {
		return true
	}

	// Answered. "finish" and "failed" complete immediately and never emit this, so
	// its absence means there is nothing left to wait for.
	promotedAt, promoting := newest(constants.EventMessagePromotingLDMGuest)
	if !promoting {
		return false
	}

	// The promotion is done once the rebuilt VM reports success, which is strictly
	// newer than the promotion starting. The success event from the first, SATA
	// build is older and must not be mistaken for it.
	succeededAt, succeeded := newest(constants.EventMessageMigrationSucessful)
	return !succeeded || !promotedAt.Before(&succeededAt)
}

// SplitEventStringOnComma splits a string by comma and returns a slice of substrings.
func SplitEventStringOnComma(input string) (reason, message string) {
	// SplitEventStringOnComma splits a string by comma and returns a slice of substrings.
	parts := strings.Split(input, ",")
	if len(parts) > 1 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	return strings.TrimSpace(parts[0]), ""
}

// GetSatusConditions returns the status conditions of a migration
func GetSatusConditions(migration *vjailbreakv1alpha1.Migration) []corev1.PodCondition {
	// GetSatusConditions returns the status conditions of a migration
	return migration.Status.Conditions
}

// GetConditonIndex returns the index of a condition in the conditions slice based on type and reasons
func GetConditonIndex(conditions []corev1.PodCondition, conditionType corev1.PodConditionType, reasons ...string) int {
	// GetConditonIndex returns the index of a condition in the conditions slice based on type and reasons
	for i, c := range conditions {
		if c.Type == conditionType && slices.Contains(reasons, c.Reason) {
			return i
		}
	}
	return -1
}

// GeneratePodCondition creates a new pod condition with the given parameters
func GeneratePodCondition(conditionType corev1.PodConditionType,
	status corev1.ConditionStatus,
	reason, message string,
	timestamp metav1.Time) *corev1.PodCondition {
	// GeneratePodCondition creates a new pod condition with the given parameters
	return &corev1.PodCondition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: timestamp,
	}
}

// SortConditionsByLastTransitionTime sorts conditions by LastTransitionTime
func SortConditionsByLastTransitionTime(conditions []corev1.PodCondition) {
	// SortConditionsByLastTransitionTime sorts conditions by LastTransitionTime
	sort.Slice(conditions, func(i, j int) bool {
		return conditions[i].LastTransitionTime.Before(&conditions[j].LastTransitionTime)
	})
}

// CreateDataCopiedCondition creates a DataCopied condition for DataOnly migrations when disk copy and conversion complete.
func CreateDataCopiedCondition(migration *vjailbreakv1alpha1.Migration, eventList *corev1.EventList) []corev1.PodCondition {
	existingConditions := migration.Status.Conditions
	for i := 0; i < len(eventList.Items); i++ {
		if eventList.Items[i].Reason != constants.MigrationReason || !strings.Contains(eventList.Items[i].Message, constants.EventMessageDataCopied) {
			continue
		}

		idx := GetConditonIndex(existingConditions, constants.MigrationConditionTypeDataCopied, constants.MigrationReason)
		statuscondition := GeneratePodCondition(constants.MigrationConditionTypeDataCopied,
			corev1.ConditionTrue,
			constants.MigrationReason,
			"DataOnly: disk copy and conversion complete",
			eventList.Items[i].LastTimestamp)

		if idx == -1 {
			existingConditions = append(existingConditions, *statuscondition)
		} else {
			existingConditions[idx] = *statuscondition
		}
		break
	}
	return existingConditions
}

// CreateStorageAcceleratedCopyCondition creates a StorageAcceleratedCopy condition for a migration based on StorageAcceleratedCopy-specific events
func CreateStorageAcceleratedCopyCondition(migration *vjailbreakv1alpha1.Migration, eventList *corev1.EventList) []corev1.PodCondition {
	existingConditions := migration.Status.Conditions
	for i := 0; i < len(eventList.Items); i++ {
		if eventList.Items[i].Reason != constants.MigrationReason {
			continue
		}

		message := eventList.Items[i].Message
		var conditionMessage string
		var conditionReason string

		// Check for StorageAcceleratedCopy specific events
		switch {
		case strings.Contains(message, "Connecting to ESXi"):
			conditionReason = "ConnectingToESXi"
			conditionMessage = message
		case strings.Contains(message, "Creating/updating initiator group"):
			conditionReason = "MappingInitiatorGroup"
			conditionMessage = message
		case strings.Contains(message, "Creating target volume"):
			conditionReason = "CreatingVolume"
			conditionMessage = message
		case strings.Contains(message, "Cinder managing the volume"):
			conditionReason = "ImportingToCinder"
			conditionMessage = message
		case strings.Contains(message, "Mapping target volume"):
			conditionReason = "MappingVolume"
			conditionMessage = message
		case strings.Contains(message, "Waiting for target volume"):
			conditionReason = "RescanningStorage"
			conditionMessage = message
		default:
			continue
		}

		idx := GetConditonIndex(existingConditions, constants.MigrationConditionTypeStorageAcceleratedCopy, conditionReason)
		statuscondition := GeneratePodCondition(constants.MigrationConditionTypeStorageAcceleratedCopy,
			corev1.ConditionTrue,
			conditionReason,
			conditionMessage,
			eventList.Items[i].LastTimestamp)

		if idx == -1 {
			existingConditions = append(existingConditions, *statuscondition)
		} else {
			existingConditions[idx] = *statuscondition
		}
		break
	}
	return existingConditions
}
