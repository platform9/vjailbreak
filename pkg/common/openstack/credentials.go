package openstack

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	corev1 "k8s.io/api/core/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	trueString = "true"
)

// GetOpenstackCredsInfo retrieves OpenStack credentials from an OpenstackCreds resource
func GetOpenstackCredsInfo(ctx context.Context, k3sclient client.Client, credsName string) (vjailbreakv1alpha1.OpenStackCredsInfo, error) {
	creds := vjailbreakv1alpha1.OpenstackCreds{}
	if err := k3sclient.Get(ctx, k8stypes.NamespacedName{Namespace: constants.NamespaceMigrationSystem, Name: credsName}, &creds); err != nil {
		return vjailbreakv1alpha1.OpenStackCredsInfo{}, errors.Wrapf(err, "failed to get OpenStack credentials '%s'", credsName)
	}
	return GetOpenstackCredentialsFromSecret(ctx, k3sclient, creds.Spec.SecretRef.Name)
}

// GetOpenstackCredentialsFromSecret retrieves and validates OpenStack credentials from a Kubernetes secret
func GetOpenstackCredentialsFromSecret(ctx context.Context, k3sclient client.Client, secretName string) (vjailbreakv1alpha1.OpenStackCredsInfo, error) {
	secret := &corev1.Secret{}
	if err := k3sclient.Get(ctx, k8stypes.NamespacedName{Namespace: constants.NamespaceMigrationSystem, Name: secretName}, secret); err != nil {
		return vjailbreakv1alpha1.OpenStackCredsInfo{}, errors.Wrap(err, "failed to get secret")
	}

	// Check which authentication method is being used
	authToken := string(secret.Data["OS_AUTH_TOKEN"])
	username := string(secret.Data["OS_USERNAME"])
	password := string(secret.Data["OS_PASSWORD"])

	// Common required fields for both auth methods
	authURL := string(secret.Data["OS_AUTH_URL"])
	tenantName := string(secret.Data["OS_TENANT_NAME"])
	regionName := string(secret.Data["OS_REGION_NAME"])

	// Validate common required fields
	if authURL == "" {
		return vjailbreakv1alpha1.OpenStackCredsInfo{}, errors.Errorf("OS_AUTH_URL is missing in secret '%s'", secretName)
	}
	if tenantName == "" {
		return vjailbreakv1alpha1.OpenStackCredsInfo{}, errors.Errorf("OS_TENANT_NAME is missing in secret '%s'", secretName)
	}
	if regionName == "" {
		return vjailbreakv1alpha1.OpenStackCredsInfo{}, errors.Errorf("OS_REGION_NAME is missing in secret '%s'", secretName)
	}

	var openstackCredsInfo vjailbreakv1alpha1.OpenStackCredsInfo

	// Determine authentication method and validate accordingly
	//nolint:gocritic
	if authToken != "" {
		// Token-based authentication
		openstackCredsInfo.AuthToken = authToken
		openstackCredsInfo.AuthURL = authURL
		openstackCredsInfo.TenantName = tenantName
		openstackCredsInfo.RegionName = regionName
		// DomainName is optional for token-based auth
		openstackCredsInfo.DomainName = string(secret.Data["OS_DOMAIN_NAME"])
	} else if username != "" && password != "" {
		// Password-based authentication
		domainName := string(secret.Data["OS_DOMAIN_NAME"])
		if domainName == "" {
			return vjailbreakv1alpha1.OpenStackCredsInfo{}, errors.Errorf("OS_DOMAIN_NAME is missing in secret '%s' for password-based auth", secretName)
		}

		openstackCredsInfo.AuthURL = authURL
		openstackCredsInfo.Username = username
		openstackCredsInfo.Password = password
		openstackCredsInfo.DomainName = domainName
		openstackCredsInfo.TenantName = tenantName
		openstackCredsInfo.RegionName = regionName
	} else {
		// Neither authentication method has complete credentials
		return vjailbreakv1alpha1.OpenStackCredsInfo{}, errors.Errorf("missing required fields in secret '%s': either OS_AUTH_TOKEN or (OS_USERNAME and OS_PASSWORD) must be provided", secretName)
	}

	// Parse insecure flag
	insecureStr := string(secret.Data["OS_INSECURE"])
	openstackCredsInfo.Insecure = strings.EqualFold(strings.TrimSpace(insecureStr), trueString)

	return openstackCredsInfo, nil
}
