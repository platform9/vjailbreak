package utils

import (
	"strconv"

	"github.com/platform9/vjailbreak/k8s/migration/pkg/constants"
)

// VjailbreakSettings holds the settings for vjailbreak components
type VjailbreakSettings struct {
	// ChangedBlocksCopyIterationThreshold is the number of iterations to copy changed blocks
	ChangedBlocksCopyIterationThreshold int
	// VMActiveWaitIntervalSeconds is the interval to wait for VM to become active
	VMActiveWaitIntervalSeconds int
	// VMActiveWaitRetryLimit is the number of retries to wait for VM to become active
	VMActiveWaitRetryLimit int
	// VolumeAvailableWaitIntervalSeconds is the interval to wait for volume to become available
	VolumeAvailableWaitIntervalSeconds int
	// VolumeAvailableWaitRetryLimit is the number of retries to wait for volume to become available
	VolumeAvailableWaitRetryLimit int
	// DefaultMigrationMethod is the default migration method (hot/cold)
	DefaultMigrationMethod string
	// VCenterScanConcurrencyLimit is the max number of vms to scan at the same time
	VCenterScanConcurrencyLimit int
	// CleanupVolumesAfterConvertFailure is whether to cleanup volumes after disk convert failure
	CleanupVolumesAfterConvertFailure bool
	// PopulateVMwareMachineFlavors is whether to automatically populate VMwareMachine objects with OpenStack flavors
	PopulateVMwareMachineFlavors bool
	// VCenterLoginRetryLimit is the number of retries for vcenter login
	VCenterLoginRetryLimit int
}

// atoi is a helper function to convert string to int with a default value of 0
func atoi(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return i
}

// getDefaultSettings returns default vjailbreak settings
func getDefaultSettings() *VjailbreakSettings {
	return &VjailbreakSettings{
		ChangedBlocksCopyIterationThreshold: constants.ChangedBlocksCopyIterationThreshold,
		VMActiveWaitIntervalSeconds:         constants.VMActiveWaitIntervalSeconds,
		VMActiveWaitRetryLimit:              constants.VMActiveWaitRetryLimit,
		VolumeAvailableWaitIntervalSeconds:  constants.VolumeAvailableWaitIntervalSeconds,
		VolumeAvailableWaitRetryLimit:       constants.VolumeAvailableWaitRetryLimit,
		DefaultMigrationMethod:              constants.DefaultMigrationMethod,
		VCenterScanConcurrencyLimit:         constants.VCenterScanConcurrencyLimit,
		CleanupVolumesAfterConvertFailure:   constants.CleanupVolumesAfterConvertFailure,
		PopulateVMwareMachineFlavors:        constants.PopulateVMwareMachineFlavors,
	}
}
