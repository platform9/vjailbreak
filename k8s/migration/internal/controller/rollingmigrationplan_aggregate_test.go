package controller

import (
	"context"
	"testing"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	pkgscope "github.com/platform9/vjailbreak/k8s/migration/pkg/scope"
	constants "github.com/platform9/vjailbreak/pkg/common/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func aggregateTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := vjailbreakv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add vjailbreak scheme: %v", err)
	}
	return scheme
}

func newRMP(name, ns string, vmMigrationPlans []string) *vjailbreakv1alpha1.RollingMigrationPlan {
	return &vjailbreakv1alpha1.RollingMigrationPlan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: vjailbreakv1alpha1.RollingMigrationPlanSpec{
			VMMigrationPlans: vmMigrationPlans,
		},
	}
}

func newMigrationPlan(name, ns, rmpName string, status corev1.PodPhase) *vjailbreakv1alpha1.MigrationPlan {
	return &vjailbreakv1alpha1.MigrationPlan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels: map[string]string{
				constants.RollingMigrationPlanLabel: rmpName,
			},
		},
		Status: vjailbreakv1alpha1.MigrationPlanStatus{
			MigrationStatus: status,
		},
	}
}

// TestAggregateAndUpdateMigrationPlanStatuses_NoPlansFound verifies that when no
// MigrationPlans exist (by label), the function returns (false, nil) without error.
func TestAggregateAndUpdateMigrationPlanStatuses_NoPlansFound(t *testing.T) {
	ctx := context.Background()
	scheme := aggregateTestScheme(t)
	ns := "migration-system"

	rmp := newRMP("rmp-1", ns, nil)
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(rmp).
		WithStatusSubresource(&vjailbreakv1alpha1.RollingMigrationPlan{}).
		Build()

	r := &RollingMigrationPlanReconciler{Client: fakeClient, Scheme: scheme}
	scope := &pkgscope.RollingMigrationPlanScope{
		Client:               fakeClient,
		RollingMigrationPlan: rmp,
	}

	updated, err := r.aggregateAndUpdateMigrationPlanStatuses(ctx, scope)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated {
		t.Error("expected updated=false when no MigrationPlans found")
	}
}

// TestAggregateAndUpdateMigrationPlanStatuses_BackfillsSpecFromLabel verifies that when
// spec.vMMigrationPlans is empty but MigrationPlans exist with the matching label,
// spec.vMMigrationPlans is backfilled and persisted to the API server.
func TestAggregateAndUpdateMigrationPlanStatuses_BackfillsSpecFromLabel(t *testing.T) {
	ctx := context.Background()
	scheme := aggregateTestScheme(t)
	ns := "migration-system"
	rmpName := "rmp-backfill"

	rmp := newRMP(rmpName, ns, nil) // empty VMMigrationPlans
	mp1 := newMigrationPlan("mp-1", ns, rmpName, corev1.PodSucceeded)
	mp2 := newMigrationPlan("mp-2", ns, rmpName, corev1.PodSucceeded)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(rmp, mp1, mp2).
		WithStatusSubresource(&vjailbreakv1alpha1.RollingMigrationPlan{}, &vjailbreakv1alpha1.MigrationPlan{}).
		Build()

	r := &RollingMigrationPlanReconciler{Client: fakeClient, Scheme: scheme}
	scope := &pkgscope.RollingMigrationPlanScope{
		Client:               fakeClient,
		RollingMigrationPlan: rmp,
	}

	_, err := r.aggregateAndUpdateMigrationPlanStatuses(ctx, scope)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Spec must be backfilled in the API server
	persisted := &vjailbreakv1alpha1.RollingMigrationPlan{}
	if err := fakeClient.Get(ctx, types.NamespacedName{Name: rmpName, Namespace: ns}, persisted); err != nil {
		t.Fatalf("get RMP failed: %v", err)
	}
	if len(persisted.Spec.VMMigrationPlans) != 2 {
		t.Errorf("expected 2 backfilled VMMigrationPlans, got %d: %v", len(persisted.Spec.VMMigrationPlans), persisted.Spec.VMMigrationPlans)
	}
}

// TestAggregateAndUpdateMigrationPlanStatuses_AllSucceeded verifies that when all
// MigrationPlans are Succeeded, the RollingMigrationPlan status phase is set to Succeeded.
func TestAggregateAndUpdateMigrationPlanStatuses_AllSucceeded(t *testing.T) {
	ctx := context.Background()
	scheme := aggregateTestScheme(t)
	ns := "migration-system"
	rmpName := "rmp-all-success"

	rmp := newRMP(rmpName, ns, []string{"mp-a"})
	mp := newMigrationPlan("mp-a", ns, rmpName, corev1.PodSucceeded)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(rmp, mp).
		WithStatusSubresource(&vjailbreakv1alpha1.RollingMigrationPlan{}, &vjailbreakv1alpha1.MigrationPlan{}).
		Build()

	r := &RollingMigrationPlanReconciler{Client: fakeClient, Scheme: scheme}
	scope := &pkgscope.RollingMigrationPlanScope{
		Client:               fakeClient,
		RollingMigrationPlan: rmp,
	}

	updated, err := r.aggregateAndUpdateMigrationPlanStatuses(ctx, scope)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated {
		t.Error("expected updated=true when status changes")
	}
	if scope.RollingMigrationPlan.Status.VMMigrationsPhase != string(vjailbreakv1alpha1.RollingMigrationPlanPhaseSucceeded) {
		t.Errorf("expected phase Succeeded, got %q", scope.RollingMigrationPlan.Status.VMMigrationsPhase)
	}
}

// TestAggregateAndUpdateMigrationPlanStatuses_SomeFailed verifies that when any
// MigrationPlan has Failed status, the overall phase becomes Failed.
func TestAggregateAndUpdateMigrationPlanStatuses_SomeFailed(t *testing.T) {
	ctx := context.Background()
	scheme := aggregateTestScheme(t)
	ns := "migration-system"
	rmpName := "rmp-some-failed"

	rmp := newRMP(rmpName, ns, []string{"mp-ok", "mp-fail"})
	mpOK := newMigrationPlan("mp-ok", ns, rmpName, corev1.PodSucceeded)
	mpFail := newMigrationPlan("mp-fail", ns, rmpName, corev1.PodFailed)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(rmp, mpOK, mpFail).
		WithStatusSubresource(&vjailbreakv1alpha1.RollingMigrationPlan{}, &vjailbreakv1alpha1.MigrationPlan{}).
		Build()

	r := &RollingMigrationPlanReconciler{Client: fakeClient, Scheme: scheme}
	scope := &pkgscope.RollingMigrationPlanScope{
		Client:               fakeClient,
		RollingMigrationPlan: rmp,
	}

	updated, err := r.aggregateAndUpdateMigrationPlanStatuses(ctx, scope)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated {
		t.Error("expected updated=true")
	}
	if scope.RollingMigrationPlan.Status.VMMigrationsPhase != string(vjailbreakv1alpha1.RollingMigrationPlanPhaseFailed) {
		t.Errorf("expected phase Failed, got %q", scope.RollingMigrationPlan.Status.VMMigrationsPhase)
	}
}

// TestAggregateAndUpdateMigrationPlanStatuses_RunningPlan verifies that when any
// MigrationPlan is Running, the overall phase is Running.
func TestAggregateAndUpdateMigrationPlanStatuses_RunningPlan(t *testing.T) {
	ctx := context.Background()
	scheme := aggregateTestScheme(t)
	ns := "migration-system"
	rmpName := "rmp-running"

	rmp := newRMP(rmpName, ns, []string{"mp-run"})
	mp := newMigrationPlan("mp-run", ns, rmpName, corev1.PodRunning)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(rmp, mp).
		WithStatusSubresource(&vjailbreakv1alpha1.RollingMigrationPlan{}, &vjailbreakv1alpha1.MigrationPlan{}).
		Build()

	r := &RollingMigrationPlanReconciler{Client: fakeClient, Scheme: scheme}
	scope := &pkgscope.RollingMigrationPlanScope{
		Client:               fakeClient,
		RollingMigrationPlan: rmp,
	}

	_, err := r.aggregateAndUpdateMigrationPlanStatuses(ctx, scope)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if scope.RollingMigrationPlan.Status.VMMigrationsPhase != string(vjailbreakv1alpha1.RollingMigrationPlanPhaseRunning) {
		t.Errorf("expected phase Running, got %q", scope.RollingMigrationPlan.Status.VMMigrationsPhase)
	}
}

// TestAggregateAndUpdateMigrationPlanStatuses_IsolatesLabels verifies that MigrationPlans
// belonging to a different RollingMigrationPlan are not counted.
func TestAggregateAndUpdateMigrationPlanStatuses_IsolatesLabels(t *testing.T) {
	ctx := context.Background()
	scheme := aggregateTestScheme(t)
	ns := "migration-system"
	rmpName := "rmp-isolated"

	rmp := newRMP(rmpName, ns, nil)
	// This MigrationPlan belongs to a different RMP — should not be counted.
	mpOther := newMigrationPlan("mp-other", ns, "other-rmp", corev1.PodFailed)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(rmp, mpOther).
		WithStatusSubresource(&vjailbreakv1alpha1.RollingMigrationPlan{}, &vjailbreakv1alpha1.MigrationPlan{}).
		Build()

	r := &RollingMigrationPlanReconciler{Client: fakeClient, Scheme: scheme}
	scope := &pkgscope.RollingMigrationPlanScope{
		Client:               fakeClient,
		RollingMigrationPlan: rmp,
	}

	updated, err := r.aggregateAndUpdateMigrationPlanStatuses(ctx, scope)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No plans for this RMP, so no status update and no phase change.
	if updated {
		t.Error("expected updated=false: no plans for this RMP")
	}
}
