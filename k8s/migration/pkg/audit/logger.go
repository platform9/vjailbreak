// Package audit provides audit logging functionality for tracking user actions and operations.
package audit

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Logger provides structured logging for audit events.
type Logger struct{}

// NewLogger creates a new Logger instance for audit logging.
func NewLogger() *Logger {
	return &Logger{}
}

// LogAction logs an audit event with user, action, resource information and success status.
func (l *Logger) LogAction(ctx context.Context, user, action, resource, resourceName string, success bool) {
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
