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
	stderrors "errors"
	"reflect"
	"testing"

	pkgerrors "github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/utils"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	commonutils "github.com/platform9/vjailbreak/pkg/common/utils"
)

// retryFixtures holds the resources required to exercise the edit-and-retry paths of the
// MigrationPlan reconciler against a fake client.
type retryFixtures struct {
	scheme       *runtime.Scheme
	plan         *vjailbreakv1alpha1.MigrationPlan
	template     *vjailbreakv1alpha1.MigrationTemplate
	vmwcreds     *vjailbreakv1alpha1.VMwareCreds
	oscreds      *vjailbreakv1alpha1.OpenstackCreds
	netmap       *vjailbreakv1alpha1.NetworkMapping
	stormap      *vjailbreakv1alpha1.StorageMapping
	vmMachine    *vjailbreakv1alpha1.VMwareMachine
	settingsCM   *corev1.ConfigMap
	newMigration *vjailbreakv1alpha1.Migration
	vmKey        string
	vmK8sName    string
}

const (
	retryTestNamespace = constants.NamespaceMigrationSystem
	retryTestCredName  = "test-vmware-creds"
	oldMigrationUID    = types.UID("old-migration-uid")
	newMigrationUID    = types.UID("new-migration-uid")
)

func newRetryFixtures(t *testing.T) *retryFixtures {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := vjailbreakv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add vjailbreak scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add corev1 scheme: %v", err)
	}
	if err := batchv1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add batchv1 scheme: %v", err)
	}

	vmKey := commonutils.GetVMUniqueKey("test-vm", "1001")
	vmK8sName, err := commonutils.GetK8sCompatibleVMWareObjectName(vmKey, retryTestCredName)
	if err != nil {
		t.Fatalf("failed to compute k8s compatible VM name: %v", err)
	}

	vmwcreds := &vjailbreakv1alpha1.VMwareCreds{
		ObjectMeta: metav1.ObjectMeta{Name: retryTestCredName, Namespace: retryTestNamespace},
	}
	oscreds := &vjailbreakv1alpha1.OpenstackCreds{
		ObjectMeta: metav1.ObjectMeta{Name: "test-openstack-creds", Namespace: retryTestNamespace},
	}

	template := &vjailbreakv1alpha1.MigrationTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "test-template", Namespace: retryTestNamespace},
		Spec: vjailbreakv1alpha1.MigrationTemplateSpec{
			NetworkMapping: "test-netmap",
			StorageMapping: "test-stormap",
			Source: vjailbreakv1alpha1.MigrationTemplateSource{
				VMwareRef: retryTestCredName,
			},
			Destination: vjailbreakv1alpha1.MigrationTemplateDestination{
				OpenstackRef: oscreds.Name,
			},
		},
	}

	plan := &vjailbreakv1alpha1.MigrationPlan{
		ObjectMeta: metav1.ObjectMeta{Name: "test-plan", Namespace: retryTestNamespace},
		Spec: vjailbreakv1alpha1.MigrationPlanSpec{
			MigrationPlanSpecPerVM: vjailbreakv1alpha1.MigrationPlanSpecPerVM{
				MigrationTemplate: template.Name,
				MigrationStrategy: vjailbreakv1alpha1.MigrationPlanStrategy{
					Type: "hot",
				},
				FirstBootScript: "echo edited",
			},
			VirtualMachines: [][]string{{vmKey}},
			SecurityGroups:  []string{"sg-new"},
		},
	}

	netmap := &vjailbreakv1alpha1.NetworkMapping{
		ObjectMeta: metav1.ObjectMeta{Name: "test-netmap", Namespace: retryTestNamespace},
		Spec: vjailbreakv1alpha1.NetworkMappingSpec{
			Networks: []vjailbreakv1alpha1.Network{{Source: "VM Network", Target: "ext-net"}},
		},
		Status: vjailbreakv1alpha1.NetworkMappingStatus{
			NetworkmappingValidationStatus: string(corev1.PodSucceeded),
		},
	}
	stormap := &vjailbreakv1alpha1.StorageMapping{
		ObjectMeta: metav1.ObjectMeta{Name: "test-stormap", Namespace: retryTestNamespace},
		Spec: vjailbreakv1alpha1.StorageMappingSpec{
			Storages: []vjailbreakv1alpha1.Storage{{Source: "ds1", Target: "fast-type"}},
		},
		Status: vjailbreakv1alpha1.StorageMappingStatus{
			StoragemappingValidationStatus: string(corev1.PodSucceeded),
		},
	}

	vmMachine := &vjailbreakv1alpha1.VMwareMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:        vmK8sName,
			Namespace:   retryTestNamespace,
			Labels:      map[string]string{constants.VMwareCredsLabel: retryTestCredName},
			Annotations: map[string]string{constants.VMwareDatacenterLabel: "dc1"},
		},
		Spec: vjailbreakv1alpha1.VMwareMachineSpec{
			// TargetFlavorID short-circuits flavor lookup so no OpenStack call is made.
			TargetFlavorID: "flavor-123",
			VMInfo: vjailbreakv1alpha1.VMInfo{
				Name:     "test-vm",
				VMID:     "1001",
				OSFamily: "linuxGuest",
				CPU:      2,
				Memory:   4096,
				Networks: []string{"VM Network"},
				Disks:    []vjailbreakv1alpha1.Disk{{Name: "Hard disk 1", Datastore: "ds1"}},
			},
		},
	}

	// Settings ConfigMap with nil data resolves to defaults.
	settingsCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.VjailbreakSettingsConfigMapName,
			Namespace: constants.NamespaceMigrationSystem,
		},
	}

	newMigration := &vjailbreakv1alpha1.Migration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.MigrationNameFromVMName(vmK8sName),
			Namespace: retryTestNamespace,
			UID:       newMigrationUID,
		},
		Spec: vjailbreakv1alpha1.MigrationSpec{
			MigrationPlan: plan.Name,
			VMName:        "test-vm",
			MigrationType: "hot",
		},
	}

	return &retryFixtures{
		scheme:       scheme,
		plan:         plan,
		template:     template,
		vmwcreds:     vmwcreds,
		oscreds:      oscreds,
		netmap:       netmap,
		stormap:      stormap,
		vmMachine:    vmMachine,
		settingsCM:   settingsCM,
		newMigration: newMigration,
		vmKey:        vmKey,
		vmK8sName:    vmK8sName,
	}
}

func (f *retryFixtures) baseObjects() []client.Object {
	return []client.Object{
		f.plan, f.template, f.vmwcreds, f.oscreds, f.netmap, f.stormap, f.vmMachine, f.settingsCM, f.newMigration,
	}
}

func (f *retryFixtures) newReconciler(extra ...client.Object) *MigrationPlanReconciler {
	objs := append(f.baseObjects(), extra...)
	fakeClient := fake.NewClientBuilder().
		WithScheme(f.scheme).
		WithObjects(objs...).
		WithStatusSubresource(
			&vjailbreakv1alpha1.Migration{},
			&vjailbreakv1alpha1.NetworkMapping{},
			&vjailbreakv1alpha1.StorageMapping{},
			&vjailbreakv1alpha1.MigrationPlan{},
		).
		Build()
	return &MigrationPlanReconciler{Client: fakeClient, Scheme: f.scheme}
}

// staleOwnerRef returns a controller reference to a Migration that no longer exists
// (deleted as part of edit and retry).
func staleOwnerRef(name string) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         "vjailbreak.k8s.pf9.io/v1alpha1",
		Kind:               "Migration",
		Name:               name,
		UID:                oldMigrationUID,
		Controller:         ptr.To(true),
		BlockOwnerDeletion: ptr.To(true),
	}
}

func controllerOwnerUID(t *testing.T, obj metav1.Object) types.UID {
	t.Helper()
	ref := metav1.GetControllerOf(obj)
	if ref == nil {
		t.Fatalf("object %s has no controller owner reference", obj.GetName())
	}
	return ref.UID
}

func TestCreateMigrationConfigMap_RebuildsStaleConfigMapAfterRetry(t *testing.T) {
	f := newRetryFixtures(t)
	configMapName := utils.GetMigrationConfigMapName(f.vmK8sName)

	staleCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            configMapName,
			Namespace:       retryTestNamespace,
			OwnerReferences: []metav1.OwnerReference{staleOwnerRef(f.newMigration.Name)},
		},
		Data: map[string]string{
			"TYPE":             "cold",
			"SECURITY_GROUPS":  "sg-old",
			"TARGET_FLAVOR_ID": "old-flavor",
		},
	}

	r := f.newReconciler(staleCM)
	ctx := context.Background()

	got, err := r.CreateMigrationConfigMap(ctx, f.plan, f.template, f.newMigration, f.oscreds, f.vmwcreds, f.vmKey, f.vmMachine, nil, nil)
	if err != nil {
		t.Fatalf("CreateMigrationConfigMap() returned error: %v", err)
	}

	stored := &corev1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: retryTestNamespace}, stored); err != nil {
		t.Fatalf("failed to fetch rebuilt ConfigMap: %v", err)
	}

	wantData := map[string]string{
		"TYPE":                  "hot",
		"SECURITY_GROUPS":       "sg-new",
		"TARGET_FLAVOR_ID":      "flavor-123",
		"NEUTRON_NETWORK_NAMES": "ext-net",
		"CINDER_VOLUME_TYPES":   "fast-type",
		"SOURCE_VM_NAME":        "test-vm",
		"OS_FAMILY":             "linuxGuest",
	}
	for key, want := range wantData {
		if stored.Data[key] != want {
			t.Errorf("rebuilt ConfigMap data[%q] = %q, want %q", key, stored.Data[key], want)
		}
	}

	if uid := controllerOwnerUID(t, stored); uid != newMigrationUID {
		t.Errorf("rebuilt ConfigMap controller owner UID = %q, want %q", uid, newMigrationUID)
	}
	for _, ref := range stored.GetOwnerReferences() {
		if ref.UID == oldMigrationUID {
			t.Errorf("rebuilt ConfigMap still references deleted Migration UID %q", oldMigrationUID)
		}
	}
	if got.Data["TYPE"] != "hot" {
		t.Errorf("returned ConfigMap data[TYPE] = %q, want %q", got.Data["TYPE"], "hot")
	}
}

func TestCreateMigrationConfigMap_SameOwnerPreservesExistingData(t *testing.T) {
	f := newRetryFixtures(t)
	f.newMigration.Spec.NetworkOverrides = `[{"network":"new-net"}]`
	configMapName := utils.GetMigrationConfigMapName(f.vmK8sName)

	ownedRef := staleOwnerRef(f.newMigration.Name)
	ownedRef.UID = newMigrationUID

	existingCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            configMapName,
			Namespace:       retryTestNamespace,
			OwnerReferences: []metav1.OwnerReference{ownedRef},
		},
		Data: map[string]string{
			"TYPE":              "cold",
			"NETWORK_OVERRIDES": "old-overrides",
		},
	}

	r := f.newReconciler(existingCM)
	ctx := context.Background()

	if _, err := r.CreateMigrationConfigMap(ctx, f.plan, f.template, f.newMigration, f.oscreds, f.vmwcreds, f.vmKey, f.vmMachine, nil, nil); err != nil {
		t.Fatalf("CreateMigrationConfigMap() returned error: %v", err)
	}

	stored := &corev1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: retryTestNamespace}, stored); err != nil {
		t.Fatalf("failed to fetch ConfigMap: %v", err)
	}

	// A ConfigMap owned by the current Migration belongs to an in-flight run: only the
	// migration-specific fields may change, the rest must stay untouched.
	if stored.Data["TYPE"] != "cold" {
		t.Errorf("ConfigMap data[TYPE] = %q, want %q (in-flight config must not be rebuilt)", stored.Data["TYPE"], "cold")
	}
	if stored.Data["NETWORK_OVERRIDES"] != f.newMigration.Spec.NetworkOverrides {
		t.Errorf("ConfigMap data[NETWORK_OVERRIDES] = %q, want %q", stored.Data["NETWORK_OVERRIDES"], f.newMigration.Spec.NetworkOverrides)
	}
}

func TestCreateFirstbootConfigMap_StaleOwnership(t *testing.T) {
	tests := []struct {
		name       string
		ownerUID   types.UID
		wantScript string
		wantOwner  types.UID
	}{
		{
			name:       "stale owner rebuilds script and transfers ownership",
			ownerUID:   oldMigrationUID,
			wantScript: "echo edited",
			wantOwner:  newMigrationUID,
		},
		{
			name:       "current owner leaves config map untouched",
			ownerUID:   newMigrationUID,
			wantScript: "echo original",
			wantOwner:  newMigrationUID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newRetryFixtures(t)
			configMapName := utils.GetFirstbootConfigMapName(f.vmK8sName)

			ref := staleOwnerRef(f.newMigration.Name)
			ref.UID = tt.ownerUID
			existingCM := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:            configMapName,
					Namespace:       retryTestNamespace,
					OwnerReferences: []metav1.OwnerReference{ref},
				},
				Data: map[string]string{"user_firstboot.sh": "echo original"},
			}

			r := f.newReconciler(existingCM)
			ctx := context.Background()

			if _, err := r.CreateFirstbootConfigMap(ctx, f.plan, f.newMigration, f.vmKey); err != nil {
				t.Fatalf("CreateFirstbootConfigMap() returned error: %v", err)
			}

			stored := &corev1.ConfigMap{}
			if err := r.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: retryTestNamespace}, stored); err != nil {
				t.Fatalf("failed to fetch firstboot ConfigMap: %v", err)
			}
			if stored.Data["user_firstboot.sh"] != tt.wantScript {
				t.Errorf("firstboot script = %q, want %q", stored.Data["user_firstboot.sh"], tt.wantScript)
			}
			if uid := controllerOwnerUID(t, stored); uid != tt.wantOwner {
				t.Errorf("firstboot ConfigMap controller owner UID = %q, want %q", uid, tt.wantOwner)
			}
		})
	}
}

func TestCreateJob_StaleJobDeletedAndRequeued(t *testing.T) {
	f := newRetryFixtures(t)
	jobName, err := utils.GetJobNameForVMName(f.vmKey, retryTestCredName)
	if err != nil {
		t.Fatalf("failed to compute job name: %v", err)
	}

	staleJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:            jobName,
			Namespace:       retryTestNamespace,
			OwnerReferences: []metav1.OwnerReference{staleOwnerRef(f.newMigration.Name)},
		},
	}

	r := f.newReconciler(staleJob)
	ctx := context.Background()

	err = r.CreateJob(ctx, f.plan, f.template, f.newMigration, f.vmKey, "fb-cm", "vmware-secret", "openstack-secret", f.vmMachine, "")
	if !stderrors.Is(err, errStaleJobCleanedUp) {
		t.Fatalf("CreateJob() error = %v, want errStaleJobCleanedUp", err)
	}

	// ReconcileMigrationPlanJob detects the sentinel through the TriggerMigration wrap.
	wrapped := pkgerrors.Wrapf(err, "failed to trigger migration")
	if !stderrors.Is(wrapped, errStaleJobCleanedUp) {
		t.Errorf("sentinel error not detectable through errors.Wrapf chain")
	}

	storedJob := &batchv1.Job{}
	getErr := r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: retryTestNamespace}, storedJob)
	if !apierrors.IsNotFound(getErr) {
		t.Errorf("stale job still exists after CreateJob, get error = %v, want NotFound", getErr)
	}
}

func TestCreateJob_OwnedJobUntouched(t *testing.T) {
	f := newRetryFixtures(t)
	jobName, err := utils.GetJobNameForVMName(f.vmKey, retryTestCredName)
	if err != nil {
		t.Fatalf("failed to compute job name: %v", err)
	}

	ref := staleOwnerRef(f.newMigration.Name)
	ref.UID = newMigrationUID
	ownedJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:            jobName,
			Namespace:       retryTestNamespace,
			OwnerReferences: []metav1.OwnerReference{ref},
		},
	}

	r := f.newReconciler(ownedJob)
	ctx := context.Background()

	if err := r.CreateJob(ctx, f.plan, f.template, f.newMigration, f.vmKey, "fb-cm", "vmware-secret", "openstack-secret", f.vmMachine, ""); err != nil {
		t.Fatalf("CreateJob() returned error for job owned by current migration: %v", err)
	}

	storedJob := &batchv1.Job{}
	if err := r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: retryTestNamespace}, storedJob); err != nil {
		t.Errorf("job owned by current migration was removed: %v", err)
	}
}

func TestAdoptResource(t *testing.T) {
	f := newRetryFixtures(t)
	r := f.newReconciler()

	nonControllerRef := metav1.OwnerReference{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Name:       "unrelated",
		UID:        types.UID("unrelated-uid"),
	}

	tests := []struct {
		name         string
		existingRefs []metav1.OwnerReference
		wantRefCount int
	}{
		{
			name:         "no existing references",
			existingRefs: nil,
			wantRefCount: 1,
		},
		{
			name:         "replaces stale controller reference",
			existingRefs: []metav1.OwnerReference{staleOwnerRef("old-migration")},
			wantRefCount: 1,
		},
		{
			name:         "preserves non-controller references",
			existingRefs: []metav1.OwnerReference{nonControllerRef, staleOwnerRef("old-migration")},
			wantRefCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "adopt-test",
					Namespace:       retryTestNamespace,
					OwnerReferences: tt.existingRefs,
				},
			}

			if err := r.adoptResource(f.newMigration, cm); err != nil {
				t.Fatalf("adoptResource() returned error: %v", err)
			}

			if got := len(cm.GetOwnerReferences()); got != tt.wantRefCount {
				t.Fatalf("owner reference count = %d, want %d (refs: %+v)", got, tt.wantRefCount, cm.GetOwnerReferences())
			}
			if uid := controllerOwnerUID(t, cm); uid != newMigrationUID {
				t.Errorf("controller owner UID = %q, want %q", uid, newMigrationUID)
			}
			for _, ref := range cm.GetOwnerReferences() {
				if ref.UID == oldMigrationUID {
					t.Errorf("stale controller reference survived adoption: %+v", ref)
				}
			}
		})
	}
}

// TestCreateMigrationConfigMap_RetryWithoutEditMatchesFreshCreate proves "retry without
// editing" semantics: when the plan is unchanged, the data written by the stale-ConfigMap
// rebuild path is identical to what a fresh create (post-GC retry, today's behavior)
// would produce.
func TestCreateMigrationConfigMap_RetryWithoutEditMatchesFreshCreate(t *testing.T) {
	ctx := context.Background()
	configMapKey := func(f *retryFixtures) types.NamespacedName {
		return types.NamespacedName{Name: utils.GetMigrationConfigMapName(f.vmK8sName), Namespace: retryTestNamespace}
	}

	// Fresh create: no pre-existing ConfigMap (GC already removed it).
	fFresh := newRetryFixtures(t)
	rFresh := fFresh.newReconciler()
	if _, err := rFresh.CreateMigrationConfigMap(ctx, fFresh.plan, fFresh.template, fFresh.newMigration, fFresh.oscreds, fFresh.vmwcreds, fFresh.vmKey, fFresh.vmMachine, nil, nil); err != nil {
		t.Fatalf("fresh CreateMigrationConfigMap() returned error: %v", err)
	}
	freshCM := &corev1.ConfigMap{}
	if err := rFresh.Get(ctx, configMapKey(fFresh), freshCM); err != nil {
		t.Fatalf("failed to fetch fresh ConfigMap: %v", err)
	}

	// Rebuild: ConfigMap from the previous run still present (GC has not caught up).
	fRebuild := newRetryFixtures(t)
	leftoverCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            utils.GetMigrationConfigMapName(fRebuild.vmK8sName),
			Namespace:       retryTestNamespace,
			OwnerReferences: []metav1.OwnerReference{staleOwnerRef(fRebuild.newMigration.Name)},
		},
		Data: map[string]string{"TYPE": "cold"},
	}
	rRebuild := fRebuild.newReconciler(leftoverCM)
	if _, err := rRebuild.CreateMigrationConfigMap(ctx, fRebuild.plan, fRebuild.template, fRebuild.newMigration, fRebuild.oscreds, fRebuild.vmwcreds, fRebuild.vmKey, fRebuild.vmMachine, nil, nil); err != nil {
		t.Fatalf("rebuild CreateMigrationConfigMap() returned error: %v", err)
	}
	rebuiltCM := &corev1.ConfigMap{}
	if err := rRebuild.Get(ctx, configMapKey(fRebuild), rebuiltCM); err != nil {
		t.Fatalf("failed to fetch rebuilt ConfigMap: %v", err)
	}

	if !reflect.DeepEqual(freshCM.Data, rebuiltCM.Data) {
		t.Errorf("retry-without-edit rebuild differs from fresh create:\nfresh:   %v\nrebuilt: %v", freshCM.Data, rebuiltCM.Data)
	}
	if uid := controllerOwnerUID(t, rebuiltCM); uid != newMigrationUID {
		t.Errorf("rebuilt ConfigMap controller owner UID = %q, want %q", uid, newMigrationUID)
	}
}

func TestReconcileMigrationPlanJob_RetryAnnotationResetsFailedPlan(t *testing.T) {
	tests := []struct {
		name             string
		annotations      map[string]string
		migrationMessage string
		wantRequeue      bool
		wantStatus       corev1.PodPhase
	}{
		{
			name:             "retry annotation resets validation-failed plan",
			annotations:      map[string]string{constants.MigrationPlanRetryRequestedAnnotation: "true"},
			migrationMessage: constants.MigrationPlanValidationFailedPrefix + ": VM failed validation",
			wantRequeue:      true,
			wantStatus:       "",
		},
		{
			name:             "validation-failed plan without annotation stays blocked",
			annotations:      nil,
			migrationMessage: constants.MigrationPlanValidationFailedPrefix + ": VM failed validation",
			wantRequeue:      false,
			wantStatus:       corev1.PodFailed,
		},
		{
			name:             "plain failure with deleted migration still resets without annotation",
			annotations:      nil,
			migrationMessage: "Migration failed for VM test-vm",
			wantRequeue:      true,
			wantStatus:       "",
		},
		{
			// Retry-of-retry: the first retry created a clone plan. If the clone plan's
			// Migration also fails, the user retries again via the clone plan (which is now
			// a 1-VM plan). The retry-requested annotation must reset the clone plan just
			// like any other plan; the clone audit annotations must not interfere.
			name: "retry annotation resets failed clone plan (retry-of-retry)",
			annotations: map[string]string{
				constants.MigrationPlanRetryRequestedAnnotation: "true",
				constants.MigrationPlanRetryCloneOfAnnotation:   "original-plan",
				constants.MigrationPlanRetryCloneVMAnnotation:   "some-vm-1001",
			},
			migrationMessage: "Migration for VM 'some-vm' failed: disk conversion error",
			wantRequeue:      true,
			wantStatus:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newRetryFixtures(t)
			f.plan.Annotations = tt.annotations
			f.plan.Status = vjailbreakv1alpha1.MigrationPlanStatus{
				MigrationStatus:  corev1.PodFailed,
				MigrationMessage: tt.migrationMessage,
			}
			// The failed Migration object has been deleted by the UI retry action;
			// only the plan remains. Remove the Migration from the fixture set.
			objs := []client.Object{f.plan, f.template, f.vmwcreds, f.oscreds, f.netmap, f.stormap, f.vmMachine, f.settingsCM}
			fakeClient := fake.NewClientBuilder().
				WithScheme(f.scheme).
				WithObjects(objs...).
				WithStatusSubresource(&vjailbreakv1alpha1.MigrationPlan{}, &vjailbreakv1alpha1.Migration{}).
				Build()
			r := &MigrationPlanReconciler{Client: fakeClient, Scheme: f.scheme}
			ctx := context.Background()

			result, err := r.ReconcileMigrationPlanJob(ctx, f.plan, nil)
			if err != nil {
				t.Fatalf("ReconcileMigrationPlanJob() returned error: %v", err)
			}
			if result.Requeue != tt.wantRequeue {
				t.Errorf("ReconcileMigrationPlanJob() requeue = %v, want %v", result.Requeue, tt.wantRequeue)
			}

			latest := &vjailbreakv1alpha1.MigrationPlan{}
			if err := r.Get(ctx, types.NamespacedName{Name: f.plan.Name, Namespace: retryTestNamespace}, latest); err != nil {
				t.Fatalf("failed to fetch plan: %v", err)
			}
			if latest.Status.MigrationStatus != tt.wantStatus {
				t.Errorf("plan status = %q, want %q", latest.Status.MigrationStatus, tt.wantStatus)
			}
			if _, ok := latest.Annotations[constants.MigrationPlanRetryRequestedAnnotation]; ok {
				t.Errorf("retry-requested annotation still present after reconcile")
			}
		})
	}
}

// TestCreateMigration_ClonePlanMigrationRefIsClonePlan verifies that when CreateMigration
// is called with a clone plan, the resulting Migration labels and spec reference the clone
// plan — not any original plan — so the plan's Migration listing is correctly isolated.
func TestCreateMigration_ClonePlanMigrationRefIsClonePlan(t *testing.T) {
	f := newRetryFixtures(t)
	ctx := context.Background()

	// Clone plan: same template/creds as original but different name + audit annotations.
	clonePlan := f.plan.DeepCopy()
	clonePlan.Name = "test-plan-r-xyz789"
	clonePlan.ResourceVersion = ""
	clonePlan.Annotations = map[string]string{
		constants.MigrationPlanRetryCloneOfAnnotation: f.plan.Name,
		constants.MigrationPlanRetryCloneVMAnnotation: f.vmKey,
	}

	// Reconciler has the clone plan only — no pre-existing Migration (it was deleted before the clone).
	objs := []client.Object{clonePlan, f.template, f.vmwcreds, f.oscreds, f.netmap, f.stormap, f.vmMachine, f.settingsCM}
	fakeClient := fake.NewClientBuilder().
		WithScheme(f.scheme).
		WithObjects(objs...).
		WithStatusSubresource(&vjailbreakv1alpha1.Migration{}, &vjailbreakv1alpha1.MigrationPlan{}).
		Build()
	r := &MigrationPlanReconciler{Client: fakeClient, Scheme: f.scheme}

	migration, err := r.CreateMigration(ctx, clonePlan, f.vmKey, f.vmMachine)
	if err != nil {
		t.Fatalf("CreateMigration() error: %v", err)
	}

	// Migration must reference the clone plan, not the original plan.
	if got := migration.Labels["migrationplan"]; got != clonePlan.Name {
		t.Errorf("Migration migrationplan label = %q, want %q", got, clonePlan.Name)
	}
	if got := migration.Spec.MigrationPlan; got != clonePlan.Name {
		t.Errorf("Migration.Spec.MigrationPlan = %q, want %q", got, clonePlan.Name)
	}

	// Listing by original plan name must return no Migrations.
	originalPlanMigrations := &vjailbreakv1alpha1.MigrationList{}
	if err := r.List(ctx, originalPlanMigrations,
		client.InNamespace(retryTestNamespace),
		client.MatchingLabels{"migrationplan": f.plan.Name},
	); err != nil {
		t.Fatalf("failed to list Migrations for original plan: %v", err)
	}
	if n := len(originalPlanMigrations.Items); n != 0 {
		t.Errorf("original plan has %d Migration(s), want 0 — clone plan must own the Migration", n)
	}
}

// TestReconcileMigrationPlanJob_ClonePlanAnnotationsPreservedNotCleared verifies that
// only the transient retry-requested annotation is removed by the controller; the
// permanent clone audit annotations (retry-clone-of, retry-clone-vm) must survive the
// reset so they remain available for auditing and future cleanup.
func TestReconcileMigrationPlanJob_ClonePlanAnnotationsPreservedNotCleared(t *testing.T) {
	f := newRetryFixtures(t)
	ctx := context.Background()

	f.plan.Annotations = map[string]string{
		constants.MigrationPlanRetryRequestedAnnotation: "true",
		constants.MigrationPlanRetryCloneOfAnnotation:   "original-plan",
		constants.MigrationPlanRetryCloneVMAnnotation:   f.vmKey,
	}
	f.plan.Status = vjailbreakv1alpha1.MigrationPlanStatus{
		MigrationStatus:  corev1.PodFailed,
		MigrationMessage: "Migration for VM 'test-vm' failed: network error",
	}

	// No Migration in the store (deleted by UI before annotation was set).
	objs := []client.Object{f.plan, f.template, f.vmwcreds, f.oscreds, f.netmap, f.stormap, f.vmMachine, f.settingsCM}
	fakeClient := fake.NewClientBuilder().
		WithScheme(f.scheme).
		WithObjects(objs...).
		WithStatusSubresource(&vjailbreakv1alpha1.MigrationPlan{}, &vjailbreakv1alpha1.Migration{}).
		Build()
	r := &MigrationPlanReconciler{Client: fakeClient, Scheme: f.scheme}

	result, err := r.ReconcileMigrationPlanJob(ctx, f.plan, nil)
	if err != nil {
		t.Fatalf("ReconcileMigrationPlanJob() returned error: %v", err)
	}
	if !result.Requeue {
		t.Errorf("requeue = false, want true — clone plan retry must reset and requeue")
	}

	latest := &vjailbreakv1alpha1.MigrationPlan{}
	if err := r.Get(ctx, types.NamespacedName{Name: f.plan.Name, Namespace: retryTestNamespace}, latest); err != nil {
		t.Fatalf("failed to fetch plan: %v", err)
	}

	// Transient annotation must be cleared.
	if _, ok := latest.Annotations[constants.MigrationPlanRetryRequestedAnnotation]; ok {
		t.Errorf("retry-requested annotation still present — must be cleared after reset")
	}
	// Permanent audit annotations must survive.
	if v := latest.Annotations[constants.MigrationPlanRetryCloneOfAnnotation]; v != "original-plan" {
		t.Errorf("retry-clone-of annotation = %q, want %q — audit annotation must be preserved", v, "original-plan")
	}
	if v := latest.Annotations[constants.MigrationPlanRetryCloneVMAnnotation]; v != f.vmKey {
		t.Errorf("retry-clone-vm annotation = %q, want %q — audit annotation must be preserved", v, f.vmKey)
	}
}

// TestReconcileMigrationPlanJob_PlanResetsAfterClonePlanRemovesValidationFailedVM tests
// the edge case where a multi-VM plan is in ValidationFailed state, the user retries the
// failed VM via the clone-plan path (removes it from the original plan and deletes its
// Migration), and remaining VMs have already succeeded. The plan must be allowed to reset
// so it can transition to Succeeded — not remain stuck in ValidationFailed forever.
func TestReconcileMigrationPlanJob_PlanResetsAfterClonePlanRemovesValidationFailedVM(t *testing.T) {
	f := newRetryFixtures(t)
	ctx := context.Background()

	// State after clone-plan approach:
	//   - vmA (the failed VM) was removed from plan.spec.virtualMachines and its Migration deleted.
	//   - vmB (f.vmKey) is the only remaining VM, and its Migration already Succeeded.
	//   - Plan is still in Failed state with ValidationFailed message (stale from vmA's failure).
	f.plan.Status = vjailbreakv1alpha1.MigrationPlanStatus{
		MigrationStatus:  corev1.PodFailed,
		MigrationMessage: constants.MigrationPlanValidationFailedPrefix + ": unsupported OS type",
	}

	// vmB's Migration: Succeeded, labeled so it is found when listing by plan name.
	vmBMigration := &vjailbreakv1alpha1.Migration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      f.newMigration.Name,
			Namespace: retryTestNamespace,
			Labels: map[string]string{
				"migrationplan":               f.plan.Name,
				constants.MigrationVMKeyLabel: commonutils.SanitizeLabelValue(f.vmKey),
			},
			Annotations: map[string]string{
				constants.OriginalVMNameAnnotation: f.vmKey,
			},
		},
		Spec: vjailbreakv1alpha1.MigrationSpec{
			MigrationPlan: f.plan.Name,
			VMName:        "test-vm",
		},
		Status: vjailbreakv1alpha1.MigrationStatus{
			Phase: vjailbreakv1alpha1.VMMigrationPhaseSucceeded,
		},
	}

	objs := []client.Object{f.plan, f.template, f.vmwcreds, f.oscreds, f.netmap, f.stormap, f.vmMachine, f.settingsCM, vmBMigration}
	fakeClient := fake.NewClientBuilder().
		WithScheme(f.scheme).
		WithObjects(objs...).
		WithStatusSubresource(&vjailbreakv1alpha1.MigrationPlan{}, &vjailbreakv1alpha1.Migration{}).
		Build()
	r := &MigrationPlanReconciler{Client: fakeClient, Scheme: f.scheme}

	result, err := r.ReconcileMigrationPlanJob(ctx, f.plan, nil)
	if err != nil {
		t.Fatalf("ReconcileMigrationPlanJob() returned error: %v", err)
	}

	// The plan must reset (requeue=true), not stay blocked in ValidationFailed.
	// retryTriggeredByDeletion=false (vmB has a Migration), so the new condition
	// "retryTriggeredByDeletion && ValidationFailed" is false → reset is allowed.
	if !result.Requeue {
		t.Errorf("requeue = false — plan stuck in ValidationFailed after the failed VM was removed via clone-plan retry; want requeue=true to allow remaining VMs to complete")
	}

	latest := &vjailbreakv1alpha1.MigrationPlan{}
	if err := r.Get(ctx, types.NamespacedName{Name: f.plan.Name, Namespace: retryTestNamespace}, latest); err != nil {
		t.Fatalf("failed to fetch plan: %v", err)
	}
	if latest.Status.MigrationStatus != "" {
		t.Errorf("plan status = %q, want \"\" — plan must reset so remaining VMs can drive it to Succeeded", latest.Status.MigrationStatus)
	}
}
