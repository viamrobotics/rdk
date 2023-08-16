package mlmodel

import (
	"context"
	"encoding/base64"

	pb "go.viam.com/api/service/mlmodel/v1"
	vprotoutils "go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/types/known/structpb"

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
	id, err := asMap(req.InputData)
	if err != nil {
		return nil, err
	}
	it, err := protoToTensors(req.InputTensors)
	if err != nil {
		return nil, err
	}
	ot, od, err := svc.Infer(ctx, it, id)
	if err != nil {
		return nil, err
	}
	outputData, err := vprotoutils.StructToStructPb(od)
	if err != nil {
		return nil, err
	}
	outputTensors, err := ot.ToProto()
	if err != nil {
		return nil, err
	}
	return &pb.InferResponse{OutputData: outputData, OutputTensors: outputTensors}, nil
}

// AsMap converts x to a general-purpose Go map.
// The map values are converted by calling Value.AsInterface.
func asMap(x *structpb.Struct) (map[string]interface{}, error) {
	f := x.GetFields()
	vs := make(map[string]interface{}, len(f))
	for k, in := range f {
		switch in.GetKind().(type) {
		case *structpb.Value_StringValue:
			out, err := base64.StdEncoding.DecodeString(in.GetStringValue())
			if err != nil {
				return nil, err
			}
			vs[k] = out
		default:
			vs[k] = in.AsInterface()
		}
	}
	return vs, nil
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
