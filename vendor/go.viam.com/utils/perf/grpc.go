package perf

import (
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/stats/view"
	"google.golang.org/grpc/stats"
)

// See: https://opencensus.io/guides/grpc/go/#1

func registerGrpcViews() error {
	return view.Register(ocgrpc.DefaultServerViews...)
}

// NewGrpcStatsHandler creates a new stats handler writing to opencensus.
//
// Example:
//
//	grpcServer, err := rpc.NewServer(logger, rpc.WithStatsHandler(perf.NewGrpcStatsHandler()))
//
// See further documentation here: https://opencensus.io/guides/grpc/go/
func NewGrpcStatsHandler() stats.Handler {
	return &ocgrpc.ServerHandler{
		IsPublicEndpoint: true,
	}
}
