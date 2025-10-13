package audioout

import (
	"context"

	pb "go.viam.com/api/component/audioout/v1"

	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/rdk/protoutils"

	"go.viam.com/rdk/resource"
)

// serviceServer implements the AudioOutService.
type serviceServer struct {
	pb.UnimplementedAudioOutServiceServer
	coll resource.APIResourceGetter[AudioOut]
}

// NewRPCServiceServer constructs an audioout gRPC service server.
func NewRPCServiceServer(coll resource.APIResourceGetter[AudioOut]) interface{} {
	return &serviceServer{coll: coll}
}

func (s *serviceServer) Play(ctx context.Context, req *pb.PlayRequest) (*pb.PlayResponse, error) {
	a, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}

	info := audioInfoPBToStruct(req.Info)

	err = a.Play(ctx, req.AudioData, info, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}

	return &pb.PlayResponse{}, nil
}

func (s *serviceServer) GetProperties(ctx context.Context, req *commonpb.GetPropertiesRequest) (*commonpb.GetPropertiesResponse, error) {
	a, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}

	props, err := a.Properties(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &commonpb.GetPropertiesResponse{
		SupportedCodecs: props.SupportedCodecs,
		SampleRate:      props.SampleRate,
		NumChannels:     props.NumChannels,
	}, nil
}

// DoCommand receives arbitrary commands.
func (s *serviceServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	audioIn, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, audioIn, req)
}
