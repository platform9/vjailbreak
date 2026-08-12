package upgrade

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// crdsManifest mirrors the shape of deploy/00crds.yaml: a vjailbreak CRD, the Namespace
// and a third-party CRD that must both be skipped, and the vjailbreak-ai-data PVC whose
// bound spec broke upgrades from v0.4.8 onward.
const crdsManifest = `
apiVersion: v1
kind: Namespace
metadata:
  name: migration-system
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: migrations.vjailbreak.k8s.pf9.io
spec:
  group: vjailbreak.k8s.pf9.io
  names:
    kind: Migration
    plural: migrations
  scope: Namespaced
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: certificates.cert-manager.io
spec:
  group: cert-manager.io
  names:
    kind: Certificate
    plural: certificates
  scope: Namespaced
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: vjailbreak-ai-data
  namespace: migration-system
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 2Gi
`

const deploymentManifest = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: vjailbreak-ai
spec:
  replicas: 1
  selector:
    matchLabels:
      app: vjailbreak-ai
`

type stubTransport struct {
	body   string
	status int
}

func (s stubTransport) RoundTrip(*http.Request) (*http.Response, error) {
	status := s.status
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Body:       io.NopCloser(strings.NewReader(s.body)),
		Header:     make(http.Header),
	}, nil
}

// serveManifest points the package HTTP client at a fixture for the duration of a test.
func serveManifest(t *testing.T, body string) {
	t.Helper()
	original := httpClient
	httpClient = &http.Client{Transport: stubTransport{body: body}}
	t.Cleanup(func() { httpClient = original })
}

type writeRecord struct {
	verb         string
	kind         string
	name         string
	patchType    types.PatchType
	fieldManager string
	force        bool
	// resourceVersion and managedFields as they arrived at the API server.
	resourceVersion  string
	hadManagedFields bool
}

// recordingClient captures every write verb. The fake client rejects apply patches
// outright (kubernetes/kubernetes#115598), so writes are intercepted rather than
// executed; what matters here is which verb and options the upgrade flow issues.
func recordingClient(t *testing.T, patchErr error) (client.Client, *[]writeRecord) {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to build scheme: %v", err)
	}
	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add apiextensions to scheme: %v", err)
	}

	records := &[]writeRecord{}

	record := func(verb string, obj client.Object) writeRecord {
		accessor, err := meta.Accessor(obj)
		if err != nil {
			t.Fatalf("failed to access object metadata: %v", err)
		}
		return writeRecord{
			verb:             verb,
			kind:             obj.GetObjectKind().GroupVersionKind().Kind,
			name:             accessor.GetName(),
			resourceVersion:  accessor.GetResourceVersion(),
			hadManagedFields: len(accessor.GetManagedFields()) > 0,
		}
	}

	base := fake.NewClientBuilder().WithScheme(scheme).Build()
	c := interceptor.NewClient(base, interceptor.Funcs{
		Patch: func(_ context.Context, _ client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			rec := record("patch", obj)
			rec.patchType = patch.Type()

			options := &client.PatchOptions{}
			for _, opt := range opts {
				opt.ApplyToPatch(options)
			}
			rec.fieldManager = options.FieldManager
			rec.force = options.Force != nil && *options.Force

			*records = append(*records, rec)
			return patchErr
		},
		Update: func(_ context.Context, _ client.WithWatch, obj client.Object, _ ...client.UpdateOption) error {
			*records = append(*records, record("update", obj))
			return nil
		},
		Create: func(_ context.Context, _ client.WithWatch, obj client.Object, _ ...client.CreateOption) error {
			*records = append(*records, record("create", obj))
			return nil
		},
		Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
			*records = append(*records, record("get", obj))
			return kerrors.NewNotFound(schema.GroupResource{}, "")
		},
	})

	return c, records
}

func findRecord(records []writeRecord, kind string) (writeRecord, bool) {
	for _, r := range records {
		if r.kind == kind {
			return r, true
		}
	}
	return writeRecord{}, false
}

func TestApplyAllCRDsUsesServerSideApply(t *testing.T) {
	tests := []struct {
		name        string
		kind        string
		wantApplied bool
		reason      string
	}{
		{
			name:        "vjailbreak CRD is applied",
			kind:        "CustomResourceDefinition",
			wantApplied: true,
		},
		{
			name:        "bound PVC is applied, never replaced",
			kind:        "PersistentVolumeClaim",
			wantApplied: true,
			reason:      "an Update would blank spec.volumeName on a bound claim and be rejected as immutable",
		},
		{
			name:        "Namespace is skipped",
			kind:        "Namespace",
			wantApplied: false,
		},
	}

	serveManifest(t, crdsManifest)
	kubeClient, records := recordingClient(t, nil)

	if err := ApplyAllCRDs(context.Background(), kubeClient, "v0.4.9"); err != nil {
		t.Fatalf("ApplyAllCRDs() error = %v, want nil", err)
	}

	for _, r := range *records {
		if r.verb != "patch" {
			t.Errorf("issued %s for %s %q; the upgrade flow must only server-side apply", r.verb, r.kind, r.name)
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, found := findRecord(*records, tt.kind)
			if found != tt.wantApplied {
				t.Fatalf("applied %s = %t, want %t (%s)", tt.kind, found, tt.wantApplied, tt.reason)
			}
			if !tt.wantApplied {
				return
			}
			if rec.patchType != types.ApplyPatchType {
				t.Errorf("patch type for %s = %q, want %q", tt.kind, rec.patchType, types.ApplyPatchType)
			}
			if rec.fieldManager != UpgradeFieldManager {
				t.Errorf("field manager for %s = %q, want %q", tt.kind, rec.fieldManager, UpgradeFieldManager)
			}
			if !rec.force {
				t.Errorf("force ownership for %s = false, want true so install-time kubectl fields can be taken over", tt.kind)
			}
		})
	}
}

func TestApplyAllCRDsSkipsThirdPartyCRDs(t *testing.T) {
	serveManifest(t, crdsManifest)
	kubeClient, records := recordingClient(t, nil)

	if err := ApplyAllCRDs(context.Background(), kubeClient, "v0.4.9"); err != nil {
		t.Fatalf("ApplyAllCRDs() error = %v, want nil", err)
	}

	for _, r := range *records {
		if r.name == "certificates.cert-manager.io" {
			t.Errorf("applied third-party CRD %q; only vjailbreak groups may be touched", r.name)
		}
	}
	if _, found := findRecord(*records, "CustomResourceDefinition"); !found {
		t.Error("no CRD applied; the vjailbreak CRD should still be applied")
	}
}

// Regression for #2279: an API server that rejects full-object writes on a bound PVC
// must not fail the upgrade, because the flow no longer issues them.
func TestApplyAllCRDsSurvivesImmutablePVCSpec(t *testing.T) {
	serveManifest(t, crdsManifest)

	immutableErr := kerrors.NewInvalid(
		schema.GroupKind{Kind: "PersistentVolumeClaim"},
		"vjailbreak-ai-data",
		field.ErrorList{
			field.Forbidden(field.NewPath("spec"), "spec is immutable after creation except resources.requests and volumeAttributesClassName for bound claims"),
		},
	)

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to build scheme: %v", err)
	}
	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add apiextensions to scheme: %v", err)
	}

	base := fake.NewClientBuilder().WithScheme(scheme).Build()
	kubeClient := interceptor.NewClient(base, interceptor.Funcs{
		// Mimic the real API server: apply merges and is accepted, anything that sends
		// the whole object is rejected for a bound claim.
		Patch: func(_ context.Context, _ client.WithWatch, obj client.Object, patch client.Patch, _ ...client.PatchOption) error {
			if patch.Type() == types.ApplyPatchType {
				return nil
			}
			if obj.GetObjectKind().GroupVersionKind().Kind == "PersistentVolumeClaim" {
				return immutableErr
			}
			return nil
		},
		Update: func(_ context.Context, _ client.WithWatch, obj client.Object, _ ...client.UpdateOption) error {
			if obj.GetObjectKind().GroupVersionKind().Kind == "PersistentVolumeClaim" {
				return immutableErr
			}
			return nil
		},
	})

	if err := ApplyAllCRDs(context.Background(), kubeClient, "v0.4.9"); err != nil {
		t.Fatalf("ApplyAllCRDs() error = %v, want nil (bound PVC must not fail the CRD phase)", err)
	}
}

func TestApplyManifestObjectStripsServerOwnedMetadata(t *testing.T) {
	kubeClient, records := recordingClient(t, nil)

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "PersistentVolumeClaim"})
	u.SetName("vjailbreak-ai-data")
	u.SetNamespace(Namespace)
	u.SetResourceVersion("12345")
	u.SetManagedFields([]metav1.ManagedFieldsEntry{{Manager: "kubectl-client-side-apply"}})

	if err := applyManifestObject(context.Background(), kubeClient, u); err != nil {
		t.Fatalf("applyManifestObject() error = %v, want nil", err)
	}

	if len(*records) != 1 {
		t.Fatalf("issued %d writes, want 1", len(*records))
	}
	rec := (*records)[0]
	if rec.resourceVersion != "" {
		t.Errorf("resourceVersion sent = %q, want empty so the apply is not an optimistic-concurrency check", rec.resourceVersion)
	}
	if rec.hadManagedFields {
		t.Error("managedFields were sent; they are server-owned and must be stripped")
	}
	if u.GetResourceVersion() != "12345" {
		t.Errorf("caller's object was mutated: resourceVersion = %q, want 12345", u.GetResourceVersion())
	}
}

func TestApplyManifestObjectRetriesOnConflict(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to build scheme: %v", err)
	}

	calls := 0
	base := fake.NewClientBuilder().WithScheme(scheme).Build()
	kubeClient := interceptor.NewClient(base, interceptor.Funcs{
		Patch: func(_ context.Context, _ client.WithWatch, _ client.Object, _ client.Patch, _ ...client.PatchOption) error {
			calls++
			if calls == 1 {
				return kerrors.NewConflict(schema.GroupResource{Resource: "persistentvolumeclaims"}, "vjailbreak-ai-data", nil)
			}
			return nil
		},
	})

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "PersistentVolumeClaim"})
	u.SetName("vjailbreak-ai-data")
	u.SetNamespace(Namespace)

	if err := applyManifestObject(context.Background(), kubeClient, u); err != nil {
		t.Fatalf("applyManifestObject() error = %v, want nil after retry", err)
	}
	if calls != 2 {
		t.Errorf("patch attempts = %d, want 2 (one conflict, one success)", calls)
	}
}

func TestApplyManifestFromGitHubUsesServerSideApply(t *testing.T) {
	serveManifest(t, deploymentManifest)
	kubeClient, records := recordingClient(t, nil)

	err := ApplyManifestFromGitHub(context.Background(), kubeClient, "v0.4.9", "deploy/08vjailbreak-ai-deployment.yaml")
	if err != nil {
		t.Fatalf("ApplyManifestFromGitHub() error = %v, want nil", err)
	}

	if len(*records) != 1 {
		t.Fatalf("issued %d writes, want 1", len(*records))
	}
	rec := (*records)[0]
	if rec.verb != "patch" || rec.patchType != types.ApplyPatchType {
		t.Errorf("issued %s/%s, want patch/%s", rec.verb, rec.patchType, types.ApplyPatchType)
	}
	if rec.fieldManager != UpgradeFieldManager {
		t.Errorf("field manager = %q, want %q", rec.fieldManager, UpgradeFieldManager)
	}
}

func TestApplyManifestFromGitHubDefaultsNamespace(t *testing.T) {
	serveManifest(t, deploymentManifest)

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to build scheme: %v", err)
	}

	var gotNamespace string
	base := fake.NewClientBuilder().WithScheme(scheme).Build()
	kubeClient := interceptor.NewClient(base, interceptor.Funcs{
		Patch: func(_ context.Context, _ client.WithWatch, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
			gotNamespace = obj.GetNamespace()
			return nil
		},
	})

	err := ApplyManifestFromGitHub(context.Background(), kubeClient, "v0.4.9", "deploy/08vjailbreak-ai-deployment.yaml")
	if err != nil {
		t.Fatalf("ApplyManifestFromGitHub() error = %v, want nil", err)
	}
	if gotNamespace != Namespace {
		t.Errorf("namespace = %q, want %q", gotNamespace, Namespace)
	}
}

// ---------------------------------------------------------------------------
// Shared fixtures for the non-apply half of the file.
// ---------------------------------------------------------------------------

// upgradeScheme carries the built-in types plus CRDs, matching what the upgrade
// executor registers at runtime.
func upgradeScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	s := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(s); err != nil {
		t.Fatalf("failed to build scheme: %v", err)
	}
	if err := apiextensionsv1.AddToScheme(s); err != nil {
		t.Fatalf("failed to add apiextensions to scheme: %v", err)
	}
	return s
}

// Returns WithWatch so the same helper can be wrapped by interceptor.NewClient.
func newFakeClient(t *testing.T, objs ...client.Object) client.WithWatch {
	t.Helper()
	return fake.NewClientBuilder().WithScheme(upgradeScheme(t)).WithObjects(objs...).Build()
}

func vjailbreakCRD(name, group, plural, kind string, versions ...string) *apiextensionsv1.CustomResourceDefinition {
	if len(versions) == 0 {
		versions = []string{"v1alpha1"}
	}
	crdVersions := make([]apiextensionsv1.CustomResourceDefinitionVersion, 0, len(versions))
	for _, v := range versions {
		crdVersions = append(crdVersions, apiextensionsv1.CustomResourceDefinitionVersion{Name: v, Served: true})
	}
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group:    group,
			Versions: crdVersions,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:     kind,
				Plural:   plural,
				Singular: strings.ToLower(kind),
			},
		},
		Status: apiextensionsv1.CustomResourceDefinitionStatus{
			Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
				{Type: apiextensionsv1.Established, Status: apiextensionsv1.ConditionTrue},
			},
		},
	}
}

func vjailbreakGVR(resource string) schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "vjailbreak.k8s.pf9.io", Version: "v1alpha1", Resource: resource}
}

// listKinds covers every resource the cleanup and validation paths LIST; the fake
// dynamic client requires each one to be registered up front.
func listKinds() map[schema.GroupVersionResource]string {
	return map[schema.GroupVersionResource]string{
		vjailbreakGVR("migrationplans"):        "MigrationPlanList",
		vjailbreakGVR("rollingmigrationplans"): "RollingMigrationPlanList",
		vjailbreakGVR("vjailbreaknodes"):       "VjailbreakNodeList",
		vjailbreakGVR("migrations"):            "MigrationList",
		vjailbreakGVR("widgets"):               "WidgetList",
		vjailbreakGVR("vmwaremachines"):        "VMwareMachineList",
	}
}

// failListOf makes one resource unlistable, standing in for a CRD whose API is broken
// or whose version was removed. The fake dynamic client panics on resources it was not
// told about, so the failure has to come from a reactor.
func failListOf(dyn *dynamicfake.FakeDynamicClient, resource string) {
	dyn.PrependReactor("list", resource, func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("list failed")
	})
}

func newCR(resourceKind, name string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "vjailbreak.k8s.pf9.io",
		Version: "v1alpha1",
		Kind:    resourceKind,
	})
	u.SetName(name)
	u.SetNamespace(Namespace)
	return u
}

func newFakeDynamic(t *testing.T, objs ...runtime.Object) *dynamicfake.FakeDynamicClient {
	t.Helper()
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds(), objs...)
}

// withFakeDynamic makes CleanupResources and deleteCRInstances use the supplied fake
// instead of dialling a real API server.
func withFakeDynamic(t *testing.T, dyn dynamic.Interface) {
	t.Helper()
	original := newDynamicClient
	newDynamicClient = func(*rest.Config) (dynamic.Interface, error) { return dyn, nil }
	t.Cleanup(func() { newDynamicClient = original })
}

func deployment(name string, replicas, ready, statusReplicas int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: Namespace},
		Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
		Status: appsv1.DeploymentStatus{
			Replicas:        statusReplicas,
			ReadyReplicas:   ready,
			UpdatedReplicas: ready,
		},
	}
}

func backupConfigMap(name, backupID, resource string, age time.Duration) *corev1.ConfigMap {
	labels := map[string]string{"vjailbreak-backup": "true"}
	if backupID != "" {
		labels["vjailbreak-backup-id"] = backupID
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         Namespace,
			Labels:            labels,
			CreationTimestamp: metav1.NewTime(time.Now().Add(-age)),
		},
		Data: map[string]string{"resource": resource},
	}
}

// ---------------------------------------------------------------------------
// DiscoverCurrentCRs
// ---------------------------------------------------------------------------

func TestDiscoverCurrentCRs(t *testing.T) {
	tests := []struct {
		name string
		crds []client.Object
		want []CRInfo
	}{
		{
			name: "only vjailbreak groups are returned",
			crds: []client.Object{
				vjailbreakCRD("migrations.vjailbreak.k8s.pf9.io", "vjailbreak.k8s.pf9.io", "migrations", "Migration"),
				vjailbreakCRD("certificates.cert-manager.io", "cert-manager.io", "certificates", "Certificate"),
			},
			want: []CRInfo{{
				Group: "vjailbreak.k8s.pf9.io", Version: "v1alpha1",
				Kind: "Migration", Plural: "migrations", Singular: "migration",
			}},
		},
		{
			name: "one entry per served version",
			crds: []client.Object{
				vjailbreakCRD("migrations.vjailbreak.k8s.pf9.io", "vjailbreak.k8s.pf9.io", "migrations", "Migration", "v1alpha1", "v1beta1"),
			},
			want: []CRInfo{
				{Group: "vjailbreak.k8s.pf9.io", Version: "v1alpha1", Kind: "Migration", Plural: "migrations", Singular: "migration"},
				{Group: "vjailbreak.k8s.pf9.io", Version: "v1beta1", Kind: "Migration", Plural: "migrations", Singular: "migration"},
			},
		},
		{
			name: "no CRDs yields no CRs",
			crds: nil,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DiscoverCurrentCRs(context.Background(), newFakeClient(t, tt.crds...))
			if err != nil {
				t.Fatalf("DiscoverCurrentCRs() error = %v, want nil", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DiscoverCurrentCRs() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestDiscoverCurrentCRsListError(t *testing.T) {
	kubeClient := interceptor.NewClient(newFakeClient(t), interceptor.Funcs{
		List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
			return errors.New("api server down")
		},
	})

	if _, err := DiscoverCurrentCRs(context.Background(), kubeClient); err == nil {
		t.Fatal("DiscoverCurrentCRs() error = nil, want the list failure surfaced")
	}
}

// ---------------------------------------------------------------------------
// RunPreUpgradeChecks / checkForAnyCustomResources
// ---------------------------------------------------------------------------

func TestRunPreUpgradeChecks(t *testing.T) {
	migrationCRD := vjailbreakCRD("migrations.vjailbreak.k8s.pf9.io", "vjailbreak.k8s.pf9.io", "migrations", "Migration")
	nodeCRD := vjailbreakCRD("vjailbreaknodes.vjailbreak.k8s.pf9.io", "vjailbreak.k8s.pf9.io", "vjailbreaknodes", "VjailbreakNode")

	secret := func(name string) client.Object {
		return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: Namespace}}
	}

	tests := []struct {
		name    string
		objects []client.Object
		crs     []runtime.Object
		want    ValidationResult
	}{
		{
			name:    "clean appliance passes everything",
			objects: []client.Object{migrationCRD, nodeCRD},
			want: ValidationResult{
				NoMigrationPlans: true, NoRollingMigrationPlans: true,
				VMwareCredsDeleted: true, OpenStackCredsDeleted: true,
				AgentsScaledDown: true, NoCustomResources: true, PassedAll: true,
			},
		},
		{
			name:    "a leftover MigrationPlan fails the check",
			objects: []client.Object{migrationCRD, nodeCRD},
			crs:     []runtime.Object{newCR("MigrationPlan", "plan-1")},
			want: ValidationResult{
				NoMigrationPlans: false, NoRollingMigrationPlans: true,
				VMwareCredsDeleted: true, OpenStackCredsDeleted: true,
				AgentsScaledDown: true, NoCustomResources: true, PassedAll: false,
			},
		},
		{
			name:    "a leftover RollingMigrationPlan fails the check",
			objects: []client.Object{migrationCRD, nodeCRD},
			crs:     []runtime.Object{newCR("RollingMigrationPlan", "rolling-1")},
			want: ValidationResult{
				NoMigrationPlans: true, NoRollingMigrationPlans: false,
				VMwareCredsDeleted: true, OpenStackCredsDeleted: true,
				AgentsScaledDown: true, NoCustomResources: true, PassedAll: false,
			},
		},
		{
			name:    "surviving vmware credentials fail the check",
			objects: []client.Object{migrationCRD, nodeCRD, secret("vmware-credentials")},
			want: ValidationResult{
				NoMigrationPlans: true, NoRollingMigrationPlans: true,
				VMwareCredsDeleted: false, OpenStackCredsDeleted: true,
				AgentsScaledDown: true, NoCustomResources: true, PassedAll: false,
			},
		},
		{
			name:    "surviving openstack credentials fail the check",
			objects: []client.Object{migrationCRD, nodeCRD, secret("openstack-credentials")},
			want: ValidationResult{
				NoMigrationPlans: true, NoRollingMigrationPlans: true,
				VMwareCredsDeleted: true, OpenStackCredsDeleted: false,
				AgentsScaledDown: true, NoCustomResources: true, PassedAll: false,
			},
		},
		{
			name:    "the master node alone still counts as scaled down",
			objects: []client.Object{migrationCRD, nodeCRD},
			crs:     []runtime.Object{newCR("VjailbreakNode", "vjailbreak-master")},
			want: ValidationResult{
				NoMigrationPlans: true, NoRollingMigrationPlans: true,
				VMwareCredsDeleted: true, OpenStackCredsDeleted: true,
				AgentsScaledDown: true, NoCustomResources: true, PassedAll: true,
			},
		},
		{
			name:    "an extra agent node is not scaled down",
			objects: []client.Object{migrationCRD, nodeCRD},
			crs: []runtime.Object{
				newCR("VjailbreakNode", "vjailbreak-master"),
				newCR("VjailbreakNode", "vjailbreak-worker-1"),
			},
			// The worker node is itself a surviving custom resource.
			want: ValidationResult{
				NoMigrationPlans: true, NoRollingMigrationPlans: true,
				VMwareCredsDeleted: true, OpenStackCredsDeleted: true,
				AgentsScaledDown: false, NoCustomResources: false, PassedAll: false,
			},
		},
		{
			name:    "a single non-master node is not scaled down",
			objects: []client.Object{migrationCRD, nodeCRD},
			crs:     []runtime.Object{newCR("VjailbreakNode", "vjailbreak-worker-1")},
			want: ValidationResult{
				NoMigrationPlans: true, NoRollingMigrationPlans: true,
				VMwareCredsDeleted: true, OpenStackCredsDeleted: true,
				AgentsScaledDown: false, NoCustomResources: false, PassedAll: false,
			},
		},
		{
			name:    "any surviving custom resource fails the check",
			objects: []client.Object{migrationCRD, nodeCRD},
			crs:     []runtime.Object{newCR("Migration", "migration-1")},
			want: ValidationResult{
				NoMigrationPlans: true, NoRollingMigrationPlans: true,
				VMwareCredsDeleted: true, OpenStackCredsDeleted: true,
				AgentsScaledDown: true, NoCustomResources: false, PassedAll: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RunPreUpgradeChecks(context.Background(),
				newFakeClient(t, tt.objects...), newFakeDynamic(t, tt.crs...), "v0.4.9")
			if err != nil {
				t.Fatalf("RunPreUpgradeChecks() error = %v, want nil", err)
			}
			if *got != tt.want {
				t.Errorf("RunPreUpgradeChecks() = %+v, want %+v", *got, tt.want)
			}
		})
	}
}

func TestRunPreUpgradeChecksSecretGetError(t *testing.T) {
	kubeClient := interceptor.NewClient(newFakeClient(t), interceptor.Funcs{
		Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
			if _, ok := obj.(*corev1.Secret); ok {
				return kerrors.NewInternalError(errors.New("etcd unavailable"))
			}
			return nil
		},
	})

	if _, err := RunPreUpgradeChecks(context.Background(), kubeClient, newFakeDynamic(t), "v0.4.9"); err == nil {
		t.Fatal("RunPreUpgradeChecks() error = nil; a broken API server must not read as 'creds deleted'")
	}
}

func TestCheckForAnyCustomResources(t *testing.T) {
	nodeCRD := vjailbreakCRD("vjailbreaknodes.vjailbreak.k8s.pf9.io", "vjailbreak.k8s.pf9.io", "vjailbreaknodes", "VjailbreakNode")
	credsCRD := vjailbreakCRD("vmwaremachines.vjailbreak.k8s.pf9.io", "vjailbreak.k8s.pf9.io", "vmwaremachines", "VMwareMachine")

	tests := []struct {
		name string
		crds []client.Object
		crs  []runtime.Object
		want bool
	}{
		{name: "nothing left", crds: []client.Object{credsCRD}, want: true},
		{
			name: "the master node is not counted as a leftover",
			crds: []client.Object{nodeCRD},
			crs:  []runtime.Object{newCR("VjailbreakNode", "vjailbreak-master")},
			want: true,
		},
		{
			name: "a leftover custom resource is counted",
			crds: []client.Object{credsCRD},
			crs:  []runtime.Object{newCR("VMwareMachine", "vm-1")},
			want: false,
		},
		{
			name: "a non-master node is counted",
			crds: []client.Object{nodeCRD},
			crs:  []runtime.Object{newCR("VjailbreakNode", "vjailbreak-worker-1")},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := checkForAnyCustomResources(context.Background(),
				newFakeClient(t, tt.crds...), newFakeDynamic(t, tt.crs...))
			if err != nil {
				t.Fatalf("checkForAnyCustomResources() error = %v, want nil", err)
			}
			if got != tt.want {
				t.Errorf("checkForAnyCustomResources() = %t, want %t", got, tt.want)
			}
		})
	}
}

// A CRD whose CRs cannot be listed must not be reported as "clean"; the loop logs and
// moves on, so the remaining resources still decide the answer.
func TestCheckForAnyCustomResourcesToleratesUnlistableCRD(t *testing.T) {
	unknownCRD := vjailbreakCRD("widgets.vjailbreak.k8s.pf9.io", "vjailbreak.k8s.pf9.io", "widgets", "Widget")
	credsCRD := vjailbreakCRD("vmwaremachines.vjailbreak.k8s.pf9.io", "vjailbreak.k8s.pf9.io", "vmwaremachines", "VMwareMachine")

	dyn := newFakeDynamic(t, newCR("VMwareMachine", "vm-1"))
	failListOf(dyn, "widgets")

	got, err := checkForAnyCustomResources(context.Background(),
		newFakeClient(t, unknownCRD, credsCRD), dyn)
	if err != nil {
		t.Fatalf("checkForAnyCustomResources() error = %v, want nil", err)
	}
	if got {
		t.Error("checkForAnyCustomResources() = true, want false: the listable CRD still has a resource")
	}
}

// ---------------------------------------------------------------------------
// Backup / restore
// ---------------------------------------------------------------------------

func TestBackupResourcesWithID(t *testing.T) {
	kubeClient := newFakeClient(t,
		vjailbreakCRD("migrations.vjailbreak.k8s.pf9.io", "vjailbreak.k8s.pf9.io", "migrations", "Migration"),
		vjailbreakCRD("certificates.cert-manager.io", "cert-manager.io", "certificates", "Certificate"),
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "version-config", Namespace: Namespace},
			Data: map[string]string{"version": "v0.4.8"}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "vjailbreak-settings", Namespace: Namespace}},
		deployment("migration-controller-manager", 1, 1, 1),
		deployment("migration-vpwned-sdk", 1, 1, 1),
		deployment("vjailbreak-ui", 1, 1, 1),
	)

	if err := BackupResourcesWithID(context.Background(), kubeClient, &rest.Config{}, "20260811T000000Z"); err != nil {
		t.Fatalf("BackupResourcesWithID() error = %v, want nil", err)
	}

	backups := &corev1.ConfigMapList{}
	if err := kubeClient.List(context.Background(), backups, client.InNamespace(Namespace),
		client.MatchingLabels{"vjailbreak-backup": "true", "vjailbreak-backup-id": "20260811T000000Z"}); err != nil {
		t.Fatalf("failed to list backups: %v", err)
	}

	got := map[string]bool{}
	for _, cm := range backups.Items {
		got[cm.Name] = true
		if cm.Data["resource"] == "" {
			t.Errorf("backup %s has no serialized resource", cm.Name)
		}
	}

	for _, want := range []string{
		"backup-crd-migrations-vjailbreak-k8s-pf9-io",
		"backup-cm-version-config",
		"backup-cm-vjailbreak-settings",
		"backup-deploy-migration-controller-manager",
		"backup-deploy-migration-vpwned-sdk",
		"backup-deploy-vjailbreak-ui",
	} {
		if !got[want] {
			t.Errorf("missing backup ConfigMap %q; got %v", want, got)
		}
	}
	if got["backup-crd-certificates-cert-manager-io"] {
		t.Error("third-party CRD was backed up; only vjailbreak CRDs belong in the snapshot")
	}
}

// Missing ConfigMaps and Deployments are warnings, not failures: the flow must still
// back up whatever does exist.
func TestBackupResourcesWithIDToleratesMissingResources(t *testing.T) {
	kubeClient := newFakeClient(t, deployment("vjailbreak-ui", 1, 1, 1))

	if err := BackupResourcesWithID(context.Background(), kubeClient, &rest.Config{}, "id-1"); err != nil {
		t.Fatalf("BackupResourcesWithID() error = %v, want nil", err)
	}

	cm := &corev1.ConfigMap{}
	if err := kubeClient.Get(context.Background(),
		client.ObjectKey{Name: "backup-deploy-vjailbreak-ui", Namespace: Namespace}, cm); err != nil {
		t.Fatalf("expected the UI deployment to still be backed up: %v", err)
	}
}

func TestRestoreResourcesWithoutBackupsIsAnError(t *testing.T) {
	err := RestoreResources(context.Background(), newFakeClient(t), "missing-id")
	if err == nil {
		t.Fatal("RestoreResources() error = nil, want an error when the backup ID has no snapshots")
	}
	if !strings.Contains(err.Error(), "no backups found") {
		t.Errorf("error = %q, want it to name the missing backup ID", err)
	}
}

func TestRestoreResourcesRestoresSnapshots(t *testing.T) {
	const crdYAML = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: migrations.vjailbreak.k8s.pf9.io
spec:
  group: vjailbreak.k8s.pf9.io
  names:
    kind: Migration
    plural: migrations
  scope: Namespaced
  versions:
    - name: v1alpha1
      served: true
      storage: true
status:
  conditions:
    - type: Established
      status: "True"
`
	const configMapYAML = `apiVersion: v1
kind: ConfigMap
metadata:
  name: version-config
  namespace: migration-system
data:
  version: v0.4.8
`
	const deploymentYAML = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: vjailbreak-ui
  namespace: migration-system
spec:
  replicas: 2
status:
  readyReplicas: 2
  updatedReplicas: 2
`

	kubeClient := newFakeClient(t,
		backupConfigMap("backup-crd-migrations-vjailbreak-k8s-pf9-io", "id-1", crdYAML, 0),
		backupConfigMap("backup-cm-version-config", "id-1", configMapYAML, 0),
		backupConfigMap("backup-deploy-vjailbreak-ui", "id-1", deploymentYAML, 0),
		// Seeded already-settled so the readiness and scale-down waits return at once.
		deployment("migration-controller-manager", 1, 1, 0),
		deployment("vjailbreak-ui", 2, 2, 0),
	)

	if err := RestoreResources(context.Background(), kubeClient, "id-1"); err != nil {
		t.Fatalf("RestoreResources() error = %v, want nil", err)
	}

	restoredCM := &corev1.ConfigMap{}
	if err := kubeClient.Get(context.Background(),
		client.ObjectKey{Name: "version-config", Namespace: Namespace}, restoredCM); err != nil {
		t.Fatalf("version-config was not restored: %v", err)
	}
	if restoredCM.Data["version"] != "v0.4.8" {
		t.Errorf("restored version = %q, want v0.4.8", restoredCM.Data["version"])
	}

	restoredCRD := &apiextensionsv1.CustomResourceDefinition{}
	if err := kubeClient.Get(context.Background(),
		client.ObjectKey{Name: "migrations.vjailbreak.k8s.pf9.io"}, restoredCRD); err != nil {
		t.Fatalf("CRD was not restored: %v", err)
	}

	restoredDeploy := &appsv1.Deployment{}
	if err := kubeClient.Get(context.Background(),
		client.ObjectKey{Name: "vjailbreak-ui", Namespace: Namespace}, restoredDeploy); err != nil {
		t.Fatalf("UI deployment was not restored: %v", err)
	}
	if restoredDeploy.Spec.Replicas == nil || *restoredDeploy.Spec.Replicas != 2 {
		t.Errorf("restored replicas = %v, want the 2 recorded in the snapshot", restoredDeploy.Spec.Replicas)
	}
}

// A backup ConfigMap without the resource key must be skipped rather than crash the
// rollback.
func TestRestoreResourcesSkipsBackupsWithoutResourceKey(t *testing.T) {
	broken := backupConfigMap("backup-cm-version-config", "id-1", "", 0)
	broken.Data = map[string]string{}

	if err := RestoreResources(context.Background(), newFakeClient(t, broken), "id-1"); err != nil {
		t.Fatalf("RestoreResources() error = %v, want nil", err)
	}
}

// ---------------------------------------------------------------------------
// Deployment helpers
// ---------------------------------------------------------------------------

func TestParseReplicasFromDeploymentYAML(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want int32
	}{
		{name: "explicit replicas", yaml: "spec:\n  replicas: 3\n", want: 3},
		{name: "missing replicas defaults to one", yaml: "spec: {}\n", want: 1},
		{name: "zero replicas is honoured", yaml: "spec:\n  replicas: 0\n", want: 0},
		{name: "unparseable yaml defaults to one", yaml: "\tnot: [valid", want: 1},
		{name: "empty input defaults to one", yaml: "", want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseReplicasFromDeploymentYAML([]byte(tt.yaml)); got != tt.want {
				t.Errorf("parseReplicasFromDeploymentYAML(%q) = %d, want %d", tt.yaml, got, tt.want)
			}
		})
	}
}

func TestScaleDeploymentTo(t *testing.T) {
	kubeClient := newFakeClient(t, deployment("vjailbreak-ui", 3, 3, 3))

	if err := scaleDeploymentTo(context.Background(), kubeClient, "vjailbreak-ui", Namespace, 0); err != nil {
		t.Fatalf("scaleDeploymentTo() error = %v, want nil", err)
	}

	dep := &appsv1.Deployment{}
	if err := kubeClient.Get(context.Background(),
		client.ObjectKey{Name: "vjailbreak-ui", Namespace: Namespace}, dep); err != nil {
		t.Fatalf("failed to re-read deployment: %v", err)
	}
	if dep.Spec.Replicas == nil || *dep.Spec.Replicas != 0 {
		t.Errorf("replicas = %v, want 0", dep.Spec.Replicas)
	}
}

func TestScaleDeploymentToMissingDeployment(t *testing.T) {
	err := scaleDeploymentTo(context.Background(), newFakeClient(t), "vjailbreak-ui", Namespace, 0)
	if err == nil {
		t.Fatal("scaleDeploymentTo() error = nil, want an error for a missing deployment")
	}
}

func TestWaitForDeploymentReadyLocal(t *testing.T) {
	tests := []struct {
		name    string
		objects []client.Object
		timeout time.Duration
		cancel  bool
		wantErr string
	}{
		{
			name:    "ready deployment returns immediately",
			objects: []client.Object{deployment("vjailbreak-ui", 2, 2, 2)},
			timeout: time.Second,
		},
		{
			name:    "missing deployment is reported as not found",
			timeout: time.Second,
			wantErr: "not found",
		},
		{
			name:    "expired timeout is reported",
			objects: []client.Object{deployment("vjailbreak-ui", 2, 0, 2)},
			timeout: 0,
			wantErr: "not ready within timeout",
		},
		{
			name:    "cancelled context aborts the wait",
			objects: []client.Object{deployment("vjailbreak-ui", 2, 0, 2)},
			timeout: time.Minute,
			cancel:  true,
			wantErr: "context canceled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tt.cancel {
				cancel()
			}

			err := waitForDeploymentReadyLocal(ctx, newFakeClient(t, tt.objects...), "vjailbreak-ui", Namespace, tt.timeout)

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("waitForDeploymentReadyLocal() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("waitForDeploymentReadyLocal() error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestWaitForDeploymentScaledDownLocal(t *testing.T) {
	tests := []struct {
		name    string
		objects []client.Object
		timeout time.Duration
		cancel  bool
		wantErr string
	}{
		{
			name:    "no running replicas returns immediately",
			objects: []client.Object{deployment("migration-controller-manager", 0, 0, 0)},
			timeout: time.Second,
		},
		{
			name:    "a deleted deployment counts as scaled down",
			timeout: time.Second,
		},
		{
			name:    "expired timeout is reported",
			objects: []client.Object{deployment("migration-controller-manager", 1, 1, 1)},
			timeout: 0,
			wantErr: "not scaled down within timeout",
		},
		{
			name:    "cancelled context aborts the wait",
			objects: []client.Object{deployment("migration-controller-manager", 1, 1, 1)},
			timeout: time.Minute,
			cancel:  true,
			wantErr: "context canceled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tt.cancel {
				cancel()
			}

			err := waitForDeploymentScaledDownLocal(ctx, newFakeClient(t, tt.objects...),
				"migration-controller-manager", Namespace, tt.timeout)

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("waitForDeploymentScaledDownLocal() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("waitForDeploymentScaledDownLocal() error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestWaitForCRDEstablished(t *testing.T) {
	established := vjailbreakCRD("migrations.vjailbreak.k8s.pf9.io", "vjailbreak.k8s.pf9.io", "migrations", "Migration")

	pending := vjailbreakCRD("migrationplans.vjailbreak.k8s.pf9.io", "vjailbreak.k8s.pf9.io", "migrationplans", "MigrationPlan")
	pending.Status.Conditions = nil

	thirdPartyPending := vjailbreakCRD("certificates.cert-manager.io", "cert-manager.io", "certificates", "Certificate")
	thirdPartyPending.Status.Conditions = nil

	tests := []struct {
		name    string
		crds    []client.Object
		timeout time.Duration
		wantErr bool
	}{
		{name: "all vjailbreak CRDs established", crds: []client.Object{established}, timeout: time.Second},
		{name: "no CRDs at all is vacuously true", timeout: time.Second},
		{
			name:    "a pending third-party CRD is ignored",
			crds:    []client.Object{established, thirdPartyPending},
			timeout: time.Second,
		},
		{
			name:    "a pending vjailbreak CRD times out",
			crds:    []client.Object{established, pending},
			timeout: 0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := waitForCRDEstablished(context.Background(), newFakeClient(t, tt.crds...), tt.timeout)
			if tt.wantErr {
				if err == nil {
					t.Fatal("waitForCRDEstablished() error = nil, want a timeout error")
				}
				return
			}
			if err != nil {
				t.Fatalf("waitForCRDEstablished() error = %v, want nil", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// applyRestoredObject
// ---------------------------------------------------------------------------

func TestApplyRestoredObject(t *testing.T) {
	const snapshot = `apiVersion: v1
kind: ConfigMap
metadata:
  name: version-config
  namespace: migration-system
  resourceVersion: "9999"
data:
  version: v0.4.8
`

	t.Run("creates the object when it is absent", func(t *testing.T) {
		kubeClient := newFakeClient(t)

		if err := applyRestoredObject(context.Background(), kubeClient, []byte(snapshot)); err != nil {
			t.Fatalf("applyRestoredObject() error = %v, want nil", err)
		}

		cm := &corev1.ConfigMap{}
		if err := kubeClient.Get(context.Background(),
			client.ObjectKey{Name: "version-config", Namespace: Namespace}, cm); err != nil {
			t.Fatalf("object was not created: %v", err)
		}
		if cm.Data["version"] != "v0.4.8" {
			t.Errorf("data = %v, want version v0.4.8", cm.Data)
		}
	})

	t.Run("overwrites an existing object", func(t *testing.T) {
		kubeClient := newFakeClient(t, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "version-config", Namespace: Namespace},
			Data:       map[string]string{"version": "v0.4.9"},
		})

		if err := applyRestoredObject(context.Background(), kubeClient, []byte(snapshot)); err != nil {
			t.Fatalf("applyRestoredObject() error = %v, want nil", err)
		}

		cm := &corev1.ConfigMap{}
		if err := kubeClient.Get(context.Background(),
			client.ObjectKey{Name: "version-config", Namespace: Namespace}, cm); err != nil {
			t.Fatalf("failed to re-read object: %v", err)
		}
		if cm.Data["version"] != "v0.4.8" {
			t.Errorf("data = %v, want the snapshot's v0.4.8", cm.Data)
		}
	})

	t.Run("rejects malformed yaml", func(t *testing.T) {
		if err := applyRestoredObject(context.Background(), newFakeClient(t), []byte("\tnope: [")); err == nil {
			t.Fatal("applyRestoredObject() error = nil, want a parse error")
		}
	})
}

// ---------------------------------------------------------------------------
// Cleanup
// ---------------------------------------------------------------------------

func TestCleanupResources(t *testing.T) {
	dyn := newFakeDynamic(t,
		newCR("MigrationPlan", "plan-1"),
		newCR("RollingMigrationPlan", "rolling-1"),
		newCR("VjailbreakNode", "vjailbreak-master"),
		newCR("VjailbreakNode", "vjailbreak-worker-1"),
		newCR("VMwareMachine", "vm-1"),
	)
	withFakeDynamic(t, dyn)

	kubeClient := newFakeClient(t,
		vjailbreakCRD("vmwaremachines.vjailbreak.k8s.pf9.io", "vjailbreak.k8s.pf9.io", "vmwaremachines", "VMwareMachine"),
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "vmware-credentials", Namespace: Namespace}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "openstack-credentials", Namespace: Namespace}},
	)

	if err := CleanupResources(context.Background(), kubeClient, &rest.Config{}); err != nil {
		t.Fatalf("CleanupResources() error = %v, want nil", err)
	}

	remaining := func(resource string) []string {
		list, err := dyn.Resource(vjailbreakGVR(resource)).Namespace(Namespace).
			List(context.Background(), metav1.ListOptions{})
		if err != nil {
			t.Fatalf("failed to list %s: %v", resource, err)
		}
		names := []string{}
		for _, item := range list.Items {
			names = append(names, item.GetName())
		}
		return names
	}

	if names := remaining("migrationplans"); len(names) != 0 {
		t.Errorf("MigrationPlans left = %v, want none", names)
	}
	if names := remaining("rollingmigrationplans"); len(names) != 0 {
		t.Errorf("RollingMigrationPlans left = %v, want none", names)
	}
	if names := remaining("vjailbreaknodes"); !reflect.DeepEqual(names, []string{"vjailbreak-master"}) {
		t.Errorf("VjailbreakNodes left = %v, want only vjailbreak-master", names)
	}
	if names := remaining("vmwaremachines"); len(names) != 0 {
		t.Errorf("VMwareMachines left = %v, want none", names)
	}

	for _, name := range []string{"vmware-credentials", "openstack-credentials"} {
		err := kubeClient.Get(context.Background(), client.ObjectKey{Name: name, Namespace: Namespace}, &corev1.Secret{})
		if !kerrors.IsNotFound(err) {
			t.Errorf("secret %s still present (err=%v), want it deleted", name, err)
		}
	}
}

// Cleanup runs before every upgrade, including on appliances where the credential
// secrets were already removed; absent objects must not fail it.
func TestCleanupResourcesWithNothingToDelete(t *testing.T) {
	withFakeDynamic(t, newFakeDynamic(t))

	if err := CleanupResources(context.Background(), newFakeClient(t), &rest.Config{}); err != nil {
		t.Fatalf("CleanupResources() error = %v, want nil", err)
	}
}

func TestCleanupResourcesDynamicClientFailure(t *testing.T) {
	original := newDynamicClient
	newDynamicClient = func(*rest.Config) (dynamic.Interface, error) {
		return nil, errors.New("no kubeconfig")
	}
	t.Cleanup(func() { newDynamicClient = original })

	err := CleanupResources(context.Background(), newFakeClient(t), &rest.Config{})
	if err == nil {
		t.Fatal("CleanupResources() error = nil, want the dynamic client failure surfaced")
	}
}

func TestDeleteAllCustomResources(t *testing.T) {
	dyn := newFakeDynamic(t, newCR("Migration", "migration-1"), newCR("VMwareMachine", "vm-1"))
	withFakeDynamic(t, dyn)

	kubeClient := newFakeClient(t,
		vjailbreakCRD("migrations.vjailbreak.k8s.pf9.io", "vjailbreak.k8s.pf9.io", "migrations", "Migration"),
		vjailbreakCRD("vmwaremachines.vjailbreak.k8s.pf9.io", "vjailbreak.k8s.pf9.io", "vmwaremachines", "VMwareMachine"),
	)

	if err := deleteAllCustomResources(context.Background(), kubeClient, &rest.Config{}); err != nil {
		t.Fatalf("deleteAllCustomResources() error = %v, want nil", err)
	}

	for _, resource := range []string{"migrations", "vmwaremachines"} {
		list, err := dyn.Resource(vjailbreakGVR(resource)).Namespace(Namespace).
			List(context.Background(), metav1.ListOptions{})
		if err != nil {
			t.Fatalf("failed to list %s: %v", resource, err)
		}
		if len(list.Items) != 0 {
			t.Errorf("%s left = %d, want 0", resource, len(list.Items))
		}
	}
}

// One unlistable CRD must not stop the others from being cleaned up, otherwise a single
// stale CRD would block every upgrade.
func TestDeleteAllCustomResourcesContinuesAfterFailure(t *testing.T) {
	dyn := newFakeDynamic(t, newCR("Migration", "migration-1"))
	failListOf(dyn, "widgets")
	withFakeDynamic(t, dyn)

	kubeClient := newFakeClient(t,
		vjailbreakCRD("widgets.vjailbreak.k8s.pf9.io", "vjailbreak.k8s.pf9.io", "widgets", "Widget"),
		vjailbreakCRD("migrations.vjailbreak.k8s.pf9.io", "vjailbreak.k8s.pf9.io", "migrations", "Migration"),
	)

	if err := deleteAllCustomResources(context.Background(), kubeClient, &rest.Config{}); err != nil {
		t.Fatalf("deleteAllCustomResources() error = %v, want nil", err)
	}

	list, err := dyn.Resource(vjailbreakGVR("migrations")).Namespace(Namespace).
		List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("failed to list migrations: %v", err)
	}
	if len(list.Items) != 0 {
		t.Errorf("migrations left = %d, want 0 despite the unlistable CRD", len(list.Items))
	}
}

func TestDeleteCRInstancesDynamicClientFailure(t *testing.T) {
	original := newDynamicClient
	newDynamicClient = func(*rest.Config) (dynamic.Interface, error) {
		return nil, errors.New("no kubeconfig")
	}
	t.Cleanup(func() { newDynamicClient = original })

	err := deleteCRInstances(context.Background(), &rest.Config{}, CRInfo{
		Group: "vjailbreak.k8s.pf9.io", Version: "v1alpha1", Kind: "Migration", Plural: "migrations",
	})
	if err == nil {
		t.Fatal("deleteCRInstances() error = nil, want the dynamic client failure surfaced")
	}
}

func TestCleanupBackupConfigMaps(t *testing.T) {
	tests := []struct {
		name     string
		backupID string
		want     []string
	}{
		{
			name:     "only the named backup ID is deleted",
			backupID: "id-1",
			want:     []string{"backup-cm-other"},
		},
		{
			name:     "an empty backup ID deletes every backup",
			backupID: "",
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kubeClient := newFakeClient(t,
				backupConfigMap("backup-cm-version-config", "id-1", "data", 0),
				backupConfigMap("backup-cm-other", "id-2", "data", 0),
			)

			if err := CleanupBackupConfigMaps(context.Background(), kubeClient, tt.backupID); err != nil {
				t.Fatalf("CleanupBackupConfigMaps() error = %v, want nil", err)
			}

			list := &corev1.ConfigMapList{}
			if err := kubeClient.List(context.Background(), list, client.InNamespace(Namespace)); err != nil {
				t.Fatalf("failed to list ConfigMaps: %v", err)
			}
			var names []string
			for _, cm := range list.Items {
				names = append(names, cm.Name)
			}
			if !reflect.DeepEqual(names, tt.want) {
				t.Errorf("remaining ConfigMaps = %v, want %v", names, tt.want)
			}
		})
	}
}

func TestCleanupAllOldBackups(t *testing.T) {
	kubeClient := newFakeClient(t,
		backupConfigMap("backup-cm-current", "current-id", "data", 3*time.Hour),
		backupConfigMap("backup-cm-old", "old-id", "data", 3*time.Hour),
		backupConfigMap("backup-cm-recent", "recent-id", "data", 5*time.Minute),
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "version-config", Namespace: Namespace}},
	)

	if err := CleanupAllOldBackups(context.Background(), kubeClient, "current-id"); err != nil {
		t.Fatalf("CleanupAllOldBackups() error = %v, want nil", err)
	}

	list := &corev1.ConfigMapList{}
	if err := kubeClient.List(context.Background(), list, client.InNamespace(Namespace)); err != nil {
		t.Fatalf("failed to list ConfigMaps: %v", err)
	}
	remaining := map[string]bool{}
	for _, cm := range list.Items {
		remaining[cm.Name] = true
	}

	if !remaining["backup-cm-current"] {
		t.Error("the in-flight backup was deleted; it must survive so rollback still works")
	}
	if !remaining["backup-cm-recent"] {
		t.Error("a backup younger than an hour was deleted; recent snapshots are kept")
	}
	if remaining["backup-cm-old"] {
		t.Error("an old backup from a previous upgrade was not cleaned up")
	}
	if !remaining["version-config"] {
		t.Error("an unlabelled ConfigMap was deleted; only backups may be touched")
	}
}

func TestCleanupAllOldBackupsListError(t *testing.T) {
	kubeClient := interceptor.NewClient(newFakeClient(t), interceptor.Funcs{
		List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
			return errors.New("api server down")
		},
	})

	if err := CleanupAllOldBackups(context.Background(), kubeClient, "id-1"); err == nil {
		t.Fatal("CleanupAllOldBackups() error = nil, want the list failure surfaced")
	}
}

func TestCleanupBackupConfigMapsListError(t *testing.T) {
	kubeClient := interceptor.NewClient(newFakeClient(t), interceptor.Funcs{
		List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
			return errors.New("api server down")
		},
	})

	if err := CleanupBackupConfigMaps(context.Background(), kubeClient, "id-1"); err == nil {
		t.Fatal("CleanupBackupConfigMaps() error = nil, want the list failure surfaced")
	}
}

// ---------------------------------------------------------------------------
// HTTP fetching
// ---------------------------------------------------------------------------

// sequenceTransport replies with one status per attempt, so retry behaviour can be
// asserted without real network flakiness.
type sequenceTransport struct {
	statuses []int
	body     string
	calls    int
}

func (s *sequenceTransport) RoundTrip(*http.Request) (*http.Response, error) {
	status := http.StatusOK
	if s.calls < len(s.statuses) {
		status = s.statuses[s.calls]
	}
	s.calls++
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Body:       io.NopCloser(strings.NewReader(s.body)),
		Header:     make(http.Header),
	}, nil
}

func serveSequence(t *testing.T, transport *sequenceTransport) {
	t.Helper()
	original := httpClient
	httpClient = &http.Client{Transport: transport}
	t.Cleanup(func() { httpClient = original })
}

func TestHTTPGetWithRetry(t *testing.T) {
	tests := []struct {
		name       string
		statuses   []int
		maxRetries int
		wantCalls  int
		wantErr    bool
		about      string
	}{
		{
			name:       "success on the first attempt",
			statuses:   []int{http.StatusOK},
			maxRetries: 3,
			wantCalls:  1,
		},
		{
			name:       "a 404 is not retried",
			statuses:   []int{http.StatusNotFound},
			maxRetries: 3,
			wantCalls:  1,
			wantErr:    true,
			about:      "a manifest missing from a tag is a permanent answer",
		},
		{
			name:       "a 500 is retried and can succeed",
			statuses:   []int{http.StatusInternalServerError, http.StatusOK},
			maxRetries: 3,
			wantCalls:  2,
		},
		{
			name:       "retries are bounded",
			statuses:   []int{http.StatusInternalServerError},
			maxRetries: 1,
			wantCalls:  1,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := &sequenceTransport{statuses: tt.statuses, body: "ok"}
			serveSequence(t, transport)

			resp, err := httpGetWithRetry(context.Background(), "https://example.invalid/manifest.yaml", tt.maxRetries)
			if resp != nil {
				resp.Body.Close()
			}

			if tt.wantErr && err == nil {
				t.Fatalf("httpGetWithRetry() error = nil, want an error (%s)", tt.about)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("httpGetWithRetry() error = %v, want nil", err)
			}
			if transport.calls != tt.wantCalls {
				t.Errorf("attempts = %d, want %d", transport.calls, tt.wantCalls)
			}
		})
	}
}

func TestHTTPGetWithRetryRespectsCancelledContext(t *testing.T) {
	serveSequence(t, &sequenceTransport{statuses: []int{http.StatusOK}, body: "ok"})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := httpGetWithRetry(ctx, "https://example.invalid/manifest.yaml", 3); err == nil {
		t.Fatal("httpGetWithRetry() error = nil, want the cancelled context surfaced")
	}
}

func TestFetchConfigsFromGitHub(t *testing.T) {
	tests := []struct {
		name  string
		fetch func(context.Context, string) ([]byte, error)
	}{
		{name: "version-config", fetch: fetchVersionConfigFromGitHub},
		{name: "vjailbreak-settings", fetch: fetchVjailbreakSettingsFromGitHub},
	}

	for _, tt := range tests {
		t.Run(tt.name+" is returned verbatim", func(t *testing.T) {
			serveManifest(t, "data:\n  version: v0.4.9\n")

			got, err := tt.fetch(context.Background(), "v0.4.9")
			if err != nil {
				t.Fatalf("fetch error = %v, want nil", err)
			}
			if !strings.Contains(string(got), "version: v0.4.9") {
				t.Errorf("body = %q, want the served document", got)
			}
		})

		t.Run(tt.name+" surfaces a missing file", func(t *testing.T) {
			original := httpClient
			httpClient = &http.Client{Transport: stubTransport{body: "not found", status: http.StatusNotFound}}
			t.Cleanup(func() { httpClient = original })

			if _, err := tt.fetch(context.Background(), "v9.9.9"); err == nil {
				t.Fatal("fetch error = nil, want an error for a missing file")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ConfigMap updates
// ---------------------------------------------------------------------------

func TestUpdateVersionConfigMapFromGitHub(t *testing.T) {
	// ${TAG} is substituted before parsing, which is how the published ConfigMap ends
	// up carrying the version being installed.
	const doc = "data:\n  version: ${TAG}\n  upgradeAvailable: \"false\"\n"

	t.Run("creates the ConfigMap when absent", func(t *testing.T) {
		serveManifest(t, doc)
		kubeClient := newFakeClient(t)

		if err := UpdateVersionConfigMapFromGitHub(context.Background(), kubeClient, "v0.4.9"); err != nil {
			t.Fatalf("UpdateVersionConfigMapFromGitHub() error = %v, want nil", err)
		}

		cm := &corev1.ConfigMap{}
		if err := kubeClient.Get(context.Background(),
			client.ObjectKey{Name: "version-config", Namespace: Namespace}, cm); err != nil {
			t.Fatalf("ConfigMap was not created: %v", err)
		}
		if cm.Data["version"] != "v0.4.9" {
			t.Errorf("version = %q, want the substituted v0.4.9", cm.Data["version"])
		}
	})

	t.Run("updates an existing ConfigMap", func(t *testing.T) {
		serveManifest(t, doc)
		kubeClient := newFakeClient(t, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "version-config", Namespace: Namespace},
			Data:       map[string]string{"version": "v0.4.8"},
		})

		if err := UpdateVersionConfigMapFromGitHub(context.Background(), kubeClient, "v0.4.9"); err != nil {
			t.Fatalf("UpdateVersionConfigMapFromGitHub() error = %v, want nil", err)
		}

		cm := &corev1.ConfigMap{}
		if err := kubeClient.Get(context.Background(),
			client.ObjectKey{Name: "version-config", Namespace: Namespace}, cm); err != nil {
			t.Fatalf("failed to re-read ConfigMap: %v", err)
		}
		if cm.Data["version"] != "v0.4.9" {
			t.Errorf("version = %q, want v0.4.9", cm.Data["version"])
		}
	})

	t.Run("surfaces a fetch failure", func(t *testing.T) {
		original := httpClient
		httpClient = &http.Client{Transport: stubTransport{body: "nope", status: http.StatusNotFound}}
		t.Cleanup(func() { httpClient = original })

		if err := UpdateVersionConfigMapFromGitHub(context.Background(), newFakeClient(t), "v9.9.9"); err == nil {
			t.Fatal("UpdateVersionConfigMapFromGitHub() error = nil, want the fetch failure surfaced")
		}
	})

	t.Run("rejects malformed yaml", func(t *testing.T) {
		serveManifest(t, "\tdata: [")

		if err := UpdateVersionConfigMapFromGitHub(context.Background(), newFakeClient(t), "v0.4.9"); err == nil {
			t.Fatal("UpdateVersionConfigMapFromGitHub() error = nil, want a parse error")
		}
	})
}

func TestUpdateVjailbreakSettingsFromGitHub(t *testing.T) {
	const doc = "data:\n  VERSION: ${TAG}\n"

	t.Run("creates the ConfigMap when absent", func(t *testing.T) {
		serveManifest(t, doc)
		kubeClient := newFakeClient(t)

		if err := UpdateVjailbreakSettingsFromGitHub(context.Background(), kubeClient, "v0.4.9"); err != nil {
			t.Fatalf("UpdateVjailbreakSettingsFromGitHub() error = %v, want nil", err)
		}

		cm := &corev1.ConfigMap{}
		if err := kubeClient.Get(context.Background(),
			client.ObjectKey{Name: "vjailbreak-settings", Namespace: Namespace}, cm); err != nil {
			t.Fatalf("ConfigMap was not created: %v", err)
		}
		if cm.Data["VERSION"] != "v0.4.9" {
			t.Errorf("VERSION = %q, want the substituted v0.4.9", cm.Data["VERSION"])
		}
	})

	t.Run("updates an existing ConfigMap", func(t *testing.T) {
		serveManifest(t, doc)
		kubeClient := newFakeClient(t, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "vjailbreak-settings", Namespace: Namespace},
			Data:       map[string]string{"VERSION": "v0.4.8"},
		})

		if err := UpdateVjailbreakSettingsFromGitHub(context.Background(), kubeClient, "v0.4.9"); err != nil {
			t.Fatalf("UpdateVjailbreakSettingsFromGitHub() error = %v, want nil", err)
		}

		cm := &corev1.ConfigMap{}
		if err := kubeClient.Get(context.Background(),
			client.ObjectKey{Name: "vjailbreak-settings", Namespace: Namespace}, cm); err != nil {
			t.Fatalf("failed to re-read ConfigMap: %v", err)
		}
		if cm.Data["VERSION"] != "v0.4.9" {
			t.Errorf("VERSION = %q, want v0.4.9", cm.Data["VERSION"])
		}
	})

	t.Run("surfaces a fetch failure", func(t *testing.T) {
		original := httpClient
		httpClient = &http.Client{Transport: stubTransport{body: "nope", status: http.StatusNotFound}}
		t.Cleanup(func() { httpClient = original })

		if err := UpdateVjailbreakSettingsFromGitHub(context.Background(), newFakeClient(t), "v9.9.9"); err == nil {
			t.Fatal("UpdateVjailbreakSettingsFromGitHub() error = nil, want the fetch failure surfaced")
		}
	})
}

// ---------------------------------------------------------------------------
// Manifest fetch failures on the apply paths
// ---------------------------------------------------------------------------

func TestApplyAllCRDsFetchFailure(t *testing.T) {
	original := httpClient
	httpClient = &http.Client{Transport: stubTransport{body: "nope", status: http.StatusNotFound}}
	t.Cleanup(func() { httpClient = original })

	if err := ApplyAllCRDs(context.Background(), newFakeClient(t), "v9.9.9"); err == nil {
		t.Fatal("ApplyAllCRDs() error = nil, want the fetch failure surfaced")
	}
}

func TestApplyManifestFromGitHubFetchFailure(t *testing.T) {
	original := httpClient
	httpClient = &http.Client{Transport: stubTransport{body: "nope", status: http.StatusNotFound}}
	t.Cleanup(func() { httpClient = original })

	err := ApplyManifestFromGitHub(context.Background(), newFakeClient(t), "v0.4.7",
		"deploy/08vjailbreak-ai-deployment.yaml")
	if err == nil {
		t.Fatal("ApplyManifestFromGitHub() error = nil, want the fetch failure surfaced")
	}
}

// A manifest document without metadata.name would otherwise be applied as an anonymous
// object and fail deep inside the API server.
func TestApplyManifestFromGitHubRejectsUnnamedObject(t *testing.T) {
	serveManifest(t, "apiVersion: v1\nkind: ConfigMap\nmetadata: {}\n")

	err := ApplyManifestFromGitHub(context.Background(), newFakeClient(t), "v0.4.9", "deploy/bad.yaml")
	if err == nil {
		t.Fatal("ApplyManifestFromGitHub() error = nil, want a validation error")
	}
	if !strings.Contains(err.Error(), "empty metadata.name") {
		t.Errorf("error = %q, want it to name the problem", err)
	}
}

func TestApplyAllCRDsApplyFailure(t *testing.T) {
	serveManifest(t, crdsManifest)

	kubeClient := interceptor.NewClient(newFakeClient(t), interceptor.Funcs{
		Patch: func(context.Context, client.WithWatch, client.Object, client.Patch, ...client.PatchOption) error {
			return kerrors.NewForbidden(schema.GroupResource{Resource: "customresourcedefinitions"},
				"migrations.vjailbreak.k8s.pf9.io", errors.New("rbac denied"))
		},
	})

	if err := ApplyAllCRDs(context.Background(), kubeClient, "v0.4.9"); err == nil {
		t.Fatal("ApplyAllCRDs() error = nil, want the apply failure surfaced")
	}
}

// deploymentSnapshot is what a backup ConfigMap holds: a serialized live Deployment.
// Status is already settled so the restore's readiness wait returns on its first check.
func deploymentSnapshot(name string, replicas int32) string {
	return fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
spec:
  replicas: %d
status:
  readyReplicas: %d
  updatedReplicas: %d
`, name, Namespace, replicas, replicas, replicas)
}

// Rollback can only restore what was snapshotted, so every deployment the upgrade
// replaces has to be backed up first.
func TestBackupResourcesWithIDBacksUpEveryDeployment(t *testing.T) {
	var objs []client.Object
	for _, cfg := range DeploymentConfigs {
		objs = append(objs, deployment(cfg.Name, 1, 1, 1))
	}
	kubeClient := newFakeClient(t, objs...)

	if err := BackupResourcesWithID(context.Background(), kubeClient, &rest.Config{}, "id-1"); err != nil {
		t.Fatalf("BackupResourcesWithID() error = %v, want nil", err)
	}

	for _, cfg := range DeploymentConfigs {
		cm := &corev1.ConfigMap{}
		key := client.ObjectKey{Name: "backup-deploy-" + cfg.Name, Namespace: Namespace}
		if err := kubeClient.Get(context.Background(), key, cm); err != nil {
			t.Errorf("no backup for %s: %v — rollback would have nothing to restore", cfg.Name, err)
			continue
		}
		if cm.Data["resource"] == "" {
			t.Errorf("backup for %s holds no serialized resource", cfg.Name)
		}
	}
}

func TestRestoreResourcesRestoresEveryDeployment(t *testing.T) {
	// Seeded settled at the snapshot's replica count: zero running replicas satisfies the
	// controller scale-down wait, and ready==desired satisfies each readiness wait, so no
	// wait sleeps through its poll interval.
	var objs []client.Object
	for _, cfg := range DeploymentConfigs {
		objs = append(objs, deployment(cfg.Name, 2, 2, 0))
		objs = append(objs, backupConfigMap("backup-deploy-"+cfg.Name, "id-1",
			deploymentSnapshot(cfg.Name, 2), 0))
	}

	kubeClient := newFakeClient(t, objs...)

	if err := RestoreResources(context.Background(), kubeClient, "id-1"); err != nil {
		t.Fatalf("RestoreResources() error = %v, want nil", err)
	}

	for _, cfg := range DeploymentConfigs {
		dep := &appsv1.Deployment{}
		key := client.ObjectKey{Name: cfg.Name, Namespace: Namespace}
		if err := kubeClient.Get(context.Background(), key, dep); err != nil {
			t.Errorf("%s was not restored: %v", cfg.Name, err)
			continue
		}
		if dep.Spec.Replicas == nil || *dep.Spec.Replicas != 2 {
			t.Errorf("%s replicas = %v, want the 2 recorded in its snapshot", cfg.Name, dep.Spec.Replicas)
		}
	}
}

// A deployment with no snapshot must be skipped without stopping the restore of the
// others.
func TestRestoreResourcesSkipsDeploymentWithoutBackup(t *testing.T) {
	var objs []client.Object
	for _, cfg := range DeploymentConfigs {
		objs = append(objs, deployment(cfg.Name, 2, 2, 0))
		if cfg.Name == "vjailbreak-ai" {
			continue // no snapshot for the AI pod
		}
		objs = append(objs, backupConfigMap("backup-deploy-"+cfg.Name, "id-1",
			deploymentSnapshot(cfg.Name, 2), 0))
	}

	kubeClient := newFakeClient(t, objs...)

	if err := RestoreResources(context.Background(), kubeClient, "id-1"); err != nil {
		t.Fatalf("RestoreResources() error = %v, want nil", err)
	}

	for _, cfg := range DeploymentConfigs {
		if cfg.Name == "vjailbreak-ai" {
			continue
		}
		dep := &appsv1.Deployment{}
		key := client.ObjectKey{Name: cfg.Name, Namespace: Namespace}
		if err := kubeClient.Get(context.Background(), key, dep); err != nil {
			t.Fatalf("%s was not restored: %v", cfg.Name, err)
		}
		if dep.Spec.Replicas == nil || *dep.Spec.Replicas != 2 {
			t.Errorf("%s replicas = %v, want 2 from its snapshot", cfg.Name, dep.Spec.Replicas)
		}
	}
}
