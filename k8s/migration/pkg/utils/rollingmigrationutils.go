package utils

import (
	"context"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
)

func CreateClusterMigration(ctx context.Context, cluster string) (*vjailbreakv1alpha1.ClusterMigration, error) {
	return &vjailbreakv1alpha1.ClusterMigration{}, nil
}

func GetClusterMigration(ctx context.Context, cluster string) (*vjailbreakv1alpha1.ClusterMigration, error) {
	return &vjailbreakv1alpha1.ClusterMigration{}, nil
}

func GetESXIMigration(ctx context.Context, esxi string) (*vjailbreakv1alpha1.ESXIMigration, error) {
	return &vjailbreakv1alpha1.ESXIMigration{}, nil
}

func CreateESXIMigration(ctx context.Context, esxi string) (*vjailbreakv1alpha1.ESXIMigration, error) {
	return &vjailbreakv1alpha1.ESXIMigration{}, nil
}
