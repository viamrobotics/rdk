package audioout

import (
	"context"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/audioout/v1"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	rutils "go.viam.com/rdk/utils"
)

// serviceServer implements the AudioOutService.
type serviceServer struct {
	pb.UnimplementedAudioOutServiceServer
	coll resource.APIResourceGetter[AudioOut]
}

// NewRPCServiceServer constructs an audioout gRPC service server.
func NewRPCServiceServer(coll resource.APIResourceGetter[AudioOut], logger logging.Logger) interface{} {
	return &serviceServer{coll: coll}
}

func (s *serviceServer) Play(ctx context.Context, req *pb.PlayRequest) (*pb.PlayResponse, error) {
	a, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}

	var info *rutils.AudioInfo
	if req.AudioInfo != nil {
		info = rutils.AudioInfoPBToStruct(req.AudioInfo)
	}

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
		SampleRateHz:    props.SampleRateHz,
		NumChannels:     props.NumChannels,
	}, nil
}

// DoCommand receives arbitrary commands.
func (s *serviceServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	audioOut, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, audioOut, req)
}
