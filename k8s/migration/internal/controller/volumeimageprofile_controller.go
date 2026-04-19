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

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/pkg/common/constants"
)

// defaultProfilesSeedMarker is the name of the ConfigMap used as a one-shot
// marker indicating that the default VolumeImageProfiles have been seeded.
// Its presence (in the migration-system namespace) short-circuits the seeder
// so that user edits or deletions of the defaults are never reversed.
const defaultProfilesSeedMarker = "vjailbreak-default-profiles-seeded"

// VolumeImageProfileSeeder seeds the default VolumeImageProfile resources
// exactly once in the cluster's lifetime. After the initial seed the marker
// ConfigMap is written and every subsequent startup is a no-op.
type VolumeImageProfileSeeder struct {
	client.Client
	Scheme *runtime.Scheme
	ctxlog logr.Logger
}

// defaultProfiles is the out-of-box profile set applied to fresh installs.
var defaultProfiles = []vjailbreakv1alpha1.VolumeImageProfile{
	{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vjailbreak-default-windows",
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
			Description: "Default profile for Windows VMs",
		},
	},
	{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vjailbreak-default-linux",
		},
		Spec: vjailbreakv1alpha1.VolumeImageProfileSpec{
			OSFamily: "linux",
			Properties: map[string]string{
				"hw_qemu_guest_agent": "yes",
			},
			Description: "Default profile for Linux VMs",
		},
	},
}

// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=volumeimageprofiles,verbs=get;list;watch;create
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;create

// SetupWithManager registers the seeder as a manager Runnable so that it
// executes once after the cache is synced. No watch or reconcile loop is
// installed — the seeder never fires again for the lifetime of the process.
func (s *VolumeImageProfileSeeder) SetupWithManager(mgr ctrl.Manager) error {
	s.Client = mgr.GetClient()
	s.Scheme = mgr.GetScheme()
	s.ctxlog = log.Log.WithName("VolumeImageProfileSeeder")
	return mgr.Add(manager.RunnableFunc(s.Run))
}

// Run seeds the default profiles if and only if the marker ConfigMap is
// absent. The marker is created after a successful seed, so user deletions
// and modifications of the default profiles survive every subsequent restart.
func (s *VolumeImageProfileSeeder) Run(ctx context.Context) error {
	marker := &corev1.ConfigMap{}
	markerKey := client.ObjectKey{
		Namespace: constants.NamespaceMigrationSystem,
		Name:      defaultProfilesSeedMarker,
	}

	err := s.Get(ctx, markerKey, marker)
	switch {
	case err == nil:
		s.ctxlog.Info("Default VolumeImageProfiles already seeded; skipping",
			"marker", defaultProfilesSeedMarker)
		return nil
	case !apierrors.IsNotFound(err):
		return errors.Wrap(err, "failed to check default-profile seed marker")
	}

	for i := range defaultProfiles {
		profile := defaultProfiles[i]
		profile.Namespace = constants.NamespaceMigrationSystem
		if err := s.Create(ctx, &profile); err != nil && !apierrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "failed to create default profile %s", profile.Name)
		}
		s.ctxlog.Info("Seeded default VolumeImageProfile", "profile", profile.Name)
	}

	marker = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: constants.NamespaceMigrationSystem,
			Name:      defaultProfilesSeedMarker,
			Annotations: map[string]string{
				"vjailbreak.k8s.pf9.io/description": "One-shot marker: default VolumeImageProfiles have been seeded. Deleting this ConfigMap will cause defaults to be re-seeded on next controller startup.",
			},
		},
	}
	if err := s.Create(ctx, marker); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "failed to create default-profile seed marker")
	}
	s.ctxlog.Info("Wrote default-profile seed marker", "marker", defaultProfilesSeedMarker)
	return nil
}
