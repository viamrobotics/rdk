package board

import (
	"context"
	"sync"

	pb "go.viam.com/api/component/board/v1"
	"go.viam.com/utils"
	"google.golang.org/protobuf/types/known/structpb"
)

type interruptStream struct {
	*client
	streamCancel  context.CancelFunc
	streamRunning bool
	streamReady   chan bool
	streamMu      sync.Mutex

	activeBackgroundWorkers sync.WaitGroup
	cancelBackgroundWorkers context.CancelFunc
	extra                   *structpb.Struct
}

func (s *interruptStream) startStream(ctx context.Context, interrupts []string, ch chan Tick) error {
	s.streamMu.Lock()
	defer s.streamMu.Unlock()

	s.streamRunning = true
	s.streamReady = make(chan bool)
	s.activeBackgroundWorkers.Add(1)
	ctx, cancel := context.WithCancel(ctx)
	s.cancelBackgroundWorkers = cancel

	streamCtx, cancel := context.WithCancel(ctx)
	s.streamCancel = cancel

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	req := &pb.StreamTicksRequest{
		Name:     s.client.info.name,
		PinNames: interrupts,
	}

	// This call won't return any errors it had until the client tries to receive.
	//nolint:errcheck
	stream, _ := s.client.client.StreamTicks(streamCtx, req)
	_, err := stream.Recv()
	if err != nil {
		s.client.logger.CError(ctx, err)
		return err
	}

	// Create a background go routine to recive from the server stream.
	utils.PanicCapturingGo(func() {
		defer s.activeBackgroundWorkers.Done()
		s.recieveFromStream(streamCtx, stream, ch)
	})

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.streamReady:
		return nil
	}
}

func (s *interruptStream) recieveFromStream(ctx context.Context, stream pb.BoardService_StreamTicksClient, ch chan Tick) {
	defer func() {
		s.streamMu.Lock()
		defer s.streamMu.Unlock()
		s.streamRunning = false
	}()
	if s.streamReady != nil {
		close(s.streamReady)
	}
	s.streamReady = nil
	defer s.closeStream(s.streamCancel)

	// repeatly receive from the stream
	for {
		select {
		case <-ctx.Done():
			s.client.logger.Debug(ctx.Err())
			return
		default:
		}
		streamResp, err := stream.Recv()
		if err != nil {
			// only debug log the context canceled error
			s.client.logger.Debug(err)
			return
		}
		// If there is a response, send to the tick channel.
		tick := Tick{
			Name:             streamResp.PinName,
			High:             streamResp.High,
			TimestampNanosec: streamResp.Time,
		}
		ch <- tick
	}
}

func (s *interruptStream) closeStream(cancel func()) {
	cancel()
	s.client.removeStream(s)
}
