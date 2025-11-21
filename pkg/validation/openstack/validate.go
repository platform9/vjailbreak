package openstack

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	constants "github.com/platform9/vjailbreak/k8s/migration/pkg/constants"
	scope "github.com/platform9/vjailbreak/k8s/migration/pkg/scope"
	utils "github.com/platform9/vjailbreak/k8s/migration/pkg/utils"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/k8sutils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	namespaceMigrationSystem = "migration-system"
)

// ValidationResult holds the outcome of credential validation
type ValidationResult struct {
	Valid   bool
	Message string
	Error   error
}

// Validate performs complete OpenStack credential validation
func Validate(ctx context.Context, k8sClient client.Client, openstackcreds *vjailbreakv1alpha1.OpenstackCreds) ValidationResult {
	// Get credentials from secret
	openstackCredential, err := getCredentialsFromSecret(ctx, k8sClient, openstackcreds.Spec.SecretRef.Name)
	if err != nil {
		return ValidationResult{
			Valid:   false,
			Message: fmt.Sprintf("Failed to get credentials from secret: %s", err.Error()),
			Error:   err,
		}
	}

	// Create provider client
	providerClient, err := openstack.NewClient(openstackCredential.AuthURL)
	if err != nil {
		return ValidationResult{
			Valid:   false,
			Message: fmt.Sprintf("Failed to create OpenStack client: %s", err.Error()),
			Error:   err,
		}
	}

	// Configure TLS
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	if openstackCredential.Insecure {
		tlsConfig.InsecureSkipVerify = true
	} else {
		fmt.Printf("Warning: TLS verification is enforced by default. If you encounter certificate errors, set OS_INSECURE=true to skip verification.\n")
	}
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	providerClient.HTTPClient = http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}

	// Authenticate
	authOpts := gophercloud.AuthOptions{
		IdentityEndpoint: openstackCredential.AuthURL,
		Username:         openstackCredential.Username,
		Password:         openstackCredential.Password,
		DomainName:       openstackCredential.DomainName,
		TenantName:       openstackCredential.TenantName,
	}
	if err := openstack.Authenticate(providerClient, authOpts); err != nil {
		var message string
		switch {
		case strings.Contains(err.Error(), "401"):
			message = "Authentication failed: invalid username, password, or project/domain. Please verify your credentials"
		case strings.Contains(err.Error(), "404"):
			message = "Authentication failed: the authentication URL or tenant/project name is incorrect"
		case strings.Contains(err.Error(), "timeout"):
			message = "Connection timeout: unable to reach the OpenStack authentication service. Please check your network connection and Auth URL"
		default:
			message = fmt.Sprintf("Authentication failed: %s. Please verify your OpenStack credentials", err.Error())
		}
		return ValidationResult{
			Valid:   false,
			Message: message,
			Error:   err,
		}
	}

	// Verify credentials match current environment
	_, err = verifyCredentialsMatchCurrentEnvironment(providerClient, openstackCredential.RegionName)
	if err != nil {
		if strings.Contains(err.Error(), "Creds are valid but for a different OpenStack environment") {
			return ValidationResult{
				Valid:   false,
				Message: "Creds are valid but for a different OpenStack environment",
				Error:   err,
			}
		}
		return ValidationResult{
			Valid:   false,
			Message: fmt.Sprintf("Failed to verify credentials against current environment: %s", err.Error()),
			Error:   err,
		}
	}

	return ValidationResult{
		Valid:   true,
		Message: "Successfully authenticated to Openstack",
		Error:   nil,
	}
}

// getCredentialsFromSecret retrieves OpenStack credentials from a Kubernetes secret
func getCredentialsFromSecret(ctx context.Context, k8sClient client.Client, secretName string) (vjailbreakv1alpha1.OpenStackCredsInfo, error) {
	var openstackCredsInfo vjailbreakv1alpha1.OpenStackCredsInfo
	secret := &corev1.Secret{}
	err := k8sClient.Get(ctx, k8stypes.NamespacedName{Name: secretName, Namespace: namespaceMigrationSystem}, secret)
	if err != nil {
		return openstackCredsInfo, errors.Wrap(err, "failed to get secret")
	}

	fields := map[string]string{
		"AuthURL":    string(secret.Data["OS_AUTH_URL"]),
		"DomainName": string(secret.Data["OS_DOMAIN_NAME"]),
		"Username":   string(secret.Data["OS_USERNAME"]),
		"Password":   string(secret.Data["OS_PASSWORD"]),
		"TenantName": string(secret.Data["OS_TENANT_NAME"]),
		"RegionName": string(secret.Data["OS_REGION_NAME"]),
	}

	for key, value := range fields {
		if value == "" {
			return openstackCredsInfo, fmt.Errorf("field %s is empty or missing in secret", key)
		}
	}

	openstackCredsInfo.AuthURL = fields["AuthURL"]
	openstackCredsInfo.Username = fields["Username"]
	openstackCredsInfo.Password = fields["Password"]
	openstackCredsInfo.DomainName = fields["DomainName"]
	openstackCredsInfo.TenantName = fields["TenantName"]
	openstackCredsInfo.RegionName = fields["RegionName"]

	insecureStr := string(secret.Data["OS_INSECURE"])
	openstackCredsInfo.Insecure = strings.EqualFold(strings.TrimSpace(insecureStr), "true")

	return openstackCredsInfo, nil
}

// verifyCredentialsMatchCurrentEnvironment checks if the provided credentials can access the current instance
func verifyCredentialsMatchCurrentEnvironment(providerClient *gophercloud.ProviderClient, regionName string) (bool, error) {
	// Get current instance metadata
	metadata, err := utils.GetCurrentInstanceMetadata()
	if err != nil {
		return false, fmt.Errorf("unable to get current instance metadata: %w. "+
			"Please ensure this is running on an OpenStack instance with metadata service enabled", err)
	}

	computeClient, err := openstack.NewComputeV2(providerClient, gophercloud.EndpointOpts{
		Region: regionName,
	})

	if err != nil {
		return false, fmt.Errorf("failed to create OpenStack compute client: %w", err)
	}
	_, err = servers.Get(computeClient, metadata.UUID).Extract()
	if err != nil {
		if strings.Contains(err.Error(), "Resource not found") ||
			strings.Contains(err.Error(), "No server with a name or ID") {
			return false, errors.New("Creds are valid but for a different OpenStack environment")
		}
		return false, fmt.Errorf("failed to verify instance access: %w. "+
			"Please check if the provided credentials have compute:get_server permission", err)
	}
	return true, nil
}

// PostValidate performs resource discovery after successful OpenStack validation
func PostValidate(ctx context.Context, k8sClient client.Client, oscreds *vjailbreakv1alpha1.OpenstackCreds, isLocal bool) error {
	ctxlog := log.FromContext(ctx)
	ctxlog.Info("Starting OpenStack resource discovery", "name", oscreds.Name)

	// Create scope for utils functions
	openstackScope, err := scope.NewOpenstackCredsScope(scope.OpenstackCredsScopeParams{
		Client:         k8sClient,
		OpenstackCreds: oscreds,
	})
	if err != nil {
		return errors.Wrap(err, "failed to create OpenstackCreds scope")
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(oscreds, constants.OpenstackCredsFinalizer) {
		controllerutil.AddFinalizer(oscreds, constants.OpenstackCredsFinalizer)
		if err := k8sClient.Update(ctx, oscreds); err != nil {
			return errors.Wrap(err, "failed to add finalizer")
		}
		ctxlog.Info("Added finalizer to OpenstackCreds", "name", oscreds.Name)
	}

	// Update master node image ID
	ctxlog.Info("Updating master node image ID", "name", oscreds.Name)
	err = utils.UpdateMasterNodeImageID(ctx, k8sClient, isLocal)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			ctxlog.Error(err, "Failed to update master node image ID (404), skipping")
		} else {
			ctxlog.Error(err, "Failed to update master node image ID")
		}
	}

	// Create dummy PCD cluster for standalone PCD hosts
	ctxlog.Info("Creating dummy PCD cluster", "name", oscreds.Name)
	err = utils.CreateDummyPCDClusterForStandAlonePCDHosts(ctx, k8sClient, oscreds)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "failed to create dummy PCD cluster")
	}

	// List all flavors
	ctxlog.Info("Fetching all OpenStack flavors", "name", oscreds.Name)
	flavors, err := utils.ListAllFlavors(ctx, k8sClient, oscreds)
	if err != nil {
		return errors.Wrap(err, "failed to get flavors")
	}
	ctxlog.Info(fmt.Sprintf("Fetched %d flavors from OpenStack", len(flavors)), "name", oscreds.Name)

	// Update spec with flavors
	oscreds.Spec.Flavors = flavors

	// Update the spec
	if err = k8sClient.Update(ctx, oscreds); err != nil {
		return errors.Wrap(err, "failed to update OpenstackCreds spec")
	}

	// Get OpenStack info (networks, clusters, etc.)
	ctxlog.Info("Fetching OpenStack info (networks, clusters)", "name", oscreds.Name)
	openstackinfo, err := utils.GetOpenstackInfo(ctx, k8sClient, oscreds)
	if err != nil {
		return errors.Wrap(err, "failed to get OpenStack info")
	}

	// Update status with OpenStack info
	oscreds.Status.Openstack = *openstackinfo
	ctxlog.Info("Updating OpenstackCreds status with info", "name", oscreds.Name)
	if err := k8sClient.Status().Update(ctx, oscreds); err != nil {
		return errors.Wrap(err, "failed to update OpenstackCreds status")
	}

	// Get vjailbreak settings to check if we should populate VMwareMachine flavors
	vjailbreakSettings, err := k8sutils.GetVjailbreakSettings(ctx, k8sClient)
	if err != nil {
		return errors.Wrap(err, "failed to get vjailbreak settings")
	}

	// Only populate flavors if the setting is enabled
	if vjailbreakSettings.PopulateVMwareMachineFlavors {
		ctxlog.Info("Populating VMwareMachine objects with OpenStack flavors", "name", oscreds.Name)

		vmwaremachineList := &vjailbreakv1alpha1.VMwareMachineList{}
		if err := k8sClient.List(ctx, vmwaremachineList); err != nil {
			return errors.Wrap(err, "failed to list VMwareMachine objects")
		}

		for i := range vmwaremachineList.Items {
			vmwaremachine := &vmwaremachineList.Items[i]
			// Get the cpu and memory of the vmwaremachine object
			cpu := vmwaremachine.Spec.VMInfo.CPU
			memory := vmwaremachine.Spec.VMInfo.Memory

			// Get the closest flavor based on the cpu and memory
			flavor, err := utils.GetClosestFlavour(cpu, memory, flavors)
			if err != nil && !strings.Contains(err.Error(), "no suitable flavor found") {
				ctxlog.Info(fmt.Sprintf("Error getting flavor for VMwareMachine '%s'", vmwaremachine.Name))
				return errors.Wrap(err, "failed to get closest flavor")
			}

			// Label the vmwaremachine object with the flavor name
			if flavor == nil {
				if err := utils.CreateOrUpdateLabel(ctx, k8sClient, vmwaremachine, oscreds.Name, "NOT_FOUND"); err != nil {
					return errors.Wrap(err, "failed to update VMwareMachine object")
				}
			} else {
				if err := utils.CreateOrUpdateLabel(ctx, k8sClient, vmwaremachine, oscreds.Name, flavor.ID); err != nil {
					return errors.Wrap(err, "failed to update VMwareMachine object")
				}
			}
		}
		ctxlog.Info("Completed populating VMwareMachine flavors", "name", oscreds.Name)
	} else {
		ctxlog.Info("Skipping VMwareMachine flavor population (disabled in settings)", "name", oscreds.Name)
	}

	// Handle PCD clusters
	if utils.IsOpenstackPCD(*oscreds) {
		if err := openstackScope.Close(); err != nil {
			return errors.Wrap(err, "failed to close scope")
		}
	}

	ctxlog.Info("OpenStack resource discovery completed successfully", "name", oscreds.Name)
	return nil
}