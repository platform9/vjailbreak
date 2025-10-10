package audit

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

type AuditLogger struct{}

func NewAuditLogger() *AuditLogger {
	return &AuditLogger{}
}

func (a *AuditLogger) LogAction(ctx context.Context, user, action, resource, resourceName string, success bool) {
	logger := log.FromContext(ctx)
	logger.Info("AUDIT",
		"timestamp", time.Now().Format(time.RFC3339),
		"user", user,
		"action", action,
		"resource", resource,
		"resourceName", resourceName,
		"success", success,
	)
}
