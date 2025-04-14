package server

import (
	"context"

	api "github.com/platform9/vjailbreak/pkg/vpwned/openapiv3/proto/service/api"
)

type providersGRPC struct {
	api.UnimplementedBMListMachinesServer
}

func (p *providersGRPC) BMListMachines(ctx context.Context, in *api.BMListMachinesRequest) (*api.BMListMachinesResponse, error) {
	return &api.BMListMachinesResponse{}, nil
}
