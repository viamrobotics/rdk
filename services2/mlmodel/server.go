package mlmodel

import (
	"context"

	pb "go.viam.com/api/service/mlmodel/v1"

	"go.viam.com/rdk/ml"
	"go.viam.com/rdk/resource"
)

// serviceServer implements the MLModelService from mlmodel.proto.
type serviceServer struct {
	pb.UnimplementedMLModelServiceServer
	coll resource.APIResourceCollection[Service]
}

// NewRPCServiceServer constructs a ML Model gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.APIResourceCollection[Service]) interface{} {
	return &serviceServer{coll: coll}
}

func (server *serviceServer) Infer(ctx context.Context, req *pb.InferRequest) (*pb.InferResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}

	var it ml.Tensors
	if req.InputTensors != nil {
		it, err = ProtoToTensors(req.InputTensors)
		if err != nil {
			return nil, err
		}
	}
	ot, err := svc.Infer(ctx, it)
	if err != nil {
		return nil, err
	}
	outputTensors, err := TensorsToProto(ot)
	if err != nil {
		return nil, err
	}
	return &pb.InferResponse{OutputTensors: outputTensors}, nil
}

func (server *serviceServer) Metadata(
	ctx context.Context,
	req *pb.MetadataRequest,
) (*pb.MetadataResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	md, err := svc.Metadata(ctx)
	if err != nil {
		return nil, err
	}
	metadata, err := md.toProto()
	if err != nil {
		return nil, err
	}
	return &pb.MetadataResponse{Metadata: metadata}, nil
}
