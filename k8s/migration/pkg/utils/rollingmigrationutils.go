package utils

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/constants"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func CreateClusterMigration(ctx context.Context, k8sClient client.Client, cluster vjailbreakv1alpha1.ClusterMigrationInfo, rollingMigrationPlan *vjailbreakv1alpha1.RollingMigrationPlan) (*vjailbreakv1alpha1.ClusterMigration, error) {
	ESXiSequence := GetESXiSequenceFromVMSequence(ctx, cluster.VMSequence)
	if len(ESXiSequence) == 0 {
		return nil, errors.New("ESXi host sequence cannot be empty")
	}
	clusterK8sName, err := ConvertToK8sName(cluster.ClusterName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert cluster name to k8s name")
	}
	clusterMigration := &vjailbreakv1alpha1.ClusterMigration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GenerateRollingMigrationObjectName(clusterK8sName, rollingMigrationPlan),
			Namespace: constants.NamespaceMigrationSystem,
		},
		Spec: vjailbreakv1alpha1.ClusterMigrationSpec{
			ClusterName:           cluster.ClusterName,
			ESXIMigrationSequence: ESXiSequence,
			RollingMigrationPlanRef: corev1.LocalObjectReference{
				Name: rollingMigrationPlan.Name,
			},
			OpenstackCredsRef: rollingMigrationPlan.Spec.OpenstackCredsRef,
			VMwareCredsRef:    rollingMigrationPlan.Spec.VMwareCredsRef,
		},
	}
	controllerutil.SetOwnerReference(rollingMigrationPlan, clusterMigration, k8sClient.Scheme())

	if err := k8sClient.Create(ctx, clusterMigration); err != nil {
		return nil, err
	}
	return clusterMigration, nil
}

func GetClusterMigration(ctx context.Context, k8sClient client.Client, clusterName string, rollingMigrationPlan *vjailbreakv1alpha1.RollingMigrationPlan) (*vjailbreakv1alpha1.ClusterMigration, error) {
	clusterK8sName, err := ConvertToK8sName(clusterName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert cluster name to k8s name")
	}
	clusterMigration := &vjailbreakv1alpha1.ClusterMigration{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: GenerateRollingMigrationObjectName(clusterK8sName, rollingMigrationPlan), Namespace: constants.NamespaceMigrationSystem}, clusterMigration); err != nil {
		return nil, err
	}
	return clusterMigration, nil
}

func GetESXIMigration(ctx context.Context, k8sClient client.Client, esxi string, rollingMigrationPlan *vjailbreakv1alpha1.RollingMigrationPlan) (*vjailbreakv1alpha1.ESXIMigration, error) {
	esxiK8sName, err := ConvertToK8sName(esxi)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert ESXi name to k8s name")
	}
	esxiMigration := &vjailbreakv1alpha1.ESXIMigration{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: GenerateRollingMigrationObjectName(esxiK8sName, rollingMigrationPlan), Namespace: constants.NamespaceMigrationSystem}, esxiMigration); err != nil {
		return nil, err
	}
	return esxiMigration, nil
}

func CreateESXIMigration(ctx context.Context, k8sClient client.Client, esxi string, rollingMigrationPlan *vjailbreakv1alpha1.RollingMigrationPlan) (*vjailbreakv1alpha1.ESXIMigration, error) {
	esxiK8sName, err := ConvertToK8sName(esxi)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert ESXi name to k8s name")
	}
	esxiMigration := &vjailbreakv1alpha1.ESXIMigration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GenerateRollingMigrationObjectName(esxiK8sName, rollingMigrationPlan),
			Namespace: constants.NamespaceMigrationSystem,
		},
		Spec: vjailbreakv1alpha1.ESXIMigrationSpec{
			ESXiName: esxi,
			RollingMigrationPlanRef: corev1.LocalObjectReference{
				Name: rollingMigrationPlan.Name,
			},
			OpenstackCredsRef: rollingMigrationPlan.Spec.OpenstackCredsRef,
			VMwareCredsRef:    rollingMigrationPlan.Spec.VMwareCredsRef,
		},
	}
	controllerutil.SetOwnerReference(rollingMigrationPlan, esxiMigration, k8sClient.Scheme())
	if err := k8sClient.Create(ctx, esxiMigration); err != nil {
		return nil, err
	}
	return esxiMigration, nil
}

func GetESXiSequenceFromVMSequence(ctx context.Context, vmSequence []vjailbreakv1alpha1.VMSequenceInfo) []string {
	esxiSequence := []string{}
	for _, vm := range vmSequence {
		esxiSequence = AppendUnique(esxiSequence, vm.ESXiName)
	}

	return esxiSequence
}

func AddVMsToESXIMigrationStatus(ctx context.Context, k8sClient client.Client, esxi string, rollingMigrationPlan *vjailbreakv1alpha1.RollingMigrationPlan) error {
	esxiK8sName, err := ConvertToK8sName(esxi)
	if err != nil {
		return errors.Wrap(err, "failed to convert ESXi name to k8s name")
	}
	esxiMigration := &vjailbreakv1alpha1.ESXIMigration{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: GenerateRollingMigrationObjectName(esxiK8sName, rollingMigrationPlan), Namespace: constants.NamespaceMigrationSystem}, esxiMigration); err != nil {
		return errors.Wrap(err, "failed to get ESXi migration status")
	}

	vmList := vjailbreakv1alpha1.VMwareMachineList{}

	if err := k8sClient.List(ctx, &vmList, client.InNamespace(constants.NamespaceMigrationSystem), client.MatchingLabels{constants.ESXiNameLabel: esxi, constants.VMwareCredsLabel: esxiMigration.Spec.VMwareCredsRef.Name}); err != nil {
		return errors.Wrap(err, "failed to get ESXi migration status")
	}

	for _, vmName := range vmList.Items {
		esxiMigration.Status.VMs = append(esxiMigration.Status.VMs, vmName.Name)
	}

	if err := k8sClient.Status().Update(ctx, esxiMigration); err != nil {
		return errors.Wrap(err, "failed to update ESXi migration status")
	}
	return nil
}

func PutESXiInMaintenanceMode(ctx context.Context, k8sClient client.Client, esxiName string, vmwareCredsRef corev1.LocalObjectReference) error {
	vmwarecreds := &vjailbreakv1alpha1.VMwareCreds{}
	err := k8sClient.Get(ctx, types.NamespacedName{Namespace: constants.NamespaceMigrationSystem, Name: vmwareCredsRef.Name}, vmwarecreds)
	if err != nil {
		return errors.Wrap(err, "failed to get vmware credentials")
	}

	c, err := ValidateVMwareCreds(ctx, k8sClient, vmwarecreds)
	if err != nil {
		return errors.Wrap(err, "failed to validate vCenter connection")
	}
	finder := find.NewFinder(c, false)
	dc, err := finder.Datacenter(ctx, vmwarecreds.Spec.DataCenter)
	if err != nil {
		return errors.Wrap(err, "failed to find datacenter")
	}
	finder.SetDatacenter(dc)
	hostSystem, err := finder.HostSystem(ctx, esxiName)
	if err != nil {
		return errors.Wrapf(err, "failed to find host %s", esxiName)
	}

	// Check host state
	var hs mo.HostSystem
	err = hostSystem.Properties(ctx, hostSystem.Reference(), []string{"runtime.connectionState"}, &hs)
	if err != nil {
		return errors.Wrap(err, "failed to get host properties")
	}

	// Put host into maintenance mode
	task, err := hostSystem.EnterMaintenanceMode(ctx, 300, true, nil)
	if err != nil {
		return errors.Wrap(err, "failed to initiate maintenance mode")
	}

	err = task.Wait(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to wait for host to enter maintenance mode")
	}

	return nil
}

func CheckESXiInMaintenanceMode(ctx context.Context, k8sClient client.Client, esxiName string, vmwareCredsRef corev1.LocalObjectReference) (bool, error) {

	vmwarecreds := &vjailbreakv1alpha1.VMwareCreds{}
	err := k8sClient.Get(ctx, types.NamespacedName{Namespace: constants.NamespaceMigrationSystem, Name: vmwareCredsRef.Name}, vmwarecreds)
	if err != nil {
		return false, errors.Wrap(err, "failed to get vmware credentials")
	}

	hs, err := GetESXiSummary(ctx, k8sClient, esxiName, vmwarecreds)
	if err != nil {
		return false, errors.Wrap(err, "failed to get ESXi summary")
	}

	if hs.Summary.Runtime.InMaintenanceMode {
		return true, nil
	}

	return false, nil
}

func GetESXiSummary(ctx context.Context, k8sClient client.Client, esxiName string, vmwareCreds *vjailbreakv1alpha1.VMwareCreds) (mo.HostSystem, error) {

	c, err := ValidateVMwareCreds(ctx, k8sClient, vmwareCreds)
	if err != nil {
		return mo.HostSystem{}, errors.Wrap(err, "failed to validate vCenter connection")
	}
	finder := find.NewFinder(c, false)
	dc, err := finder.Datacenter(ctx, vmwareCreds.Spec.DataCenter)
	if err != nil {
		return mo.HostSystem{}, errors.Wrap(err, "failed to find datacenter")
	}
	finder.SetDatacenter(dc)
	hostSystem, err := finder.HostSystem(ctx, esxiName)
	if err != nil {
		return mo.HostSystem{}, errors.Wrapf(err, "failed to find host %s", esxiName)
	}

	pc := property.DefaultCollector(c)

	var hs mo.HostSystem
	err = pc.RetrieveOne(ctx, hostSystem.Reference(), []string{"summary", "config", "hardware"}, &hs)
	if err != nil {
		return mo.HostSystem{}, errors.Wrap(err, "failed to get host properties")
	}
	return hs, nil
}

func CountVMsOnESXi(ctx context.Context, k8sClient client.Client, esxiName string, vmwareCredsRef corev1.LocalObjectReference) (int, error) {
	vmwarecreds := &vjailbreakv1alpha1.VMwareCreds{}
	err := k8sClient.Get(ctx, types.NamespacedName{Namespace: constants.NamespaceMigrationSystem, Name: vmwareCredsRef.Name}, vmwarecreds)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get vmware credentials")
	}

	c, err := ValidateVMwareCreds(ctx, k8sClient, vmwarecreds)
	if err != nil {
		return 0, errors.Wrap(err, "failed to validate vCenter connection")
	}
	finder := find.NewFinder(c, false)
	dc, err := finder.Datacenter(ctx, vmwarecreds.Spec.DataCenter)
	if err != nil {
		return 0, errors.Wrap(err, "failed to find datacenter")
	}
	finder.SetDatacenter(dc)
	hostSystem, err := finder.HostSystem(ctx, esxiName)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to find host %s", esxiName)
	}

	// Get the VMs on the host
	var host mo.HostSystem
	err = hostSystem.Properties(ctx, hostSystem.Reference(), []string{"vm"}, &host)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get VM properties")
	}

	return len(host.Vm), nil
}

// deepMerge performs a deep merge of src into dst
// src values override dst values when there's a conflict
func deepMerge(dst, src map[string]interface{}) map[string]interface{} {
	for key, srcVal := range src {
		dstVal, exists := dst[key]

		// If the key doesn't exist in dst, just set it
		if !exists {
			dst[key] = srcVal
			continue
		}

		// If both values are maps, recursively merge them
		srcMap, srcIsMap := srcVal.(map[string]interface{})
		dstMap, dstIsMap := dstVal.(map[string]interface{})

		if srcIsMap && dstIsMap {
			dst[key] = deepMerge(dstMap, srcMap)
			continue
		}

		// For lists/arrays, we need special handling
		srcSlice, srcIsSlice := srcVal.([]interface{})
		dstSlice, dstIsSlice := dstVal.([]interface{})

		if srcIsSlice && dstIsSlice {
			// Simple merge: just append elements from src to dst
			// Another option could be to replace, or do a merge based on specific rules
			combined := append(dstSlice, srcSlice...)
			dst[key] = combined
			continue
		}

		// In all other cases (primitive types, or type mismatch), src overwrites dst
		dst[key] = srcVal
	}

	return dst
}

func GenerateRollingMigrationObjectName(objectName string, rollingMigrationPlan *vjailbreakv1alpha1.RollingMigrationPlan) string {
	return fmt.Sprintf("%s-%s", objectName, rollingMigrationPlan.Name)
}
