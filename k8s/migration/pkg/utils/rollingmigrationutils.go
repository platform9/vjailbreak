package utils

import (
	"context"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	scope "github.com/platform9/vjailbreak/k8s/migration/pkg/scope"
)

func CreateClusterMigration(ctx context.Context, scope *scope.RollingMigrationPlanScope) error {
	return nil
}

func GetClusterMigration(ctx context.Context, scope *scope.RollingMigrationPlanScope) (vjailbreakv1alpha1.ClusterMigration, error) {
	return vjailbreakv1alpha1.ClusterMigration{}, nil
}
