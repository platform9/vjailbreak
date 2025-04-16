package keystone

import (
	"context"
	"time"
)

type FakeClient struct {
}

func NewFakeClient() *FakeClient {
	return &FakeClient{}
}

var _ Client = (*FakeClient)(nil)

func (f FakeClient) Auth(ctx context.Context, credentials Credentials) (AuthInfo, error) {
	return AuthInfo{
		Token:     "faketoken",
		UserID:    "fakeuserid",
		ProjectID: "fakeprojectid",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}, nil
}

func (f FakeClient) GetTokenInfo(ctx context.Context, token string) (AuthResponse, error) {
	panic("implement me")
}

func (f FakeClient) GetProjects(ctx context.Context, token string) ([]Project, error) {
	panic("implement me")
}

func (f FakeClient) CreateUser(ctx context.Context, token string, input CreateUserRequest) (*CreateUserResponse, error) {
	panic("implement me")
}

func (f FakeClient) DeleteUser(ctx context.Context, token string, userID string) error {
	panic("implement me")
}

func (f FakeClient) ListUser(ctx context.Context, token, filter string) ([]User, error) {
	panic("implement me")
}

func (f FakeClient) ListRoles(ctx context.Context, token, filter string) ([]Role, error) {
	panic("implement me")
}

func (f FakeClient) AssignRoleToUserOnProject(ctx context.Context, token string, projectID string, userID string, roleID string) error {
	panic("implement me")
}

func (f FakeClient) ListProjects(ctx context.Context, token, filter string) ([]Project, error) {
	panic("implement me")
}
