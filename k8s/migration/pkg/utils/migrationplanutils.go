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

// DetectScriptOSType detects whether a script is intended for Windows or Linux
// Returns "windows", "linux", or "unknown"
func DetectScriptOSType(script string) string {
	if script == "" {
		return "unknown"
	}

	script = strings.TrimSpace(script)
	lines := strings.Split(script, "\n")

	// Check first few lines for shebang or strong indicators
	for i := 0; i < min(len(lines), 5); i++ {
		line := strings.TrimSpace(lines[i])
		
		// Linux shebang indicators
		if strings.HasPrefix(line, "#!/bin/bash") ||
			strings.HasPrefix(line, "#!/bin/sh") ||
			strings.HasPrefix(line, "#!/usr/bin/env bash") ||
			strings.HasPrefix(line, "#!/usr/bin/env sh") ||
			strings.HasPrefix(line, "#!/usr/bin/python") {
			return "linux"
		}

		// Windows batch file indicators
		if strings.HasPrefix(line, "@echo off") ||
			strings.HasPrefix(line, "REM ") ||
			strings.HasPrefix(line, "rem ") {
			return "windows"
		}

		// PowerShell indicators
		if strings.Contains(line, "param(") ||
			strings.Contains(line, "Param(") ||
			regexp.MustCompile(`\$[A-Za-z_]`).MatchString(line) {
			return "windows"
		}
	}

	// Count Windows-specific vs Linux-specific commands
	windowsScore := 0
	linuxScore := 0

	scriptLower := strings.ToLower(script)

	// Windows-specific patterns
	windowsPatterns := []string{
		"cmd.exe", "powershell", ".bat", ".ps1", "set ", "echo off",
		"reg add", "reg delete", "net start", "net stop", "sc query",
		"tasklist", "taskkill", "wmic", "systeminfo", "ipconfig",
		"\\windows\\", "c:\\", "%programfiles%", "$env:",
	}
	for _, pattern := range windowsPatterns {
		if strings.Contains(scriptLower, pattern) {
			windowsScore++
		}
	}

	// Linux-specific patterns
	linuxPatterns := []string{
		"/bin/", "/usr/", "/etc/", "/var/", "/home/",
		"apt-get", "yum install", "dnf install", "systemctl",
		"chmod ", "chown ", "grep ", "sed ", "awk ",
		"export ", "source ", "alias ", "sudo ",
	}
	for _, pattern := range linuxPatterns {
		if strings.Contains(scriptLower, pattern) {
			linuxScore++
		}
	}

	// Determine based on scores
	if windowsScore > linuxScore && windowsScore > 0 {
		return "windows"
	}
	if linuxScore > windowsScore && linuxScore > 0 {
		return "linux"
	}

	return "unknown"
}

// IsScriptCompatibleWithOS checks if a script is compatible with the given OS family
func IsScriptCompatibleWithOS(script string, osFamily string) bool {
	scriptType := DetectScriptOSType(script)

	// If we can't detect the script type, allow it (backward compatibility)
	if scriptType == "unknown" {
		return true
	}

	// Normalize OS family
	osLower := strings.ToLower(osFamily)

	// Check compatibility
	if scriptType == "windows" && strings.Contains(osLower, "windows") {
		return true
	}
	if scriptType == "linux" && (strings.Contains(osLower, "linux") || strings.Contains(osLower, "centos") || 
		strings.Contains(osLower, "rhel") || strings.Contains(osLower, "ubuntu")) {
		return true
	}

	return false
}
