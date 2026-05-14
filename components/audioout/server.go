package audioout

import (
	"context"
	"errors"
	"io"

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
	coll   resource.APIResourceGetter[AudioOut]
	logger logging.Logger
}

// NewRPCServiceServer constructs an audioout gRPC service server.
func NewRPCServiceServer(coll resource.APIResourceGetter[AudioOut], logger logging.Logger) interface{} {
	return &serviceServer{coll: coll, logger: logger}
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

func (s *serviceServer) PlayStream(stream pb.AudioOutService_PlayStreamServer) error {
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	init := first.GetInit()
	if init == nil {
		return errors.New("first PlayStreamRequest must be PlayStreamInit")
	}

	a, err := s.coll.Resource(init.GetName())
	if err != nil {
		return err
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
			if err == io.EOF {
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

	playErr := a.PlayStream(ctx, info, chunks, nil)
	if playErr != nil {
		// Returning will cancel stream.Context(), which unblocks the receive
		// goroutine's stream.Recv(). It writes to the buffered recvErr (cap 1)
		// and exits
		return playErr
	}
	// PlayStream returning nil means the goroutine closed `chunks` after EOF,
	// so it has already written to recvErr and exited — this won't block.
	if recvDone := <-recvErr; recvDone != nil {
		return recvDone
	}

	return stream.SendAndClose(&pb.PlayStreamResponse{})
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

// GetStatus returns the status of the audioout.
func (s *serviceServer) GetStatus(ctx context.Context, req *commonpb.GetStatusRequest) (*commonpb.GetStatusResponse, error) {
	res, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	return protoutils.GetStatusFromResourceServer(ctx, res, req)
}
