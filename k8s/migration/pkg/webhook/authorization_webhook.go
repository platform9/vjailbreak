// Package webhook provides admission webhook handlers for authorization and validation.
package webhook

import (
	"context"
	"log"

	admissionv1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// AuthorizationWebhook handles admission requests for custom authorization logic.
type AuthorizationWebhook struct{}

// Handle processes admission requests and applies authorization rules.
func (a *AuthorizationWebhook) Handle(_ context.Context, req admission.Request) admission.Response {
	// Extract user and groups from request
	userInfo := req.UserInfo
	log.Printf("User: %s, Groups: %v", userInfo.Username, userInfo.Groups)

	// Implement custom authorization logic
	// For example: prevent deletion of migrations in "Running" state
	if req.Operation == admissionv1.Delete {
		// TODO: Check resource state and deny if necessary
		log.Printf("Delete operation requested by user: %s", userInfo.Username)
	}

	return admission.Allowed("")
}
