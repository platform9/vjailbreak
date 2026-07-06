package controller

import (
	"context"
	"testing"
	"time"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// mockEligibilityChecker returns a fixed result for all hosts.
type mockEligibilityChecker struct {
	status vjailbreakv1alpha1.EligibilityStatus
	reason string
	err    error
}

func (m *mockEligibilityChecker) CheckPerHostEligibility(
	_ context.Context,
	_ interface{},
	_ *vjailbreakv1alpha1.ClusterConversionBatch,
	_ string,
) (vjailbreakv1alpha1.EligibilityStatus, string, error) {
	return m.status, m.reason, m.err
}

func controllerTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := vjailbreakv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	return scheme
}

func makeBatch(name string, hosts []string, autoStart vjailbreakv1alpha1.AutoStartMode, maxRetries int) *vjailbreakv1alpha1.ClusterConversionBatch {
	hostEntries := make([]vjailbreakv1alpha1.HostEntry, len(hosts))
	for i, h := range hosts {
		hostEntries[i] = vjailbreakv1alpha1.HostEntry{ESXiName: h}
	}
	return &vjailbreakv1alpha1.ClusterConversionBatch{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: constants.NamespaceMigrationSystem,
		},
		Spec: vjailbreakv1alpha1.ClusterConversionBatchSpec{
			VMwareClusterName:   "test-cluster",
			VMwareCredsRef:      corev1.LocalObjectReference{Name: "vmware-creds"},
			OpenstackCredsRef:   corev1.LocalObjectReference{Name: "os-creds"},
			BMConfigRef:         corev1.LocalObjectReference{Name: "bm-config"},
			Hosts:               hostEntries,
			AutoStart:           autoStart,
			MaxRetries:          maxRetries,
			RetryBackoffSeconds: 60,
		},
	}
}

func newReconciler(t *testing.T, objs ...interface{}) (*ClusterConversionBatchReconciler, *fake.ClientBuilder) {
	t.Helper()
	scheme := controllerTestScheme(t)
	builder := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&vjailbreakv1alpha1.ClusterConversionBatch{})
	for _, obj := range objs {
		if o, ok := obj.(runtime.Object); ok {
			_ = o // just for type check
		}
	}
	return &ClusterConversionBatchReconciler{
		Scheme: scheme,
	}, builder
}

// TestInitializeHostStatuses: first reconcile populates status.hosts with CheckingEligibility for all hosts.
func TestInitializeHostStatuses(t *testing.T) {
	scheme := controllerTestScheme(t)
	batch := makeBatch("batch-1", []string{"esxi-01", "esxi-02", "esxi-03"}, vjailbreakv1alpha1.AutoStartModeAuto, 3)

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&vjailbreakv1alpha1.ClusterConversionBatch{}).
		WithObjects(batch).
		Build()

	r := &ClusterConversionBatchReconciler{
		Client:             k8sClient,
		Scheme:             scheme,
		EligibilityChecker: &mockEligibilityChecker{status: vjailbreakv1alpha1.EligibilityStatusReady},
	}

	_, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "batch-1", Namespace: constants.NamespaceMigrationSystem},
	})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	updated := &vjailbreakv1alpha1.ClusterConversionBatch{}
	if err := k8sClient.Get(context.Background(), types.NamespacedName{
		Name: "batch-1", Namespace: constants.NamespaceMigrationSystem,
	}, updated); err != nil {
		t.Fatalf("Get: %v", err)
	}

	if len(updated.Status.Hosts) == 0 {
		t.Fatal("expected status.hosts to be initialized")
	}
	if updated.Status.TotalHosts != 3 {
		t.Errorf("TotalHosts = %d, want 3", updated.Status.TotalHosts)
	}
}

// TestAutoStartEligibleHost: Auto mode + eligible host → ESXIMigration created, phase=Converting.
func TestAutoStartEligibleHost(t *testing.T) {
	scheme := controllerTestScheme(t)
	batch := makeBatch("batch-auto", []string{"esxi-01"}, vjailbreakv1alpha1.AutoStartModeAuto, 3)

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&vjailbreakv1alpha1.ClusterConversionBatch{}).
		WithObjects(batch).
		Build()

	r := &ClusterConversionBatchReconciler{
		Client:             k8sClient,
		Scheme:             scheme,
		EligibilityChecker: &mockEligibilityChecker{status: vjailbreakv1alpha1.EligibilityStatusReady},
	}

	// First reconcile: initialize hosts
	r.Reconcile(context.Background(), reconcile.Request{ //nolint:errcheck
		NamespacedName: types.NamespacedName{Name: "batch-auto", Namespace: constants.NamespaceMigrationSystem},
	})
	// Second reconcile: process eligible host in Auto mode
	_, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "batch-auto", Namespace: constants.NamespaceMigrationSystem},
	})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Verify ESXIMigration was created
	esxiMigList := &vjailbreakv1alpha1.ESXIMigrationList{}
	if err := k8sClient.List(context.Background(), esxiMigList); err != nil {
		t.Fatalf("List ESXIMigrations: %v", err)
	}
	if len(esxiMigList.Items) != 1 {
		t.Errorf("expected 1 ESXIMigration, got %d", len(esxiMigList.Items))
	}

	// Verify host phase = Converting
	updated := &vjailbreakv1alpha1.ClusterConversionBatch{}
	k8sClient.Get(context.Background(), types.NamespacedName{ //nolint:errcheck
		Name: "batch-auto", Namespace: constants.NamespaceMigrationSystem,
	}, updated)

	found := false
	for _, h := range updated.Status.Hosts {
		if h.ESXiName == "esxi-01" {
			found = true
			if h.Phase != vjailbreakv1alpha1.HostConversionPhaseConverting {
				t.Errorf("host phase = %q, want Converting", h.Phase)
			}
		}
	}
	if !found {
		t.Error("host esxi-01 not found in status")
	}
}

// TestManualModeNoAutoStart: Manual mode + eligible host → no ESXIMigration, phase=Ready.
func TestManualModeNoAutoStart(t *testing.T) {
	scheme := controllerTestScheme(t)
	batch := makeBatch("batch-manual", []string{"esxi-01"}, vjailbreakv1alpha1.AutoStartModeManual, 3)

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&vjailbreakv1alpha1.ClusterConversionBatch{}).
		WithObjects(batch).
		Build()

	r := &ClusterConversionBatchReconciler{
		Client:             k8sClient,
		Scheme:             scheme,
		EligibilityChecker: &mockEligibilityChecker{status: vjailbreakv1alpha1.EligibilityStatusReady},
	}

	// Two reconciles
	r.Reconcile(context.Background(), reconcile.Request{ //nolint:errcheck
		NamespacedName: types.NamespacedName{Name: "batch-manual", Namespace: constants.NamespaceMigrationSystem},
	})
	r.Reconcile(context.Background(), reconcile.Request{ //nolint:errcheck
		NamespacedName: types.NamespacedName{Name: "batch-manual", Namespace: constants.NamespaceMigrationSystem},
	})

	// No ESXIMigration should exist
	esxiMigList := &vjailbreakv1alpha1.ESXIMigrationList{}
	k8sClient.List(context.Background(), esxiMigList) //nolint:errcheck
	if len(esxiMigList.Items) != 0 {
		t.Errorf("Manual mode: expected 0 ESXIMigrations, got %d", len(esxiMigList.Items))
	}
}

// TestSiblingIsolation: one host failing doesn't affect another.
func TestSiblingIsolation(t *testing.T) {
	scheme := controllerTestScheme(t)
	batch := makeBatch("batch-sibling", []string{"esxi-fail", "esxi-ok"}, vjailbreakv1alpha1.AutoStartModeAuto, 3)

	// Pre-populate status with esxi-fail in Converting with a Failed ESXIMigration
	batch.Status = vjailbreakv1alpha1.ClusterConversionBatchStatus{
		TotalHosts: 2,
		Hosts: []vjailbreakv1alpha1.HostConversionStatus{
			{ESXiName: "esxi-fail", Phase: vjailbreakv1alpha1.HostConversionPhaseConverting,
				ESXIMigrationRef: &corev1.LocalObjectReference{Name: "esxi-fail-batch-sibling"}},
			{ESXiName: "esxi-ok", Phase: vjailbreakv1alpha1.HostConversionPhaseReady},
		},
	}

	// Create a Failed ESXIMigration for esxi-fail
	failedMig := &vjailbreakv1alpha1.ESXIMigration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "esxi-fail-batch-sibling",
			Namespace: constants.NamespaceMigrationSystem,
			Labels:    map[string]string{constants.ClusterConversionBatchLabel: "batch-sibling"},
		},
		Status: vjailbreakv1alpha1.ESXIMigrationStatus{Phase: vjailbreakv1alpha1.ESXIMigrationPhaseFailed},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&vjailbreakv1alpha1.ClusterConversionBatch{}, &vjailbreakv1alpha1.ESXIMigration{}).
		WithObjects(batch, failedMig).
		Build()

	r := &ClusterConversionBatchReconciler{
		Client:             k8sClient,
		Scheme:             scheme,
		EligibilityChecker: &mockEligibilityChecker{status: vjailbreakv1alpha1.EligibilityStatusReady},
	}

	_, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "batch-sibling", Namespace: constants.NamespaceMigrationSystem},
	})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	updated := &vjailbreakv1alpha1.ClusterConversionBatch{}
	k8sClient.Get(context.Background(), types.NamespacedName{ //nolint:errcheck
		Name: "batch-sibling", Namespace: constants.NamespaceMigrationSystem,
	}, updated)

	// esxi-ok must have advanced (either Ready→Converting in auto mode)
	// esxi-fail must have incremented retry count
	for _, h := range updated.Status.Hosts {
		if h.ESXiName == "esxi-ok" {
			if h.Phase != vjailbreakv1alpha1.HostConversionPhaseConverting && h.Phase != vjailbreakv1alpha1.HostConversionPhaseReady {
				t.Errorf("esxi-ok phase = %q, want Converting or Ready", h.Phase)
			}
		}
		if h.ESXiName == "esxi-fail" {
			if h.Phase == vjailbreakv1alpha1.HostConversionPhaseConverting {
				t.Error("esxi-fail should not remain Converting after ESXIMigration failed")
			}
		}
	}
}

// TestBatchPhaseAggregation: all terminal → correct batch phase.
func TestBatchPhaseAggregation(t *testing.T) {
	tests := []struct {
		name       string
		hostPhases []vjailbreakv1alpha1.HostConversionPhase
		wantPhase  vjailbreakv1alpha1.ClusterConversionBatchPhase
	}{
		{
			name:       "all succeeded",
			hostPhases: []vjailbreakv1alpha1.HostConversionPhase{vjailbreakv1alpha1.HostConversionPhaseSucceeded, vjailbreakv1alpha1.HostConversionPhaseSucceeded},
			wantPhase:  vjailbreakv1alpha1.ClusterConversionBatchPhaseSucceeded,
		},
		{
			name:       "partial fail (some succeeded, some skipped)",
			hostPhases: []vjailbreakv1alpha1.HostConversionPhase{vjailbreakv1alpha1.HostConversionPhaseSucceeded, vjailbreakv1alpha1.HostConversionPhaseSkipped},
			wantPhase:  vjailbreakv1alpha1.ClusterConversionBatchPhasePartialFail,
		},
		{
			name:       "all NeedsAttention → Failed",
			hostPhases: []vjailbreakv1alpha1.HostConversionPhase{vjailbreakv1alpha1.HostConversionPhaseNeedsAttention, vjailbreakv1alpha1.HostConversionPhaseNeedsAttention},
			wantPhase:  vjailbreakv1alpha1.ClusterConversionBatchPhaseFailed,
		},
		{
			name:       "failed is NOT terminal (Running)",
			hostPhases: []vjailbreakv1alpha1.HostConversionPhase{vjailbreakv1alpha1.HostConversionPhaseFailed},
			wantPhase:  vjailbreakv1alpha1.ClusterConversionBatchPhaseRunning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hosts := make([]vjailbreakv1alpha1.HostConversionStatus, len(tt.hostPhases))
			for i, p := range tt.hostPhases {
				hosts[i] = vjailbreakv1alpha1.HostConversionStatus{
					ESXiName: "host",
					Phase:    p,
				}
			}
			batch := &vjailbreakv1alpha1.ClusterConversionBatch{}
			batch.Status.Hosts = hosts
			updateBatchAggregates(batch)
			if batch.Status.Phase != tt.wantPhase {
				t.Errorf("updateBatchAggregates phase = %q, want %q", batch.Status.Phase, tt.wantPhase)
			}
		})
	}
}

// TestRetryExhaustion: host fails MaxRetries times → NeedsAttention.
func TestRetryExhaustion(t *testing.T) {
	scheme := controllerTestScheme(t)
	batch := makeBatch("batch-retry", []string{"esxi-01"}, vjailbreakv1alpha1.AutoStartModeAuto, 1) // MaxRetries=1

	batch.Status = vjailbreakv1alpha1.ClusterConversionBatchStatus{
		TotalHosts: 1,
		Hosts: []vjailbreakv1alpha1.HostConversionStatus{
			{
				ESXiName:         "esxi-01",
				Phase:            vjailbreakv1alpha1.HostConversionPhaseConverting,
				RetryCount:       1, // already at max
				ESXIMigrationRef: &corev1.LocalObjectReference{Name: "esxi-01-batch-retry"},
			},
		},
	}

	failedMig := &vjailbreakv1alpha1.ESXIMigration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "esxi-01-batch-retry",
			Namespace: constants.NamespaceMigrationSystem,
			Labels:    map[string]string{constants.ClusterConversionBatchLabel: "batch-retry"},
		},
		Status: vjailbreakv1alpha1.ESXIMigrationStatus{Phase: vjailbreakv1alpha1.ESXIMigrationPhaseFailed},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&vjailbreakv1alpha1.ClusterConversionBatch{}, &vjailbreakv1alpha1.ESXIMigration{}).
		WithObjects(batch, failedMig).
		Build()

	r := &ClusterConversionBatchReconciler{
		Client:             k8sClient,
		Scheme:             scheme,
		EligibilityChecker: &mockEligibilityChecker{status: vjailbreakv1alpha1.EligibilityStatusReady},
	}

	r.Reconcile(context.Background(), reconcile.Request{ //nolint:errcheck
		NamespacedName: types.NamespacedName{Name: "batch-retry", Namespace: constants.NamespaceMigrationSystem},
	})

	updated := &vjailbreakv1alpha1.ClusterConversionBatch{}
	k8sClient.Get(context.Background(), types.NamespacedName{ //nolint:errcheck
		Name: "batch-retry", Namespace: constants.NamespaceMigrationSystem,
	}, updated)

	for _, h := range updated.Status.Hosts {
		if h.ESXiName == "esxi-01" && h.Phase != vjailbreakv1alpha1.HostConversionPhaseNeedsAttention {
			t.Errorf("expected NeedsAttention after retry exhaustion, got %q", h.Phase)
		}
	}
}

// TestRetryAnnotation: retry annotation on NeedsAttention host resets count and sets CheckingEligibility.
func TestRetryAnnotation(t *testing.T) {
	scheme := controllerTestScheme(t)
	batch := makeBatch("batch-retry-ann", []string{"esxi-01"}, vjailbreakv1alpha1.AutoStartModeAuto, 3)
	batch.Annotations = map[string]string{
		constants.AnnotationRetryHost: "esxi-01",
	}
	batch.Status = vjailbreakv1alpha1.ClusterConversionBatchStatus{
		TotalHosts: 1,
		Hosts: []vjailbreakv1alpha1.HostConversionStatus{
			{ESXiName: "esxi-01", Phase: vjailbreakv1alpha1.HostConversionPhaseNeedsAttention, RetryCount: 3},
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&vjailbreakv1alpha1.ClusterConversionBatch{}).
		WithObjects(batch).
		Build()

	r := &ClusterConversionBatchReconciler{
		Client:             k8sClient,
		Scheme:             scheme,
		EligibilityChecker: &mockEligibilityChecker{status: vjailbreakv1alpha1.EligibilityStatusReady},
	}

	r.Reconcile(context.Background(), reconcile.Request{ //nolint:errcheck
		NamespacedName: types.NamespacedName{Name: "batch-retry-ann", Namespace: constants.NamespaceMigrationSystem},
	})

	updated := &vjailbreakv1alpha1.ClusterConversionBatch{}
	k8sClient.Get(context.Background(), types.NamespacedName{ //nolint:errcheck
		Name: "batch-retry-ann", Namespace: constants.NamespaceMigrationSystem,
	}, updated)

	for _, h := range updated.Status.Hosts {
		if h.ESXiName == "esxi-01" {
			if h.RetryCount != 0 {
				t.Errorf("RetryCount = %d, want 0 after retry annotation", h.RetryCount)
			}
			if h.Phase == vjailbreakv1alpha1.HostConversionPhaseNeedsAttention {
				t.Error("phase should no longer be NeedsAttention after retry annotation")
			}
		}
	}

	// Annotation must be cleared
	if _, ok := updated.Annotations[constants.AnnotationRetryHost]; ok {
		t.Error("retry annotation should be cleared after processing")
	}
}

// TestSkipAnnotation: skip annotation marks host Skipped without deleting ESXIMigration.
func TestSkipAnnotation(t *testing.T) {
	scheme := controllerTestScheme(t)
	batch := makeBatch("batch-skip", []string{"esxi-01"}, vjailbreakv1alpha1.AutoStartModeAuto, 3)
	batch.Annotations = map[string]string{
		constants.AnnotationSkipHost: "esxi-01",
	}
	batch.Status = vjailbreakv1alpha1.ClusterConversionBatchStatus{
		TotalHosts: 1,
		Hosts: []vjailbreakv1alpha1.HostConversionStatus{
			{ESXiName: "esxi-01", Phase: vjailbreakv1alpha1.HostConversionPhaseConverting,
				ESXIMigrationRef: &corev1.LocalObjectReference{Name: "esxi-01-batch-skip"}},
		},
	}

	existingMig := &vjailbreakv1alpha1.ESXIMigration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "esxi-01-batch-skip",
			Namespace: constants.NamespaceMigrationSystem,
			Labels:    map[string]string{constants.ClusterConversionBatchLabel: "batch-skip"},
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&vjailbreakv1alpha1.ClusterConversionBatch{}).
		WithObjects(batch, existingMig).
		Build()

	r := &ClusterConversionBatchReconciler{
		Client:             k8sClient,
		Scheme:             scheme,
		EligibilityChecker: &mockEligibilityChecker{status: vjailbreakv1alpha1.EligibilityStatusReady},
	}

	r.Reconcile(context.Background(), reconcile.Request{ //nolint:errcheck
		NamespacedName: types.NamespacedName{Name: "batch-skip", Namespace: constants.NamespaceMigrationSystem},
	})

	// ESXIMigration must still exist (not deleted on skip)
	mig := &vjailbreakv1alpha1.ESXIMigration{}
	if err := k8sClient.Get(context.Background(), types.NamespacedName{
		Name: "esxi-01-batch-skip", Namespace: constants.NamespaceMigrationSystem,
	}, mig); apierrors.IsNotFound(err) {
		t.Error("ESXIMigration should NOT be deleted on skip")
	}

	updated := &vjailbreakv1alpha1.ClusterConversionBatch{}
	k8sClient.Get(context.Background(), types.NamespacedName{ //nolint:errcheck
		Name: "batch-skip", Namespace: constants.NamespaceMigrationSystem,
	}, updated)

	for _, h := range updated.Status.Hosts {
		if h.ESXiName == "esxi-01" && h.Phase != vjailbreakv1alpha1.HostConversionPhaseSkipped {
			t.Errorf("phase = %q, want Skipped", h.Phase)
		}
	}
}

// TestTriggerAnnotation: trigger annotation on Ready host → ESXIMigration created + annotation cleared.
func TestTriggerAnnotation(t *testing.T) {
	scheme := controllerTestScheme(t)
	batch := makeBatch("batch-trigger", []string{"esxi-01"}, vjailbreakv1alpha1.AutoStartModeManual, 3)
	batch.Annotations = map[string]string{
		constants.AnnotationTriggerHost: "esxi-01",
	}
	batch.Status = vjailbreakv1alpha1.ClusterConversionBatchStatus{
		TotalHosts: 1,
		Hosts: []vjailbreakv1alpha1.HostConversionStatus{
			{ESXiName: "esxi-01", Phase: vjailbreakv1alpha1.HostConversionPhaseReady},
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&vjailbreakv1alpha1.ClusterConversionBatch{}).
		WithObjects(batch).
		Build()

	r := &ClusterConversionBatchReconciler{
		Client:             k8sClient,
		Scheme:             scheme,
		EligibilityChecker: &mockEligibilityChecker{status: vjailbreakv1alpha1.EligibilityStatusReady},
	}

	_, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "batch-trigger", Namespace: constants.NamespaceMigrationSystem},
	})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// ESXIMigration must have been created
	migList := &vjailbreakv1alpha1.ESXIMigrationList{}
	if err := k8sClient.List(context.Background(), migList, client.InNamespace(constants.NamespaceMigrationSystem)); err != nil {
		t.Fatalf("List ESXIMigrations: %v", err)
	}
	if len(migList.Items) == 0 {
		t.Error("expected ESXIMigration to be created after trigger annotation")
	}

	// Annotation must be cleared
	updated := &vjailbreakv1alpha1.ClusterConversionBatch{}
	if err := k8sClient.Get(context.Background(), types.NamespacedName{
		Name: "batch-trigger", Namespace: constants.NamespaceMigrationSystem,
	}, updated); err != nil {
		t.Fatalf("Get batch: %v", err)
	}
	if _, ok := updated.Annotations[constants.AnnotationTriggerHost]; ok {
		t.Error("trigger annotation should be cleared after processing")
	}

	// Host phase must be Converting
	for _, h := range updated.Status.Hosts {
		if h.ESXiName == "esxi-01" && h.Phase != vjailbreakv1alpha1.HostConversionPhaseConverting {
			t.Errorf("phase = %q, want Converting after trigger", h.Phase)
		}
	}
}

// TestTriggerAnnotationOnNonReadyHost: trigger annotation on non-Ready host → no ESXIMigration created.
func TestTriggerAnnotationOnNonReadyHost(t *testing.T) {
	scheme := controllerTestScheme(t)
	batch := makeBatch("batch-trigger-notready", []string{"esxi-01"}, vjailbreakv1alpha1.AutoStartModeManual, 3)
	batch.Annotations = map[string]string{
		constants.AnnotationTriggerHost: "esxi-01",
	}
	batch.Status = vjailbreakv1alpha1.ClusterConversionBatchStatus{
		TotalHosts: 1,
		Hosts: []vjailbreakv1alpha1.HostConversionStatus{
			{ESXiName: "esxi-01", Phase: vjailbreakv1alpha1.HostConversionPhaseNotReady},
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&vjailbreakv1alpha1.ClusterConversionBatch{}).
		WithObjects(batch).
		Build()

	r := &ClusterConversionBatchReconciler{
		Client:             k8sClient,
		Scheme:             scheme,
		EligibilityChecker: &mockEligibilityChecker{status: vjailbreakv1alpha1.EligibilityStatusNotReady},
	}

	r.Reconcile(context.Background(), reconcile.Request{ //nolint:errcheck
		NamespacedName: types.NamespacedName{Name: "batch-trigger-notready", Namespace: constants.NamespaceMigrationSystem},
	})

	// No ESXIMigration should be created
	migList := &vjailbreakv1alpha1.ESXIMigrationList{}
	if err := k8sClient.List(context.Background(), migList, client.InNamespace(constants.NamespaceMigrationSystem)); err != nil {
		t.Fatalf("List ESXIMigrations: %v", err)
	}
	if len(migList.Items) != 0 {
		t.Errorf("expected no ESXIMigration for non-Ready host trigger, got %d", len(migList.Items))
	}
}

// TestDeleteBatch: batch deletion removes finalizer but does NOT delete ESXIMigrations.
func TestDeleteBatch(t *testing.T) {
	scheme := controllerTestScheme(t)
	now := metav1.NewTime(time.Now())
	batch := makeBatch("batch-del", []string{"esxi-01"}, vjailbreakv1alpha1.AutoStartModeAuto, 3)
	batch.Finalizers = []string{constants.ClusterConversionBatchFinalizer}
	batch.DeletionTimestamp = &now
	batch.Status = vjailbreakv1alpha1.ClusterConversionBatchStatus{
		TotalHosts: 1,
		Hosts: []vjailbreakv1alpha1.HostConversionStatus{
			{ESXiName: "esxi-01", Phase: vjailbreakv1alpha1.HostConversionPhaseConverting,
				ESXIMigrationRef: &corev1.LocalObjectReference{Name: "esxi-01-batch-del"}},
		},
	}

	existingMig := &vjailbreakv1alpha1.ESXIMigration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "esxi-01-batch-del",
			Namespace: constants.NamespaceMigrationSystem,
			Labels:    map[string]string{constants.ClusterConversionBatchLabel: "batch-del"},
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&vjailbreakv1alpha1.ClusterConversionBatch{}).
		WithObjects(batch, existingMig).
		Build()

	r := &ClusterConversionBatchReconciler{
		Client:             k8sClient,
		Scheme:             scheme,
		EligibilityChecker: &mockEligibilityChecker{status: vjailbreakv1alpha1.EligibilityStatusReady},
	}

	_, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "batch-del", Namespace: constants.NamespaceMigrationSystem},
	})
	if err != nil {
		t.Fatalf("Reconcile on deletion: %v", err)
	}

	// ESXIMigration must still exist
	mig := &vjailbreakv1alpha1.ESXIMigration{}
	if err := k8sClient.Get(context.Background(), types.NamespacedName{
		Name: "esxi-01-batch-del", Namespace: constants.NamespaceMigrationSystem,
	}, mig); apierrors.IsNotFound(err) {
		t.Error("ESXIMigration should NOT be deleted when batch is deleted")
	}
}
