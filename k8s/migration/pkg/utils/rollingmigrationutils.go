package utils

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/constants"
	scope "github.com/platform9/vjailbreak/k8s/migration/pkg/scope"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
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
	migrationTemplate := vjailbreakv1alpha1.MigrationTemplate{}
	if err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      rollingMigrationPlan.Spec.MigrationTemplate,
		Namespace: constants.NamespaceMigrationSystem},
		&migrationTemplate); err != nil {
		return nil, errors.Wrap(err, "failed to get migration template")
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
			OpenstackCredsRef: corev1.LocalObjectReference{
				Name: migrationTemplate.Spec.Destination.OpenstackRef,
			},
			VMwareCredsRef: corev1.LocalObjectReference{
				Name: migrationTemplate.Spec.Source.VMwareRef,
			},
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
	migrationTemplate := vjailbreakv1alpha1.MigrationTemplate{}
	if err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      rollingMigrationPlan.Spec.MigrationTemplate,
		Namespace: constants.NamespaceMigrationSystem},
		&migrationTemplate); err != nil {
		return nil, errors.Wrap(err, "failed to get migration template")
	}
	esxiMigration := &vjailbreakv1alpha1.ESXIMigration{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: GenerateRollingMigrationObjectName(esxiK8sName, rollingMigrationPlan), Namespace: constants.NamespaceMigrationSystem}, esxiMigration); err != nil {
		return nil, err
	}
	return esxiMigration, nil
}

func CreateESXIMigration(ctx context.Context, scope *scope.ClusterMigrationScope, esxi string) (*vjailbreakv1alpha1.ESXIMigration, error) {
	esxiK8sName, err := ConvertToK8sName(esxi)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert ESXi name to k8s name")
	}
	esxiMigration := &vjailbreakv1alpha1.ESXIMigration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GenerateRollingMigrationObjectName(esxiK8sName, scope.RollingMigrationPlan),
			Namespace: constants.NamespaceMigrationSystem,
			Labels: map[string]string{
				constants.ESXiNameLabel:             esxi,
				constants.VMwareCredsLabel:          scope.ClusterMigration.Spec.VMwareCredsRef.Name,
				constants.RollingMigrationPlanLabel: scope.RollingMigrationPlan.Name,
				constants.ClusterMigrationLabel:     scope.ClusterMigration.Name,
			},
		},
		Spec: vjailbreakv1alpha1.ESXIMigrationSpec{
			ESXiName: esxi,
			RollingMigrationPlanRef: corev1.LocalObjectReference{
				Name: scope.RollingMigrationPlan.Name,
			},
			OpenstackCredsRef: scope.ClusterMigration.Spec.OpenstackCredsRef,
			VMwareCredsRef:    scope.ClusterMigration.Spec.VMwareCredsRef,
		},
	}
	controllerutil.SetOwnerReference(scope.RollingMigrationPlan, esxiMigration, scope.Client.Scheme())
	if err := scope.Client.Create(ctx, esxiMigration); err != nil {
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
func boolPtr(b bool) *bool {
	return &b
}

// generateMigrationTemplate creates a new MigrationTemplate resource based on the RollingMigrationPlan
// and returns the name of the created template
// func generateMigrationTemplate(ctx context.Context, scope *scope.RollingMigrationPlanScope) (string, error) {
// 	rollingMigrationPlan := scope.RollingMigrationPlan

// 	// Create a unique name for the template
// 	templateName := GenerateRollingMigrationObjectName("template", rollingMigrationPlan)

// 	// Get the client
// 	client := scope.Client

// 	// Create the template
// 	template := &vjailbreakv1alpha1.MigrationTemplate{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:      templateName,
// 			Namespace: rollingMigrationPlan.Namespace,
// 			// Set owner reference to the rolling migration plan
// 			OwnerReferences: []metav1.OwnerReference{
// 				{
// 					APIVersion: rollingMigrationPlan.APIVersion,
// 					Kind:       rollingMigrationPlan.Kind,
// 					Name:       rollingMigrationPlan.Name,
// 					UID:        rollingMigrationPlan.UID,
// 					Controller: boolPtr(true),
// 				},
// 			},
// 		},
// 		Spec: vjailbreakv1alpha1.MigrationTemplateSpec{
// 			// Default to empty OS type, v2v-helper will try to figure out OS
// 			OSType: "",
// 			// Create network and storage mapping names based on the rolling migration plan
// 			NetworkMapping: rollingMigrationPlan.Spec.NetworkMapping,
// 			StorageMapping: rollingMigrationPlan.Spec.StorageMapping,
// 			Source: vjailbreakv1alpha1.MigrationTemplateSource{
// 				// Reference VMware credentials
// 				VMwareRef: rollingMigrationPlan.Spec.VMwareCredsRef.Name,
// 			},
// 			Destination: vjailbreakv1alpha1.MigrationTemplateDestination{
// 				// Reference OpenStack credentials
// 				OpenstackRef: rollingMigrationPlan.Spec.OpenstackCredsRef.Name,
// 			},
// 		},
// 	}

// 	// Create the template
// 	err := client.Create(ctx, template)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to create migration template: %w", err)
// 	}

// 	return templateName, nil
// }

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

func UpdateESXiNamesInRollingMigrationPlan(ctx context.Context, scope *scope.RollingMigrationPlanScope) error {
	// Update ESXi Name in RollingMigrationPlan for each VM in VM Sequence
	for i, cluster := range scope.RollingMigrationPlan.Spec.ClusterSequence {
		for j := range cluster.VMSequence {
			k8sVMName, err := ConvertToK8sName(cluster.VMSequence[j].VMName)
			if err != nil {
				return errors.Wrap(err, "failed to convert vm name to k8s name")
			}
			vm := &vjailbreakv1alpha1.VMwareMachine{}
			err = scope.Client.Get(ctx, client.ObjectKey{
				Name:      k8sVMName,
				Namespace: scope.Namespace(),
			}, vm)
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("Error getting VMInfo for VM '%s'", cluster.VMSequence[j].VMName))
			}
			scope.RollingMigrationPlan.Spec.ClusterSequence[i].VMSequence[j].ESXiName = vm.Spec.VMInfo.ESXiName
		}
	}
	return nil
}

func ConvertVMSequenceToMigrationPlans(ctx context.Context, scope *scope.RollingMigrationPlanScope, batchSize int) error {
	log := scope.Logger

	if batchSize <= 0 {
		return fmt.Errorf("batch size must be greater than 0")
	}

	rollingMigrationPlan := scope.RollingMigrationPlan

	if len(rollingMigrationPlan.Spec.VMMigrationPlans) != 0 {
		log.Info("Migration plans already added")
		return nil
	}

	// Collect all VM names from all clusters
	batches, err := convertVMSequenceToBatches(ctx, scope, batchSize)
	if err != nil {
		return errors.Wrap(err, "failed to convert VM sequence to batches")
	}

	// Create a MigrationPlan for each batch
	for i, batch := range batches {
		err := convertBatchToMigrationPlan(ctx, scope, batch, i, rollingMigrationPlan.Spec.MigrationTemplate)
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				continue
			}
			return errors.Wrap(err, "failed to convert batch to migration plan")
		}
	}

	// Log summary of batches created
	klog.Infof("Created %d batches with a maximum of %d VMs per batch", len(batches), batchSize)

	return nil
}

func convertBatchToMigrationPlan(ctx context.Context, scope *scope.RollingMigrationPlanScope, batch []string, i int, templateName string) error {
	rollingMigrationPlan := scope.RollingMigrationPlan

	// Create a migration plan for this batch
	migrationPlan := vjailbreakv1alpha1.MigrationPlan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-batch-%d", rollingMigrationPlan.Name, i),
			Namespace: rollingMigrationPlan.Namespace,
			// Set owner reference to the rolling migration plan
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: rollingMigrationPlan.APIVersion,
					Kind:       rollingMigrationPlan.Kind,
					Name:       rollingMigrationPlan.Name,
					UID:        rollingMigrationPlan.UID,
					Controller: boolPtr(true),
				},
			},
			Labels: map[string]string{
				constants.PauseRollingMigrationPlanLabel: trueString,
			},
		},
		Spec: vjailbreakv1alpha1.MigrationPlanSpec{
			// Use the template name from existing plans or default
			MigrationPlanSpecPerVM: vjailbreakv1alpha1.MigrationPlanSpecPerVM{
				MigrationTemplate: templateName,
				MigrationStrategy: vjailbreakv1alpha1.MigrationPlanStrategy{
					Type:                "OnDemand",
					PerformHealthChecks: true,
				},
				// Copy advanced options if needed
				AdvancedOptions: vjailbreakv1alpha1.AdvancedOptions{},
			},
			// Include VM batch as a single group for migration
			VirtualMachines: [][]string{batch},
		},
		Status: vjailbreakv1alpha1.MigrationPlanStatus{
			MigrationStatus:  "Pending",
			MigrationMessage: fmt.Sprintf("Created by RollingMigrationPlan %s", rollingMigrationPlan.Name),
		},
	}

	// Create the migration plan
	err := scope.Client.Create(ctx, &migrationPlan)
	if err != nil {
		return errors.Wrap(err, "failed to create migration plan")
	}
	// Add the migration plan to the rolling migration plan
	rollingMigrationPlan.Spec.VMMigrationPlans = append(rollingMigrationPlan.Spec.VMMigrationPlans, migrationPlan.Name)
	return nil
}

func convertVMSequenceToBatches(ctx context.Context, scope *scope.RollingMigrationPlanScope, batchSize int) ([][]string, error) {
	var batches [][]string
	rollingMigrationPlan := scope.RollingMigrationPlan

	for _, cluster := range rollingMigrationPlan.Spec.ClusterSequence {
		var allVMs []string
		for _, vm := range cluster.VMSequence {
			allVMs = append(allVMs, vm.VMName)
		}

		// Create batches of VMs
		for i := 0; i < len(allVMs); i += batchSize {
			end := i + batchSize
			if end > len(allVMs) {
				end = len(allVMs)
			}
			batches = append(batches, allVMs[i:end])
		}
	}
	return batches, nil
}
