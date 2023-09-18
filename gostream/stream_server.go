package gostream

import (
	"context"
	"errors"
	"fmt"
	"github.com/pion/webrtc/v3"
	"sync"

	"go.uber.org/multierr"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"

	streampb "go.viam.com/rdk/gostream/proto/stream/v1"
)

// StreamAlreadyRegisteredError indicates that a stream has a name that is already registered on
// the stream server.
type StreamAlreadyRegisteredError struct {
	name string
}

func (e *StreamAlreadyRegisteredError) Error() string {
	return fmt.Sprintf("stream %q already registered", e.name)
}

// A StreamServer manages a collection of streams. Streams can be
// added over time for future new connections.
type StreamServer interface {
	// ServiceServer returns a service server for gRPC.
	ServiceServer() streampb.StreamServiceServer

	// NewStream creates a new stream from config and adds it for new connections to see.
	// Returns the added stream if it is successfully added to the server.
	NewStream(config StreamConfig) (Stream, error)

	// AddStream adds the given stream for new connections to see.
	AddStream(stream Stream) error

	// Close closes the server.
	Close() error
}

// NewStreamServer returns a server that will run on the given port and initially starts
// with the given stream.
func NewStreamServer(streams ...Stream) (StreamServer, error) {
	ss := &streamServer{
		nameToStream:      map[string]Stream{},
		activePeerStreams: map[*webrtc.PeerConnection]map[string]*peerState{},
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()
	for _, stream := range streams {
		if err := ss.addStream(stream); err != nil {
			return nil, err
		}
	}
	return ss, nil
}

type streamState struct {
	mu          sync.Mutex
	stream      Stream
	activePeers int
}

func (ss *streamState) Start() {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.activePeers++
	if ss.activePeers == 1 {
		ss.stream.Start()
	}
}

func (ss *streamState) Stop() {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.activePeers--
	if ss.activePeers <= 0 {
		ss.activePeers = 0
		ss.stream.Stop()
	}
}

type peerState struct {
	stream  *streamState
	senders []*webrtc.RTPSender
}

type streamServer struct {
	mu                      sync.RWMutex
	streams                 []*streamState
	nameToStream            map[string]Stream
	activePeerStreams       map[*webrtc.PeerConnection]map[string]*peerState
	activeBackgroundWorkers sync.WaitGroup
}

func (ss *streamServer) ServiceServer() streampb.StreamServiceServer {
	return &streamRPCServer{ss: ss}
}

func (ss *streamServer) NewStream(config StreamConfig) (Stream, error) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if _, ok := ss.nameToStream[config.Name]; ok {
		return nil, &StreamAlreadyRegisteredError{config.Name}
	}
	stream, err := NewStream(config)
	if err != nil {
		return nil, err
	}
	if err := ss.addStream(stream); err != nil {
		return nil, err
	}
	return stream, nil
}

func (ss *streamServer) AddStream(stream Stream) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	return ss.addStream(stream)
}

func (ss *streamServer) addStream(stream Stream) error {
	streamName := stream.Name()
	if _, ok := ss.nameToStream[streamName]; ok {
		return &StreamAlreadyRegisteredError{streamName}
	}
	ss.nameToStream[streamName] = stream
	ss.streams = append(ss.streams, &streamState{stream: stream})
	return nil
}

func (ss *streamServer) Close() error {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	for _, stream := range ss.streams {
		stream.stream.Stop()
		stream.activePeers = 0
	}
	ss.activeBackgroundWorkers.Wait()
	return nil
}

type streamRPCServer struct {
	streampb.UnimplementedStreamServiceServer
	ss *streamServer
}

func (srs *streamRPCServer) ListStreams(ctx context.Context, req *streampb.ListStreamsRequest) (*streampb.ListStreamsResponse, error) {
	srs.ss.mu.RLock()
	names := make([]string, 0, len(srs.ss.streams))
	for _, stream := range srs.ss.streams {
		names = append(names, stream.stream.Name())
	}
	srs.ss.mu.RUnlock()
	return &streampb.ListStreamsResponse{Names: names}, nil
}

func (srs *streamRPCServer) AddStream(ctx context.Context, req *streampb.AddStreamRequest) (*streampb.AddStreamResponse, error) {
	pc, ok := rpc.ContextPeerConnection(ctx)
	if !ok {
		return nil, errors.New("can only add a stream over a WebRTC based connection")
	}

	var streamToAdd *streamState
	for _, stream := range srs.ss.streams {
		if stream.stream.Name() == req.Name {
			streamToAdd = stream
			break
		}
	}

	if streamToAdd == nil {
		return nil, fmt.Errorf("no stream for %q", req.Name)
	}

	srs.ss.mu.Lock()
	defer srs.ss.mu.Unlock()

	if _, ok := srs.ss.activePeerStreams[pc][req.Name]; ok {
		return nil, errors.New("stream already active")
	}
	pcStreams, ok := srs.ss.activePeerStreams[pc]
	if !ok {
		pcStreams = map[string]*peerState{}
		pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
			switch state {
			case webrtc.PeerConnectionStateDisconnected,
				webrtc.PeerConnectionStateFailed,
				webrtc.PeerConnectionStateClosed:

				srs.ss.activeBackgroundWorkers.Add(1)
				utils.PanicCapturingGo(func() {
					defer srs.ss.activeBackgroundWorkers.Done()
					srs.ss.mu.Lock()
					defer srs.ss.mu.Unlock()
					defer delete(srs.ss.activePeerStreams, pc)
					for _, ps := range srs.ss.activePeerStreams[pc] {
						ps.stream.Stop()
					}
				})
			case webrtc.PeerConnectionStateConnected,
				webrtc.PeerConnectionStateConnecting,
				webrtc.PeerConnectionStateNew:
				fallthrough
			default:
				return
			}
		})
		srs.ss.activePeerStreams[pc] = pcStreams
	}

	ps, ok := pcStreams[req.Name]
	if !ok {
		ps = &peerState{stream: streamToAdd}
		pcStreams[req.Name] = ps
	}

	var successful bool
	defer func() {
		if !successful {
			for _, sender := range ps.senders {
				utils.UncheckedError(pc.RemoveTrack(sender))
			}
		}
	}()

	addTrack := func(track webrtc.TrackLocal) error {
		sender, err := pc.AddTrack(track)
		if err != nil {
			return err
		}
		ps.senders = append(ps.senders, sender)
		return nil
	}

	if trackLocal, haveTrackLocal := streamToAdd.stream.VideoTrackLocal(); haveTrackLocal {
		if err := addTrack(trackLocal); err != nil {
			return nil, err
		}
	}
	if trackLocal, haveTrackLocal := streamToAdd.stream.AudioTrackLocal(); haveTrackLocal {
		if err := addTrack(trackLocal); err != nil {
			return nil, err
		}
	}
	streamToAdd.Start()

	successful = true
	return &streampb.AddStreamResponse{}, nil
}

func (srs *streamRPCServer) RemoveStream(ctx context.Context, req *streampb.RemoveStreamRequest) (*streampb.RemoveStreamResponse, error) {
	pc, ok := rpc.ContextPeerConnection(ctx)
	if !ok {
		return nil, errors.New("can only remove a stream over a WebRTC based connection")
	}

	var streamToRemove *streamState
	for _, stream := range srs.ss.streams {
		if stream.stream.Name() == req.Name {
			streamToRemove = stream
			break
		}
	}

	if streamToRemove == nil {
		return nil, fmt.Errorf("no stream for %q", req.Name)
	}

	srs.ss.mu.Lock()
	defer srs.ss.mu.Unlock()

	if _, ok := srs.ss.activePeerStreams[pc][req.Name]; !ok {
		return nil, errors.New("stream already inactive")
	}
	defer func() {
		delete(srs.ss.activePeerStreams[pc], req.Name)
	}()

	var errs error
	for _, sender := range srs.ss.activePeerStreams[pc][req.Name].senders {
		errs = multierr.Combine(errs, pc.RemoveTrack(sender))
	}
	if errs != nil {
		return nil, errs
	}
	srs.ss.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer srs.ss.activeBackgroundWorkers.Done()
		streamToRemove.Stop()
	})

	return &streampb.RemoveStreamResponse{}, nil
}
