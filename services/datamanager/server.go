// Package datamanager contains a gRPC based datamanager service server
package datamanager

import (
	"context"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/datamanager/v1"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// serviceServer implements the DataManagerService from datamanager.proto.
type serviceServer struct {
	pb.UnimplementedDataManagerServiceServer
	coll resource.APIResourceCollection[Service]
}

// NewRPCServiceServer constructs a datamanager gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.APIResourceCollection[Service]) interface{} {
	return &serviceServer{coll: coll}
}

func (server *serviceServer) Sync(ctx context.Context, req *pb.SyncRequest) (*pb.SyncResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	if err := svc.Sync(ctx, req.Extra.AsMap()); err != nil {
		return nil, err
	}
	return &pb.SyncResponse{}, nil
}

// DoCommand receives arbitrary commands.
func (server *serviceServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, svc, req)
}
