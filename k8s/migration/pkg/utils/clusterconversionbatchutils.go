package utils

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	pkgscope "github.com/platform9/vjailbreak/k8s/migration/pkg/scope"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	commonutils "github.com/platform9/vjailbreak/pkg/common/utils"
	providers "github.com/platform9/vjailbreak/pkg/vpwned/sdk/providers"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// BatchActionType identifies which operator action was requested via annotation.
type BatchActionType string

// Annotation-driven action types for ClusterConversionBatch operator workflows.
const (
	BatchActionTypeTrigger BatchActionType = "trigger"
	BatchActionTypeRetry   BatchActionType = "retry"
	BatchActionTypeSkip    BatchActionType = "skip"
)

// BatchAction represents a single operator action parsed from a ClusterConversionBatch annotation.
type BatchAction struct {
	Type     BatchActionType
	ESXiName string
}

// ComputeRetryBackoff returns the wait duration before the Nth retry attempt.
// Formula: baseSeconds * 2^(retryCount-1)
// retryCount=1 → baseSeconds, retryCount=2 → 2*baseSeconds, etc.
func ComputeRetryBackoff(baseSeconds, retryCount int) time.Duration {
	backoff := baseSeconds
	for i := 1; i < retryCount; i++ {
		backoff *= 2
	}
	return time.Duration(backoff) * time.Second
}

// ProcessBatchAnnotations reads trigger/retry/skip annotations from batch, returns the list of
// actions to perform, and removes processed annotations from batch.Annotations.
func ProcessBatchAnnotations(batch *vjailbreakv1alpha1.ClusterConversionBatch) []BatchAction {
	if batch.Annotations == nil {
		return nil
	}

	var actions []BatchAction

	annotationToType := []struct {
		key        string
		actionType BatchActionType
	}{
		{constants.AnnotationTriggerHost, BatchActionTypeTrigger},
		{constants.AnnotationRetryHost, BatchActionTypeRetry},
		{constants.AnnotationSkipHost, BatchActionTypeSkip},
	}

	for _, entry := range annotationToType {
		if esxiName, ok := batch.Annotations[entry.key]; ok && esxiName != "" {
			actions = append(actions, BatchAction{Type: entry.actionType, ESXiName: esxiName})
			delete(batch.Annotations, entry.key)
		}
	}

	return actions
}

// CreateESXIMigrationForBatch creates a new ESXIMigration for a single host within a ClusterConversionBatch.
// No owner reference is set (intentional — prevents GC cascade when batch is deleted).
func CreateESXIMigrationForBatch(
	ctx context.Context,
	k8sClient client.Client,
	batch *vjailbreakv1alpha1.ClusterConversionBatch,
	esxiName string,
) (*vjailbreakv1alpha1.ESXIMigration, error) {
	esxiK8sName, err := commonutils.GetK8sCompatibleVMWareObjectName(esxiName, batch.Spec.VMwareCredsRef.Name)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert ESXi name to k8s name")
	}

	esxiMigration := &vjailbreakv1alpha1.ESXIMigration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", esxiK8sName, batch.Name),
			Namespace: constants.NamespaceMigrationSystem,
			Labels: map[string]string{
				constants.ESXiNameLabel:               esxiK8sName,
				constants.VMwareCredsLabel:            batch.Spec.VMwareCredsRef.Name,
				constants.ClusterConversionBatchLabel: batch.Name,
			},
		},
		Spec: vjailbreakv1alpha1.ESXIMigrationSpec{
			ESXiName:                  esxiName,
			OpenstackCredsRef:         batch.Spec.OpenstackCredsRef,
			VMwareCredsRef:            batch.Spec.VMwareCredsRef,
			BMConfigRef:               &corev1.LocalObjectReference{Name: batch.Spec.BMConfigRef.Name},
			ClusterConversionBatchRef: &corev1.LocalObjectReference{Name: batch.Name},
		},
	}

	if err := k8sClient.Create(ctx, esxiMigration); err != nil {
		return nil, errors.Wrap(err, "failed to create ESXIMigration")
	}
	return esxiMigration, nil
}

// GetESXIMigrationForBatch retrieves the ESXIMigration created by ClusterConversionBatch for a given host.
func GetESXIMigrationForBatch(
	ctx context.Context,
	k8sClient client.Client,
	batch *vjailbreakv1alpha1.ClusterConversionBatch,
	esxiName string,
) (*vjailbreakv1alpha1.ESXIMigration, error) {
	esxiK8sName, err := commonutils.GetK8sCompatibleVMWareObjectName(esxiName, batch.Spec.VMwareCredsRef.Name)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert ESXi name to k8s name")
	}

	esxiMigration := &vjailbreakv1alpha1.ESXIMigration{}
	err = k8sClient.Get(ctx, types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s", esxiK8sName, batch.Name),
		Namespace: constants.NamespaceMigrationSystem,
	}, esxiMigration)
	return esxiMigration, err
}

// CheckPerHostEligibility evaluates all eligibility criteria for a single ESXi host within a batch.
// Returns (EligibilityStatus, reason, error). A non-nil error means a transient failure (Unknown status).
// NotReady means a criterion definitively failed. Ready means all criteria passed.
func CheckPerHostEligibility(
	ctx context.Context,
	k8sClient client.Client,
	batch *vjailbreakv1alpha1.ClusterConversionBatch,
	hostName string,
) (vjailbreakv1alpha1.EligibilityStatus, string, error) {
	// 1. BMConfig ValidationStatus = Succeeded
	if !isBMConfigValid(ctx, k8sClient, batch.Spec.BMConfigRef.Name) {
		return vjailbreakv1alpha1.EligibilityStatusNotReady,
			fmt.Sprintf("BMConfig %s validation has not succeeded", batch.Spec.BMConfigRef.Name),
			nil
	}

	// 2. At least one PCD cluster configured for the OpenStack creds
	openstackCreds := &vjailbreakv1alpha1.OpenstackCreds{}
	if err := k8sClient.Get(ctx, client.ObjectKey{
		Namespace: constants.NamespaceMigrationSystem,
		Name:      batch.Spec.OpenstackCredsRef.Name,
	}, openstackCreds); err != nil {
		return vjailbreakv1alpha1.EligibilityStatusUnknown, "",
			errors.Wrap(err, "failed to get OpenStack creds")
	}
	clusters, err := filterPCDClustersOnOpenstackCreds(ctx, k8sClient, *openstackCreds)
	if err != nil {
		return vjailbreakv1alpha1.EligibilityStatusUnknown, "",
			errors.Wrap(err, "failed to list PCD clusters")
	}
	if len(clusters) == 0 {
		return vjailbreakv1alpha1.EligibilityStatusNotReady,
			fmt.Sprintf("no PCD cluster configured for OpenStack creds %s", batch.Spec.OpenstackCredsRef.Name),
			nil
	}

	// 3. Host exists in MAAS (hardware UUID or MAC address match) and is Deployed/Allocated
	vmwareHost, err := GetVMwareHostFromESXiName(ctx, k8sClient, hostName, batch.Spec.VMwareCredsRef.Name)
	if err != nil {
		return vjailbreakv1alpha1.EligibilityStatusNotReady,
			fmt.Sprintf("VMwareHost object not found for %s", hostName),
			nil
	}
	inMAAS, maasReason, err := checkESXiInMAASForBatch(ctx, k8sClient, batch, *vmwareHost)
	if err != nil {
		return vjailbreakv1alpha1.EligibilityStatusUnknown, "", err
	}
	if !inMAAS {
		return vjailbreakv1alpha1.EligibilityStatusNotReady, maasReason, nil
	}

	// 4. DRS enabled, DRS fully automated, cluster has >1 host, remaining capacity sufficient, VMs not blocked
	vmwCreds := &vjailbreakv1alpha1.VMwareCreds{}
	if err := k8sClient.Get(ctx, client.ObjectKey{
		Namespace: constants.NamespaceMigrationSystem,
		Name:      batch.Spec.VMwareCredsRef.Name,
	}, vmwCreds); err != nil {
		return vjailbreakv1alpha1.EligibilityStatusUnknown, "",
			errors.Wrap(err, "failed to get VMware creds")
	}
	// CanEnterMaintenanceMode only uses scope.Client — no RollingMigrationPlan fields accessed.
	rmpScope := &pkgscope.RollingMigrationPlanScope{Client: k8sClient}
	config := RollingMigartionValidationConfig{
		CheckDRSEnabled:                         true,
		CheckDRSIsFullyAutomated:                true,
		CheckIfThereAreMoreThanOneHostInCluster: true,
		CheckClusterRemainingHostCapacity:       true,
		CheckVMsAreNotBlockedForMigration:       true,
		MigrationVMNames:                        map[string]struct{}{},
	}
	canMaintenance, maintReason, err := CanEnterMaintenanceMode(ctx, rmpScope, vmwCreds, hostName, config)
	if err != nil {
		return vjailbreakv1alpha1.EligibilityStatusUnknown, "", err
	}
	if !canMaintenance {
		return vjailbreakv1alpha1.EligibilityStatusNotReady, maintReason, nil
	}

	return vjailbreakv1alpha1.EligibilityStatusReady, "", nil
}

// checkESXiInMAASForBatch checks if an ESXi host is registered in MAAS using the batch's
// creds directly (no RollingMigrationPlan or MigrationTemplate needed).
// Tries hardware UUID first; falls back to MAC address matching.
func checkESXiInMAASForBatch(
	ctx context.Context,
	k8sClient client.Client,
	batch *vjailbreakv1alpha1.ClusterConversionBatch,
	vmwareHost vjailbreakv1alpha1.VMwareHost,
) (bool, string, error) {
	ctxlog := log.FromContext(ctx)

	bmConfig := &vjailbreakv1alpha1.BMConfig{}
	if err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      batch.Spec.BMConfigRef.Name,
		Namespace: constants.NamespaceMigrationSystem,
	}, bmConfig); err != nil {
		return false, "", errors.Wrap(err, "failed to get BMConfig")
	}

	provider, err := providers.GetProvider(string(bmConfig.Spec.ProviderType))
	if err != nil {
		return false, "", errors.Wrap(err, "failed to get MAAS provider")
	}

	machines, err := provider.ListResources(ctx)
	if err != nil {
		return false, "", errors.Wrap(err, "failed to list MAAS machines")
	}

	ctxlog.Info("MAAS eligibility check", "machineCount", len(machines),
		"esxiName", vmwareHost.Spec.Name, "esxiHardwareUUID", vmwareHost.Spec.HardwareUUID)

	matchedIdx := -1

	// Primary: hardware UUID
	if vmwareHost.Spec.HardwareUUID != "" {
		for i := range machines {
			if machines[i].HardwareUuid != "" && machines[i].HardwareUuid == vmwareHost.Spec.HardwareUUID {
				matchedIdx = i
				break
			}
		}
	}

	// Fallback: MAC address matching
	if matchedIdx == -1 {
		vmwCreds := &vjailbreakv1alpha1.VMwareCreds{}
		if err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      batch.Spec.VMwareCredsRef.Name,
			Namespace: constants.NamespaceMigrationSystem,
		}, vmwCreds); err != nil {
			return false, "", errors.Wrap(err, "failed to get VMware creds for MAC matching")
		}

		hs, err := GetESXiSummary(ctx, k8sClient, vmwareHost.Spec.Name, vmwCreds, "")
		if err != nil {
			return false, "", errors.Wrap(err, "failed to get ESXi summary for MAC matching")
		}

		var hostMACs []string
		if hs.Config != nil && hs.Config.Network != nil {
			for _, pnic := range hs.Config.Network.Pnic {
				if pnic.Mac != "" {
					hostMACs = append(hostMACs, strings.ToLower(pnic.Mac))
				}
			}
		}
		if len(hostMACs) == 0 {
			return false, "", errors.New("no hardware UUID or MAC addresses available for MAAS matching")
		}

		for i := range machines {
			machineMac := strings.ToLower(machines[i].MacAddress)
			for _, hostMac := range hostMACs {
				if hostMac != "" && hostMac == machineMac {
					matchedIdx = i
					break
				}
			}
			if matchedIdx != -1 {
				break
			}
		}
	}

	if matchedIdx == -1 {
		return false, fmt.Sprintf("ESXi %s not found in MAAS", vmwareHost.Spec.Name), nil
	}

	matchedStatus := machines[matchedIdx].Status
	if matchedStatus == "Deployed" || matchedStatus == "Allocated" {
		return true, "", nil
	}
	return false, fmt.Sprintf("ESXi %s found in MAAS but status is %q (want Deployed or Allocated)", vmwareHost.Spec.Name, matchedStatus), nil
}
