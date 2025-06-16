package server

import (
	"context"

	api "github.com/platform9/vjailbreak/pkg/vpwned/api/proto/v1/service"
	version "github.com/platform9/vjailbreak/pkg/vpwned/version"
)

type VpwnedVersion struct {
	api.UnimplementedVersionServer
}

func (s *VpwnedVersion) Version(ctx context.Context, in *api.VersionRequest) (*api.VersionResponse, error) {
	return &api.VersionResponse{Version: version.Version}, nil
}
