// Package datamanager contains a gRPC based datamanager service server
package datamanager

import (
	"context"

	pb "go.viam.com/rdk/proto/api/service/datamanager/v1"
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

func (server *subtypeServer) service() (Service, error) {
	resource := server.subtypeSvc.Resource(Name.Name)
	if resource == nil {
		return nil, utils.NewResourceNotFoundError(Name)
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("datamanager.Service", resource)
	}
	return svc, nil
}

func (server *subtypeServer) Sync(ctx context.Context, req *pb.SyncRequest) (*pb.SyncResponse, error) {
	svc, err := server.service()
	if err != nil {
		return nil, err
	}
	err = svc.Sync(
		ctx,
	)
	if err != nil {
		return nil, err
	}
	return &pb.SyncResponse{}, nil
}
