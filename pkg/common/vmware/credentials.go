package vmware

import (
	"context"
	"fmt"
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

// GetVMwareCredsInfo retrieves vCenter credentials from a VMwareCreds resource
func GetVMwareCredsInfo(ctx context.Context, k3sclient client.Client, credsName string) (vjailbreakv1alpha1.VMwareCredsInfo, error) {
	creds := vjailbreakv1alpha1.VMwareCreds{}
	if err := k3sclient.Get(ctx, k8stypes.NamespacedName{Namespace: constants.NamespaceMigrationSystem, Name: credsName}, &creds); err != nil {
		return vjailbreakv1alpha1.VMwareCredsInfo{}, errors.Wrapf(err, "failed to get VMware credentials '%s'", credsName)
	}
	return GetVMwareCredentialsFromSecret(ctx, k3sclient, creds.Spec.SecretRef.Name)
}

// GetVMwareCredentialsFromSecret retrieves vCenter credentials from a Kubernetes secret
func GetVMwareCredentialsFromSecret(ctx context.Context, k3sclient client.Client, secretName string) (vjailbreakv1alpha1.VMwareCredsInfo, error) {
	secret := &corev1.Secret{}

	if err := k3sclient.Get(ctx, k8stypes.NamespacedName{Namespace: constants.NamespaceMigrationSystem, Name: secretName}, secret); err != nil {
		return vjailbreakv1alpha1.VMwareCredsInfo{}, errors.Wrapf(err, "failed to get secret '%s'", secretName)
	}

	if secret.Data == nil {
		return vjailbreakv1alpha1.VMwareCredsInfo{}, fmt.Errorf("no data in secret '%s'", secretName)
	}

	host := string(secret.Data["VCENTER_HOST"])
	username := string(secret.Data["VCENTER_USERNAME"])
	password := string(secret.Data["VCENTER_PASSWORD"])
	insecureStr := string(secret.Data["VCENTER_INSECURE"])
	datacenter := string(secret.Data["VCENTER_DATACENTER"])

	if host == "" {
		return vjailbreakv1alpha1.VMwareCredsInfo{}, errors.Errorf("VCENTER_HOST is missing in secret '%s'", secretName)
	}
	if username == "" {
		return vjailbreakv1alpha1.VMwareCredsInfo{}, errors.Errorf("VCENTER_USERNAME is missing in secret '%s'", secretName)
	}
	if password == "" {
		return vjailbreakv1alpha1.VMwareCredsInfo{}, errors.Errorf("VCENTER_PASSWORD is missing in secret '%s'", secretName)
	}

	insecure := strings.EqualFold(strings.TrimSpace(insecureStr), trueString)

	return vjailbreakv1alpha1.VMwareCredsInfo{
		Host:       host,
		Username:   username,
		Password:   password,
		Datacenter: datacenter,
		Insecure:   insecure,
	}, nil
}
