package utils

import (
	"context"
	"testing"
	"time"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestComputeRetryBackoff verifies exponential backoff formula: base * 2^(retryCount-1)
func TestComputeRetryBackoff(t *testing.T) {
	tests := []struct {
		name        string
		baseSeconds int
		retryCount  int
		want        time.Duration
	}{
		{"first retry", 60, 1, 60 * time.Second},
		{"second retry", 60, 2, 120 * time.Second},
		{"third retry", 60, 3, 240 * time.Second},
		{"custom base first", 30, 1, 30 * time.Second},
		{"custom base second", 30, 2, 60 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeRetryBackoff(tt.baseSeconds, tt.retryCount)
			if got != tt.want {
				t.Errorf("ComputeRetryBackoff(%d, %d) = %v, want %v", tt.baseSeconds, tt.retryCount, got, tt.want)
			}
		})
	}
}

// TestProcessBatchAnnotations verifies annotation parsing and removal.
func TestProcessBatchAnnotations(t *testing.T) {
	t.Run("trigger annotation", func(t *testing.T) {
		batch := &vjailbreakv1alpha1.ClusterConversionBatch{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					constants.AnnotationTriggerHost: "esxi-01.example.com",
				},
			},
		}
		actions := ProcessBatchAnnotations(batch)
		if len(actions) != 1 {
			t.Fatalf("expected 1 action, got %d", len(actions))
		}
		if actions[0].Type != BatchActionTypeTrigger {
			t.Errorf("expected trigger action, got %q", actions[0].Type)
		}
		if actions[0].ESXiName != "esxi-01.example.com" {
			t.Errorf("expected esxi name %q, got %q", "esxi-01.example.com", actions[0].ESXiName)
		}
		// Annotation must be removed
		if _, ok := batch.Annotations[constants.AnnotationTriggerHost]; ok {
			t.Error("trigger annotation should have been removed from batch")
		}
	})

	t.Run("retry annotation", func(t *testing.T) {
		batch := &vjailbreakv1alpha1.ClusterConversionBatch{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					constants.AnnotationRetryHost: "esxi-02.example.com",
				},
			},
		}
		actions := ProcessBatchAnnotations(batch)
		if len(actions) != 1 {
			t.Fatalf("expected 1 action, got %d", len(actions))
		}
		if actions[0].Type != BatchActionTypeRetry {
			t.Errorf("expected retry action, got %q", actions[0].Type)
		}
		if _, ok := batch.Annotations[constants.AnnotationRetryHost]; ok {
			t.Error("retry annotation should have been removed")
		}
	})

	t.Run("skip annotation", func(t *testing.T) {
		batch := &vjailbreakv1alpha1.ClusterConversionBatch{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					constants.AnnotationSkipHost: "esxi-03.example.com",
				},
			},
		}
		actions := ProcessBatchAnnotations(batch)
		if len(actions) != 1 {
			t.Fatalf("expected 1 action, got %d", len(actions))
		}
		if actions[0].Type != BatchActionTypeSkip {
			t.Errorf("expected skip action, got %q", actions[0].Type)
		}
	})

	t.Run("no annotations", func(t *testing.T) {
		batch := &vjailbreakv1alpha1.ClusterConversionBatch{}
		actions := ProcessBatchAnnotations(batch)
		if len(actions) != 0 {
			t.Errorf("expected 0 actions, got %d", len(actions))
		}
	})

	t.Run("nil annotations map", func(t *testing.T) {
		batch := &vjailbreakv1alpha1.ClusterConversionBatch{
			ObjectMeta: metav1.ObjectMeta{Annotations: nil},
		}
		actions := ProcessBatchAnnotations(batch)
		if len(actions) != 0 {
			t.Errorf("expected 0 actions for nil annotations, got %d", len(actions))
		}
	})
}

// TestCreateESXIMigrationForBatch verifies labels, spec fields, and absence of owner reference.
func TestCreateESXIMigrationForBatch(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)

	batch := &vjailbreakv1alpha1.ClusterConversionBatch{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-batch",
			Namespace: constants.NamespaceMigrationSystem,
		},
		Spec: vjailbreakv1alpha1.ClusterConversionBatchSpec{
			VMwareCredsRef:    corev1.LocalObjectReference{Name: "vmware-creds"},
			OpenstackCredsRef: corev1.LocalObjectReference{Name: "os-creds"},
			BMConfigRef:       corev1.LocalObjectReference{Name: "bm-config"},
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	esxiName := "esxi-01.example.com"
	migration, err := CreateESXIMigrationForBatch(ctx, k8sClient, batch, esxiName)
	if err != nil {
		t.Fatalf("CreateESXIMigrationForBatch failed: %v", err)
	}

	// Verify cluster-conversion-batch label is set
	if got := migration.Labels[constants.ClusterConversionBatchLabel]; got != "my-batch" {
		t.Errorf("ClusterConversionBatchLabel = %q, want %q", got, "my-batch")
	}
	// Verify vmwarecreds label is set
	if got := migration.Labels[constants.VMwareCredsLabel]; got != "vmware-creds" {
		t.Errorf("VMwareCredsLabel = %q, want %q", got, "vmware-creds")
	}
	// Verify spec.bmConfigRef is set
	if migration.Spec.BMConfigRef == nil || migration.Spec.BMConfigRef.Name != "bm-config" {
		t.Errorf("spec.BMConfigRef = %v, want {Name: bm-config}", migration.Spec.BMConfigRef)
	}
	// Verify clusterConversionBatchRef is set
	if migration.Spec.ClusterConversionBatchRef == nil || migration.Spec.ClusterConversionBatchRef.Name != "my-batch" {
		t.Errorf("spec.ClusterConversionBatchRef = %v, want {Name: my-batch}", migration.Spec.ClusterConversionBatchRef)
	}
	// NO owner references (intentional — prevents GC cascade on batch delete)
	if len(migration.OwnerReferences) != 0 {
		t.Errorf("expected no ownerReferences, got %v", migration.OwnerReferences)
	}
	// Verify rollingMigrationPlanRef is NOT set
	if migration.Spec.RollingMigrationPlanRef.Name != "" {
		t.Errorf("RollingMigrationPlanRef should be empty, got %q", migration.Spec.RollingMigrationPlanRef.Name)
	}
}
