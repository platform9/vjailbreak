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

package v1alpha1

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("VMwareCreds Webhook", func() {

	Context("When creating VMwareCreds under Validating Webhook", func() {
		It("Should deny if a required field is empty", func() {
		})

		It("Should admit if all required fields are provided", func() {
		})
	})

})

// TestVMwareCredsCustomValidator tests the type-assertion logic in the validator.
// ValidateDelete needs a real Client (unlike the OpenstackCreds no-op validator)
// since it looks up in-progress Migrations, so this uses an empty fake client
// rather than the zero value.
func TestVMwareCredsCustomValidator(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v", err)
	}
	v := &VMwareCredsCustomValidator{Client: fake.NewClientBuilder().WithScheme(scheme).Build()}
	ctx := context.Background()
	validObj := &VMwareCreds{}
	wrongObj := &corev1.Pod{}

	tests := []struct {
		name    string
		op      string
		wantErr bool
	}{
		{"ValidateCreate correct type", "create", false},
		{"ValidateUpdate correct type", "update", false},
		{"ValidateDelete correct type", "delete", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			switch tt.op {
			case "create":
				_, err = v.ValidateCreate(ctx, validObj)
			case "update":
				_, err = v.ValidateUpdate(ctx, nil, validObj)
			case "delete":
				_, err = v.ValidateDelete(ctx, validObj)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("got err=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}

	wrongTests := []struct {
		name string
		op   string
	}{
		{"ValidateCreate wrong type", "create"},
		{"ValidateUpdate wrong type", "update"},
		{"ValidateDelete wrong type", "delete"},
	}
	for _, tt := range wrongTests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			switch tt.op {
			case "create":
				_, err = v.ValidateCreate(ctx, wrongObj)
			case "update":
				_, err = v.ValidateUpdate(ctx, nil, wrongObj)
			case "delete":
				_, err = v.ValidateDelete(ctx, wrongObj)
			}
			if err == nil {
				t.Errorf("expected error for wrong type, got nil")
			}
		})
	}
}

// TestVMwareCredsCustomValidator_ValidateDelete_InProgressMigration covers the
// regression: deleting VMwareCreds while a Migration sourced from it is
// still running must be rejected, so the reconciler never gets a chance to
// delete the VMwareMachine CRs that migration depends on.
func TestVMwareCredsCustomValidator_ValidateDelete_InProgressMigration(t *testing.T) {
	const namespace = "migration-system"

	newScheme := func() *runtime.Scheme {
		scheme := runtime.NewScheme()
		if err := AddToScheme(scheme); err != nil {
			t.Fatalf("AddToScheme() error = %v", err)
		}
		return scheme
	}

	newCreds := func(name string) *VMwareCreds {
		return &VMwareCreds{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		}
	}

	newTemplate := func(name, vmwareRef string) *MigrationTemplate {
		return &MigrationTemplate{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
			Spec: MigrationTemplateSpec{
				Source: MigrationTemplateSource{VMwareRef: vmwareRef},
			},
		}
	}

	newPlan := func(name, templateName string) *MigrationPlan {
		return &MigrationPlan{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
			Spec: MigrationPlanSpec{
				MigrationPlanSpecPerVM: MigrationPlanSpecPerVM{
					MigrationTemplate: templateName,
				},
			},
		}
	}

	newMigration := func(name, planName string, phase VMMigrationPhase) *Migration {
		return &Migration{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
			Spec:       MigrationSpec{MigrationPlan: planName},
			Status:     MigrationStatus{Phase: phase},
		}
	}

	tests := []struct {
		name       string
		objects    []runtime.Object
		wantDenied bool
	}{
		{
			name:       "no migrations at all",
			objects:    nil,
			wantDenied: false,
		},
		{
			name: "in-progress migration referencing these creds",
			objects: []runtime.Object{
				newTemplate("tmpl-a", "creds-a"),
				newPlan("plan-a", "tmpl-a"),
				newMigration("mig-a", "plan-a", VMMigrationPhaseConvertingDisk),
			},
			wantDenied: true,
		},
		{
			name: "succeeded migration referencing these creds",
			objects: []runtime.Object{
				newTemplate("tmpl-a", "creds-a"),
				newPlan("plan-a", "tmpl-a"),
				newMigration("mig-a", "plan-a", VMMigrationPhaseSucceeded),
			},
			wantDenied: false,
		},
		{
			name: "failed migration referencing these creds",
			objects: []runtime.Object{
				newTemplate("tmpl-a", "creds-a"),
				newPlan("plan-a", "tmpl-a"),
				newMigration("mig-a", "plan-a", VMMigrationPhaseFailed),
			},
			wantDenied: false,
		},
		{
			name: "in-progress migration referencing a different creds",
			objects: []runtime.Object{
				newTemplate("tmpl-b", "creds-b"),
				newPlan("plan-b", "tmpl-b"),
				newMigration("mig-b", "plan-b", VMMigrationPhaseConvertingDisk),
			},
			wantDenied: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().
				WithScheme(newScheme()).
				WithRuntimeObjects(tt.objects...).
				Build()

			v := &VMwareCredsCustomValidator{Client: fakeClient}
			_, err := v.ValidateDelete(context.Background(), newCreds("creds-a"))

			if tt.wantDenied && err == nil {
				t.Errorf("expected deletion to be denied, got nil error")
			}
			if !tt.wantDenied && err != nil {
				t.Errorf("expected deletion to be allowed, got error: %v", err)
			}
		})
	}
}
