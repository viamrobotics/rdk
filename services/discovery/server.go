package discovery

import (
	"context"

	"go.opencensus.io/trace"
	apppb "go.viam.com/api/app/v1"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/discovery/v1"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// serviceServer implements the DiscoveryService from the discovery proto.
type serviceServer struct {
	pb.UnimplementedDiscoveryServiceServer
	coll resource.APIResourceCollection[Service]
}

// NewRPCServiceServer constructs a the discovery gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.APIResourceCollection[Service]) interface{} {
	return &serviceServer{coll: coll}
}

// DiscoverResources returns a list of components discovered by a discovery service.
func (server *serviceServer) DiscoverResources(ctx context.Context, req *pb.DiscoverResourcesRequest) (
	*pb.DiscoverResourcesResponse, error,
) {
	ctx, span := trace.StartSpan(ctx, "discovery::server::DiscoverResources")
	defer span.End()

	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}

	configs, err := svc.DiscoverResources(ctx, req.GetExtra().AsMap())
	if err != nil {
		return nil, err
	}
	if configs == nil {
		return nil, ErrNilResponse
	}

	protoConfigs := []*apppb.ComponentConfig{}
	for _, cfg := range configs {
		proto, err := config.ComponentConfigToProto(&cfg)
		if err != nil {
			return nil, err
		}
		protoConfigs = append(protoConfigs, proto)
	}

	return &pb.DiscoverResourcesResponse{Discoveries: protoConfigs}, nil
}

// DoCommand receives arbitrary commands.
func (server *serviceServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	ctx, span := trace.StartSpan(ctx, "discovery::server::DoCommand")
	defer span.End()

	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, svc, req)
}
