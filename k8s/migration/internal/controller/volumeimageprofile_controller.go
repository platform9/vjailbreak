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
	"reflect"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/pkg/common/constants"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// VolumeImageProfileReconciler reconciles VolumeImageProfile objects.
// It seeds the default Windows and Linux profiles on startup if they don't exist.
type VolumeImageProfileReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	ctxlog logr.Logger
}

// Default profiles that are seeded at startup
var defaultProfiles = []vjailbreakv1alpha1.VolumeImageProfile{
	{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vjailbreak-default-windows",
			Annotations: map[string]string{
				"description": "Default profile for Windows VMs - enables QEMU guest agent and sets virtio drivers",
			},
		},
		Spec: vjailbreakv1alpha1.VolumeImageProfileSpec{
			OSFamily: "windows",
			Properties: map[string]string{
				"hw_disk_bus":         "virtio",
				"hw_vif_model":        "virtio",
				"os_type":             "windows",
				"hw_qemu_guest_agent": "yes",
				"hw_video_model":      "qxl",
			},
			Description: "Default Windows profile with virtio drivers and QEMU guest agent",
		},
	},
	{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vjailbreak-default-linux",
			Annotations: map[string]string{
				"description": "Default profile for Linux VMs - enables QEMU guest agent",
			},
		},
		Spec: vjailbreakv1alpha1.VolumeImageProfileSpec{
			OSFamily: "linux",
			Properties: map[string]string{
				"hw_qemu_guest_agent": "yes",
			},
			Description: "Default Linux profile with QEMU guest agent enabled",
		},
	},
}

// seedOnce ensures default profiles are seeded only once
var seedOnce sync.Once

// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=volumeimageprofiles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=volumeimageprofiles/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=volumeimageprofiles/finalizers,verbs=update

// Reconcile ensures the default VolumeImageProfile resources exist.
// For user-created profiles, it simply acknowledges their existence.
func (r *VolumeImageProfileReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.ctxlog = log.FromContext(ctx)

	profile := &vjailbreakv1alpha1.VolumeImageProfile{}
	if err := r.Get(ctx, req.NamespacedName, profile); err != nil {
		if apierrors.IsNotFound(err) {
			// If a default profile was deleted, recreate it
			for _, defaultProfile := range defaultProfiles {
				if req.Name == defaultProfile.Name && req.Namespace == constants.NamespaceMigrationSystem {
					r.ctxlog.Info("Recreating deleted default VolumeImageProfile", "profile", defaultProfile.Name)
					if err := r.createProfile(ctx, &defaultProfile); err != nil {
						return ctrl.Result{}, errors.Wrapf(err, "failed to recreate default profile %s", defaultProfile.Name)
					}
					return ctrl.Result{}, nil
				}
			}
			// Regular user profile was deleted, nothing to do
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, errors.Wrap(err, "failed to get VolumeImageProfile")
	}

	// For default profiles, ensure they are not modified (except by the controller itself)
	// We check the generation to avoid fighting with our own updates
	if profile.Generation > 1 {
		for _, defaultProfile := range defaultProfiles {
			if profile.Name == defaultProfile.Name {
				// Check if the spec has been modified from the default
				if profile.Spec.OSFamily != defaultProfile.Spec.OSFamily ||
					profile.Spec.Description != defaultProfile.Spec.Description ||
					!reflect.DeepEqual(profile.Spec.Properties, defaultProfile.Spec.Properties) {
					r.ctxlog.Info("Default profile was modified, restoring to default", "profile", profile.Name)
					profile.Spec = defaultProfile.Spec
					if err := r.Update(ctx, profile); err != nil {
						return ctrl.Result{}, errors.Wrapf(err, "failed to restore default profile %s", profile.Name)
					}
					return ctrl.Result{}, nil
				}
			}
		}
	}

	return ctrl.Result{}, nil
}

// SeedDefaultProfiles creates the default profiles on controller startup.
// It is called from SetupWithManager.
func (r *VolumeImageProfileReconciler) SeedDefaultProfiles(ctx context.Context) error {
	var seedErr error
	seedOnce.Do(func() {
		for _, profile := range defaultProfiles {
			profile.Namespace = constants.NamespaceMigrationSystem
			existing := &vjailbreakv1alpha1.VolumeImageProfile{}
			err := r.Get(ctx, client.ObjectKeyFromObject(&profile), existing)
			if err != nil {
				if apierrors.IsNotFound(err) {
					r.ctxlog.Info("Seeding default VolumeImageProfile", "profile", profile.Name)
					if createErr := r.createProfile(ctx, &profile); createErr != nil {
						seedErr = errors.Wrapf(createErr, "failed to create default profile %s", profile.Name)
						return
					}
				} else {
					seedErr = errors.Wrapf(err, "failed to check existence of default profile %s", profile.Name)
					return
				}
			} else {
				r.ctxlog.Info("Default VolumeImageProfile already exists", "profile", profile.Name)
			}
		}
	})
	return seedErr
}

func (r *VolumeImageProfileReconciler) createProfile(ctx context.Context, profile *vjailbreakv1alpha1.VolumeImageProfile) error {
	profile.Namespace = constants.NamespaceMigrationSystem
	if err := r.Create(ctx, profile); err != nil {
		return errors.Wrap(err, "failed to create profile")
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VolumeImageProfileReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Client = mgr.GetClient()
	r.Scheme = mgr.GetScheme()
	r.ctxlog = log.Log.WithName("VolumeImageProfile")

	// Seed default profiles on startup
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := r.SeedDefaultProfiles(ctx); err != nil {
		r.ctxlog.Error(err, "Failed to seed default VolumeImageProfiles on startup")
		// Continue even if seeding fails - the controller will try to recreate on reconcile
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&vjailbreakv1alpha1.VolumeImageProfile{}).
		Complete(r)
}
