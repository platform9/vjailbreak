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
	"fmt"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var vmwarecredslog = logf.Log.WithName("vmwarecreds-resource")

// SetupWebhookWithManager registers the webhook for VMwareCreds in the manager.
func (r *VMwareCreds) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithValidator(&VMwareCredsCustomValidator{Client: mgr.GetClient()}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/validate-vjailbreak-k8s-pf9-io-v1alpha1-vmwarecreds,mutating=false,failurePolicy=fail,sideEffects=None,groups=vjailbreak.k8s.pf9.io,resources=vmwarecreds,verbs=create;update;delete,versions=v1alpha1,name=vvmwarecreds.kb.io,admissionReviewVersions=v1

// VMwareCredsCustomValidator validates VMwareCreds.
// +kubebuilder:object:generate=false
type VMwareCredsCustomValidator struct {
	// Client is used to look up Migrations that reference the VMwareCreds
	// being deleted. Set by SetupWebhookWithManager; a zero-value validator
	// (e.g. in unit tests) must not call ValidateDelete's Migration lookup.
	Client client.Client
}

// ValidateCreate validates VMwareCreds on creation.
func (v *VMwareCredsCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	creds, ok := obj.(*VMwareCreds)
	if !ok {
		return nil, fmt.Errorf("expected VMwareCreds but got %T", obj)
	}
	vmwarecredslog.Info("validate create", "name", creds.Name)
	return nil, nil
}

// ValidateUpdate validates VMwareCreds on update.
func (v *VMwareCredsCustomValidator) ValidateUpdate(_ context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	creds, ok := newObj.(*VMwareCreds)
	if !ok {
		return nil, fmt.Errorf("expected VMwareCreds but got %T", newObj)
	}
	vmwarecredslog.Info("validate update", "name", creds.Name)
	return nil, nil
}

// ValidateDelete validates VMwareCreds on deletion. It rejects the deletion
// if any Migration that traces back to these credentials (via
// MigrationPlan -> MigrationTemplate.Spec.Source.VMwareRef) has not yet
// reached a terminal phase (Succeeded/Failed). Without this check,
// deleting VMwareCreds mid-migration deletes the VMwareMachine CRs the
// running migration depends on, leaving it stuck in the UI forever even
// though the v2v-helper pod completes the conversion.
func (v *VMwareCredsCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	creds, ok := obj.(*VMwareCreds)
	if !ok {
		return nil, fmt.Errorf("expected VMwareCreds but got %T", obj)
	}
	vmwarecredslog.Info("validate delete", "name", creds.Name)

	blocking, err := v.findInProgressMigration(ctx, creds.Namespace, creds.Name)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check for in-progress migrations")
	}
	if blocking != nil {
		return nil, fmt.Errorf(
			"cannot delete VMwareCreds %q: migration %q is still in progress (phase %s)",
			creds.Name, blocking.Name, blocking.Status.Phase)
	}
	return nil, nil
}

// findInProgressMigration returns the first non-terminal Migration whose
// MigrationTemplate sources VMs from the VMwareCreds named credsName, or
// nil if none exists. It resolves the reference in the forward direction
// (VMwareCreds -> MigrationTemplate -> MigrationPlan -> Migration) with one
// List per kind, rather than walking each Migration's chain individually,
// so cost stays proportional to the number of templates/plans/migrations
// rather than to Migrations x chain-depth.
func (v *VMwareCredsCustomValidator) findInProgressMigration(ctx context.Context, namespace, credsName string) (*Migration, error) {
	ns := client.InNamespace(namespace)

	var templates MigrationTemplateList
	if err := v.Client.List(ctx, &templates, ns); err != nil {
		return nil, errors.Wrap(err, "failed to list migration templates")
	}
	templateNames := make(map[string]bool)
	for i := range templates.Items {
		if templates.Items[i].Spec.Source.VMwareRef == credsName {
			templateNames[templates.Items[i].Name] = true
		}
	}
	if len(templateNames) == 0 {
		return nil, nil
	}

	var plans MigrationPlanList
	if err := v.Client.List(ctx, &plans, ns); err != nil {
		return nil, errors.Wrap(err, "failed to list migration plans")
	}
	planNames := make(map[string]bool)
	for i := range plans.Items {
		if templateNames[plans.Items[i].Spec.MigrationTemplate] {
			planNames[plans.Items[i].Name] = true
		}
	}
	if len(planNames) == 0 {
		return nil, nil
	}

	var migrations MigrationList
	if err := v.Client.List(ctx, &migrations, ns); err != nil {
		return nil, errors.Wrap(err, "failed to list migrations")
	}
	for i := range migrations.Items {
		m := &migrations.Items[i]
		if !planNames[m.Spec.MigrationPlan] {
			continue
		}
		if m.Status.Phase != VMMigrationPhaseSucceeded && m.Status.Phase != VMMigrationPhaseFailed {
			return m, nil
		}
	}
	return nil, nil
}
