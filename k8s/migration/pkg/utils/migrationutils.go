package utils

import (
	"slices"
	"sort"
	"strings"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/constants"
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
			"Migrating VM from VMware to Openstack",
			eventList.Items[i].LastTimestamp)

		if idx == -1 {
			existingConditions = append(existingConditions, *statuscondition)
		} else {
			existingConditions[idx] = *statuscondition
		}
	}
	return existingConditions
}

// SetCutoverLabel sets the cutover label for a migration
func SetCutoverLabel(initiateCutover bool, currentLabel string) string {
	if initiateCutover {
		if currentLabel != constants.StartCutOverYes {
			currentLabel = constants.StartCutOverYes
		}
	} else {
		if currentLabel != constants.StartCutOverNo {
			currentLabel = constants.StartCutOverNo
		}
	}
	return currentLabel
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
