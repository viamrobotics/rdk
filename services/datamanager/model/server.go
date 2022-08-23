// Package model implements model storage/deployment client.
package model

import (
	"context"
	"fmt"

	pb "go.viam.com/api/proto/viam/model/v1"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

type subtypeServer struct {
	pb.UnimplementedModelServiceServer
	subtypeSvc subtype.Service
}

func NewServer(s subtype.Service) pb.ModelServiceServer {
	fmt.Println("NewServer()")
	return &subtypeServer{subtypeSvc: s}
}

func (server *subtypeServer) service() (Service, error) {
	fmt.Println("service()")
	resource := server.subtypeSvc.Resource("Deploy")
	if resource == nil {
		return nil, utils.NewResourceNotFoundError(Name)
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("datamanager.Service", resource)
	}
	return svc, nil
}

func (server *subtypeServer) Deploy(ctx context.Context, req *pb.DeployRequest) (*pb.DeployResponse, error) {
	svc, err := server.service()
	if err != nil {
		return nil, err
	}
	err = svc.Deploy(
		ctx,
		req,
	)
	if err != nil {
		return nil, err
	}
	return &pb.DeployResponse{}, nil
}
