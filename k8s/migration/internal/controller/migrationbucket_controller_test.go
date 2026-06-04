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

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newBucketReconciler(objs ...client.Object) (*MigrationBucketReconciler, *runtime.Scheme, client.Client, error) {
	scheme := runtime.NewScheme()
	if err := vjailbreakv1alpha1.AddToScheme(scheme); err != nil {
		return nil, nil, nil, err
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&vjailbreakv1alpha1.MigrationBucket{}).
		Build()
	return &MigrationBucketReconciler{Client: c, Scheme: scheme}, scheme, c, nil
}

func bucket(name string, isDefault bool, vms ...string) *vjailbreakv1alpha1.MigrationBucket {
	return &vjailbreakv1alpha1.MigrationBucket{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "migration-system"},
		Spec: vjailbreakv1alpha1.MigrationBucketSpec{
			VMwareCredsRef: corev1.LocalObjectReference{Name: "vmware-creds"},
			VMs:            vms,
			IsDefault:      isDefault,
		},
	}
}

func reconcileBucket(t *testing.T, r *MigrationBucketReconciler, name string) {
	t.Helper()
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: name, Namespace: "migration-system"},
	})
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}
}

func TestMigrationBucket_DefaultsPhase(t *testing.T) {
	r, _, c, err := newBucketReconciler(bucket("b1", false, "vm-a"))
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	reconcileBucket(t, r, "b1")

	got := &vjailbreakv1alpha1.MigrationBucket{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: "b1", Namespace: "migration-system"}, got); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status.Phase != vjailbreakv1alpha1.MigrationBucketPhaseNotMigrated {
		t.Errorf("expected phase NotMigrated, got %q", got.Status.Phase)
	}
	if got.Status.Message != "" {
		t.Errorf("expected empty message for a valid bucket, got %q", got.Status.Message)
	}
}

func TestMigrationBucket_EmptyBucketReportsViolation(t *testing.T) {
	r, _, c, err := newBucketReconciler(bucket("empty", false))
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	reconcileBucket(t, r, "empty")

	got := &vjailbreakv1alpha1.MigrationBucket{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: "empty", Namespace: "migration-system"}, got); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status.Message == "" {
		t.Error("expected a non-empty status message for an empty bucket (no-empty-bucket invariant)")
	}
}

func TestMigrationBucket_NotFoundIsNoError(t *testing.T) {
	r, _, _, err := newBucketReconciler()
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "missing", Namespace: "migration-system"},
	}); err != nil {
		t.Errorf("expected no error for a missing bucket, got %v", err)
	}
}
