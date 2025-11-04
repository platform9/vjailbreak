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

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = ginkgo.Describe("StorageArrayMapping Controller", func() {
	ctx := context.Background()

	ginkgo.Context("When reconciling a valid StorageArrayMapping", func() {
		const resourceName = "test-valid-mapping"
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		ginkgo.BeforeEach(func() {
			ginkgo.By("creating a valid StorageArrayMapping resource")
			err := k8sClient.Get(ctx, typeNamespacedName, &vjailbreakv1alpha1.StorageArrayMapping{})
			if err != nil && errors.IsNotFound(err) {
				resource := &vjailbreakv1alpha1.StorageArrayMapping{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: vjailbreakv1alpha1.StorageArrayMappingSpec{
						Arrays: []vjailbreakv1alpha1.StorageArray{
							{
								Name:               "pure-array-01",
								Type:               "pure",
								ManagementEndpoint: "10.0.0.100",
								CredentialsSecret:  "pure-creds",
								ISCSI: &vjailbreakv1alpha1.ISCSIConfig{
									Targets: []string{"10.0.0.101", "10.0.0.102"},
									Port:    3260,
								},
							},
						},
						Datastores: []vjailbreakv1alpha1.DatastoreMapping{
							{
								Name:      "datastore1",
								ArrayName: "pure-array-01",
								ArrayType: "pure",
							},
						},
					},
				}
				gomega.Expect(k8sClient.Create(ctx, resource)).To(gomega.Succeed())
			}
		})

		ginkgo.AfterEach(func() {
			resource := &vjailbreakv1alpha1.StorageArrayMapping{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(k8sClient.Delete(ctx, resource)).To(gomega.Succeed())
		})

		ginkgo.It("should successfully reconcile and mark as Valid", func() {
			ginkgo.By("Reconciling the created resource")
			controllerReconciler := &StorageArrayMappingReconciler{
				BaseReconciler: BaseReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				},
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			ginkgo.By("Checking that status is Valid")
			resource := &vjailbreakv1alpha1.StorageArrayMapping{}
			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(resource.Status.ValidationStatus).To(gomega.Equal("Valid"))
		})
	})

	ginkgo.Context("When datastore references undefined array", func() {
		const resourceName = "test-undefined-array"
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		ginkgo.BeforeEach(func() {
			ginkgo.By("creating StorageArrayMapping with undefined array reference")
			err := k8sClient.Get(ctx, typeNamespacedName, &vjailbreakv1alpha1.StorageArrayMapping{})
			if err != nil && errors.IsNotFound(err) {
				resource := &vjailbreakv1alpha1.StorageArrayMapping{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: vjailbreakv1alpha1.StorageArrayMappingSpec{
						Arrays: []vjailbreakv1alpha1.StorageArray{
							{
								Name:               "pure-array-01",
								Type:               "pure",
								ManagementEndpoint: "10.0.0.100",
								CredentialsSecret:  "pure-creds",
							},
						},
						Datastores: []vjailbreakv1alpha1.DatastoreMapping{
							{
								Name:      "datastore1",
								ArrayName: "nonexistent-array",
								ArrayType: "pure",
							},
						},
					},
				}
				gomega.Expect(k8sClient.Create(ctx, resource)).To(gomega.Succeed())
			}
		})

		ginkgo.AfterEach(func() {
			resource := &vjailbreakv1alpha1.StorageArrayMapping{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(k8sClient.Delete(ctx, resource)).To(gomega.Succeed())
		})

		ginkgo.It("should mark as Invalid with appropriate error", func() {
			controllerReconciler := &StorageArrayMappingReconciler{
				BaseReconciler: BaseReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				},
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			resource := &vjailbreakv1alpha1.StorageArrayMapping{}
			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(resource.Status.ValidationStatus).To(gomega.Equal("Invalid"))
			gomega.Expect(resource.Status.ValidationMessage).To(gomega.ContainSubstring("references undefined array"))
		})
	})

	ginkgo.Context("When array type is mismatched", func() {
		const resourceName = "test-type-mismatch"
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		ginkgo.BeforeEach(func() {
			ginkgo.By("creating StorageArrayMapping with type mismatch")
			err := k8sClient.Get(ctx, typeNamespacedName, &vjailbreakv1alpha1.StorageArrayMapping{})
			if err != nil && errors.IsNotFound(err) {
				resource := &vjailbreakv1alpha1.StorageArrayMapping{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: vjailbreakv1alpha1.StorageArrayMappingSpec{
						Arrays: []vjailbreakv1alpha1.StorageArray{
							{
								Name:               "pure-array-01",
								Type:               "pure",
								ManagementEndpoint: "10.0.0.100",
								CredentialsSecret:  "pure-creds",
							},
						},
						Datastores: []vjailbreakv1alpha1.DatastoreMapping{
							{
								Name:      "datastore1",
								ArrayName: "pure-array-01",
								ArrayType: "netapp", // Mismatch!
							},
						},
					},
				}
				gomega.Expect(k8sClient.Create(ctx, resource)).To(gomega.Succeed())
			}
		})

		ginkgo.AfterEach(func() {
			resource := &vjailbreakv1alpha1.StorageArrayMapping{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(k8sClient.Delete(ctx, resource)).To(gomega.Succeed())
		})

		ginkgo.It("should mark as Invalid with type mismatch error", func() {
			controllerReconciler := &StorageArrayMappingReconciler{
				BaseReconciler: BaseReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				},
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			resource := &vjailbreakv1alpha1.StorageArrayMapping{}
			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(resource.Status.ValidationStatus).To(gomega.Equal("Invalid"))
			gomega.Expect(resource.Status.ValidationMessage).To(gomega.ContainSubstring("does not match"))
		})
	})

	ginkgo.Context("When array has missing credentials", func() {
		const resourceName = "test-missing-creds"
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		ginkgo.BeforeEach(func() {
			ginkgo.By("creating StorageArrayMapping with missing credentials")
			err := k8sClient.Get(ctx, typeNamespacedName, &vjailbreakv1alpha1.StorageArrayMapping{})
			if err != nil && errors.IsNotFound(err) {
				resource := &vjailbreakv1alpha1.StorageArrayMapping{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: vjailbreakv1alpha1.StorageArrayMappingSpec{
						Arrays: []vjailbreakv1alpha1.StorageArray{
							{
								Name:               "pure-array-01",
								Type:               "pure",
								ManagementEndpoint: "10.0.0.100",
								CredentialsSecret:  "", // Missing!
							},
						},
						Datastores: []vjailbreakv1alpha1.DatastoreMapping{
							{
								Name:      "datastore1",
								ArrayName: "pure-array-01",
								ArrayType: "pure",
							},
						},
					},
				}
				gomega.Expect(k8sClient.Create(ctx, resource)).To(gomega.Succeed())
			}
		})

		ginkgo.AfterEach(func() {
			resource := &vjailbreakv1alpha1.StorageArrayMapping{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(k8sClient.Delete(ctx, resource)).To(gomega.Succeed())
		})

		ginkgo.It("should mark as Invalid with credentials error", func() {
			controllerReconciler := &StorageArrayMappingReconciler{
				BaseReconciler: BaseReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				},
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			resource := &vjailbreakv1alpha1.StorageArrayMapping{}
			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(resource.Status.ValidationStatus).To(gomega.Equal("Invalid"))
			gomega.Expect(resource.Status.ValidationMessage).To(gomega.ContainSubstring("credentials secret cannot be empty"))
		})
	})

	ginkgo.Context("When there are duplicate datastores", func() {
		const resourceName = "test-duplicate-datastores"
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		ginkgo.BeforeEach(func() {
			ginkgo.By("creating StorageArrayMapping with duplicate datastores")
			err := k8sClient.Get(ctx, typeNamespacedName, &vjailbreakv1alpha1.StorageArrayMapping{})
			if err != nil && errors.IsNotFound(err) {
				resource := &vjailbreakv1alpha1.StorageArrayMapping{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: vjailbreakv1alpha1.StorageArrayMappingSpec{
						Arrays: []vjailbreakv1alpha1.StorageArray{
							{
								Name:               "pure-array-01",
								Type:               "pure",
								ManagementEndpoint: "10.0.0.100",
								CredentialsSecret:  "pure-creds",
							},
						},
						Datastores: []vjailbreakv1alpha1.DatastoreMapping{
							{
								Name:      "datastore1",
								ArrayName: "pure-array-01",
								ArrayType: "pure",
							},
							{
								Name:      "datastore1", // Duplicate!
								ArrayName: "pure-array-01",
								ArrayType: "pure",
							},
						},
					},
				}
				gomega.Expect(k8sClient.Create(ctx, resource)).To(gomega.Succeed())
			}
		})

		ginkgo.AfterEach(func() {
			resource := &vjailbreakv1alpha1.StorageArrayMapping{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(k8sClient.Delete(ctx, resource)).To(gomega.Succeed())
		})

		ginkgo.It("should mark as Invalid with duplicate datastore error", func() {
			controllerReconciler := &StorageArrayMappingReconciler{
				BaseReconciler: BaseReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				},
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			resource := &vjailbreakv1alpha1.StorageArrayMapping{}
			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(resource.Status.ValidationStatus).To(gomega.Equal("Invalid"))
			gomega.Expect(resource.Status.ValidationMessage).To(gomega.ContainSubstring("duplicate datastore"))
		})
	})

	ginkgo.Context("When using unsupported array type", func() {
		const resourceName = "test-unsupported-type"
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		ginkgo.BeforeEach(func() {
			ginkgo.By("creating StorageArrayMapping with unsupported array type")
			err := k8sClient.Get(ctx, typeNamespacedName, &vjailbreakv1alpha1.StorageArrayMapping{})
			if err != nil && errors.IsNotFound(err) {
				resource := &vjailbreakv1alpha1.StorageArrayMapping{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: vjailbreakv1alpha1.StorageArrayMappingSpec{
						Arrays: []vjailbreakv1alpha1.StorageArray{
							{
								Name:               "emc-array-01",
								Type:               "emc", // Unsupported!
								ManagementEndpoint: "10.0.0.100",
								CredentialsSecret:  "emc-creds",
							},
						},
						Datastores: []vjailbreakv1alpha1.DatastoreMapping{
							{
								Name:      "datastore1",
								ArrayName: "emc-array-01",
								ArrayType: "emc",
							},
						},
					},
				}
				gomega.Expect(k8sClient.Create(ctx, resource)).To(gomega.Succeed())
			}
		})

		ginkgo.AfterEach(func() {
			resource := &vjailbreakv1alpha1.StorageArrayMapping{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(k8sClient.Delete(ctx, resource)).To(gomega.Succeed())
		})

		ginkgo.It("should mark as Invalid with unsupported type error", func() {
			controllerReconciler := &StorageArrayMappingReconciler{
				BaseReconciler: BaseReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				},
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			resource := &vjailbreakv1alpha1.StorageArrayMapping{}
			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(resource.Status.ValidationStatus).To(gomega.Equal("Invalid"))
			gomega.Expect(resource.Status.ValidationMessage).To(gomega.ContainSubstring("unsupported array type"))
		})
	})
})
