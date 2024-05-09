package webstream

import (
	"context"
	"fmt"
	"sync"

	streampb "go.viam.com/api/stream/v1"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/robot"
)

// Server implements the gRPC audio/video streaming service.
type Server struct {
	streampb.UnimplementedStreamServiceServer
	logger logging.Logger
	r      robot.Robot

	mu                      sync.RWMutex
	streamNames             []string
	activeBackgroundWorkers sync.WaitGroup
	isAlive                 bool
}

// NewServer returns a server that will run on the given port and initially starts with the given
// stream.
func NewServer(
	r robot.Robot,
	logger logging.Logger,
) (*Server, error) {
	ss := &Server{
		r:       r,
		logger:  logger,
		isAlive: true,
	}

	return ss, nil
}

// StreamAlreadyRegisteredError indicates that a stream has a name that is already registered on
// the stream server.
type StreamAlreadyRegisteredError struct {
	name string
}

func (e *StreamAlreadyRegisteredError) Error() string {
	return fmt.Sprintf("stream %q already registered", e.name)
}

// ListStreams implements part of the StreamServiceServer.
func (ss *Server) ListStreams(ctx context.Context, req *streampb.ListStreamsRequest) (*streampb.ListStreamsResponse, error) {
	return &streampb.ListStreamsResponse{}, nil
}

// AddStream implements part of the StreamServiceServer.
func (ss *Server) AddStream(ctx context.Context, req *streampb.AddStreamRequest) (*streampb.AddStreamResponse, error) {
	return &streampb.AddStreamResponse{}, nil
}

// RemoveStream implements part of the StreamServiceServer.
func (ss *Server) RemoveStream(ctx context.Context, req *streampb.RemoveStreamRequest) (*streampb.RemoveStreamResponse, error) {
	return &streampb.RemoveStreamResponse{}, nil
}

// Close closes the Server and waits for spun off goroutines to complete.
func (ss *Server) Close() error {
	ss.mu.Lock()
	ss.isAlive = false

	ss.mu.Unlock()
	ss.activeBackgroundWorkers.Wait()
	return nil
}
