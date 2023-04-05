package mlmodel

import (
	"context"

	pb "go.viam.com/api/service/mlmodel/v1"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
	vprotoutils "go.viam.com/utils/protoutils"
)

// subtypeServer implements the MLModelService from mlmodel.proto.
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
	outputData, err := vprotoutils.StructToStructPb(od)
	if err != nil {
		return nil, err
	}
	return &pb.InferResponse{OutputData: outputData}, nil
}

func (server *subtypeServer) Metadata(
	ctx context.Context,
	req *pb.MetadataRequest,
) (*pb.MetadataResponse, error) {
	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}
	md, err := svc.Metadata(ctx)
	if err != nil {
		return nil, err
	}
	metadata, err := md.ToProto()
	if err != nil {
		return nil, err
	}
	return &pb.MetadataResponse{Metadata: metadata}, nil
}
