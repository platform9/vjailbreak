package upgrade

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

// fastPolling shrinks the deployment wait cadence so phase tests do not sleep through
// production intervals.
func fastPolling(t *testing.T) {
	t.Helper()

	originalInterval, originalTimeout := deploymentPollInterval, deploymentWaitTimeout
	deploymentPollInterval = time.Millisecond
	deploymentWaitTimeout = 100 * time.Millisecond
	t.Cleanup(func() {
		deploymentPollInterval, deploymentWaitTimeout = originalInterval, originalTimeout
	})
}

// githubRouter answers the raw.githubusercontent paths the upgrade flow fetches, and
// records which ones were requested. Paths listed in notFound return 404.
type githubRouter struct {
	notFound  map[string]bool
	requested []string
}

func (g *githubRouter) RoundTrip(r *http.Request) (*http.Response, error) {
	g.requested = append(g.requested, r.URL.Path)

	reply := func(status int, body string) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Status:     http.StatusText(status),
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	}

	for path := range g.notFound {
		if strings.HasSuffix(r.URL.Path, path) {
			return reply(http.StatusNotFound, "404: Not Found")
		}
	}

	switch {
	case strings.HasSuffix(r.URL.Path, "deploy/00crds.yaml"):
		return reply(http.StatusOK, `apiVersion: apiextensions.k8s.io/v1
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
`)
	case strings.HasSuffix(r.URL.Path, "image_builder/configs/version-config.yaml"):
		return reply(http.StatusOK, "data:\n  version: ${TAG}\n")
	case strings.HasSuffix(r.URL.Path, "image_builder/configs/vjailbreak-settings.yaml"):
		return reply(http.StatusOK, "data:\n  VERSION: ${TAG}\n")
	}

	for _, cfg := range DeploymentConfigs {
		if strings.HasSuffix(r.URL.Path, deploymentManifestPath(cfg.Name)) {
			return reply(http.StatusOK, fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
spec:
  replicas: 1
  selector:
    matchLabels:
      app: %s
`, cfg.Name, Namespace, cfg.Name))
		}
	}

	return reply(http.StatusNotFound, "404: Not Found")
}

func (g *githubRouter) fetched(suffix string) bool {
	for _, p := range g.requested {
		if strings.HasSuffix(p, suffix) {
			return true
		}
	}
	return false
}

// deploymentManifestPath maps a deployment to the manifest the upgrade flow applies for
// it. Kept in the test so a rename in deploy/ has to be reflected deliberately.
func deploymentManifestPath(name string) string {
	switch name {
	case "migration-controller-manager":
		return "deploy/05controller-deployment.yaml"
	case "migration-vpwned-sdk":
		return "deploy/06vpwned-deployment.yaml"
	case "vjailbreak-ui":
		return "deploy/07ui-deployment.yaml"
	case "vjailbreak-ai":
		return "deploy/08vjailbreak-ai-deployment.yaml"
	}
	return ""
}

// serveGitHub points the package HTTP client at the router for one test.
func serveGitHub(t *testing.T, missing ...string) *githubRouter {
	t.Helper()

	router := &githubRouter{notFound: map[string]bool{}}
	for _, path := range missing {
		router.notFound[path] = true
	}

	original := httpClient
	httpClient = &http.Client{Transport: router}
	t.Cleanup(func() { httpClient = original })

	return router
}

// versionConfigClientset returns a clientset backed by a stub API server that serves the
// version-config ConfigMap, which is how Execute learns the current version.
func versionConfigClientset(t *testing.T, version string) *kubernetes.Clientset {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/configmaps/version-config") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(corev1.ConfigMap{
			TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
			ObjectMeta: metav1.ObjectMeta{Name: "version-config", Namespace: Namespace},
			Data:       map[string]string{"version": version},
		})
	}))
	t.Cleanup(server.Close)

	clientset, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	if err != nil {
		t.Fatalf("failed to build clientset: %v", err)
	}
	return clientset
}

// settledDeployments returns every vjailbreak deployment running one ready replica.
func settledDeployments() []client.Object {
	objs := make([]client.Object, 0, len(DeploymentConfigs))
	for _, cfg := range DeploymentConfigs {
		objs = append(objs, deployment(cfg.Name, 1, 1, 1))
	}
	return objs
}

// newSettlingClient stands in for the pieces of a real API server the upgrade flow needs:
//
//   - it emulates server-side apply, which the fake client rejects outright
//     (kubernetes/kubernetes#115598), by creating or replacing the applied object;
//   - it mirrors spec.replicas into status after every write, standing in for the
//     deployment controller. Without that the phase waits deadlock, because scaling to
//     zero would never be observed as scaled down and a rollout would never look
//     complete. Status is a subresource, so it has to go through Status().Update.
func newSettlingClient(t *testing.T, objs ...client.Object) client.WithWatch {
	t.Helper()

	// Manifests are applied as unstructured objects, so the written object is re-read as a
	// typed Deployment rather than type-asserted.
	settle := func(ctx context.Context, c client.WithWatch, obj client.Object) error {
		_, typed := obj.(*appsv1.Deployment)
		if !typed && obj.GetObjectKind().GroupVersionKind().Kind != "Deployment" {
			return nil
		}

		dep := &appsv1.Deployment{}
		if err := c.Get(ctx, client.ObjectKeyFromObject(obj), dep); err != nil {
			return err
		}

		replicas := int32(1)
		if dep.Spec.Replicas != nil {
			replicas = *dep.Spec.Replicas
		}
		dep.Status.Replicas = replicas
		dep.Status.ReadyReplicas = replicas
		dep.Status.UpdatedReplicas = replicas
		dep.Status.AvailableReplicas = replicas
		dep.Status.ObservedGeneration = dep.Generation
		dep.Status.Conditions = []appsv1.DeploymentCondition{
			{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue},
		}
		return c.Status().Update(ctx, dep)
	}

	// The applied object is the full intent, so create it or replace it wholesale. The
	// upgrade flow's own use of apply semantics is covered by the version_validator tests.
	emulateApply := func(ctx context.Context, c client.WithWatch, obj client.Object) error {
		existing, ok := obj.DeepCopyObject().(client.Object)
		if !ok {
			return fmt.Errorf("applied object %T is not a client.Object", obj)
		}

		err := c.Get(ctx, client.ObjectKeyFromObject(obj), existing)
		if kerrors.IsNotFound(err) {
			obj.SetResourceVersion("")
			return c.Create(ctx, obj)
		}
		if err != nil {
			return err
		}
		obj.SetResourceVersion(existing.GetResourceVersion())
		return c.Update(ctx, obj)
	}

	return interceptor.NewClient(newFakeClient(t, objs...), interceptor.Funcs{
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			if err := c.Update(ctx, obj, opts...); err != nil {
				return err
			}
			return settle(ctx, c, obj)
		},
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			if patch.Type() == types.ApplyPatchType {
				if err := emulateApply(ctx, c, obj); err != nil {
					return err
				}
			} else if err := c.Patch(ctx, obj, patch, opts...); err != nil {
				return err
			}
			return settle(ctx, c, obj)
		},
	})
}

// ---------------------------------------------------------------------------
// DeploymentConfigs / step accounting
// ---------------------------------------------------------------------------

// Every vjailbreak workload whose image is replaced during an upgrade must be listed
// here: DeploymentConfigs drives the post-upgrade stability check.
func TestDeploymentConfigsCoversEveryWorkload(t *testing.T) {
	want := map[string]struct {
		containerName string
		imagePrefix   string
	}{
		"migration-controller-manager": {"manager", "quay.io/platform9/vjailbreak-controller"},
		"migration-vpwned-sdk":         {"vpwned", "quay.io/platform9/vjailbreak-vpwned"},
		"vjailbreak-ui":                {"vjailbreak-ui-container", "quay.io/platform9/vjailbreak-ui"},
		"vjailbreak-ai":                {"vjailbreak-ai", "quay.io/platform9/vjailbreak-ai"},
	}

	if len(DeploymentConfigs) != len(want) {
		t.Fatalf("DeploymentConfigs has %d entries, want %d", len(DeploymentConfigs), len(want))
	}

	for _, cfg := range DeploymentConfigs {
		expected, ok := want[cfg.Name]
		if !ok {
			t.Errorf("unexpected deployment %q in DeploymentConfigs", cfg.Name)
			continue
		}
		if cfg.ContainerName != expected.containerName {
			t.Errorf("%s container = %q, want %q", cfg.Name, cfg.ContainerName, expected.containerName)
		}
		if cfg.ImagePrefix != expected.imagePrefix {
			t.Errorf("%s image prefix = %q, want %q", cfg.Name, cfg.ImagePrefix, expected.imagePrefix)
		}
		if cfg.Namespace != Namespace {
			t.Errorf("%s namespace = %q, want %q", cfg.Name, cfg.Namespace, Namespace)
		}
		if deploymentManifestPath(cfg.Name) == "" {
			t.Errorf("%s has no manifest in the upgrade flow", cfg.Name)
		}
	}
}

// The deployment phase reports one completed step per manifest it applies, so the
// declared total must move when a deployment is added.
func TestTotalUpgradeStepsMatchesDeploymentCount(t *testing.T) {
	const stepsBesidesDeploymentApplies = 8

	want := stepsBesidesDeploymentApplies + len(DeploymentConfigs)
	if TotalUpgradeSteps != want {
		t.Errorf("TotalUpgradeSteps = %d, want %d (%d other steps + one apply per deployment)",
			TotalUpgradeSteps, want, stepsBesidesDeploymentApplies)
	}
}

// ---------------------------------------------------------------------------
// getProgressConfigMapName / NewUpgradeExecutor
// ---------------------------------------------------------------------------

func TestGetProgressConfigMapName(t *testing.T) {
	tests := []struct {
		name   string
		jobUID string
		want   string
		about  string
	}{
		{
			name:   "job UID is appended",
			jobUID: "abc-123",
			want:   ProgressConfigMapNamePrefix + "-abc-123",
			about:  "concurrent jobs must not share a progress ConfigMap",
		},
		{
			name:   "no job UID falls back to the bare prefix",
			jobUID: "",
			want:   ProgressConfigMapNamePrefix,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("JOB_UID", tt.jobUID)

			if got := getProgressConfigMapName(); got != tt.want {
				t.Errorf("getProgressConfigMapName() = %q, want %q (%s)", got, tt.want, tt.about)
			}
		})
	}
}

// The executor is in-cluster only; outside a cluster it must fail rather than silently
// build a client pointed nowhere.
func TestNewUpgradeExecutorOutsideCluster(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")

	e, err := NewUpgradeExecutor()
	if err == nil {
		t.Fatal("NewUpgradeExecutor() error = nil, want an in-cluster config error")
	}
	if e != nil {
		t.Error("NewUpgradeExecutor() returned an executor alongside an error")
	}
}

// ---------------------------------------------------------------------------
// Progress state
// ---------------------------------------------------------------------------

func TestProgressStateHelpers(t *testing.T) {
	e := &UpgradeExecutor{progress: &UpgradeProgress{}}

	e.updateProgress("Applying CRDs", StatusDeploying, "")
	if e.progress.CurrentStep != "Applying CRDs" || e.progress.Status != StatusDeploying {
		t.Errorf("updateProgress() = %q/%q, want Applying CRDs/%s",
			e.progress.CurrentStep, e.progress.Status, StatusDeploying)
	}
	if e.progress.Error != "" {
		t.Error("an empty error message overwrote the stored error")
	}

	e.updateProgress("Failed", StatusFailed, "boom")
	if e.progress.Error != "boom" {
		t.Errorf("error = %q, want boom", e.progress.Error)
	}

	// An empty message must not clear an error already recorded.
	e.updateProgress("Rolling back", StatusRollingBack, "")
	if e.progress.Error != "boom" {
		t.Errorf("error = %q, want the original boom to survive", e.progress.Error)
	}

	e.incrementCompletedSteps()
	e.incrementCompletedSteps()
	if e.progress.CompletedSteps != 2 {
		t.Errorf("CompletedSteps = %d, want 2", e.progress.CompletedSteps)
	}

	e.setBackupID("20260812T000000Z")
	if e.progress.BackupID != "20260812T000000Z" {
		t.Errorf("BackupID = %q, want 20260812T000000Z", e.progress.BackupID)
	}

	e.setResult("success")
	if e.progress.Result != "success" {
		t.Errorf("Result = %q, want success", e.progress.Result)
	}

	now := time.Now()
	e.setEndTime(now)
	if e.progress.EndTime == nil || !e.progress.EndTime.Equal(now) {
		t.Errorf("EndTime = %v, want %v", e.progress.EndTime, now)
	}

	e.recordPhaseTiming("deployment_phase", 90*time.Second)
	if got := e.progress.PhaseTimings["deployment_phase"]; got != "1m30s" {
		t.Errorf("PhaseTimings[deployment_phase] = %q, want 1m30s", got)
	}
}

// Replica counts are stored so a rollback can restore the controller to the size it had,
// rather than assuming one.
func TestOriginalReplicasRoundTrip(t *testing.T) {
	e := &UpgradeExecutor{progress: &UpgradeProgress{}}

	if _, ok := e.getOriginalReplicas("migration-controller-manager"); ok {
		t.Error("getOriginalReplicas() reported a value before any was stored")
	}

	e.setOriginalReplicas("migration-controller-manager", 3)

	got, ok := e.getOriginalReplicas("migration-controller-manager")
	if !ok {
		t.Fatal("getOriginalReplicas() = not found after storing")
	}
	if got != 3 {
		t.Errorf("getOriginalReplicas() = %d, want 3", got)
	}
}

func TestSaveAndLoadProgress(t *testing.T) {
	t.Setenv("JOB_UID", "")

	e := &UpgradeExecutor{
		kubeClient: newFakeClient(t),
		progress: &UpgradeProgress{
			CurrentStep:   "Backing up resources",
			Status:        StatusInProgress,
			TargetVersion: "v0.4.9",
			TotalSteps:    TotalUpgradeSteps,
		},
	}

	e.saveProgress(context.Background())

	loaded, err := e.loadProgress(context.Background())
	if err != nil {
		t.Fatalf("loadProgress() error = %v, want nil", err)
	}
	if loaded.CurrentStep != "Backing up resources" || loaded.TargetVersion != "v0.4.9" {
		t.Errorf("loaded progress = %+v, want the saved step and target version", loaded)
	}

	// Saving again must update the existing ConfigMap, not fail on AlreadyExists.
	e.progress.CurrentStep = "Upgrading deployments"
	e.saveProgress(context.Background())

	loaded, err = e.loadProgress(context.Background())
	if err != nil {
		t.Fatalf("loadProgress() after update error = %v, want nil", err)
	}
	if loaded.CurrentStep != "Upgrading deployments" {
		t.Errorf("CurrentStep = %q, want the updated value", loaded.CurrentStep)
	}
}

func TestLoadProgressFailures(t *testing.T) {
	t.Setenv("JOB_UID", "")

	t.Run("missing ConfigMap", func(t *testing.T) {
		e := &UpgradeExecutor{kubeClient: newFakeClient(t)}

		if _, err := e.loadProgress(context.Background()); err == nil {
			t.Fatal("loadProgress() error = nil, want a not-found error")
		}
	})

	t.Run("ConfigMap without the progress key", func(t *testing.T) {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: getProgressConfigMapName(), Namespace: Namespace},
		}
		e := &UpgradeExecutor{kubeClient: newFakeClient(t, cm)}

		_, err := e.loadProgress(context.Background())
		if err == nil || !strings.Contains(err.Error(), "progress key not found") {
			t.Fatalf("loadProgress() error = %v, want it to report the missing key", err)
		}
	})

	t.Run("unparseable progress", func(t *testing.T) {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: getProgressConfigMapName(), Namespace: Namespace},
			Data:       map[string]string{"progress": "{not json"},
		}
		e := &UpgradeExecutor{kubeClient: newFakeClient(t, cm)}

		_, err := e.loadProgress(context.Background())
		if err == nil || !strings.Contains(err.Error(), "unmarshal") {
			t.Fatalf("loadProgress() error = %v, want an unmarshal error", err)
		}
	})
}

// ---------------------------------------------------------------------------
// scaleDeployment and the waits
// ---------------------------------------------------------------------------

func TestScaleDeployment(t *testing.T) {
	e := &UpgradeExecutor{
		kubeClient: newFakeClient(t, deployment("migration-controller-manager", 3, 3, 3)),
		progress:   &UpgradeProgress{},
	}

	original, err := e.scaleDeployment(context.Background(), DeploymentConfigs[0], 0, "Scaling down controller")
	if err != nil {
		t.Fatalf("scaleDeployment() error = %v, want nil", err)
	}
	if original != 3 {
		t.Errorf("scaleDeployment() = %d, want the previous 3 so rollback can restore it", original)
	}

	dep := &appsv1.Deployment{}
	key := client.ObjectKey{Name: "migration-controller-manager", Namespace: Namespace}
	if err := e.kubeClient.Get(context.Background(), key, dep); err != nil {
		t.Fatalf("failed to re-read deployment: %v", err)
	}
	if dep.Spec.Replicas == nil || *dep.Spec.Replicas != 0 {
		t.Errorf("replicas = %v, want 0", dep.Spec.Replicas)
	}
}

func TestScaleDeploymentMissingDeployment(t *testing.T) {
	e := &UpgradeExecutor{kubeClient: newFakeClient(t), progress: &UpgradeProgress{}}

	if _, err := e.scaleDeployment(context.Background(), DeploymentConfigs[0], 0, "step"); err == nil {
		t.Fatal("scaleDeployment() error = nil, want an error for a missing deployment")
	}
}

func TestWaitForDeploymentReady(t *testing.T) {
	fastPolling(t)

	available := func(dep *appsv1.Deployment) *appsv1.Deployment {
		dep.Status.Conditions = []appsv1.DeploymentCondition{
			{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue},
		}
		return dep
	}

	tests := []struct {
		name    string
		objects []client.Object
		cancel  bool
		wantErr string
	}{
		{
			name:    "ready deployment returns",
			objects: []client.Object{available(deployment("vjailbreak-ai", 1, 1, 1))},
		},
		{
			name:    "missing deployment is reported",
			wantErr: "not found",
		},
		{
			name:    "not enough ready replicas times out",
			objects: []client.Object{available(deployment("vjailbreak-ai", 2, 1, 2))},
			wantErr: "not ready within timeout",
		},
		{
			name:    "missing Available condition times out",
			objects: []client.Object{deployment("vjailbreak-ai", 1, 1, 1)},
			wantErr: "not ready within timeout",
		},
		{
			name:    "cancelled context aborts the wait",
			objects: []client.Object{available(deployment("vjailbreak-ai", 2, 0, 2))},
			cancel:  true,
			wantErr: "context canceled",
		},
	}

	aiConfig := DeploymentConfig{Name: "vjailbreak-ai", Namespace: Namespace}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tt.cancel {
				cancel()
			}

			e := &UpgradeExecutor{kubeClient: newFakeClient(t, tt.objects...)}
			err := e.waitForDeploymentReady(ctx, aiConfig)

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("waitForDeploymentReady() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("waitForDeploymentReady() error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestWaitForDeploymentScaledDown(t *testing.T) {
	fastPolling(t)

	tests := []struct {
		name    string
		objects []client.Object
		cancel  bool
		wantErr string
	}{
		{
			name:    "no running replicas returns",
			objects: []client.Object{deployment("migration-controller-manager", 0, 0, 0)},
		},
		{
			name: "a deleted deployment counts as scaled down",
		},
		{
			name:    "running replicas time out",
			objects: []client.Object{deployment("migration-controller-manager", 1, 1, 1)},
			wantErr: "not scaled down within timeout",
		},
		{
			name:    "cancelled context aborts the wait",
			objects: []client.Object{deployment("migration-controller-manager", 1, 1, 1)},
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

			e := &UpgradeExecutor{kubeClient: newFakeClient(t, tt.objects...)}
			err := e.waitForDeploymentScaledDown(ctx, DeploymentConfigs[0])

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("waitForDeploymentScaledDown() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("waitForDeploymentScaledDown() error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Phases
// ---------------------------------------------------------------------------

func TestRunPreUpgradePhase(t *testing.T) {
	t.Run("empty target version is rejected", func(t *testing.T) {
		e := &UpgradeExecutor{progress: &UpgradeProgress{}}

		err := e.runPreUpgradePhase(context.Background(), "", false)
		if err == nil || !strings.Contains(err.Error(), "targetVersion cannot be empty") {
			t.Fatalf("runPreUpgradePhase() error = %v, want the empty-version error", err)
		}
	})

	t.Run("a clean appliance passes", func(t *testing.T) {
		e := &UpgradeExecutor{
			kubeClient:    newFakeClient(t),
			dynamicClient: newFakeDynamic(t),
			progress:      &UpgradeProgress{},
		}

		if err := e.runPreUpgradePhase(context.Background(), "v0.4.9", false); err != nil {
			t.Fatalf("runPreUpgradePhase() error = %v, want nil", err)
		}
		if e.progress.CompletedSteps != 1 {
			t.Errorf("CompletedSteps = %d, want 1", e.progress.CompletedSteps)
		}
	})

	t.Run("leftover resources fail when auto-cleanup is off", func(t *testing.T) {
		e := &UpgradeExecutor{
			kubeClient:    newFakeClient(t),
			dynamicClient: newFakeDynamic(t, newCR("MigrationPlan", "plan-1")),
			progress:      &UpgradeProgress{},
		}

		err := e.runPreUpgradePhase(context.Background(), "v0.4.9", false)
		if err == nil || !strings.Contains(err.Error(), "pre-upgrade checks failed") {
			t.Fatalf("runPreUpgradePhase() error = %v, want the checks to fail", err)
		}
		if !strings.Contains(err.Error(), "migrations=false") {
			t.Errorf("error = %q, want it to name which check failed", err)
		}
	})

	t.Run("resources that survive auto-cleanup still fail", func(t *testing.T) {
		// The cleanup path builds its own dynamic client from the rest config, so the
		// seeded MigrationPlan is still present on the re-check.
		e := &UpgradeExecutor{
			kubeClient:    newFakeClient(t),
			dynamicClient: newFakeDynamic(t, newCR("MigrationPlan", "plan-1")),
			config:        &rest.Config{Host: "http://127.0.0.1:1"},
			progress:      &UpgradeProgress{},
		}

		err := e.runPreUpgradePhase(context.Background(), "v0.4.9", true)
		if err == nil || !strings.Contains(err.Error(), "still failing after cleanup") {
			t.Fatalf("runPreUpgradePhase() error = %v, want the post-cleanup failure", err)
		}
	})
}

func TestRunBackupAndCRDPhase(t *testing.T) {
	t.Run("backs up, applies CRDs and returns a backup ID", func(t *testing.T) {
		router := serveGitHub(t)

		e := &UpgradeExecutor{
			kubeClient: newSettlingClient(t, settledDeployments()...),
			progress:   &UpgradeProgress{},
		}

		backupID, err := e.runBackupAndCRDPhase(context.Background(), "v0.4.9")
		if err != nil {
			t.Fatalf("runBackupAndCRDPhase() error = %v, want nil", err)
		}
		if backupID == "" {
			t.Error("backupID is empty; rollback would have nothing to select")
		}
		if e.progress.BackupID != backupID {
			t.Errorf("progress BackupID = %q, want %q", e.progress.BackupID, backupID)
		}
		if !router.fetched("deploy/00crds.yaml") {
			t.Error("CRDs were never fetched")
		}

		cm := &corev1.ConfigMap{}
		key := client.ObjectKey{Name: "backup-deploy-vjailbreak-ai", Namespace: Namespace}
		if err := e.kubeClient.Get(context.Background(), key, cm); err != nil {
			t.Errorf("AI deployment was not backed up: %v", err)
		}
	})

	t.Run("a CRD fetch failure fails the phase", func(t *testing.T) {
		serveGitHub(t, "deploy/00crds.yaml")

		e := &UpgradeExecutor{kubeClient: newFakeClient(t), progress: &UpgradeProgress{}}

		if _, err := e.runBackupAndCRDPhase(context.Background(), "v0.4.9"); err == nil {
			t.Fatal("runBackupAndCRDPhase() error = nil, want the CRD failure surfaced")
		}
	})
}

func TestRunConfigMapPhase(t *testing.T) {
	t.Run("writes both ConfigMaps at the target version", func(t *testing.T) {
		serveGitHub(t)

		e := &UpgradeExecutor{kubeClient: newFakeClient(t), progress: &UpgradeProgress{}}

		if err := e.runConfigMapPhase(context.Background(), "v0.4.9"); err != nil {
			t.Fatalf("runConfigMapPhase() error = %v, want nil", err)
		}

		for name, key := range map[string]string{"version-config": "version", "vjailbreak-settings": "VERSION"} {
			cm := &corev1.ConfigMap{}
			if err := e.kubeClient.Get(context.Background(),
				client.ObjectKey{Name: name, Namespace: Namespace}, cm); err != nil {
				t.Errorf("%s was not written: %v", name, err)
				continue
			}
			if cm.Data[key] != "v0.4.9" {
				t.Errorf("%s[%s] = %q, want v0.4.9", name, key, cm.Data[key])
			}
		}
	})

	// ConfigMap updates are advisory: the upgrade continues even when GitHub does not
	// serve them, so the phase must not return an error.
	t.Run("fetch failures are warnings", func(t *testing.T) {
		serveGitHub(t,
			"image_builder/configs/version-config.yaml",
			"image_builder/configs/vjailbreak-settings.yaml")

		e := &UpgradeExecutor{kubeClient: newFakeClient(t), progress: &UpgradeProgress{}}

		if err := e.runConfigMapPhase(context.Background(), "v0.4.9"); err != nil {
			t.Fatalf("runConfigMapPhase() error = %v, want nil", err)
		}
	})
}

func TestRunDeploymentPhase(t *testing.T) {
	fastPolling(t)

	t.Run("applies every deployment manifest and completes", func(t *testing.T) {
		router := serveGitHub(t)

		e := &UpgradeExecutor{
			kubeClient: newSettlingClient(t, settledDeployments()...),
			progress:   &UpgradeProgress{TotalSteps: TotalUpgradeSteps},
		}

		if err := e.runDeploymentPhase(context.Background(), "v0.4.9", "backup-1"); err != nil {
			t.Fatalf("runDeploymentPhase() error = %v, want nil", err)
		}

		for _, cfg := range DeploymentConfigs {
			path := deploymentManifestPath(cfg.Name)
			if !router.fetched(path) {
				t.Errorf("%s was never applied; %s would keep running the old image", path, cfg.Name)
			}
		}
		if e.progress.Status != StatusCompleted {
			t.Errorf("status = %q, want %q", e.progress.Status, StatusCompleted)
		}
		if e.progress.Result != "success" {
			t.Errorf("result = %q, want success", e.progress.Result)
		}
		if e.progress.CompletedSteps > TotalUpgradeSteps {
			t.Errorf("CompletedSteps = %d, want at most %d", e.progress.CompletedSteps, TotalUpgradeSteps)
		}
		if _, ok := e.progress.PhaseTimings["deployment_phase"]; !ok {
			t.Error("deployment_phase timing was not recorded")
		}
	})

	t.Run("the controller replica count is remembered", func(t *testing.T) {
		serveGitHub(t)

		objs := settledDeployments()
		objs[0] = deployment("migration-controller-manager", 2, 2, 2)

		e := &UpgradeExecutor{
			kubeClient: newSettlingClient(t, objs...),
			progress:   &UpgradeProgress{TotalSteps: TotalUpgradeSteps},
		}

		_ = e.runDeploymentPhase(context.Background(), "v0.4.9", "backup-1")

		got, ok := e.getOriginalReplicas("migration-controller-manager")
		if !ok || got != 2 {
			t.Errorf("stored original replicas = %d (found=%t), want 2", got, ok)
		}
	})

	t.Run("a missing deployment manifest fails the phase", func(t *testing.T) {
		serveGitHub(t, deploymentManifestPath("vjailbreak-ai"))

		e := &UpgradeExecutor{
			kubeClient: newSettlingClient(t, settledDeployments()...),
			progress:   &UpgradeProgress{TotalSteps: TotalUpgradeSteps},
		}

		err := e.runDeploymentPhase(context.Background(), "v0.4.9", "backup-1")
		if err == nil {
			t.Fatal("runDeploymentPhase() error = nil, want the missing AI manifest to fail")
		}
		if !strings.Contains(err.Error(), "AI deployment") {
			t.Errorf("error = %q, want it to name the AI deployment", err)
		}
	})

}

// ---------------------------------------------------------------------------
// Execute
// ---------------------------------------------------------------------------

// A restarted job must not run a second upgrade on top of one already in flight.
func TestExecuteAbortsWhenAnUpgradeIsInProgress(t *testing.T) {
	t.Setenv("JOB_UID", "")

	inFlight, err := json.Marshal(UpgradeProgress{Status: StatusDeploying, TargetVersion: "v0.4.9"})
	if err != nil {
		t.Fatalf("failed to marshal progress: %v", err)
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: getProgressConfigMapName(), Namespace: Namespace},
		Data:       map[string]string{"progress": string(inFlight)},
	}

	e := &UpgradeExecutor{kubeClient: newFakeClient(t, cm)}

	err = e.Execute(context.Background(), "v0.5.0", false)
	if err == nil || !strings.Contains(err.Error(), "existing upgrade in progress") {
		t.Fatalf("Execute() error = %v, want the duplicate-upgrade guard to fire", err)
	}
}

// A pending record is the server handing work over, not a concurrent job, so the upgrade
// proceeds - here it proceeds far enough to fail on the empty target version.
func TestExecuteProceedsPastPendingProgress(t *testing.T) {
	t.Setenv("JOB_UID", "")
	serveGitHub(t)

	pending, err := json.Marshal(UpgradeProgress{Status: StatusPending, TargetVersion: "v0.4.9"})
	if err != nil {
		t.Fatalf("failed to marshal progress: %v", err)
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: getProgressConfigMapName(), Namespace: Namespace},
		Data:       map[string]string{"progress": string(pending)},
	}

	e := &UpgradeExecutor{
		kubeClient:    newFakeClient(t, cm),
		clientset:     versionConfigClientset(t, "v0.4.8"),
		dynamicClient: newFakeDynamic(t),
	}

	err = e.Execute(context.Background(), "", false)
	if err == nil || !strings.Contains(err.Error(), "targetVersion cannot be empty") {
		t.Fatalf("Execute() error = %v, want it to reach the pre-upgrade phase", err)
	}
	if e.progress.PreviousVersion != "v0.4.8" {
		t.Errorf("PreviousVersion = %q, want the version read from version-config", e.progress.PreviousVersion)
	}
}

func TestExecuteRunsEveryPhase(t *testing.T) {
	fastPolling(t)
	t.Setenv("JOB_UID", "")
	router := serveGitHub(t)

	e := &UpgradeExecutor{
		kubeClient:    newSettlingClient(t, settledDeployments()...),
		clientset:     versionConfigClientset(t, "v0.4.8"),
		dynamicClient: newFakeDynamic(t),
		config:        &rest.Config{},
	}

	if err := e.Execute(context.Background(), "v0.4.9", false); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	for _, path := range []string{
		"deploy/00crds.yaml",
		"image_builder/configs/version-config.yaml",
		"image_builder/configs/vjailbreak-settings.yaml",
	} {
		if !router.fetched(path) {
			t.Errorf("%s was never fetched", path)
		}
	}
	for _, cfg := range DeploymentConfigs {
		if !router.fetched(deploymentManifestPath(cfg.Name)) {
			t.Errorf("%s was never applied", cfg.Name)
		}
	}
	if e.progress.Status != StatusCompleted {
		t.Errorf("status = %q, want %q", e.progress.Status, StatusCompleted)
	}
	if e.progress.PreviousVersion != "v0.4.8" || e.progress.TargetVersion != "v0.4.9" {
		t.Errorf("versions = %q -> %q, want v0.4.8 -> v0.4.9",
			e.progress.PreviousVersion, e.progress.TargetVersion)
	}
}

// An unreadable version-config must not stop the upgrade; the previous version is
// recorded as unknown, which only limits manifest-driven rollback.
func TestExecuteToleratesUnknownCurrentVersion(t *testing.T) {
	fastPolling(t)
	t.Setenv("JOB_UID", "")
	serveGitHub(t)

	clientset, err := kubernetes.NewForConfig(&rest.Config{Host: "http://127.0.0.1:1"})
	if err != nil {
		t.Fatalf("failed to build clientset: %v", err)
	}

	e := &UpgradeExecutor{
		kubeClient:    newSettlingClient(t, settledDeployments()...),
		clientset:     clientset,
		dynamicClient: newFakeDynamic(t),
		config:        &rest.Config{},
	}

	if err := e.Execute(context.Background(), "v0.4.9", false); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if e.progress.PreviousVersion != "unknown" {
		t.Errorf("PreviousVersion = %q, want unknown", e.progress.PreviousVersion)
	}
}

// ---------------------------------------------------------------------------
// Failure handling and rollback
// ---------------------------------------------------------------------------

func TestHandleFailureRollsBack(t *testing.T) {
	t.Setenv("JOB_UID", "")
	serveGitHub(t)

	e := &UpgradeExecutor{
		kubeClient: newFakeClient(t,
			backupConfigMap("backup-cm-version-config", "id-1",
				"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: version-config\n  namespace: migration-system\ndata:\n  version: v0.4.8\n", 0),
		),
		progress: &UpgradeProgress{PreviousVersion: "v0.4.8", BackupID: "id-1"},
	}

	phaseErr := fmt.Errorf("deployment phase blew up")
	err := e.handleFailure(context.Background(), phaseErr, "Deployment phase failed")
	if err != phaseErr {
		t.Fatalf("handleFailure() = %v, want the original error returned unchanged", err)
	}

	if e.progress.Result != "failure" {
		t.Errorf("result = %q, want failure", e.progress.Result)
	}
	if e.progress.Status != StatusRolledBack {
		t.Errorf("status = %q, want %q after a successful rollback", e.progress.Status, StatusRolledBack)
	}
	if e.progress.Error != phaseErr.Error() {
		t.Errorf("recorded error = %q, want %q", e.progress.Error, phaseErr.Error())
	}
	if e.progress.EndTime == nil {
		t.Error("EndTime was not set")
	}
}

// Without a previous version there is nothing to fetch manifests from, so a rollback
// needs a snapshot; with neither it must fail loudly rather than no-op.
func TestExecuteRollbackRequiresVersionOrBackup(t *testing.T) {
	t.Setenv("JOB_UID", "")

	e := &UpgradeExecutor{kubeClient: newFakeClient(t)}

	err := e.ExecuteRollback(context.Background(), "", "v0.4.9", "")
	if err == nil || !strings.Contains(err.Error(), "rollback requires") {
		t.Fatalf("ExecuteRollback() error = %v, want the missing-input error", err)
	}
	if e.progress.Status != StatusRollbackFailed {
		t.Errorf("status = %q, want %q", e.progress.Status, StatusRollbackFailed)
	}
}

func TestExecuteRollbackSnapshotBased(t *testing.T) {
	t.Setenv("JOB_UID", "")

	e := &UpgradeExecutor{
		kubeClient: newFakeClient(t,
			backupConfigMap("backup-cm-version-config", "id-1",
				"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: version-config\n  namespace: migration-system\ndata:\n  version: v0.4.8\n", 0),
		),
	}

	if err := e.ExecuteRollback(context.Background(), "unknown", "v0.4.9", "id-1"); err != nil {
		t.Fatalf("ExecuteRollback() error = %v, want nil", err)
	}
	if e.progress.Status != StatusRolledBack {
		t.Errorf("status = %q, want %q", e.progress.Status, StatusRolledBack)
	}
	if e.progress.CompletedSteps != TotalRollbackSteps {
		t.Errorf("CompletedSteps = %d, want all %d steps", e.progress.CompletedSteps, TotalRollbackSteps)
	}

	cm := &corev1.ConfigMap{}
	if err := e.kubeClient.Get(context.Background(),
		client.ObjectKey{Name: "version-config", Namespace: Namespace}, cm); err != nil {
		t.Fatalf("version-config was not restored from the snapshot: %v", err)
	}
	if cm.Data["version"] != "v0.4.8" {
		t.Errorf("restored version = %q, want v0.4.8", cm.Data["version"])
	}
}

// A snapshot-based rollback with no snapshots for the given ID must fail rather than
// report success.
func TestExecuteRollbackSnapshotMissing(t *testing.T) {
	t.Setenv("JOB_UID", "")

	e := &UpgradeExecutor{kubeClient: newFakeClient(t)}

	err := e.ExecuteRollback(context.Background(), "unknown", "v0.4.9", "id-missing")
	if err == nil {
		t.Fatal("ExecuteRollback() error = nil, want a missing-backup error")
	}
	if e.progress.Status != StatusRollbackFailed {
		t.Errorf("status = %q, want %q", e.progress.Status, StatusRollbackFailed)
	}
}

func TestExecuteRollbackManifestDriven(t *testing.T) {
	fastPolling(t)
	t.Setenv("JOB_UID", "")
	router := serveGitHub(t)

	e := &UpgradeExecutor{kubeClient: newSettlingClient(t, settledDeployments()...)}

	if err := e.ExecuteRollback(context.Background(), "v0.4.8", "v0.4.9", ""); err != nil {
		t.Fatalf("ExecuteRollback() error = %v, want nil", err)
	}

	for _, cfg := range DeploymentConfigs {
		path := deploymentManifestPath(cfg.Name)
		if !router.fetched(path) {
			t.Errorf("%s was not restored during rollback; %s would stay on the new image",
				path, cfg.Name)
		}
	}
	if !router.fetched("deploy/00crds.yaml") {
		t.Error("CRDs were not restored to the previous version")
	}
	if e.progress.Status != StatusRolledBack {
		t.Errorf("status = %q, want %q", e.progress.Status, StatusRolledBack)
	}
}

// The controller is the one deployment a rollback cannot proceed without.
func TestExecuteRollbackFailsWhenControllerManifestIsMissing(t *testing.T) {
	fastPolling(t)
	t.Setenv("JOB_UID", "")
	serveGitHub(t, deploymentManifestPath("migration-controller-manager"))

	e := &UpgradeExecutor{kubeClient: newSettlingClient(t, settledDeployments()...)}

	if err := e.ExecuteRollback(context.Background(), "v0.4.8", "v0.4.9", ""); err == nil {
		t.Fatal("ExecuteRollback() error = nil, want the controller failure surfaced")
	}
	if e.progress.Status != StatusRollbackFailed {
		t.Errorf("status = %q, want %q", e.progress.Status, StatusRollbackFailed)
	}
}

// When a deployment will not come up, the wait logs pod-level diagnostics; that path
// must survive pods in every unhealthy shape rather than panicking on a nil state.
func TestWaitForDeploymentReadyLogsPodDiagnostics(t *testing.T) {
	fastPolling(t)

	dep := deployment("vjailbreak-ai", 1, 0, 1)
	dep.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"app": "vjailbreak-ai"}}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vjailbreak-ai-abc123",
			Namespace: Namespace,
			Labels:    map[string]string{"app": "vjailbreak-ai"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "vjailbreak-ai",
					Ready: false,
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff", Message: "manifest unknown"},
					},
					LastTerminationState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{Reason: "Error", ExitCode: 1},
					},
					RestartCount: 3,
				},
				{
					Name:  "sidecar",
					Ready: false,
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{Reason: "OOMKilled", ExitCode: 137},
					},
				},
			},
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionFalse, Reason: "ContainersNotReady"},
			},
		},
	}

	e := &UpgradeExecutor{kubeClient: newFakeClient(t, dep, pod)}

	err := e.waitForDeploymentReady(context.Background(),
		DeploymentConfig{Name: "vjailbreak-ai", Namespace: Namespace})
	if err == nil || !strings.Contains(err.Error(), "not ready within timeout") {
		t.Fatalf("waitForDeploymentReady() error = %v, want a timeout", err)
	}
}

func TestSaveProgressWithoutProgressIsANoOp(t *testing.T) {
	t.Setenv("JOB_UID", "")

	e := &UpgradeExecutor{kubeClient: newFakeClient(t)}
	e.saveProgress(context.Background())

	cm := &corev1.ConfigMap{}
	err := e.kubeClient.Get(context.Background(),
		client.ObjectKey{Name: getProgressConfigMapName(), Namespace: Namespace}, cm)
	if err == nil {
		t.Error("a progress ConfigMap was written with no progress to report")
	}
}

// Progress reporting is best-effort telemetry: a write failure must not abort the
// upgrade, only be logged.
func TestSaveProgressToleratesWriteFailure(t *testing.T) {
	t.Setenv("JOB_UID", "")

	kubeClient := interceptor.NewClient(newFakeClient(t), interceptor.Funcs{
		Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
			return errors.New("api server down")
		},
	})

	e := &UpgradeExecutor{kubeClient: kubeClient, progress: &UpgradeProgress{CurrentStep: "step"}}
	e.saveProgress(context.Background()) // must not panic
}

// An upgrade that fails before any backup exists cannot roll back, and that has to be
// reported as rollback_failed rather than rolled_back.
func TestHandleFailureReportsRollbackFailure(t *testing.T) {
	t.Setenv("JOB_UID", "")
	serveGitHub(t)

	e := &UpgradeExecutor{
		kubeClient: newFakeClient(t),
		progress:   &UpgradeProgress{PreviousVersion: "v0.4.8", BackupID: "id-missing"},
	}

	if err := e.handleFailure(context.Background(), errors.New("boom"), "phase failed"); err == nil {
		t.Fatal("handleFailure() error = nil, want the original error")
	}
	if e.progress.Status != StatusRollbackFailed {
		t.Errorf("status = %q, want %q when no snapshot exists to restore", e.progress.Status, StatusRollbackFailed)
	}
}

// A failing CRD phase must abort the upgrade and roll back rather than continue to the
// deployments.
func TestExecuteRollsBackWhenCRDPhaseFails(t *testing.T) {
	fastPolling(t)
	t.Setenv("JOB_UID", "")
	router := serveGitHub(t, "deploy/00crds.yaml")

	e := &UpgradeExecutor{
		kubeClient:    newSettlingClient(t, settledDeployments()...),
		clientset:     versionConfigClientset(t, "v0.4.8"),
		dynamicClient: newFakeDynamic(t),
		config:        &rest.Config{},
	}

	if err := e.Execute(context.Background(), "v0.4.9", false); err == nil {
		t.Fatal("Execute() error = nil, want the CRD failure to abort the upgrade")
	}
	for _, cfg := range DeploymentConfigs {
		if router.fetched(deploymentManifestPath(cfg.Name)) {
			t.Errorf("%s was applied after the CRD phase failed", cfg.Name)
		}
	}
}

// A failing controller manifest must abort the deployment phase; the controller is the
// one deployment the upgrade cannot proceed without.
func TestRunDeploymentPhaseFailsWhenControllerManifestIsMissing(t *testing.T) {
	fastPolling(t)
	serveGitHub(t, deploymentManifestPath("migration-controller-manager"))

	e := &UpgradeExecutor{
		kubeClient: newSettlingClient(t, settledDeployments()...),
		progress:   &UpgradeProgress{TotalSteps: TotalUpgradeSteps},
	}

	err := e.runDeploymentPhase(context.Background(), "v0.4.9", "backup-1")
	if err == nil || !strings.Contains(err.Error(), "controller deployment") {
		t.Fatalf("runDeploymentPhase() error = %v, want the controller apply failure", err)
	}
}

// Rollback runs as its own Job, so the replica count the upgrade recorded has to come
// back from the progress ConfigMap rather than from memory.
func TestExecuteRollbackUsesStoredOriginalReplicas(t *testing.T) {
	fastPolling(t)
	t.Setenv("JOB_UID", "")

	serveGitHub(t)

	stored, err := json.Marshal(UpgradeProgress{
		Status:           StatusFailed,
		BackupID:         "id-1",
		OriginalReplicas: map[string]int32{"migration-controller-manager": 3},
	})
	if err != nil {
		t.Fatalf("failed to marshal progress: %v", err)
	}

	objs := settledDeployments()
	// The failed upgrade left the controller scaled to zero.
	objs[0] = deployment("migration-controller-manager", 0, 0, 0)
	objs = append(objs, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: getProgressConfigMapName(), Namespace: Namespace},
		Data:       map[string]string{"progress": string(stored)},
	})

	e := &UpgradeExecutor{kubeClient: newSettlingClient(t, objs...)}

	if err := e.ExecuteRollback(context.Background(), "v0.4.8", "v0.4.9", ""); err != nil {
		t.Fatalf("ExecuteRollback() error = %v, want nil", err)
	}

	dep := &appsv1.Deployment{}
	if err := e.kubeClient.Get(context.Background(),
		client.ObjectKey{Name: "migration-controller-manager", Namespace: Namespace}, dep); err != nil {
		t.Fatalf("failed to read the controller: %v", err)
	}
	if dep.Spec.Replicas == nil || *dep.Spec.Replicas != 3 {
		t.Errorf("controller replicas = %v, want the stored 3", dep.Spec.Replicas)
	}
}

// With no stored count and a controller already at zero, restoring zero would report a
// successful rollback while leaving the appliance without a controller.
func TestExecuteRollbackNeverRestoresZeroReplicas(t *testing.T) {
	fastPolling(t)
	t.Setenv("JOB_UID", "")

	serveGitHub(t)

	objs := settledDeployments()
	objs[0] = deployment("migration-controller-manager", 0, 0, 0)

	e := &UpgradeExecutor{kubeClient: newSettlingClient(t, objs...)}

	if err := e.ExecuteRollback(context.Background(), "v0.4.8", "v0.4.9", ""); err != nil {
		t.Fatalf("ExecuteRollback() error = %v, want nil", err)
	}

	dep := &appsv1.Deployment{}
	if err := e.kubeClient.Get(context.Background(),
		client.ObjectKey{Name: "migration-controller-manager", Namespace: Namespace}, dep); err != nil {
		t.Fatalf("failed to read the controller: %v", err)
	}
	if dep.Spec.Replicas == nil || *dep.Spec.Replicas < 1 {
		t.Errorf("controller replicas = %v, want at least 1 so migrations keep reconciling",
			dep.Spec.Replicas)
	}
	if e.progress.Status != StatusRolledBack {
		t.Errorf("status = %q, want %q", e.progress.Status, StatusRolledBack)
	}
}
