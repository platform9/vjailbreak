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
	"testing"

	. "github.com/onsi/ginkgo/v2" //nolint:revive // dot imports are common in Ginkgo tests
	. "github.com/onsi/gomega"    //nolint:revive // dot imports are common in Gomega assertions
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/pkg/common/constants"
)

// esxiControllerTestScheme builds a minimal scheme for fake-client unit tests.
func esxiControllerTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := vjailbreakv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	return scheme
}

// TestResolveVMwareCreds_DirectRef: spec.vmwareCredsRef set → direct lookup, no RMP needed.
func TestResolveVMwareCreds_DirectRef(t *testing.T) {
	scheme := esxiControllerTestScheme(t)
	vmwareCreds := &vjailbreakv1alpha1.VMwareCreds{
		ObjectMeta: metav1.ObjectMeta{Name: "my-vmware-creds", Namespace: constants.NamespaceMigrationSystem},
	}
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(vmwareCreds).Build()

	esxiMig := &vjailbreakv1alpha1.ESXIMigration{
		Spec: vjailbreakv1alpha1.ESXIMigrationSpec{
			VMwareCredsRef: corev1.LocalObjectReference{Name: "my-vmware-creds"},
		},
	}
	creds, err := resolveVMwareCreds(context.Background(), k8sClient, esxiMig, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.Name != "my-vmware-creds" {
		t.Errorf("creds.Name = %q, want %q", creds.Name, "my-vmware-creds")
	}
}

// TestResolveVMwareCreds_NoRefNoRMP: no spec.vmwareCredsRef and no RMP → error.
func TestResolveVMwareCreds_NoRefNoRMP(t *testing.T) {
	scheme := esxiControllerTestScheme(t)
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	esxiMig := &vjailbreakv1alpha1.ESXIMigration{}
	_, err := resolveVMwareCreds(context.Background(), k8sClient, esxiMig, nil)
	if err == nil {
		t.Fatal("expected error when no vmwareCredsRef and no RMP")
	}
}

// TestResolveOpenstackCreds_DirectRef: spec.openstackCredsRef set → direct lookup, no RMP needed.
func TestResolveOpenstackCreds_DirectRef(t *testing.T) {
	scheme := esxiControllerTestScheme(t)
	osCreds := &vjailbreakv1alpha1.OpenstackCreds{
		ObjectMeta: metav1.ObjectMeta{Name: "my-os-creds", Namespace: constants.NamespaceMigrationSystem},
	}
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(osCreds).Build()

	esxiMig := &vjailbreakv1alpha1.ESXIMigration{
		Spec: vjailbreakv1alpha1.ESXIMigrationSpec{
			OpenstackCredsRef: corev1.LocalObjectReference{Name: "my-os-creds"},
		},
	}
	creds, err := resolveOpenstackCreds(context.Background(), k8sClient, esxiMig, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.Name != "my-os-creds" {
		t.Errorf("creds.Name = %q, want %q", creds.Name, "my-os-creds")
	}
}

// TestResolveOpenstackCreds_NoRefNoRMP: no spec.openstackCredsRef and no RMP → error.
func TestResolveOpenstackCreds_NoRefNoRMP(t *testing.T) {
	scheme := esxiControllerTestScheme(t)
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	esxiMig := &vjailbreakv1alpha1.ESXIMigration{}
	_, err := resolveOpenstackCreds(context.Background(), k8sClient, esxiMig, nil)
	if err == nil {
		t.Fatal("expected error when no openstackCredsRef and no RMP")
	}
}

var _ = Describe("resolveBMConfigName", func() {
	It("returns bmConfigRef.Name when set on ESXIMigration (new flow)", func() {
		esxiMig := &vjailbreakv1alpha1.ESXIMigration{
			Spec: vjailbreakv1alpha1.ESXIMigrationSpec{
				BMConfigRef: &corev1.LocalObjectReference{Name: "my-bmconfig"},
			},
		}
		name, err := resolveBMConfigName(esxiMig, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(name).To(Equal("my-bmconfig"))
	})

	It("falls back to RollingMigrationPlan bmConfigRef when ESXIMigration.BMConfigRef is nil (old flow)", func() {
		esxiMig := &vjailbreakv1alpha1.ESXIMigration{}
		rmp := &vjailbreakv1alpha1.RollingMigrationPlan{
			Spec: vjailbreakv1alpha1.RollingMigrationPlanSpec{
				BMConfigRef: corev1.LocalObjectReference{Name: "rmp-bmconfig"},
			},
		}
		name, err := resolveBMConfigName(esxiMig, rmp)
		Expect(err).NotTo(HaveOccurred())
		Expect(name).To(Equal("rmp-bmconfig"))
	})

	It("prefers ESXIMigration.BMConfigRef over RollingMigrationPlan bmConfigRef", func() {
		esxiMig := &vjailbreakv1alpha1.ESXIMigration{
			Spec: vjailbreakv1alpha1.ESXIMigrationSpec{
				BMConfigRef: &corev1.LocalObjectReference{Name: "direct-bmconfig"},
			},
		}
		rmp := &vjailbreakv1alpha1.RollingMigrationPlan{
			Spec: vjailbreakv1alpha1.RollingMigrationPlanSpec{
				BMConfigRef: corev1.LocalObjectReference{Name: "rmp-bmconfig"},
			},
		}
		name, err := resolveBMConfigName(esxiMig, rmp)
		Expect(err).NotTo(HaveOccurred())
		Expect(name).To(Equal("direct-bmconfig"))
	})

	It("returns error when both refs are absent", func() {
		esxiMig := &vjailbreakv1alpha1.ESXIMigration{}
		_, err := resolveBMConfigName(esxiMig, nil)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("no BMConfig reference"))
	})
})

var _ = Describe("ESXIMigration Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		esximigration := &vjailbreakv1alpha1.ESXIMigration{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind ESXIMigration")
			err := k8sClient.Get(ctx, typeNamespacedName, esximigration)
			if err != nil && errors.IsNotFound(err) {
				resource := &vjailbreakv1alpha1.ESXIMigration{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					// TODO(user): Specify other spec details if needed.
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &vjailbreakv1alpha1.ESXIMigration{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ESXIMigration")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &ESXIMigrationReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})
