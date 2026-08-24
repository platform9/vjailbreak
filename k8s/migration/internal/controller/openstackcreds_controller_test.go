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
	"fmt"
	"testing"

	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/flavors"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	openstackvalidation "github.com/platform9/vjailbreak/pkg/common/validation/openstack"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	scope "github.com/platform9/vjailbreak/k8s/migration/pkg/scope"
	constants "github.com/platform9/vjailbreak/pkg/common/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// TestApplyValidationResult_ValidationFailure tests that applyValidationResult
// marks validation failures with the single terminal Failed status.
func TestApplyValidationResult_ValidationFailure(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = vjailbreakv1alpha1.AddToScheme(scheme)

	const ns = "default"
	const name = "test-oscreds"

	tests := []struct {
		name                 string
		result               openstackvalidation.ValidationResult
		wantValidationStatus string
	}{
		{
			name:                 "auth failure marks failed",
			result:               openstackvalidation.ValidationResult{Valid: false, Error: fmt.Errorf("auth failed"), Message: "auth failed"},
			wantValidationStatus: constants.ValidationStatusFailed,
		},
		{
			name:                 "connection failure marks failed",
			result:               openstackvalidation.ValidationResult{Valid: false, Error: fmt.Errorf("connection refused"), Message: "connection refused"},
			wantValidationStatus: constants.ValidationStatusFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oscreds := &vjailbreakv1alpha1.OpenstackCreds{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			}
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(oscreds).
				WithStatusSubresource(&vjailbreakv1alpha1.OpenstackCreds{}).
				Build()

			credScope, _ := scope.NewOpenstackCredsScope(scope.OpenstackCredsScopeParams{
				Client:         fakeClient,
				OpenstackCreds: oscreds,
			})
			r := &OpenstackCredsReconciler{Client: fakeClient, Scheme: scheme}

			_ = r.applyValidationResult(context.Background(), credScope, tt.result)

			updated := &vjailbreakv1alpha1.OpenstackCreds{}
			if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: name, Namespace: ns}, updated); err != nil {
				t.Fatalf("failed to get updated OpenstackCreds: %v", err)
			}
			if updated.Status.OpenStackValidationStatus != tt.wantValidationStatus {
				t.Errorf("OpenStackValidationStatus = %q, want %q", updated.Status.OpenStackValidationStatus, tt.wantValidationStatus)
			}
		})
	}
}

// TestReconcileDelete_DeletesNonMasterVjailbreakNodes verifies that reconcileDelete
// issues delete on non-master nodes, requeues while they are pending, then
// removes the OpenstackCreds finalizer only after all target nodes are gone.
func TestReconcileDelete_DeletesNonMasterVjailbreakNodes(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = vjailbreakv1alpha1.AddToScheme(scheme)

	const ns = "migration-system"
	const credName = "test-creds"

	oscreds := &vjailbreakv1alpha1.OpenstackCreds{
		ObjectMeta: metav1.ObjectMeta{
			Name:       credName,
			Namespace:  ns,
			Finalizers: []string{constants.OpenstackCredsFinalizer},
		},
	}

	workerNode := &vjailbreakv1alpha1.VjailbreakNode{
		ObjectMeta: metav1.ObjectMeta{Name: "vjailbreak-agent-abc", Namespace: ns},
		Spec: vjailbreakv1alpha1.VjailbreakNodeSpec{
			NodeRole: "worker",
			OpenstackCreds: corev1.ObjectReference{
				Name:      credName,
				Namespace: ns,
			},
			OpenstackFlavorID: "f1",
			OpenstackImageID:  "i1",
		},
	}

	masterNode := &vjailbreakv1alpha1.VjailbreakNode{
		ObjectMeta: metav1.ObjectMeta{Name: "vjailbreak-master", Namespace: ns},
		Spec: vjailbreakv1alpha1.VjailbreakNodeSpec{
			NodeRole: constants.NodeRoleMaster,
			OpenstackCreds: corev1.ObjectReference{
				Name:      credName,
				Namespace: ns,
			},
			OpenstackFlavorID: "f1",
			OpenstackImageID:  "i1",
		},
	}

	otherCredsNode := &vjailbreakv1alpha1.VjailbreakNode{
		ObjectMeta: metav1.ObjectMeta{Name: "vjailbreak-agent-other", Namespace: ns},
		Spec: vjailbreakv1alpha1.VjailbreakNodeSpec{
			NodeRole: "worker",
			OpenstackCreds: corev1.ObjectReference{
				Name:      "other-creds",
				Namespace: ns,
			},
			OpenstackFlavorID: "f1",
			OpenstackImageID:  "i1",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(oscreds, workerNode, masterNode, otherCredsNode).
		WithStatusSubresource(&vjailbreakv1alpha1.OpenstackCreds{}).
		Build()

	credScope, err := scope.NewOpenstackCredsScope(scope.OpenstackCredsScopeParams{
		Client:         fakeClient,
		OpenstackCreds: oscreds,
	})
	if err != nil {
		t.Fatalf("failed to create scope: %v", err)
	}

	r := &OpenstackCredsReconciler{Client: fakeClient, Scheme: scheme}

	// First call: issues Delete on the worker node, then returns an error to
	// requeue because the node is still counted as pending.
	if err := r.reconcileDelete(context.Background(), credScope); err == nil {
		t.Fatal("expected requeue error while worker node still pending, got nil")
	}

	// Fake client removes no-finalizer objects immediately on Delete, so the
	// worker node is already gone from the store. Second call should succeed.
	remaining := &vjailbreakv1alpha1.VjailbreakNode{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "vjailbreak-agent-abc", Namespace: ns}, remaining); err == nil {
		t.Error("expected worker node to be deleted after first reconcileDelete call")
	}

	if err := r.reconcileDelete(context.Background(), credScope); err != nil {
		t.Fatalf("second reconcileDelete returned unexpected error: %v", err)
	}

	// Master node must survive.
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "vjailbreak-master", Namespace: ns}, remaining); err != nil {
		t.Errorf("master node should not be deleted: %v", err)
	}

	// Node referencing different creds must survive.
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "vjailbreak-agent-other", Namespace: ns}, remaining); err != nil {
		t.Errorf("node for other creds should not be deleted: %v", err)
	}
}

// TestUpdateVMwareMachineWithFlavor_ErrorClassification checks that an
// unschedulable VM shape is a per-VM skip ("NOT_FOUND" label), while an
// empty candidate list still fails the sync.
func TestUpdateVMwareMachineWithFlavor_ErrorClassification(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = vjailbreakv1alpha1.AddToScheme(scheme)

	const ns = "default"
	const credName = "test-oscreds"

	smallFlavor := flavors.Flavor{ID: "small", VCPUs: 2, RAM: 4096}

	tests := []struct {
		name         string
		vmCPU        int
		vmMemory     int
		flavorList   []flavors.Flavor
		wantErr      bool
		wantFlavorID string
	}{
		{
			name:         "no candidate flavor fits the VM's shape: sentinel error, VM skipped not failed",
			vmCPU:        64,
			vmMemory:     262144,
			flavorList:   []flavors.Flavor{smallFlavor},
			wantErr:      false,
			wantFlavorID: "NOT_FOUND",
		},
		{
			name:       "empty candidate list: generic error, sync must fail",
			vmCPU:      2,
			vmMemory:   4096,
			flavorList: nil,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oscreds := &vjailbreakv1alpha1.OpenstackCreds{
				ObjectMeta: metav1.ObjectMeta{Name: credName, Namespace: ns},
			}
			vmwaremachine := &vjailbreakv1alpha1.VMwareMachine{
				ObjectMeta: metav1.ObjectMeta{Name: "test-vm", Namespace: ns},
				Spec: vjailbreakv1alpha1.VMwareMachineSpec{
					VMInfo: vjailbreakv1alpha1.VMInfo{
						Name:   "test-vm",
						CPU:    tt.vmCPU,
						Memory: tt.vmMemory,
					},
				},
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(oscreds, vmwaremachine).
				Build()

			credScope, err := scope.NewOpenstackCredsScope(scope.OpenstackCredsScopeParams{
				Client:         fakeClient,
				OpenstackCreds: oscreds,
			})
			if err != nil {
				t.Fatalf("failed to create scope: %v", err)
			}

			r := &OpenstackCredsReconciler{Client: fakeClient, Scheme: scheme}

			err = updateVMwareMachineWithFlavor(context.Background(), r, credScope, vmwaremachine, tt.flavorList)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an empty candidate list to fail the sync, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("expected an unschedulable VM shape to be classified as a per-VM skip (no error), got: %v", err)
			}

			updated := &vjailbreakv1alpha1.VMwareMachine{}
			if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "test-vm", Namespace: ns}, updated); err != nil {
				t.Fatalf("failed to get updated VMwareMachine: %v", err)
			}
			if got := updated.Labels[credName]; got != tt.wantFlavorID {
				t.Errorf("flavor label = %q, want %q", got, tt.wantFlavorID)
			}
		})
	}
}

var _ = ginkgo.Describe("OpenstackCreds Controller", func() {
	ginkgo.Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		openstackcreds := &vjailbreakv1alpha1.OpenstackCreds{}

		ginkgo.BeforeEach(func() {
			ginkgo.By("creating the custom resource for the Kind OpenstackCreds")
			err := k8sClient.Get(ctx, typeNamespacedName, openstackcreds)
			if err != nil && errors.IsNotFound(err) {
				resource := &vjailbreakv1alpha1.OpenstackCreds{
					ObjectMeta: metav1.ObjectMeta{
						Name: resourceName,
					},
				}
				gomega.Expect(k8sClient.Create(ctx, resource)).To(gomega.Succeed())
			}
		})

		ginkgo.AfterEach(func() {
			resource := &vjailbreakv1alpha1.OpenstackCreds{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			ginkgo.By("Cleanup the specific resource instance OpenstackCreds")
			gomega.Expect(k8sClient.Delete(ctx, resource)).To(gomega.Succeed())
		})

		ginkgo.It("should successfully reconcile the resource", func() {
			ginkgo.By("Reconciling the created resource")
			controllerReconciler := &OpenstackCredsReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})
})
