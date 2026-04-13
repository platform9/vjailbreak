// Package utils provides utility functions for migration plan management and validation.
// It includes functions for generating Kubernetes-compatible resource names, validating
// migration plans between VMware and OpenStack environments, managing migration-related
// resources like config maps, and ensuring proper resource naming conventions.
// These utilities support the core migration planning process, including name conversion,
// path handling, and validation of migration specifications.
package utils

import (
	"fmt"

	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"

	"github.com/platform9/vjailbreak/pkg/common/constants"
	commonutils "github.com/platform9/vjailbreak/pkg/common/utils"
)

// MigrationNameFromVMName generates a migration name from a VM name
func MigrationNameFromVMName(vmname string) string {
	return fmt.Sprintf("migration-%s", vmname)
}

// GetMigrationConfigMapName generates a config map name for a migration
func GetMigrationConfigMapName(vmname string) string {
	return fmt.Sprintf("migration-config-%s", vmname)
}

// GetFirstbootConfigMapName generates a config map name for a migration
func GetFirstbootConfigMapName(vmname string) string {
	return fmt.Sprintf("firstboot-config-%s", vmname)
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

	// Check if granular options (VM-specific) are set
	hasGranularOptions := len(migrationplan.Spec.AdvancedOptions.GranularVolumeTypes) > 0 ||
		len(migrationplan.Spec.AdvancedOptions.GranularNetworks) > 0 ||
		len(migrationplan.Spec.AdvancedOptions.GranularPorts) > 0

	// If granular options are set, then there should only be 1 VM in the migrationplan
	// Periodic sync options are plan-level and can apply to multiple VMs
	if hasGranularOptions &&
		(len(migrationplan.Spec.VirtualMachines) != 1 || len(migrationplan.Spec.VirtualMachines[0]) != 1) {
		return fmt.Errorf(`granular options (volumes/networks/ports) can only be set for a single VM.
			Please remove granular options or reduce the number of VMs in the migrationplan`)
	}
	return nil
}

// GetJobNameForVMName generates a unique name for a job resource
func GetJobNameForVMName(vmname string, credName string) (string, error) {
	vmk8sname, err := commonutils.GetK8sCompatibleVMWareObjectName(vmname, credName)
	if err != nil {
		return "", errors.Wrap(err, "failed to convert vm name to k8s name")
	}
	return fmt.Sprintf("v2v-helper-%s-%s", vmk8sname[:min(len(vmk8sname), constants.MaxJobNameLength)], commonutils.GenerateSha256Hash(vmname)[:constants.HashSuffixLength]), nil
}
