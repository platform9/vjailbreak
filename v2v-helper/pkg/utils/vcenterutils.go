package utils

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/pkg/errors"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/constants"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

type MigrationParams struct {
	// vCenter params
	SourceVMName string

	// openstack params
	OpenstackNetworkNames string
	OpenstackNetworkPorts string
	OpenstackVolumeTypes  string
	OpenstackVirtioWin    string
	OpenstackOSType       string
	OpenstackConvert      bool

	// Migration params
	DataCopyStart       string
	VMcutoverStart      string
	VMcutoverEnd        string
	MigrationType       string
	PerformHealthChecks bool
	HealthCheckPort     string
	Debug               bool
}

// GetVCenterClient is function that returns vCenter client based on values picked from a secret
// func GetVCenterClient(ctx context.Context, configmap *v1.ConfigMap) (*vcenter.VCenterClient, error) {
// 	// Get the secret
// 	secret := &v1.Secret{}
// }

// GetMigrationSecretName is function that returns the name of the secret
func GetMigrationSecretName(vmname string) string {
	return fmt.Sprintf("migration-secret-%s", vmname)
}

// GetMigrationParams is function that returns the migration parameters
func GetMigrationParams(ctx context.Context, client client.Client) (*MigrationParams, error) {
	// Get the values from the secret
	secret := &v1.Secret{}
	err := client.Get(ctx, types.NamespacedName{
		Name:      GetMigrationSecretName(os.Getenv("SOURCE_VM_NAME")),
		Namespace: constants.MigrationSystemNamespace,
	}, secret)
	if err != nil {
		log.Printf("Failed to get secret: %v\n", err)
		return nil, errors.Wrap(err, "Failed to get secret")
	}
	return &MigrationParams{
		SourceVMName:          string(secret.Data["SOURCE_VM_NAME"]),
		OpenstackNetworkNames: string(secret.Data["NEUTRON_NETWORK_NAMES"]),
		OpenstackNetworkPorts: string(secret.Data["NEUTRON_PORT_IDS"]),
		OpenstackVolumeTypes:  string(secret.Data["CINDER_VOLUME_TYPES"]),
		OpenstackVirtioWin:    string(secret.Data["VIRTIO_WIN_DRIVER"]),
		OpenstackOSType:       string(secret.Data["OS_TYPE"]),
		OpenstackConvert:      string(secret.Data["CONVERT"]) == constants.TrueString,
		DataCopyStart:         string(secret.Data["DATACOPYSTART"]),
		VMcutoverStart:        string(secret.Data["CUTOVERSTART"]),
		VMcutoverEnd:          string(secret.Data["CUTOVEREND"]),
		MigrationType:         string(secret.Data["TYPE"]),
		PerformHealthChecks:   string(secret.Data["PERFORM_HEALTH_CHECKS"]) == constants.TrueString,
		HealthCheckPort:       string(secret.Data["HEALTH_CHECK_PORT"]),
		Debug:                 string(secret.Data["DEBUG"]) == constants.TrueString,
	}, nil
}
