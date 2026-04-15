// Package utils provides shared utility functions for Kubernetes name compatibility and vCenter object naming.
package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/pkg/errors"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	"k8s.io/apimachinery/pkg/util/validation"
)

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

// GetK8sCompatibleVMWareObjectName returns a k8s compatible name for a vCenter object
func GetK8sCompatibleVMWareObjectName(vCenterObjectName, credName string) (string, error) {
	// get a unique string for the cluster + credentials
	vCenterObjectCredsName := fmt.Sprintf("%s-%s", vCenterObjectName, credName)

	// hash the cluster + credentials string
	hash := GenerateSha256Hash(vCenterObjectCredsName)[:constants.HashSuffixLength]

	// convert the cluster name to a k8s name
	k8sClusterName, err := ConvertToK8sName(vCenterObjectName)
	if err != nil {
		return "", errors.Wrap(err, "failed to convert cluster name to k8s name")
	}

	// truncate the k8s cluster name to the max length
	name := fmt.Sprintf("%s-%s", k8sClusterName[:min(len(k8sClusterName), constants.VMNameMaxLength)], hash)
	return name[:min(len(name), constants.K8sNameMaxLength)], nil
}

// GetVMK8sCompatibleName returns a k8s compatible name for a VM, using name-<moid> as
// the stable unique key before hashing. This ensures uniqueness even when two VMs share the same display name.
func GetVMK8sCompatibleName(vmName, vmid, credName string) (string, error) {
	vmNameForK8s := fmt.Sprintf("%s-%s", vmName, strings.TrimPrefix(vmid, "vm-"))
	return GetK8sCompatibleVMWareObjectName(vmNameForK8s, credName)
}
