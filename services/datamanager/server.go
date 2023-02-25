// Package datamanager contains a gRPC based datamanager service server
package datamanager

import (
	"context"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/datamanager/v1"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

// subtypeServer implements the DataManagerService from datamanager.proto.
type subtypeServer struct {
	pb.UnimplementedDataManagerServiceServer
	subtypeSvc subtype.Service
}

// NewServer constructs a datamanager gRPC service server.
func NewServer(s subtype.Service) pb.DataManagerServiceServer {
	return &subtypeServer{subtypeSvc: s}
}

func (server *subtypeServer) service(serviceName string) (Service, error) {
	resource := server.subtypeSvc.Resource(serviceName)
	if resource == nil {
		return nil, utils.NewResourceNotFoundError(Named(serviceName))
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, NewUnimplementedInterfaceError(resource)
	}
	return svc, nil
}

func (server *subtypeServer) Sync(ctx context.Context, req *pb.SyncRequest) (*pb.SyncResponse, error) {
	svc, err := server.service(req.Name)
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
	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, svc, req)
}
