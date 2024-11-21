package app

import (
	"context"
	"sync"

	pb "go.viam.com/api/app/v1"
	"go.viam.com/utils"
)

type robotPartLogStream struct {
	client       *AppClient
	streamCancel context.CancelFunc
	streamMu     sync.Mutex

	activeBackgroundWorkers sync.WaitGroup
}

func (s *robotPartLogStream) startStream(ctx context.Context, id string, errorsOnly bool, filter *string, ch chan []*LogEntry) error {
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

	// This call won't return any errors it had until the client tries to receive.
	//nolint:errcheck
	stream, _ := s.client.client.TailRobotPartLogs(ctx, &pb.TailRobotPartLogsRequest{
		Id:         id,
		ErrorsOnly: errorsOnly,
		Filter:     filter,
	})
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

func (s *robotPartLogStream) receiveFromStream(ctx context.Context, stream pb.AppService_TailRobotPartLogsClient, ch chan []*LogEntry) {
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
			logs = append(logs, logEntryFromProto(log))
		}
		ch <- logs
	}
}

type uploadModuleFileStream struct {
	client       *AppClient
	streamCancel context.CancelFunc
	streamMu     sync.Mutex

	activeBackgroundWorkers sync.WaitGroup
}

func (s *uploadModuleFileStream) startStream(
	ctx context.Context, info *ModuleFileInfo, file []byte,
) (string, error) {
	s.streamMu.Lock()
	defer s.streamMu.Unlock()

	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	ctx, cancel := context.WithCancel(ctx)
	s.streamCancel = cancel

	stream, err := s.client.client.UploadModuleFile(ctx)
	if err != nil {
		return "", err
	}

	err = stream.Send(&pb.UploadModuleFileRequest{
		ModuleFile: &pb.UploadModuleFileRequest_ModuleFileInfo{
			ModuleFileInfo: moduleFileInfoToProto(info),
		},
	})
	if err != nil {
		s.client.logger.CError(ctx, err)
		return "", err
	}

	// Create a background go routine to send to the server stream.
	// We rely on calling the Done function here rather than in close stream
	// since managed go calls that function when the routine exits.
	s.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		s.sendToStream(ctx, stream, file)
	},
		s.activeBackgroundWorkers.Done)

	resp, err := stream.CloseAndRecv()
	if err != nil {
		return "", err
	}
	return resp.Url, err
}

func (s *uploadModuleFileStream) sendToStream(
	ctx context.Context, stream pb.AppService_UploadModuleFileClient, file []byte,
) {
	defer s.streamCancel()

	uploadChunkSize := 64 * 1024 //64 kB in bytes
	for start := 0; start < len(file); start += uploadChunkSize {
		select {
		case <- ctx.Done():
			s.client.logger.Debug(ctx.Err())
			return
		default:
		}

		end := start + uploadChunkSize
		if end > len(file) {
			end = len(file)
		}

		chunk := file[start:end]
		err := stream.Send(&pb.UploadModuleFileRequest{
			ModuleFile: &pb.UploadModuleFileRequest_File{
				File: chunk,
			},
		})
		if err != nil {
			// only debug log the context canceled error
		s.client.logger.Debug(err)
		return
		}
	}
}
