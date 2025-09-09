package k8sutils

import (
	"context"
	"os"
	"strconv"

	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8stypes "k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	client "sigs.k8s.io/controller-runtime/pkg/client"
)

func GetInclusterClient() (client.Client, error) {
	// Create a direct Kubernetes client
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get in-cluster config")
	}
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(vjailbreakv1alpha1.AddToScheme(scheme))
	clientset, err := client.New(config, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get in-cluster config")
	}

	return clientset, err
}

func GetVMwareMachine(ctx context.Context, vmName string) (*vjailbreakv1alpha1.VMwareMachine, error) {
	client, err := GetInclusterClient()
	if err != nil {
		return nil, err
	}
	vmwareMachine := &vjailbreakv1alpha1.VMwareMachine{}
	vmK8sName, err := GetVMwareMachineName()
	if err != nil {
		return nil, err
	}
	err = client.Get(ctx, types.NamespacedName{
		Name:      vmK8sName,
		Namespace: constants.NamespaceMigrationSystem,
	}, vmwareMachine)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get vmware machine")
	}
	return vmwareMachine, nil
}

func GetVMwareMachineName() (string, error) {
	vmK8sName := os.Getenv("VMWARE_MACHINE_OBJECT_NAME")
	if vmK8sName == "" {
		return "", errors.New("VMWARE_MACHINE_OBJECT_NAME environment variable is not set")
	}
	return vmK8sName, nil
}

func GetRDMDisk(ctx context.Context, diskName string) (*vjailbreakv1alpha1.RDMDisk, error) {
	client, err := GetInclusterClient()
	if err != nil {
		return nil, err
	}
	rdmDisk := &vjailbreakv1alpha1.RDMDisk{}
	if err != nil {
		return nil, err
	}
	err = client.Get(ctx, types.NamespacedName{
		Name:      diskName,
		Namespace: constants.NamespaceMigrationSystem,
	}, rdmDisk)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get vmware machine")
	}
	return rdmDisk, nil
}

// atoi is a helper function to convert string to int with a default value of 0
func atoi(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return i
}

func GetVjailbreakSettings(ctx context.Context, k8sClient client.Client) (*VjailbreakSettings, error) {
	vjailbreakSettingsCM := &corev1.ConfigMap{}
	if err := k8sClient.Get(ctx, k8stypes.NamespacedName{Name: constants.VjailbreakSettingsConfigMapName, Namespace: constants.NamespaceMigrationSystem}, vjailbreakSettingsCM); err != nil {
		return nil, errors.Wrap(err, "failed to get vjailbreak settings configmap")
	}

	if vjailbreakSettingsCM.Data == nil {
		return &VjailbreakSettings{
			ChangedBlocksCopyIterationThreshold: constants.ChangedBlocksCopyIterationThreshold,
			VMActiveWaitIntervalSeconds:         constants.VMActiveWaitIntervalSeconds,
			VMActiveWaitRetryLimit:              constants.VMActiveWaitRetryLimit,
			DefaultMigrationMethod:              constants.DefaultMigrationMethod,
			VCenterScanConcurrencyLimit:         constants.VCenterScanConcurrencyLimit,
			CleanupVolumesAfterConvertFailure:   constants.CleanupVolumesAfterConvertFailure,
			PopulateVMwareMachineFlavors:        constants.PopulateVMwareMachineFlavors,
			VolumeAvailableWaitIntervalSeconds:  constants.VolumeAvailableWaitIntervalSeconds,
			VolumeAvailableWaitRetryLimit:       constants.VolumeAvailableWaitRetryLimit,
			VCenterLoginRetryLimit:              constants.VCenterLoginRetryLimit,
		}, nil
	}

	if vjailbreakSettingsCM.Data["CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD"] == "" {
		vjailbreakSettingsCM.Data["CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD"] = strconv.Itoa(constants.ChangedBlocksCopyIterationThreshold)
	}

	if vjailbreakSettingsCM.Data["VM_ACTIVE_WAIT_INTERVAL_SECONDS"] == "" {
		vjailbreakSettingsCM.Data["VM_ACTIVE_WAIT_INTERVAL_SECONDS"] = strconv.Itoa(constants.VMActiveWaitIntervalSeconds)
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

	return &VjailbreakSettings{
		ChangedBlocksCopyIterationThreshold: atoi(vjailbreakSettingsCM.Data["CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD"]),
		VMActiveWaitIntervalSeconds:         atoi(vjailbreakSettingsCM.Data["VM_ACTIVE_WAIT_INTERVAL_SECONDS"]),
		VMActiveWaitRetryLimit:              atoi(vjailbreakSettingsCM.Data["VM_ACTIVE_WAIT_RETRY_LIMIT"]),
		DefaultMigrationMethod:              vjailbreakSettingsCM.Data["DEFAULT_MIGRATION_METHOD"],
		VCenterScanConcurrencyLimit:         atoi(vjailbreakSettingsCM.Data["VCENTER_SCAN_CONCURRENCY_LIMIT"]),
		CleanupVolumesAfterConvertFailure:   vjailbreakSettingsCM.Data["CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE"] == "true",
		PopulateVMwareMachineFlavors:        vjailbreakSettingsCM.Data["POPULATE_VMWARE_MACHINE_FLAVORS"] == "true",
		VCenterLoginRetryLimit:              atoi(vjailbreakSettingsCM.Data["VCENTER_LOGIN_RETRY_LIMIT"]),
	}, nil
}
