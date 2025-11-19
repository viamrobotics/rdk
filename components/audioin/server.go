package audioin

import (
	"context"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/audioin/v1"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// serviceServer implements the AudioInService.
type serviceServer struct {
	pb.UnimplementedAudioInServiceServer
	coll   resource.APIResourceGetter[AudioIn]
	logger logging.Logger
}

// NewRPCServiceServer constructs an audioin gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.APIResourceGetter[AudioIn], logger logging.Logger) interface{} {
	return &serviceServer{coll: coll, logger: logger.Sublogger("audioin_server")}
}

func (s *serviceServer) GetAudio(req *pb.GetAudioRequest, stream pb.AudioInService_GetAudioServer) error {
	audio, err := s.coll.Resource(req.Name)
	if err != nil {
		return err
	}

	chunkChan, err := audio.GetAudio(stream.Context(), req.Codec, req.DurationSeconds, req.PreviousTimestampNanoseconds, req.Extra.AsMap())
	if err != nil {
		return err
	}

	// Stream audio chunks
	for {
		select {
		case <-stream.Context().Done():
			s.logger.Debugf("context done, returning from GetAudio: %v", stream.Context().Err())
			return nil

		case chunk, ok := <-chunkChan:
			if !ok {
				return nil
			}

			pbChunk := audioChunkToPb(chunk)

			resp := &pb.GetAudioResponse{Audio: pbChunk, RequestId: req.RequestId}

			if err := stream.Send(resp); err != nil {
				return err
			}
		}
	}
}

func (s *serviceServer) GetProperties(ctx context.Context, req *commonpb.GetPropertiesRequest) (*commonpb.GetPropertiesResponse, error) {
	audio, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}

	props, err := audio.Properties(ctx, req.Extra.AsMap())
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
	audioIn, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, audioIn, req)
}
