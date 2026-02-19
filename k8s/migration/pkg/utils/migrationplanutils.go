// Package utils provides utility functions for migration plan management and validation.
// It includes functions for generating Kubernetes-compatible resource names, validating
// migration plans between VMware and OpenStack environments, managing migration-related
// resources like config maps, and ensuring proper resource naming conventions.
// These utilities support the core migration planning process, including name conversion,
// path handling, and validation of migration specifications.
package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/pkg/errors"
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

// GetFirstbootConfigMapName generates a config map name for a migration
func GetFirstbootConfigMapName(vmname string) string {
	return fmt.Sprintf("firstboot-config-%s", vmname)
}

// ConvertToK8sName converts a name to be Kubernetes-compatible
func ConvertToK8sName(name string) (string, error) {
	// Convert to lowercase
	name = strings.ToLower(name)
	// Replace separators with hyphens
	re := regexp.MustCompile(`[_\s]`)
	name = re.ReplaceAllString(name, "-")
	// Remove all characters that are not lowercase alphanumeric or hyphens
	re = regexp.MustCompile(`[^a-z0-9\-]`)
	name = re.ReplaceAllString(name, "")

	// Truncate to 63 characters, as we prepend v2v-helper- to the name
	if len(name) > constants.K8sNameMaxLength {
		name = name[:constants.K8sNameMaxLength]
	}
	// if last character is not alphanumeric, remove it
	if len(name) > 0 && !unicode.IsLetter(rune(name[len(name)-1])) && !unicode.IsNumber(rune(name[len(name)-1])) {
		name = name[:len(name)-1]
	}

	// Remove leading and trailing hyphens
	name = strings.Trim(name, "-")

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
	vmk8sname, err := GetK8sCompatibleVMWareObjectName(vmname, credName)
	if err != nil {
		return "", errors.Wrap(err, "failed to convert vm name to k8s name")
	}
	return fmt.Sprintf("v2v-helper-%s-%s", vmk8sname[:min(len(vmk8sname), constants.MaxJobNameLength)], GenerateSha256Hash(vmname)[:constants.HashSuffixLength]), nil
}

// GenerateSha256Hash generates a SHA256 hash of the input string
func GenerateSha256Hash(input string) string {
	sha256Hash := sha256.New()
	sha256Hash.Write([]byte(input))
	hashStr := hex.EncodeToString(sha256Hash.Sum(nil))

	// Check if the last character is already alphanumeric
	lastChar := hashStr[len(hashStr)-1]
	if (lastChar >= '0' && lastChar <= '9') || (lastChar >= 'a' && lastChar <= 'z') || (lastChar >= 'A' && lastChar <= 'Z') {
		return hashStr
	}

	// Replace the last character with an alphanumeric one
	// Use the first character of the hash as a seed to select a replacement
	seed := int(hashStr[0])
	alphanumeric := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	replacement := alphanumeric[seed%len(alphanumeric)]

	return hashStr[:len(hashStr)-1] + string(replacement)
}

// ValidateMixedOSWithFirstboot checks if a migration plan has both Windows and Linux VMs
// when a firstboot script is provided. Returns an error if this condition is detected.
func ValidateMixedOSWithFirstboot(migrationplan *vjailbreakv1alpha1.MigrationPlan, vmMachines []*vjailbreakv1alpha1.VMwareMachine) error {
	// If no firstboot script is provided, no validation needed
	firstbootScript := strings.TrimSpace(migrationplan.Spec.FirstBootScript)
	defaultScript := `echo "Add your startup script here!"`
	
	// Skip validation if script is empty or is the default placeholder
	if firstbootScript == "" || firstbootScript == defaultScript {
		return nil
	}

	// Collect unique OS families from all VMs
	osFamilies := make(map[string]bool)
	for _, vmMachine := range vmMachines {
		osFamily := strings.ToLower(strings.TrimSpace(vmMachine.Spec.VMInfo.OSFamily))
		if osFamily == "" || osFamily == "unknown" {
			continue
		}
		
		// Normalize OS family to either "windows" or "linux"
		if strings.Contains(osFamily, "windows") {
			osFamilies["windows"] = true
		} else if strings.Contains(osFamily, "linux") || strings.Contains(osFamily, "centos") || 
			strings.Contains(osFamily, "rhel") || strings.Contains(osFamily, "ubuntu") ||
			strings.Contains(osFamily, "debian") || strings.Contains(osFamily, "fedora") {
			osFamilies["linux"] = true
		}
	}

	// Check if we have both Windows and Linux VMs
	if osFamilies["windows"] && osFamilies["linux"] {
		return fmt.Errorf("firstboot scripts cannot be used when migrating both Windows and Linux VMs together. " +
			"Please either: (1) remove the firstboot script, (2) migrate Windows and Linux VMs separately, " +
			"or (3) create separate migration plans for each OS type")
	}

	return nil
}
