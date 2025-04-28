package utils

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/constants"
	scope "github.com/platform9/vjailbreak/k8s/migration/pkg/scope"
	"github.com/platform9/vjailbreak/pkg/vpwned/api/proto/v1/service"
	providers "github.com/platform9/vjailbreak/pkg/vpwned/sdk/providers"
	_ "github.com/platform9/vjailbreak/pkg/vpwned/sdk/providers/base"
	_ "github.com/platform9/vjailbreak/pkg/vpwned/sdk/providers/maas"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func ConvertESXiToPCDHost(ctx context.Context,
	scope *scope.ESXIMigrationScope,
	bmProvider providers.BMCProvider) error {
	ctxlog := log.FromContext(ctx).WithName(constants.ESXIMigrationControllerName)

	vmwarecreds := &vjailbreakv1alpha1.VMwareCreds{}
	err := scope.Client.Get(ctx, types.NamespacedName{Namespace: constants.NamespaceMigrationSystem, Name: scope.RollingMigrationPlan.Spec.VMwareCredsRef.Name}, vmwarecreds)
	if err != nil {
		return errors.Wrap(err, "failed to get vmware credentials")
	}

	// list Mass machines
	resources, err := bmProvider.ListResources(ctx)
	if err != nil {
		return err
	}

	hs, err := GetESXiSummary(ctx, scope.Client, scope.ESXIMigration.Spec.ESXiName, vmwarecreds)
	if err != nil {
		return errors.Wrap(err, "failed to get ESXi summary")
	}

	for i := 0; i < len(resources); i++ {
		if resources[i].HardwareUuid == hs.Hardware.SystemInfo.Uuid {
			ctxlog.Info("Found a matching resource", "resource", resources[i].HardwareUuid, "name", resources[i].Hostname, "serial", resources[i].Id)
			err := ReclaimESXi(ctx, scope, bmProvider, resources[i].Id)
			if err != nil {
				scope.ESXIMigration.Status.Phase = vjailbreakv1alpha1.ESXIMigrationPhaseFailed
				updateErr := scope.Client.Status().Update(ctx, scope.ESXIMigration)
				if updateErr != nil {
					return errors.Wrap(updateErr, "failed to update ESXi migration status")
				}
				return errors.Wrap(err, "failed to reclaim ESXi")
			}
			break
		}
	}

	// TODO(vPwned): Update ESXi migration status after checking host on PCD
	scope.ESXIMigration.Status.Phase = vjailbreakv1alpha1.ESXIMigrationPhaseSucceeded
	err = scope.Client.Status().Update(ctx, scope.ESXIMigration)
	if err != nil {
		return errors.Wrap(err, "failed to update ESXi migration status")
	}

	return nil
}

func PrettyPrint(v interface{}) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(string(b))
}

func ReclaimESXi(ctx context.Context, scope *scope.ESXIMigrationScope, bmProvider providers.BMCProvider, resourceId string) error {
	rollingMigrationPlan := scope.RollingMigrationPlan
	// Get BMConfig for the rolling migration plan
	bmConfig, err := GetBMConfigForRollingMigrationPlan(ctx, scope.Client, scope.RollingMigrationPlan)
	if err != nil {
		return errors.Wrap(err, "failed to get BMConfig for rolling migration plan")
	}
	// Get cloud init secret from rolling migration plan
	secret, err := GetCloudInitSecretFromRollingMigrationPlan(ctx, scope.Client, rollingMigrationPlan)
	if err != nil {
		return errors.Wrap(err, "failed to get cloud init secret from rolling migration plan")
	}
	if _, ok := secret.Data[constants.CloudInitConfigKey]; !ok {
		return errors.New("cloud init secret is empty")
	}

	cloudInit := string(secret.Data[constants.CloudInitConfigKey])
	err = bmProvider.ReclaimBM(ctx, service.ReclaimBMRequest{
		AccessInfo: &service.BMProvisionerAccessInfo{
			BaseUrl:     bmConfig.Spec.APIUrl,
			ApiKey:      bmConfig.Spec.APIKey,
			UseInsecure: bmConfig.Spec.Insecure,
		},
		UserData:   cloudInit,
		ResourceId: resourceId,
		PowerCycle: true,
		BootSource: &service.BootsourceSelections{
			Release: bmConfig.Spec.BootSource.Release,
		},
	})
	if err != nil {
		return errors.Wrap(err, "failed to reclaim BM")
	}
	return nil
}

// MergeCloudInit merges two cloud-init YAML configurations
// userData and cloudInit are expected to be valid YAML strings in cloud-init format
// The function returns the merged configuration as a YAML string with the #cloud-config directive
func MergeCloudInit(userData, cloudInit string) (string, error) {
	// Parse both YAML strings into maps
	userDataMap := make(map[string]interface{})
	cloudInitMap := make(map[string]interface{})

	// Unmarshal the YAML strings
	if err := yaml.Unmarshal([]byte(userData), &userDataMap); err != nil {
		return "", fmt.Errorf("failed to parse userData: %v", err)
	}

	if err := yaml.Unmarshal([]byte(cloudInit), &cloudInitMap); err != nil {
		return "", fmt.Errorf("failed to parse cloudInit: %v", err)
	}

	// Deep merge the maps (cloudInit takes precedence over userData)
	mergedMap := deepMerge(userDataMap, cloudInitMap)

	// Marshal the result back to YAML
	result, err := yaml.Marshal(mergedMap)
	if err != nil {
		return "", fmt.Errorf("failed to marshal merged config: %v", err)
	}

	// Ensure the #cloud-config directive is at the beginning of the YAML
	return "#cloud-config\n" + string(result), nil
}

func MergeCloudInitAndCreateSecret(ctx context.Context, scope *scope.RollingMigrationPlanScope) error {
	// Get BMConfig for the rolling migration plan
	bmConfig, err := GetBMConfigForRollingMigrationPlan(ctx, scope.Client, scope.RollingMigrationPlan)
	if err != nil {
		return errors.Wrap(err, "failed to get BMConfig for rolling migration plan")
	}

	// Get user data from the secret
	userDataSecret, err := GetUserDataSecretForBMConfig(ctx, scope.Client, bmConfig)
	if err != nil {
		return errors.Wrap(err, "failed to get user data secret for BMConfig")
	}

	// Decode user data
	if _, ok := userDataSecret.Data[constants.UserDataSecretKey]; !ok {
		return errors.New("user data secret is empty")
	}

	userData := string(userDataSecret.Data[constants.UserDataSecretKey])

	cloudInit, err := generatePCDOnboardingCloudInit(ctx, scope)
	if err != nil {
		return errors.Wrap(err, "failed to generate cloud init for BMConfig")
	}

	// merge cloud init and user data
	mergedCloudInit, err := MergeCloudInit(userData, cloudInit)
	if err != nil {
		return errors.Wrap(err, "failed to merge cloud init and user data")
	}

	finalCloudInitSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-merged", userDataSecret.Name),
			Namespace: constants.NamespaceMigrationSystem,
		},
		Data: map[string][]byte{
			constants.CloudInitConfigKey: []byte(mergedCloudInit),
		},
	}

	// Set the owner reference
	controllerutil.SetOwnerReference(scope.RollingMigrationPlan, finalCloudInitSecret, scope.Client.Scheme())

	// Create or update the secret
	if err := scope.Client.Create(ctx, finalCloudInitSecret); err != nil {
		// Check if the error is because the resource already exists
		if strings.Contains(err.Error(), "already exists") {
			// Secret already exists, update it
			if err := scope.Client.Update(ctx, finalCloudInitSecret); err != nil {
				return errors.Wrap(err, "failed to update merged cloud init secret")
			}
		} else {
			return errors.Wrap(err, "failed to create merged cloud init secret")
		}
	}

	scope.RollingMigrationPlan.Spec.CloudInitConfigRef = &corev1.SecretReference{
		Name:      finalCloudInitSecret.Name,
		Namespace: constants.NamespaceMigrationSystem,
	}
	err = scope.Close()
	if err != nil {
		return errors.Wrap(err, "failed to close scope")
	}

	return nil
}

func generatePCDOnboardingCloudInit(ctx context.Context, scope *scope.RollingMigrationPlanScope) (string, error) {
	openstackCreds, err := GetOpenstackCredentials(ctx, scope.Client, scope.RollingMigrationPlan.Spec.OpenstackCredsRef.Name)
	if err != nil {
		return "", errors.Wrap(err, "failed to get openstack credentials")
	}
	// read cloud-init template
	cloudInitTemplateStr, err := os.ReadFile("/pkg/scripts/cloud-init.tmpl.yaml")
	if err != nil {
		return "", errors.Wrap(err, "failed to read cloud-init template")
	}
	cloudInitTemplate, err := template.New("cloud-init").Parse(string(cloudInitTemplateStr))
	if err != nil {
		return "", errors.Wrap(err, "failed to parse cloud-init template")
	}
	var cloudInitBuffer bytes.Buffer
	err = cloudInitTemplate.Execute(&cloudInitBuffer, openstackCreds)
	if err != nil {
		return "", errors.Wrap(err, "failed to execute cloud-init template")
	}
	return cloudInitBuffer.String(), nil
}

func GetBMConfigForRollingMigrationPlan(ctx context.Context,
	k8sClient client.Client,
	rollingMigrationPlan *vjailbreakv1alpha1.RollingMigrationPlan) (*vjailbreakv1alpha1.BMConfig, error) {

	bmConfig := &vjailbreakv1alpha1.BMConfig{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: rollingMigrationPlan.Spec.BMConfigRef.Name, Namespace: constants.NamespaceMigrationSystem}, bmConfig); err != nil {
		return nil, errors.Wrap(err, "failed to get BMConfig for rolling migration plan")
	}
	return bmConfig, nil
}

func GetUserDataForBMConfig(ctx context.Context, scope *scope.RollingMigrationPlanScope) (string, error) {
	bmConfig, err := GetBMConfig(ctx, scope.Client, scope.RollingMigrationPlan.Spec.BMConfigRef)
	if err != nil {
		return "", errors.Wrap(err, "failed to get BMConfig for rolling migration plan")
	}
	secret, err := GetUserDataSecretForBMConfig(ctx, scope.Client, bmConfig)
	if err != nil {
		return "", errors.Wrap(err, "failed to get user data secret for BMConfig")
	}
	if _, ok := secret.Data[constants.UserDataSecretKey]; !ok {
		return "", errors.New("user data secret is empty")
	}
	userData, err := base64.StdEncoding.DecodeString(string(secret.Data[constants.UserDataSecretKey]))
	if err != nil {
		return "", errors.Wrap(err, "failed to decode user data secret for BMConfig")
	}

	return string(userData), nil
}

func GetUserDataSecretForBMConfig(ctx context.Context, k8sClient client.Client, bmConfig *vjailbreakv1alpha1.BMConfig) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: bmConfig.Spec.UserDataSecretRef.Name, Namespace: constants.NamespaceMigrationSystem}, secret); err != nil {
		return nil, errors.Wrap(err, "failed to get user data secret for BMConfig")
	}
	return secret, nil
}

func GetCloudInitSecretFromRollingMigrationPlan(ctx context.Context,
	k8sClient client.Client,
	rollingMigrationPlan *vjailbreakv1alpha1.RollingMigrationPlan) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: rollingMigrationPlan.Spec.CloudInitConfigRef.Name, Namespace: constants.NamespaceMigrationSystem}, secret); err != nil {
		return nil, errors.Wrap(err, "failed to get cloud init secret from rolling migration plan")
	}
	return secret, nil
}

func GetBMConfig(ctx context.Context, k8sClient client.Client, bmConfigRef corev1.LocalObjectReference) (*vjailbreakv1alpha1.BMConfig, error) {
	bmConfig := &vjailbreakv1alpha1.BMConfig{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: bmConfigRef.Name, Namespace: constants.NamespaceMigrationSystem}, bmConfig); err != nil {
		return nil, errors.Wrap(err, "failed to get BMConfig")
	}
	return bmConfig, nil
}

func ValidateOpenstackIsPCD(ctx context.Context, k8sClient client.Client, rollingMigrationPlan *vjailbreakv1alpha1.RollingMigrationPlan) (bool, error) {
	openstackCreds := vjailbreakv1alpha1.OpenstackCreds{}
	if err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      rollingMigrationPlan.Spec.OpenstackCredsRef.Name,
		Namespace: constants.NamespaceMigrationSystem},
		&openstackCreds); err != nil {
		return false, errors.Wrap(err, "failed to get openstack credentials")
	}
	if _, ok := openstackCreds.Labels[constants.IsPCDCredsLabel]; !ok {
		return false, nil

	}
	return true, nil
}
