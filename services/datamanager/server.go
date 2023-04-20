// Package datamanager contains a gRPC based datamanager service server
package datamanager

import (
	"context"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/datamanager/v1"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// subtypeServer implements the DataManagerService from datamanager.proto.
type subtypeServer struct {
	pb.UnimplementedDataManagerServiceServer
	coll resource.SubtypeCollection[Service]
}

// NewServer constructs a datamanager gRPC service server.
func NewServer(coll resource.SubtypeCollection[Service]) pb.DataManagerServiceServer {
	return &subtypeServer{coll: coll}
}

func (server *subtypeServer) Sync(ctx context.Context, req *pb.SyncRequest) (*pb.SyncResponse, error) {
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
func (server *subtypeServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, svc, req)
}
