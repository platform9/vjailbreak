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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var openstackcredslog = logf.Log.WithName("openstackcreds-resource")

// SetupWebhookWithManager registers the webhook for OpenstackCreds in the manager.
func (r *OpenstackCreds) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithValidator(&OpenstackCredsCustomValidator{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/validate-vjailbreak-k8s-pf9-io-v1alpha1-openstackcreds,mutating=false,failurePolicy=fail,sideEffects=None,groups=vjailbreak.k8s.pf9.io,resources=openstackcreds,verbs=create;update,versions=v1alpha1,name=vopenstackcreds.kb.io,admissionReviewVersions=v1

// OpenstackCredsCustomValidator validates OpenstackCreds.
// +kubebuilder:object:generate=false
type OpenstackCredsCustomValidator struct{}

// ValidateCreate validates OpenstackCreds on creation.
func (v *OpenstackCredsCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	creds, ok := obj.(*OpenstackCreds)
	if !ok {
		return nil, fmt.Errorf("expected OpenstackCreds but got %T", obj)
	}
	openstackcredslog.Info("validate create", "name", creds.Name)
	return nil, nil
}

// ValidateUpdate validates OpenstackCreds on update.
func (v *OpenstackCredsCustomValidator) ValidateUpdate(_ context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	creds, ok := newObj.(*OpenstackCreds)
	if !ok {
		return nil, fmt.Errorf("expected OpenstackCreds but got %T", newObj)
	}
	openstackcredslog.Info("validate update", "name", creds.Name)
	return nil, nil
}

// ValidateDelete validates OpenstackCreds on deletion.
func (v *OpenstackCredsCustomValidator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	creds, ok := obj.(*OpenstackCreds)
	if !ok {
		return nil, fmt.Errorf("expected OpenstackCreds but got %T", obj)
	}
	openstackcredslog.Info("validate delete", "name", creds.Name)
	return nil, nil
}
