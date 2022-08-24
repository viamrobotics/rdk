// Package datamanager contains a gRPC based datamanager service server
package datamanager

import (
	"context"
	"fmt"

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
	fmt.Println("server.go/NewServer()")
	return &subtypeServer{subtypeSvc: s}
}

func (server *subtypeServer) service() (Service, error) {
	fmt.Println("server.go/service()")
	resource := server.subtypeSvc.Resource(Name.String())
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

// ==========================-------------------
// subtypeServer implements the DataManagerService from datamanager.proto.
// type subtypeMServer struct {
// 	mb.UnimplementedModelServiceServer
// 	subtypeSvc subtype.Service
// }

// NewServer constructs a datamanager gRPC service server.
// func NewMServer(s subtype.Service) mb.ModelServiceServer {
// 	fmt.Println("server.go/NewMServer()")
// 	return &subtypeMServer{subtypeSvc: s}
// }

// func (server *subtypeMServer) mservice() (MService, error) {
// 	fmt.Println("server.go/mservice()")
// 	resource := server.subtypeSvc.Resource(Name.String())
// 	if resource == nil {
// 		return nil, utils.NewResourceNotFoundError(Name)
// 	}
// 	svc, ok := resource.(MService)
// 	if !ok {
// 		return nil, utils.NewUnimplementedInterfaceError("datamanager.Model", resource)
// 	}
// 	return svc, nil
// }

// func (server *subtypeMServer) Deploy(ctx context.Context, req *mb.DeployRequest) (*mb.DeployResponse, error) {
// 	svc, err := server.mservice()
// 	if err != nil {
// 		return nil, err
// 	}
// 	resp, err := svc.Deploy(
// 		ctx,
// 		req,
// 	)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return resp, nil
// }
