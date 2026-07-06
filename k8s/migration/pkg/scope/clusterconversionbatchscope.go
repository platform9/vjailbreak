package scope

import (
	"context"
	"reflect"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClusterConversionBatchScopeParams defines the input parameters used to create a new ClusterConversionBatchScope.
type ClusterConversionBatchScopeParams struct {
	Logger                 logr.Logger
	Client                 client.Client
	ClusterConversionBatch *vjailbreakv1alpha1.ClusterConversionBatch
}

// NewClusterConversionBatchScope creates a new ClusterConversionBatchScope from the supplied parameters.
func NewClusterConversionBatchScope(params ClusterConversionBatchScopeParams) (*ClusterConversionBatchScope, error) {
	if reflect.DeepEqual(params.Logger, logr.Logger{}) {
		params.Logger = ctrl.Log
	}

	return &ClusterConversionBatchScope{
		Logger:                 params.Logger,
		Client:                 params.Client,
		ClusterConversionBatch: params.ClusterConversionBatch,
	}, nil
}

// ClusterConversionBatchScope holds the context for a single ClusterConversionBatch reconcile iteration.
type ClusterConversionBatchScope struct {
	logr.Logger
	Client                 client.Client
	ClusterConversionBatch *vjailbreakv1alpha1.ClusterConversionBatch
}

// Close persists ClusterConversionBatch spec/metadata changes (not status — use Status().Update() for that).
func (s *ClusterConversionBatchScope) Close() error {
	return s.Client.Update(context.TODO(), s.ClusterConversionBatch, &client.UpdateOptions{})
}

// Name returns the ClusterConversionBatch name.
func (s *ClusterConversionBatchScope) Name() string {
	return s.ClusterConversionBatch.GetName()
}

// Namespace returns the ClusterConversionBatch namespace.
func (s *ClusterConversionBatchScope) Namespace() string {
	return s.ClusterConversionBatch.GetNamespace()
}
