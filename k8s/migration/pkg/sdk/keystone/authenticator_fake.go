package keystone

import (
	"context"
	"time"
)

type FakeAuthenticator struct {
}

func NewFakeAuthenticator() *FakeAuthenticator {
	return &FakeAuthenticator{}
}

func (a *FakeAuthenticator) Auth(ctx context.Context, opts ...AuthOptions) (AuthInfo, error) {
	return AuthInfo{
		Token:     "token-qwertyuiop",
		UserID:    "user-id-123",
		ProjectID: "project-id-123",
		ExpiresAt: time.Now(),
	}, nil
}

func (a *FakeAuthenticator) ResetCache() error {
	return nil
}
