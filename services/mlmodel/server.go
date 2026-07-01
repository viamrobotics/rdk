package mlmodel

import (
	"context"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/mlmodel/v1"

	"braces.dev/errtrace"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/ml"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// serviceServer implements the MLModelService from mlmodel.proto.
type serviceServer struct {
	pb.UnimplementedMLModelServiceServer
	coll resource.APIResourceGetter[Service]
}

// NewRPCServiceServer constructs a ML Model gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.APIResourceGetter[Service], logger logging.Logger) interface{} {
	return &serviceServer{coll: coll}
}

func (server *serviceServer) Infer(ctx context.Context, req *pb.InferRequest) (*pb.InferResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	var it ml.Tensors
	if req.InputTensors != nil {
		it, err = ml.ProtoToTensors(req.InputTensors)
		if err != nil {
			return nil, errtrace.Wrap(err)
		}
	}
	ot, err := svc.Infer(ctx, it)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	outputTensors, err := TensorsToProto(ot)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return &pb.InferResponse{OutputTensors: outputTensors}, nil
}

func (server *serviceServer) Metadata(
	ctx context.Context,
	req *pb.MetadataRequest,
) (*pb.MetadataResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	md, err := svc.Metadata(ctx)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	metadata, err := md.toProto()
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return &pb.MetadataResponse{Metadata: metadata}, nil
}

// GetStatus returns the status of the mlmodel service.
func (server *serviceServer) GetStatus(ctx context.Context, req *commonpb.GetStatusRequest) (*commonpb.GetStatusResponse, error) {
	res, err := server.coll.Resource(req.GetName())
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return errtrace.Wrap2(protoutils.GetStatusFromResourceServer(ctx, res, req))
}
