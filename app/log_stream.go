package app

import (
	"context"
	"sync"

	pb "go.viam.com/api/app/v1"
	"go.viam.com/utils"
)

type logStream struct {
	client       *AppClient
	streamCancel context.CancelFunc
	streamMu     sync.Mutex

	activeBackgroundWorkers sync.WaitGroup
}

func (s *logStream) startStream(ctx context.Context, id string, errorsOnly bool, filter *string, ch chan []*LogEntry) error {
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

	req := &pb.TailRobotPartLogsRequest{
		Id:         id,
		ErrorsOnly: errorsOnly,
		Filter:     filter,
	}

	// This call won't return any errors it had until the client tries to receive.
	//nolint:errcheck
	stream, _ := s.client.client.TailRobotPartLogs(ctx, req)
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

func (s *logStream) receiveFromStream(ctx context.Context, stream pb.AppService_TailRobotPartLogsClient, ch chan []*LogEntry) {
	defer s.streamCancel()

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
		// If there is a response, send to the logs channel.
		var logs []*LogEntry
		for _, log := range streamResp.Logs {
			l, err := ProtoToLogEntry(log)
			if err != nil {
				s.client.logger.Debug(err)
				return
			}
			logs = append(logs, l)
		}
		ch <- logs
	}
}
