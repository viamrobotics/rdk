package mlmodels

import (
	"context"

	"go.viam.com/rdk/subtype"
	"google.golang.org/protobuf/types/known/structpb"
)

// subtypeServer implements the MLModelService from mlmodels.proto.
type subtypeServer struct {
	pb.UnimplementedMLModelServiceServer
	subtypeSvc subtype.Service
}

// NewServer constructs a ML Model gRPC service server.
func NewServer(s subtype.Service) pb.MLModelServiceServer {
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

func (server *subtypeServer) Infer(ctx context.Context, req *pb.InferRequest) (*pb.InferResponse, error) {
	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}
	od, err := svc.Infer(ctx, req.InputData.AsMap())
	if err != nil {
		return nil, err
	}
	outputData, err := structpb.NewStruct(od)
	if err != nil {
		return nil, err
	}
	return &pb.InferResponse{OutputData: outputData}, err
}
