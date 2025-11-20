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
	"github.com/platform9/vjailbreak/k8s/migration/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	requiredFields := []string{"auth_url", "username", "password", "domain_name", "tenant_name", "region_name"}
	for _, field := range requiredFields {
		if _, ok := secret.Data[field]; !ok {
			return openstackCredsInfo, fmt.Errorf("missing required field '%s' in secret", field)
		}
	}

	openstackCredsInfo.AuthURL = string(secret.Data["auth_url"])
	openstackCredsInfo.Username = string(secret.Data["username"])
	openstackCredsInfo.Password = string(secret.Data["password"])
	openstackCredsInfo.DomainName = string(secret.Data["domain_name"])
	openstackCredsInfo.TenantName = string(secret.Data["tenant_name"])
	openstackCredsInfo.RegionName = string(secret.Data["region_name"])

	if insecureVal, ok := secret.Data["insecure"]; ok {
		openstackCredsInfo.Insecure = string(insecureVal) == "true"
	}

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
