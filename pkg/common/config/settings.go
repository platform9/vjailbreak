package config

import (
	"context"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	corev1 "k8s.io/api/core/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// VjailbreakSettings holds configuration settings for vjailbreak operations
type VjailbreakSettings struct {
	ChangedBlocksCopyIterationThreshold int
	PeriodicSyncInterval                string
	VMActiveWaitIntervalSeconds         int
	VMActiveWaitRetryLimit              int
	DefaultMigrationMethod              string
	VCenterScanConcurrencyLimit         int
	CleanupVolumesAfterConvertFailure   bool
	CleanupPortsAfterMigrationFailure   bool
	PopulateVMwareMachineFlavors        bool
	VolumeAvailableWaitIntervalSeconds  int
	VolumeAvailableWaitRetryLimit       int
	VCenterLoginRetryLimit              int
	OpenstackCredsRequeueAfterMinutes   int
	VMwareCredsRequeueAfterMinutes      int
	ValidateRDMOwnerVMs                 bool
	PeriodicSyncMaxRetries              uint64
	PeriodicSyncRetryCap                string
	AutoFstabUpdate                     bool
	AutoPXEBootOnConversion             bool
	V2VHelperPodCPURequest              string
	V2VHelperPodMemoryRequest           string
	V2VHelperPodCPULimit                string
	V2VHelperPodMemoryLimit             string
	V2VHelperPodEphemeralStorageRequest string
	V2VHelperPodEphemeralStorageLimit   string
	Timezone                            string
	NTPServers                          string
	HTTPTimeoutSeconds                  int
}

// Atoi is a helper function to convert string to int with a default value of 0
func Atoi(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return i
}

// GetVjailbreakSettings retrieves vjailbreak configuration settings from a ConfigMap
// and applies default values for any missing settings.
func GetVjailbreakSettings(ctx context.Context, k8sClient client.Client) (*VjailbreakSettings, error) {
	vjailbreakSettingsCM := &corev1.ConfigMap{}
	if err := k8sClient.Get(ctx, k8stypes.NamespacedName{Name: constants.VjailbreakSettingsConfigMapName, Namespace: constants.NamespaceMigrationSystem}, vjailbreakSettingsCM); err != nil {
		return nil, errors.Wrap(err, "failed to get vjailbreak settings configmap")
	}

	if vjailbreakSettingsCM.Data == nil {
		return &VjailbreakSettings{
			ChangedBlocksCopyIterationThreshold: constants.ChangedBlocksCopyIterationThreshold,
			PeriodicSyncInterval:                constants.PeriodicSyncInterval,
			VMActiveWaitIntervalSeconds:         constants.VMActiveWaitIntervalSeconds,
			VMActiveWaitRetryLimit:              constants.VMActiveWaitRetryLimit,
			DefaultMigrationMethod:              constants.DefaultMigrationMethod,
			VCenterScanConcurrencyLimit:         constants.VCenterScanConcurrencyLimit,
			CleanupVolumesAfterConvertFailure:   constants.CleanupVolumesAfterConvertFailure,
			CleanupPortsAfterMigrationFailure:   constants.CleanupPortsAfterMigrationFailure,
			PopulateVMwareMachineFlavors:        constants.PopulateVMwareMachineFlavors,
			VolumeAvailableWaitIntervalSeconds:  constants.VolumeAvailableWaitIntervalSeconds,
			VolumeAvailableWaitRetryLimit:       constants.VolumeAvailableWaitRetryLimit,
			VCenterLoginRetryLimit:              constants.VCenterLoginRetryLimit,
			OpenstackCredsRequeueAfterMinutes:   constants.OpenstackCredsRequeueAfterMinutes,
			VMwareCredsRequeueAfterMinutes:      constants.VMwareCredsRequeueAfterMinutes,
			ValidateRDMOwnerVMs:                 constants.ValidateRDMOwnerVMs,
			PeriodicSyncMaxRetries:              constants.PeriodicSyncMaxRetries,
			PeriodicSyncRetryCap:                constants.PeriodicSyncRetryCap,
			AutoFstabUpdate:                     constants.AutoFstabUpdate,
			AutoPXEBootOnConversion:             constants.AutoPXEBootOnConversionDefault,
			V2VHelperPodCPURequest:              constants.V2VHelperPodCPURequest,
			V2VHelperPodMemoryRequest:           constants.V2VHelperPodMemoryRequest,
			V2VHelperPodCPULimit:                constants.V2VHelperPodCPULimit,
			V2VHelperPodMemoryLimit:             constants.V2VHelperPodMemoryLimit,
			V2VHelperPodEphemeralStorageRequest: constants.V2VHelperPodEphemeralStorageRequest,
			V2VHelperPodEphemeralStorageLimit:   constants.V2VHelperPodEphemeralStorageLimit,
			Timezone:                            "",
			NTPServers:                          "",
			HTTPTimeoutSeconds:                  constants.HTTPTimeoutSeconds,
		}, nil
	}

	if vjailbreakSettingsCM.Data["CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD"] == "" {
		vjailbreakSettingsCM.Data["CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD"] = strconv.Itoa(constants.ChangedBlocksCopyIterationThreshold)
	}
	if vjailbreakSettingsCM.Data["PERIODIC_SYNC_MAX_RETRIES"] == "" {
		vjailbreakSettingsCM.Data["PERIODIC_SYNC_MAX_RETRIES"] = strconv.Itoa(constants.PeriodicSyncMaxRetries)
	}
	if vjailbreakSettingsCM.Data["PERIODIC_SYNC_RETRY_CAP"] == "" {
		vjailbreakSettingsCM.Data["PERIODIC_SYNC_RETRY_CAP"] = constants.PeriodicSyncRetryCap
	}
	if vjailbreakSettingsCM.Data["VM_ACTIVE_WAIT_INTERVAL_SECONDS"] == "" {
		vjailbreakSettingsCM.Data["VM_ACTIVE_WAIT_INTERVAL_SECONDS"] = strconv.Itoa(constants.VMActiveWaitIntervalSeconds)
	}
	if vjailbreakSettingsCM.Data["PERIODIC_SYNC_INTERVAL"] == "" {
		vjailbreakSettingsCM.Data["PERIODIC_SYNC_INTERVAL"] = constants.PeriodicSyncInterval
	}
	if vjailbreakSettingsCM.Data["VM_ACTIVE_WAIT_RETRY_LIMIT"] == "" {
		vjailbreakSettingsCM.Data["VM_ACTIVE_WAIT_RETRY_LIMIT"] = strconv.Itoa(constants.VMActiveWaitRetryLimit)
	}

	if vjailbreakSettingsCM.Data["DEFAULT_MIGRATION_METHOD"] == "" {
		vjailbreakSettingsCM.Data["DEFAULT_MIGRATION_METHOD"] = constants.DefaultMigrationMethod
	}

	if vjailbreakSettingsCM.Data["VCENTER_SCAN_CONCURRENCY_LIMIT"] == "" {
		vjailbreakSettingsCM.Data["VCENTER_SCAN_CONCURRENCY_LIMIT"] = strconv.Itoa(constants.VCenterScanConcurrencyLimit)
	}

	if vjailbreakSettingsCM.Data["CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE"] == "" {
		vjailbreakSettingsCM.Data["CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE"] = strconv.FormatBool(constants.CleanupVolumesAfterConvertFailure)
	}

	if vjailbreakSettingsCM.Data["CLEANUP_PORTS_AFTER_MIGRATION_FAILURE"] == "" {
		vjailbreakSettingsCM.Data["CLEANUP_PORTS_AFTER_MIGRATION_FAILURE"] = strconv.FormatBool(constants.CleanupPortsAfterMigrationFailure)
	}

	if vjailbreakSettingsCM.Data["POPULATE_VMWARE_MACHINE_FLAVORS"] == "" {
		vjailbreakSettingsCM.Data["POPULATE_VMWARE_MACHINE_FLAVORS"] = strconv.FormatBool(constants.PopulateVMwareMachineFlavors)
	}

	if vjailbreakSettingsCM.Data["VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS"] == "" {
		vjailbreakSettingsCM.Data["VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS"] = strconv.Itoa(constants.VolumeAvailableWaitIntervalSeconds)
	}

	if vjailbreakSettingsCM.Data["VOLUME_AVAILABLE_WAIT_RETRY_LIMIT"] == "" {
		vjailbreakSettingsCM.Data["VOLUME_AVAILABLE_WAIT_RETRY_LIMIT"] = strconv.Itoa(constants.VolumeAvailableWaitRetryLimit)
	}

	if vjailbreakSettingsCM.Data["VCENTER_LOGIN_RETRY_LIMIT"] == "" {
		vjailbreakSettingsCM.Data["VCENTER_LOGIN_RETRY_LIMIT"] = strconv.Itoa(constants.VCenterLoginRetryLimit)
	}

	if vjailbreakSettingsCM.Data["OPENSTACK_CREDS_REQUEUE_AFTER_MINUTES"] == "" {
		vjailbreakSettingsCM.Data["OPENSTACK_CREDS_REQUEUE_AFTER_MINUTES"] = strconv.Itoa(constants.OpenstackCredsRequeueAfterMinutes)
	}

	if vjailbreakSettingsCM.Data["VMWARE_CREDS_REQUEUE_AFTER_MINUTES"] == "" {
		vjailbreakSettingsCM.Data["VMWARE_CREDS_REQUEUE_AFTER_MINUTES"] = strconv.Itoa(constants.VMwareCredsRequeueAfterMinutes)
	}

	if vjailbreakSettingsCM.Data[constants.ValidateRDMOwnerVMsKey] == "" {
		vjailbreakSettingsCM.Data[constants.ValidateRDMOwnerVMsKey] = strconv.FormatBool(constants.ValidateRDMOwnerVMs)
	}

	if vjailbreakSettingsCM.Data[constants.AutoFstabUpdateKey] == "" {
		vjailbreakSettingsCM.Data[constants.AutoFstabUpdateKey] = strconv.FormatBool(constants.AutoFstabUpdate)
	}

	if vjailbreakSettingsCM.Data[constants.AutoPXEBootOnConversionKey] == "" {
		vjailbreakSettingsCM.Data[constants.AutoPXEBootOnConversionKey] = strconv.FormatBool(constants.AutoPXEBootOnConversionDefault)
	}

	if vjailbreakSettingsCM.Data[constants.V2VHelperPodCPURequestKey] == "" {
		vjailbreakSettingsCM.Data[constants.V2VHelperPodCPURequestKey] = constants.V2VHelperPodCPURequest
	}

	if vjailbreakSettingsCM.Data[constants.V2VHelperPodMemoryRequestKey] == "" {
		vjailbreakSettingsCM.Data[constants.V2VHelperPodMemoryRequestKey] = constants.V2VHelperPodMemoryRequest
	}

	if vjailbreakSettingsCM.Data[constants.V2VHelperPodCPULimitKey] == "" {
		vjailbreakSettingsCM.Data[constants.V2VHelperPodCPULimitKey] = constants.V2VHelperPodCPULimit
	}

	if vjailbreakSettingsCM.Data[constants.V2VHelperPodMemoryLimitKey] == "" {
		vjailbreakSettingsCM.Data[constants.V2VHelperPodMemoryLimitKey] = constants.V2VHelperPodMemoryLimit
	}

	if vjailbreakSettingsCM.Data[constants.V2VHelperPodEphemeralStorageRequestKey] == "" {
		vjailbreakSettingsCM.Data[constants.V2VHelperPodEphemeralStorageRequestKey] = constants.V2VHelperPodEphemeralStorageRequest
	}

	if vjailbreakSettingsCM.Data[constants.V2VHelperPodEphemeralStorageLimitKey] == "" {
		vjailbreakSettingsCM.Data[constants.V2VHelperPodEphemeralStorageLimitKey] = constants.V2VHelperPodEphemeralStorageLimit
	}

	if vjailbreakSettingsCM.Data["TIMEZONE"] == "" {
		vjailbreakSettingsCM.Data["TIMEZONE"] = ""
	}
	if vjailbreakSettingsCM.Data["NTP_SERVERS"] == "" {
		vjailbreakSettingsCM.Data["NTP_SERVERS"] = ""
	if vjailbreakSettingsCM.Data[constants.HTTPTimeoutSecondsKey] == "" {
		vjailbreakSettingsCM.Data[constants.HTTPTimeoutSecondsKey] = strconv.Itoa(constants.HTTPTimeoutSeconds)
	}

	return &VjailbreakSettings{
		ChangedBlocksCopyIterationThreshold: Atoi(vjailbreakSettingsCM.Data["CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD"]),
		PeriodicSyncInterval:                vjailbreakSettingsCM.Data["PERIODIC_SYNC_INTERVAL"],
		VMActiveWaitIntervalSeconds:         Atoi(vjailbreakSettingsCM.Data["VM_ACTIVE_WAIT_INTERVAL_SECONDS"]),
		VMActiveWaitRetryLimit:              Atoi(vjailbreakSettingsCM.Data["VM_ACTIVE_WAIT_RETRY_LIMIT"]),
		DefaultMigrationMethod:              vjailbreakSettingsCM.Data["DEFAULT_MIGRATION_METHOD"],
		VCenterScanConcurrencyLimit:         Atoi(vjailbreakSettingsCM.Data["VCENTER_SCAN_CONCURRENCY_LIMIT"]),
		CleanupVolumesAfterConvertFailure:   vjailbreakSettingsCM.Data["CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE"] == "true",
		CleanupPortsAfterMigrationFailure:   vjailbreakSettingsCM.Data["CLEANUP_PORTS_AFTER_MIGRATION_FAILURE"] == "true",
		PopulateVMwareMachineFlavors:        vjailbreakSettingsCM.Data["POPULATE_VMWARE_MACHINE_FLAVORS"] == "true",
		VolumeAvailableWaitIntervalSeconds:  Atoi(vjailbreakSettingsCM.Data["VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS"]),
		VolumeAvailableWaitRetryLimit:       Atoi(vjailbreakSettingsCM.Data["VOLUME_AVAILABLE_WAIT_RETRY_LIMIT"]),
		VCenterLoginRetryLimit:              Atoi(vjailbreakSettingsCM.Data["VCENTER_LOGIN_RETRY_LIMIT"]),
		OpenstackCredsRequeueAfterMinutes:   Atoi(vjailbreakSettingsCM.Data["OPENSTACK_CREDS_REQUEUE_AFTER_MINUTES"]),
		VMwareCredsRequeueAfterMinutes:      Atoi(vjailbreakSettingsCM.Data["VMWARE_CREDS_REQUEUE_AFTER_MINUTES"]),
		ValidateRDMOwnerVMs:                 strings.ToLower(strings.TrimSpace(vjailbreakSettingsCM.Data[constants.ValidateRDMOwnerVMsKey])) == "true",
		PeriodicSyncMaxRetries:              uint64(Atoi(vjailbreakSettingsCM.Data["PERIODIC_SYNC_MAX_RETRIES"])),
		PeriodicSyncRetryCap:                vjailbreakSettingsCM.Data["PERIODIC_SYNC_RETRY_CAP"],
		AutoFstabUpdate:                     strings.ToLower(strings.TrimSpace(vjailbreakSettingsCM.Data[constants.AutoFstabUpdateKey])) == "true",
		AutoPXEBootOnConversion:             strings.ToLower(strings.TrimSpace(vjailbreakSettingsCM.Data[constants.AutoPXEBootOnConversionKey])) == "true",
		V2VHelperPodCPURequest:              vjailbreakSettingsCM.Data[constants.V2VHelperPodCPURequestKey],
		V2VHelperPodMemoryRequest:           vjailbreakSettingsCM.Data[constants.V2VHelperPodMemoryRequestKey],
		V2VHelperPodCPULimit:                vjailbreakSettingsCM.Data[constants.V2VHelperPodCPULimitKey],
		V2VHelperPodMemoryLimit:             vjailbreakSettingsCM.Data[constants.V2VHelperPodMemoryLimitKey],
		V2VHelperPodEphemeralStorageRequest: vjailbreakSettingsCM.Data[constants.V2VHelperPodEphemeralStorageRequestKey],
		V2VHelperPodEphemeralStorageLimit:   vjailbreakSettingsCM.Data[constants.V2VHelperPodEphemeralStorageLimitKey],
		Timezone:                            strings.TrimSpace(vjailbreakSettingsCM.Data["TIMEZONE"]),
		NTPServers:                          strings.TrimSpace(vjailbreakSettingsCM.Data["NTP_SERVERS"]),
		HTTPTimeoutSeconds:                  Atoi(vjailbreakSettingsCM.Data[constants.HTTPTimeoutSecondsKey]),
	}, nil
}
