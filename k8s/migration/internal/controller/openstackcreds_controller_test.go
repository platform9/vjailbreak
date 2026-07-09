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
// removes non-master VjailbreakNodes referencing the deleted OpenstackCreds.
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
	if err := r.reconcileDelete(context.Background(), credScope); err != nil {
		t.Fatalf("reconcileDelete returned error: %v", err)
	}

	// Worker node referencing the creds should be gone.
	remaining := &vjailbreakv1alpha1.VjailbreakNode{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "vjailbreak-agent-abc", Namespace: ns}, remaining); err == nil {
		t.Error("expected worker node to be deleted, but it still exists")
	}

	// Master node should survive.
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "vjailbreak-master", Namespace: ns}, remaining); err != nil {
		t.Errorf("master node should not be deleted: %v", err)
	}

	// Node referencing different creds should survive.
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "vjailbreak-agent-other", Namespace: ns}, remaining); err != nil {
		t.Errorf("node for other creds should not be deleted: %v", err)
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
