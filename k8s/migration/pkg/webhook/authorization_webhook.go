package webhook

import (
	"context"
	"log"

	admissionv1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type AuthorizationWebhook struct {
	decoder *admission.Decoder
}

func (a *AuthorizationWebhook) Handle(ctx context.Context, req admission.Request) admission.Response {
	// Extract user and groups from request
	userInfo := req.UserInfo
	log.Println("User: %s, Groups: %v", userInfo.Username, userInfo.Groups)
	// Implement custom authorization logic
	// For example: prevent deletion of migrations in "Running" state

	if req.Operation == admissionv1.Delete {
		// Check resource state
		// Deny if necessary
	}

	return admission.Allowed("")
}
