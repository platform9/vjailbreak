package utils

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"

	"github.com/platform9/vjailbreak/k8s/migration/pkg/constants"
	"k8s.io/apimachinery/pkg/util/validation"
)

// MigrationNameFromVMName generates a migration name from a VM name
func MigrationNameFromVMName(vmname string) string {
	return fmt.Sprintf("migration-%s", vmname)
}

// GetMigrationConfigMapName generates a config map name for a migration
func GetMigrationConfigMapName(vmname string) string {
	return fmt.Sprintf("migration-config-%s", vmname)
}

// ConvertToK8sName converts a name to be Kubernetes-compatible
func ConvertToK8sName(name string) (string, error) {
	// Convert to lowercase
	name = strings.ToLower(name)
	// Replace separators with hyphens
	re := regexp.MustCompile(`[_\s]`)
	name = re.ReplaceAllString(name, "-")
	// Remove all characters that are not lowercase alphanumeric, hyphens, or periods
	re = regexp.MustCompile(`[^a-z0-9\-.]`)
	name = re.ReplaceAllString(name, "")
	// Remove leading and trailing hyphens
	name = strings.Trim(name, "-")
	// Truncate to 242 characters, as we prepend v2v-helper- to the name
	if len(name) > constants.NameMaxLength {
		name = name[:constants.NameMaxLength]
	}
	nameerrors := validation.IsQualifiedName(name)
	if len(nameerrors) == 0 {
		return name, nil
	}
	return name, fmt.Errorf("name '%s' is not a valid K8s name: %v", name, nameerrors)
}

// NewHostPathType creates a new HostPathType from a string
func NewHostPathType(pathType string) *corev1.HostPathType {
	hostPathType := corev1.HostPathType(pathType)
	return &hostPathType
}

// ValidateMigrationPlan validates a MigrationPlan object
func ValidateMigrationPlan(migrationplan *vjailbreakv1alpha1.MigrationPlan) error {
	// Validate Time Field
	if migrationplan.Spec.MigrationStrategy.VMCutoverStart.After(migrationplan.Spec.MigrationStrategy.VMCutoverEnd.Time) {
		return fmt.Errorf("cutover start time is after cutover end time")
	}

	// If advanced options are set, then there should only be 1 VM in the migrationplan
	if !reflect.DeepEqual(migrationplan.Spec.AdvancedOptions, vjailbreakv1alpha1.AdvancedOptions{}) &&
		(len(migrationplan.Spec.VirtualMachines) != 1 || len(migrationplan.Spec.VirtualMachines[0]) != 1) {
		return fmt.Errorf(`advanced options can only be set for a single VM.
			Please remove advanced options or reduce the number of VMs in the migrationplan`)
	}
	return nil
}
