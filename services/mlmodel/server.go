package mlmodel

import (
	"context"
	"encoding/base64"

	pb "go.viam.com/api/service/mlmodel/v1"
	vprotoutils "go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
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
	id, err := asMap(req.InputData)
	if err != nil {
		return nil, err
	}
	od, err := svc.Infer(ctx, id)
	if err != nil {
		return nil, err
	}
	outputData, err := vprotoutils.StructToStructPb(od)
	if err != nil {
		return nil, err
	}
	return &pb.InferResponse{OutputData: outputData}, nil
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
	metadata, err := md.toProto()
	if err != nil {
		return nil, err
	}
	return &pb.MetadataResponse{Metadata: metadata}, nil
}
