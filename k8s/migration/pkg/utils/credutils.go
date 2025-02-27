package utils

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// VMwareCredsFromSecret holds the actual credentials after decoding
type VMwareCredsFromSecret struct {
	Host     string
	Username string
	Password string
	Insecure bool
}

// OpenStackCredentials holds the actual credentials after decoding
type OpenStackCredentialsFromSecret struct {
	AuthURL           string
	DomainName        string
	Username          string
	Password          string
	RegionName        string
	TenantName        string
	Insecure          bool
	ProjectName       string
	UserDomainName    string
	ProjectDomainName string
}

// getVMwareCredsFromSecret retrieves vCenter credentials from a secret
func GetVMwareCredsFromSecret(ctx context.Context, secretName string) (VMwareCredsFromSecret, error) {
	secret := &corev1.Secret{}

	// Get In cluster client
	c, err := GetInclusterClient()
	if err != nil {
		return VMwareCredsFromSecret{}, fmt.Errorf("failed to get in cluster client: %w", err)
	}

	if err := c.Get(ctx, client.ObjectKey{Namespace: "migration-system", Name: secretName}, secret); err != nil {
		return VMwareCredsFromSecret{}, fmt.Errorf("failed to get secret '%s': %w", secretName, err)
	}

	if secret.Data == nil {
		return VMwareCredsFromSecret{}, fmt.Errorf("no data in secret '%s'", secretName)
	}

	host := string(secret.Data["VCENTER_HOST"])
	username := string(secret.Data["VCENTER_USERNAME"])
	password := string(secret.Data["VCENTER_PASSWORD"])
	insecureStr := string(secret.Data["VCENTER_INSECURE"])

	if host == "" {
		return VMwareCredsFromSecret{}, fmt.Errorf("VCENTER_HOST is missing in secret '%s'", secretName)
	}
	if username == "" {
		return VMwareCredsFromSecret{}, fmt.Errorf("VCENTER_USERNAME is missing in secret '%s'", secretName)
	}
	if password == "" {
		return VMwareCredsFromSecret{}, fmt.Errorf("VCENTER_PASSWORD is missing in secret '%s'", secretName)
	}

	insecure := insecureStr == "true"

	return VMwareCredsFromSecret{
		Host:     host,
		Username: username,
		Password: password,
		Insecure: insecure,
	}, nil
}

// getOpenStackCreds retrieves and checks the secret
func GetOpenstackCredsFromSecret(ctx context.Context, secretName string) (OpenStackCredentialsFromSecret, error) {
	secret := &corev1.Secret{}
	// Get In cluster client
	c, err := GetInclusterClient()
	if err != nil {
		return OpenStackCredentialsFromSecret{}, fmt.Errorf("failed to get in cluster client: %w", err)
	}
	if err := c.Get(ctx, client.ObjectKey{Namespace: "migration-system", Name: secretName}, secret); err != nil {
		return OpenStackCredentialsFromSecret{}, fmt.Errorf("failed to get secret: %w", err)
	}

	// Extract and validate each field
	fields := map[string]string{
		"AuthURL":           string(secret.Data["OS_AUTH_URL"]),
		"DomainName":        string(secret.Data["OS_DOMAIN_NAME"]),
		"Username":          string(secret.Data["OS_USERNAME"]),
		"Password":          string(secret.Data["OS_PASSWORD"]),
		"RegionName":        string(secret.Data["OS_REGION_NAME"]),
		"TenantName":        string(secret.Data["OS_TENANT_NAME"]),
		"ProjectName":       string(secret.Data["OS_PROJECT_NAME"]),
		"UserDomainName":    string(secret.Data["OS_USER_DOMAIN_NAME"]),
		"ProjectDomainName": string(secret.Data["OS_PROJECT_DOMAIN_NAME"]),
	}

	for key, value := range fields {
		if value == "" {
			return OpenStackCredentialsFromSecret{}, fmt.Errorf("%s is missing in secret '%s'", key, secretName)
		}
	}

	insecureStr := string(secret.Data["OS_INSECURE"])
	insecure := insecureStr == "true"

	return OpenStackCredentialsFromSecret{
		AuthURL:           fields["AuthURL"],
		DomainName:        fields["DomainName"],
		Username:          fields["Username"],
		Password:          fields["Password"],
		RegionName:        fields["RegionName"],
		TenantName:        fields["TenantName"],
		ProjectName:       fields["ProjectName"],
		UserDomainName:    fields["UserDomainName"],
		ProjectDomainName: fields["ProjectDomainName"],
		Insecure:          insecure,
	}, nil
}
