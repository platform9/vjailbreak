package controller

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func findEnvFromSecret(t *testing.T, envFrom []corev1.EnvFromSource, name string) *corev1.EnvFromSource {
	t.Helper()
	for i := range envFrom {
		if envFrom[i].SecretRef != nil && envFrom[i].SecretRef.Name == name {
			return &envFrom[i]
		}
	}
	return nil
}

func TestBuildV2VHelperEnvFrom_AlwaysIncludesVMwareAndOpenstackSecrets(t *testing.T) {
	envFrom := buildV2VHelperEnvFrom("vmware-secret", "openstack-secret", "")

	if findEnvFromSecret(t, envFrom, "vmware-secret") == nil {
		t.Error("expected vmware-secret to be present in EnvFrom")
	}
	if findEnvFromSecret(t, envFrom, "openstack-secret") == nil {
		t.Error("expected openstack-secret to be present in EnvFrom")
	}
}

func TestBuildV2VHelperEnvFrom_OmitsArraySecretWhenEmpty(t *testing.T) {
	envFrom := buildV2VHelperEnvFrom("vmware-secret", "openstack-secret", "")

	for _, e := range envFrom {
		if e.SecretRef != nil && e.SecretRef.Name == "" {
			t.Error("expected no EnvFromSource with an empty secret name")
		}
	}
	if len(envFrom) != 4 {
		t.Errorf("expected 4 EnvFromSource entries without array creds, got %d", len(envFrom))
	}
}

func TestBuildV2VHelperEnvFrom_IncludesArraySecretWhenSet(t *testing.T) {
	envFrom := buildV2VHelperEnvFrom("vmware-secret", "openstack-secret", "array-secret")

	if findEnvFromSecret(t, envFrom, "array-secret") == nil {
		t.Error("expected array-secret to be present in EnvFrom when arrayCredsSecretRef is set")
	}
	if len(envFrom) != 5 {
		t.Errorf("expected 5 EnvFromSource entries with array creds, got %d", len(envFrom))
	}
}

func TestBuildV2VHelperEnvFrom_IncludesPf9EnvConfigMap(t *testing.T) {
	envFrom := buildV2VHelperEnvFrom("vmware-secret", "openstack-secret", "")

	var found *corev1.EnvFromSource
	for i := range envFrom {
		if envFrom[i].ConfigMapRef != nil && envFrom[i].ConfigMapRef.Name == "pf9-env" {
			found = &envFrom[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected pf9-env ConfigMapRef to be present in EnvFrom")
	}
}

func TestBuildV2VHelperEnvFrom_ProxyCredsSecretIsOptional(t *testing.T) {
	envFrom := buildV2VHelperEnvFrom("vmware-secret", "openstack-secret", "")

	proxyCreds := findEnvFromSecret(t, envFrom, "pf9-proxy-creds")
	if proxyCreds == nil {
		t.Fatal("expected pf9-proxy-creds SecretRef to be present in EnvFrom")
	}
	if proxyCreds.SecretRef.Optional == nil || !*proxyCreds.SecretRef.Optional {
		t.Error("expected pf9-proxy-creds SecretRef to be marked Optional so the job still runs without it")
	}
}
