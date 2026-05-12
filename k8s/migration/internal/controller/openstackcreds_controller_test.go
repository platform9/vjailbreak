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

// TestPCDSyncStatusUpdate_RegressionForUpdateResetBug guards against the bug where
// runPCDSyncAsync called r.Update() before r.Status().Update(), causing the
// in-memory status to be reset by the server response from Update() (which carries
// the server-side status, since the status subresource ignores Update() writes).
// The visible symptom was OpenStack/PCD credentials stuck in "Revalidating"
// forever in the UI. The fix saves the desired status to a local variable and
// re-applies it after r.Update() but before r.Status().Update().
func TestPCDSyncStatusUpdate_RegressionForUpdateResetBug(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = vjailbreakv1alpha1.AddToScheme(scheme)

	const ns = "migration-system"
	const name = "test-pcd"

	// Server starts with status=Revalidating (set earlier by applyValidationResult).
	oscreds := &vjailbreakv1alpha1.OpenstackCreds{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   ns,
			Annotations: map[string]string{"pcd-sync-in-progress": "true"},
		},
	}
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(oscreds).
		WithStatusSubresource(&vjailbreakv1alpha1.OpenstackCreds{}).
		Build()

	oscreds.Status.OpenStackValidationStatus = constants.ValidationStatusRevalidating
	if err := fakeClient.Status().Update(context.Background(), oscreds); err != nil {
		t.Fatalf("seed status: %v", err)
	}

	// Reproduce the post-sync update sequence used by runPCDSyncAsync.
	latestCreds := &vjailbreakv1alpha1.OpenstackCreds{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: name, Namespace: ns}, latestCreds); err != nil {
		t.Fatalf("get: %v", err)
	}
	delete(latestCreds.Annotations, "pcd-sync-in-progress")
	desiredValidationStatus := "Succeeded"
	latestCreds.Status.OpenStackValidationStatus = desiredValidationStatus

	if err := fakeClient.Update(context.Background(), latestCreds); err != nil {
		t.Fatalf("update: %v", err)
	}
	// Critical: r.Update() reset latestCreds.Status to the server-side value
	// (Revalidating). Re-apply the desired status before Status().Update().
	latestCreds.Status.OpenStackValidationStatus = desiredValidationStatus
	if err := fakeClient.Status().Update(context.Background(), latestCreds); err != nil {
		t.Fatalf("status update: %v", err)
	}

	final := &vjailbreakv1alpha1.OpenstackCreds{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: name, Namespace: ns}, final); err != nil {
		t.Fatalf("final get: %v", err)
	}
	if final.Status.OpenStackValidationStatus != desiredValidationStatus {
		t.Errorf("OpenStackValidationStatus = %q, want %q (status was clobbered by Update() reset)",
			final.Status.OpenStackValidationStatus, desiredValidationStatus)
	}
	if _, exists := final.Annotations["pcd-sync-in-progress"]; exists {
		t.Errorf("pcd-sync-in-progress annotation should have been deleted")
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
