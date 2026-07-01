package audioout

import (
	"context"
	"errors"
	"io"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/audioout/v1"

	"braces.dev/errtrace"
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
		return nil, errtrace.Wrap(err)
	}

	var info *rutils.AudioInfo
	if req.AudioInfo != nil {
		info = rutils.AudioInfoPBToStruct(req.AudioInfo)
	}

	err = a.Play(ctx, req.AudioData, info, req.Extra.AsMap())
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	return &pb.PlayResponse{}, nil
}

func (s *serviceServer) PlayStream(stream pb.AudioOutService_PlayStreamServer) error {
	first, err := stream.Recv()
	if err != nil {
		return errtrace.Wrap(err)
	}
	init := first.GetInit()
	if init == nil {
		return errtrace.Wrap(errors.New("first PlayStreamRequest must be PlayStreamInit"))
	}

	a, err := s.coll.Resource(init.GetName())
	if err != nil {
		return errtrace.Wrap(err)
	}

	var info *rutils.AudioInfo
	if init.GetAudioInfo() != nil {
		info = rutils.AudioInfoPBToStruct(init.GetAudioInfo())
	}

	ctx := stream.Context()
	chunks := make(chan []byte, 8)

	// Forward chunks from the gRPC stream into the channel until EOF or error.
	recvErr := make(chan error, 1)
	go func() {
		defer close(chunks)
		for {
			msg, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				recvErr <- nil
				return
			}
			if err != nil {
				recvErr <- err
				return
			}
			chunkMsg := msg.GetAudioChunk()
			if chunkMsg == nil {
				// Skip non-chunk payloads
				continue
			}
			select {
			case <-ctx.Done():
				recvErr <- ctx.Err()
				return
			case chunks <- chunkMsg.GetAudioData():
			}
		}
	}()

	playErr := a.PlayStream(ctx, info, chunks, init.GetExtra().AsMap())
	if playErr != nil {
		// Returning cancels stream.Context() so the recv goroutine exits.
		return errtrace.Wrap(playErr)
	}
	// PlayStream returning nil means recv already wrote to recvErr and exited.
	if recvDone := <-recvErr; recvDone != nil {
		return errtrace.Wrap(recvDone)
	}

	return errtrace.Wrap(stream.SendAndClose(&pb.PlayStreamResponse{}))
}

func (s *serviceServer) GetProperties(ctx context.Context, req *commonpb.GetPropertiesRequest) (*commonpb.GetPropertiesResponse, error) {
	a, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	props, err := a.Properties(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, errtrace.Wrap(err)
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
		return nil, errtrace.Wrap(err)
	}
	return errtrace.Wrap2(protoutils.DoFromResourceServer(ctx, audioOut, req))
}

// GetStatus returns the status of the audioout.
func (s *serviceServer) GetStatus(ctx context.Context, req *commonpb.GetStatusRequest) (*commonpb.GetStatusResponse, error) {
	res, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return errtrace.Wrap2(protoutils.GetStatusFromResourceServer(ctx, res, req))
}
