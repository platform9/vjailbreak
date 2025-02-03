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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/constants"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/scope"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/utils"
)

// VjailbreakNodeReconciler reconciles a VjailbreakNode object
type VjailbreakNodeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=vjailbreaknodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=vjailbreaknodes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=vjailbreaknodes/finalizers,verbs=update
// +kubebuilder:rbac:groups=,resources=nodes,verbs=get;list;watch

func (r *VjailbreakNodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := log.FromContext(ctx).WithName(constants.VjailbreakNodeControllerName)

	// Fetch the VjailbreakNode instance.
	log.Info("Fetching VjailbreakNode")
	vjailbreakNode := vjailbreakv1alpha1.VjailbreakNode{}
	err := r.Client.Get(ctx, req.NamespacedName, &vjailbreakNode, &client.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	vjailbreakNodeScope, err := scope.NewVjailbreakNodeScope(scope.VjailbreakNodeScopeParams{
		Logger:         log,
		Client:         r.Client,
		VjailbreakNode: &vjailbreakNode,
	})
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to create vjailbreak node scope")
	}

	// Always close the scope when exiting this function such that we can persist any VjailbreakNode changes.
	defer func() {
		if err := vjailbreakNodeScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted VjailbreakNode
	if !vjailbreakNode.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, vjailbreakNodeScope)
	}

	// Handle regular VjailbreakNode reconcile
	return r.reconcileNormal(ctx, vjailbreakNodeScope)
}

// reconcileNormal handles regular VjailbreakNode reconcile
func (r *VjailbreakNodeReconciler) reconcileNormal(ctx context.Context, scope *scope.VjailbreakNodeScope) (ctrl.Result, error) {
	log := scope.Logger
	vjNode := scope.VjailbreakNode

	// Check and create master node entry
	err := utils.CheckAndCreateMasterNodeEntry(ctx, r.Client)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to check and create master node entry")
	}

	if vjNode.Spec.NodeRole == constants.NodeRoleMaster {
		log.Info("Skipping master node")
		return ctrl.Result{}, nil
	}

	// Create Openstack VM for worker node
	vmid, err := utils.CreateOpenstackVMForWorkerNode(ctx, r.Client, scope)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to create openstack vm for worker node")
	}

	if vmid != "" {
		vjNode.Status.OpenstackUUID = vmid
	}

	return ctrl.Result{}, nil
}

// reconcileDelete handles deleted VjailbreakNode
func (r *VjailbreakNodeReconciler) reconcileDelete(ctx context.Context, scope *scope.VjailbreakNodeScope) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VjailbreakNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vjailbreakv1alpha1.VjailbreakNode{}).
		Complete(r)
}
