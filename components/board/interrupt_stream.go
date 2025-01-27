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
	streamCancel context.CancelFunc
	streamMu     sync.Mutex

	activeBackgroundWorkers sync.WaitGroup
	extra                   *structpb.Struct
}

func (s *interruptStream) startStream(ctx context.Context, interrupts []DigitalInterrupt, ch chan Tick) error {
	s.streamMu.Lock()
	defer s.streamMu.Unlock()

	if ctx.Err() != nil {
		return ctx.Err()
	}

	ctx, cancel := context.WithCancel(ctx)
	s.streamCancel = cancel

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	names := []string{}
	for _, i := range interrupts {
		names = append(names, i.Name())
	}

	req := &pb.StreamTicksRequest{
		Name:     s.client.boardName,
		PinNames: names,
		Extra:    s.extra,
	}

	// This call won't return any errors it had until the client tries to receive.
	//nolint:errcheck
	stream, _ := s.client.client.StreamTicks(ctx, req)
	_, err := stream.Recv()
	if err != nil {
		s.client.logger.CError(ctx, err)
		return err
	}

	// Create a background go routine to receive from the server stream.
	// We rely on calling the Done function here rather than in close stream
	// since managed go calls that function when the routine exits.
	s.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		s.receiveFromStream(ctx, stream, ch)
	},
		s.activeBackgroundWorkers.Done)
	return nil
}

func (s *interruptStream) receiveFromStream(ctx context.Context, stream pb.BoardService_StreamTicksClient, ch chan Tick) {
	defer s.closeStream()

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

func (s *interruptStream) closeStream() {
	s.streamCancel()
	s.client.removeStream(s)
}
