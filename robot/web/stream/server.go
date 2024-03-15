//go:build !no_cgo

package webstream

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/pion/webrtc/v3"
	"go.uber.org/multierr"
	streampb "go.viam.com/api/stream/v1"
	"go.viam.com/rdk/gostream"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
)

type Server struct {
	streampb.UnimplementedStreamServiceServer

	mu                      sync.RWMutex
	streams                 []*streamState
	nameToStream            map[string]gostream.Stream
	activePeerStreams       map[*webrtc.PeerConnection]map[string]*peerState
	activeBackgroundWorkers sync.WaitGroup
}

// NewStreamServer returns a server that will run on the given port and initially starts
// with the given stream.
func NewServer(streams ...gostream.Stream) (*Server, error) {
	ss := &Server{
		nameToStream:      map[string]gostream.Stream{},
		activePeerStreams: map[*webrtc.PeerConnection]map[string]*peerState{},
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()
	for _, stream := range streams {
		if err := ss.add(stream); err != nil {
			return nil, err
		}
	}
	return ss, nil
}

func (ss *Server) NewStream(config gostream.StreamConfig) (gostream.Stream, error) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if _, ok := ss.nameToStream[config.Name]; ok {
		return nil, &StreamAlreadyRegisteredError{config.Name}
	}
	stream, err := gostream.NewStream(config)
	if err != nil {
		return nil, err
	}
	if err := ss.add(stream); err != nil {
		return nil, err
	}
	return stream, nil
}

func (ss *Server) Add(stream gostream.Stream) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	return ss.add(stream)
}

func (ss *Server) add(stream gostream.Stream) error {
	streamName := stream.Name()
	if _, ok := ss.nameToStream[streamName]; ok {
		return &StreamAlreadyRegisteredError{streamName}
	}
	ss.nameToStream[streamName] = stream
	ss.streams = append(ss.streams, &streamState{stream: stream})
	return nil
}

func (ss *Server) Close() error {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	for _, stream := range ss.streams {
		stream.stream.Stop()
		stream.activePeers = 0
	}
	ss.activeBackgroundWorkers.Wait()
	return nil
}

func (ss *Server) ListStreams(ctx context.Context, req *streampb.ListStreamsRequest) (*streampb.ListStreamsResponse, error) {
	ss.mu.RLock()
	names := make([]string, 0, len(ss.streams))
	for _, stream := range ss.streams {
		names = append(names, stream.stream.Name())
	}
	ss.mu.RUnlock()
	return &streampb.ListStreamsResponse{Names: names}, nil
}

func (ss *Server) AddStream(ctx context.Context, req *streampb.AddStreamRequest) (*streampb.AddStreamResponse, error) {
	pc, ok := rpc.ContextPeerConnection(ctx)
	if !ok {
		return nil, errors.New("can only add a stream over a WebRTC based connection")
	}

	var streamToAdd *streamState
	for _, stream := range ss.streams {
		if stream.stream.Name() == req.Name {
			streamToAdd = stream
			break
		}
	}

	if streamToAdd == nil {
		return nil, fmt.Errorf("no stream for %q", req.Name)
	}

	ss.mu.Lock()
	defer ss.mu.Unlock()

	if _, ok := ss.activePeerStreams[pc][req.Name]; ok {
		return nil, errors.New("stream already active")
	}
	pcStreams, ok := ss.activePeerStreams[pc]
	if !ok {
		pcStreams = map[string]*peerState{}
		pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
			switch state {
			case webrtc.PeerConnectionStateDisconnected,
				webrtc.PeerConnectionStateFailed,
				webrtc.PeerConnectionStateClosed:

				ss.activeBackgroundWorkers.Add(1)
				utils.PanicCapturingGo(func() {
					defer ss.activeBackgroundWorkers.Done()
					ss.mu.Lock()
					defer ss.mu.Unlock()
					defer delete(ss.activePeerStreams, pc)
					for _, ps := range ss.activePeerStreams[pc] {
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
		ss.activePeerStreams[pc] = pcStreams
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

func (ss *Server) RemoveStream(ctx context.Context, req *streampb.RemoveStreamRequest) (*streampb.RemoveStreamResponse, error) {
	pc, ok := rpc.ContextPeerConnection(ctx)
	if !ok {
		return nil, errors.New("can only remove a stream over a WebRTC based connection")
	}

	var streamToRemove *streamState
	for _, stream := range ss.streams {
		if stream.stream.Name() == req.Name {
			streamToRemove = stream
			break
		}
	}

	if streamToRemove == nil {
		return nil, fmt.Errorf("no stream for %q", req.Name)
	}

	ss.mu.Lock()
	defer ss.mu.Unlock()

	if _, ok := ss.activePeerStreams[pc][req.Name]; !ok {
		return nil, errors.New("stream already inactive")
	}
	defer func() {
		delete(ss.activePeerStreams[pc], req.Name)
	}()

	var errs error
	for _, sender := range ss.activePeerStreams[pc][req.Name].senders {
		errs = multierr.Combine(errs, pc.RemoveTrack(sender))
	}
	if errs != nil {
		return nil, errs
	}
	ss.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer ss.activeBackgroundWorkers.Done()
		streamToRemove.Stop()
	})

	return &streampb.RemoveStreamResponse{}, nil
}

// StreamAlreadyRegisteredError indicates that a stream has a name that is already registered on
// the stream server.
type StreamAlreadyRegisteredError struct {
	name string
}

func (e *StreamAlreadyRegisteredError) Error() string {
	return fmt.Sprintf("stream %q already registered", e.name)
}

type streamState struct {
	mu          sync.Mutex
	stream      gostream.Stream
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
