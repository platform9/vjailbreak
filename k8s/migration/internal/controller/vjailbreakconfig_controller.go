/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/constants"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/utils"
)

// VjailbreakConfigReconciler reconciles a VjailbreakConfig object
type VjailbreakConfigReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=vjailbreakconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=vjailbreakconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=vjailbreakconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;delete

func (r *VjailbreakConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctxlog := log.FromContext(ctx).WithName(constants.VjailbreakConfigControllerName)
	ctxlog.Info("Reconciling VjailbreakConfig")

	vjailbreakConfig := vjailbreakv1alpha1.VjailbreakConfig{}
	if err := r.Get(ctx, req.NamespacedName, &vjailbreakConfig); err != nil {
		return ctrl.Result{}, err
	}

	if vjailbreakConfig.Spec.Debug {
		ctxlog.Info("Debug mode enabled")
		// list all the current migrations and add "debug: true" to the configmap
		migrationObjects := &vjailbreakv1alpha1.MigrationList{}
		if err := r.List(ctx, migrationObjects); err != nil {
			return ctrl.Result{}, err
		}
		for _, migration := range migrationObjects.Items {
			vmName, err := utils.ConvertToK8sName(migration.Spec.VMName)
			if err != nil {
				return ctrl.Result{}, errors.Wrap(err, "Failed to convert vmName to k8s appropriate name")
			}
			configMapName := utils.GetMigrationConfigMapName(vmName)

			configMap := &corev1.ConfigMap{}
			if err := r.Get(ctx, types.NamespacedName{Name: configMapName}, configMap); err != nil {
				return ctrl.Result{}, errors.Wrap(err, "Failed to get configmap")
			}
			configMap.Data["DEBUG"] = "true"
			if err := r.Update(ctx, configMap); err != nil {
				return ctrl.Result{}, errors.Wrap(err, "Failed to update configmap")
			}
			ctxlog.Info("Updated configmap", "configmap", configMapName)
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VjailbreakConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vjailbreakv1alpha1.VjailbreakConfig{}).
		Complete(r)
}
