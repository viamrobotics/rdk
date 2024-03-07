package gostream

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/pion/webrtc/v3"
	"go.uber.org/multierr"
	streampb "go.viam.com/api/stream/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/logging"
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
func NewStreamServer(logger logging.Logger, streams ...Stream) (StreamServer, error) {
	logger.Info(" DBG: NewStreamServer BEGIN")
	defer logger.Info(" DBG: NewStreamServer END")
	ss := &streamServer{
		nameToStream:      map[string]Stream{},
		activePeerStreams: map[*webrtc.PeerConnection]map[string]*peerState{},
		logger:            logger,
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
	logger      logging.Logger
}

func (ss *streamState) Start() {
	ss.logger.Info(" DBG: (ss *streamState) Start BEGIN")
	defer ss.logger.Info(" DBG: (ss *streamState) Start END")
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.activePeers++
	if ss.activePeers == 1 {
		ss.stream.Start()
	}
}

func (ss *streamState) Stop() {
	ss.logger.Info(" DBG: (ss *streamState) Stop BEGIN")
	defer ss.logger.Info(" DBG: (ss *streamState) Stop END")
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
	logger                  logging.Logger
}

func (ss *streamServer) ServiceServer() streampb.StreamServiceServer {
	return &streamRPCServer{ss: ss}
}

func (ss *streamServer) NewStream(config StreamConfig) (Stream, error) {
	ss.logger.Info(" DBG: (ss *streamServer) NewStream BEGIN")
	defer ss.logger.Info(" DBG: (ss *streamServer) NewStream END")
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
	ss.logger.Infof(" DBG: (ss *streamServer) AddStream BEGIN %s", stream.Name())
	defer ss.logger.Info(" DBG: (ss *streamServer) AddStream END")
	ss.mu.Lock()
	defer ss.mu.Unlock()
	return ss.addStream(stream)
}

func (ss *streamServer) addStream(stream Stream) error {
	ss.logger.Infof(" DBG: (ss *streamServer) addStream BEGIN %s", stream.Name())
	defer ss.logger.Info(" DBG: (ss *streamServer) addStream END")
	streamName := stream.Name()
	if _, ok := ss.nameToStream[streamName]; ok {
		return &StreamAlreadyRegisteredError{streamName}
	}
	ss.nameToStream[streamName] = stream
	ss.streams = append(ss.streams, &streamState{stream: stream, logger: ss.logger})
	return nil
}

func (ss *streamServer) Close() error {
	ss.logger.Info(" DBG: (ss *streamServer) Close BEGIN")
	defer ss.logger.Info("(ss *streamServer) Close END")
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
	srs.ss.logger.Info("DBG: (srs *streamRPCServer) ListStreams BEGIN")
	defer srs.ss.logger.Info("DBG: (srs *streamRPCServer) ListStreams END")
	srs.ss.mu.RLock()
	names := make([]string, 0, len(srs.ss.streams))
	for _, stream := range srs.ss.streams {
		names = append(names, stream.stream.Name())
	}
	srs.ss.logger.Info("DBG: (srs *streamRPCServer) ListStreams Names %#v", names)
	srs.ss.mu.RUnlock()
	return &streampb.ListStreamsResponse{Names: names}, nil
}

func (srs *streamRPCServer) AddStream(ctx context.Context, req *streampb.AddStreamRequest) (*streampb.AddStreamResponse, error) {
	srs.ss.logger.Infof(" DBG: (srs *streamRPCServer) AddStream BEGIN %s", req.Name)
	defer srs.ss.logger.Infof(" DBG: (srs *streamRPCServer) AddStream END %s", req.Name)
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
		srs.ss.logger.Info("(srs *streamRPCServer) AddStream no stream to add")
		return nil, fmt.Errorf("no stream for %q", req.Name)
	}
	srs.ss.logger.Infof("(srs *streamRPCServer) AddStream: Stream to add: %s", streamToAdd.stream.Name())

	srs.ss.mu.Lock()
	defer srs.ss.mu.Unlock()

	if _, ok := srs.ss.activePeerStreams[pc][req.Name]; ok {
		srs.ss.logger.Infof("(srs *streamRPCServer) AddStream: Stream %s is already active", streamToAdd.stream.Name())
		return nil, errors.New("stream already active")
	}
	msg := "(srs *streamRPCServer) AddStream: len(srs.ss.activePeerStreams): %d, srs.ss.activePeerStreams: %#v"
	srs.ss.logger.Infof(msg, len(srs.ss.activePeerStreams), srs.ss.activePeerStreams)
	pcStreams, ok := srs.ss.activePeerStreams[pc]
	srs.ss.logger.Infof("(srs *streamRPCServer) AddStream: srs.ss.activePeerStreams[pc] pc: %p, ok: %t, pcStreams: %#v", pc, ok, pcStreams)
	if !ok {
		pcStreams = map[string]*peerState{}
		srs.ss.logger.Infof("(srs *streamRPCServer) AddStream: Setting OnConnectionStateChange callback from %#v", pc.OnConnectionStateChange)
		pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
			srs.ss.logger.Infof("(srs *streamRPCServer) AddStream: OnConnectionStateChange state: %s", state)
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
	srs.ss.logger.Infof("(srs *streamRPCServer) AddStream: len(srs.ss.activePeerStreams[pc]): %d", len(pcStreams))

	ps, ok := pcStreams[req.Name]
	if !ok {
		srs.ss.logger.Infof("(srs *streamRPCServer) AddStream: adding req.Name: %s to pcStreams: %#v", req.Name, pcStreams)
		ps = &peerState{stream: streamToAdd}
		pcStreams[req.Name] = ps
	}
	srs.ss.logger.Infof("(srs *streamRPCServer) AddStream: len(ps.senders): %d", len(ps.senders))

	var successful bool
	defer func() {
		if !successful {
			srs.ss.logger.Infof("(srs *streamRPCServer) AddStream: unsuccessful, removing %d senders", len(ps.senders))
			for _, sender := range ps.senders {
				srs.ss.logger.Infof("(srs *streamRPCServer) AddStream: pc.RemoveTrack(%s)", sender)
				utils.UncheckedError(pc.RemoveTrack(sender))
			}
		}
	}()

	addTrack := func(track webrtc.TrackLocal) error {
		msg := "(srs *streamRPCServer) addTrack: trackLocal: ID: %s, StreamID: %s, RID: %s"
		srs.ss.logger.Infof(msg, track.ID(), track.StreamID(), track.RID())
		sender, err := pc.AddTrack(track)
		if err != nil {
			srs.ss.logger.Infof("(srs *streamRPCServer) addTrack: err: %s", err.Error())
			return err
		}
		srs.ss.logger.Infof("(srs *streamRPCServer) addTrack: new sender: %p", sender)
		ps.senders = append(ps.senders, sender)
		return nil
	}

	if trackLocal, haveTrackLocal := streamToAdd.stream.VideoTrackLocal(); haveTrackLocal {
		msg := "(srs *streamRPCServer) AddStream: haveTrackLocal: %t, trackLocal: ID: %s, StreamID: %s"
		srs.ss.logger.Infof(msg, haveTrackLocal, trackLocal.ID(), trackLocal.StreamID())
		if err := addTrack(trackLocal); err != nil {
			srs.ss.logger.Infof("(srs *streamRPCServer) AddStream: addTrack err: %s", err.Error())
			return nil, err
		}
	} else {
		srs.ss.logger.Infof("(srs *streamRPCServer) AddStream: haveTrackLocal: %t", haveTrackLocal)
	}
	if trackLocal, haveTrackLocal := streamToAdd.stream.AudioTrackLocal(); haveTrackLocal {
		if err := addTrack(trackLocal); err != nil {
			return nil, err
		}
	}
	srs.ss.logger.Info("(srs *streamRPCServer) calling streamToAdd.Start()")
	streamToAdd.Start()

	successful = true
	return &streampb.AddStreamResponse{}, nil
}

func (srs *streamRPCServer) RemoveStream(ctx context.Context, req *streampb.RemoveStreamRequest) (*streampb.RemoveStreamResponse, error) {
	srs.ss.logger.Infof(" DBG: (srs *streamRPCServer) RemoveStream BEGIN %s", req.Name)
	defer srs.ss.logger.Infof(" DBG: (srs *streamRPCServer) RemoveStream END %s", req.Name)
	pc, ok := rpc.ContextPeerConnection(ctx)
	if !ok {
		srs.ss.logger.Info(" DBG: (srs *streamRPCServer) RemoveStream not ok")
		return nil, errors.New("can only remove a stream over a WebRTC based connection")
	}
	srs.ss.logger.Infof(" DBG: (srs *streamRPCServer) RemoveStream pc %p", pc)

	var streamToRemove *streamState
	for _, stream := range srs.ss.streams {
		if stream.stream.Name() == req.Name {
			streamToRemove = stream
			break
		}
	}

	if streamToRemove == nil {
		srs.ss.logger.Info(" DBG: (srs *streamRPCServer) RemoveStream has no stream to remove")
		return nil, fmt.Errorf("no stream for %q", req.Name)
	}
	srs.ss.logger.Infof(" DBG: (srs *streamRPCServer) RemoveStream streamToRemove %s", streamToRemove.stream.Name())

	srs.ss.mu.Lock()
	defer srs.ss.mu.Unlock()

	if _, ok := srs.ss.activePeerStreams[pc][req.Name]; !ok {
		srs.ss.logger.Info(" DBG: (srs *streamRPCServer) RemoveStream stream already inactive")
		return nil, errors.New("stream already inactive")
	}
	defer func() {
		delete(srs.ss.activePeerStreams[pc], req.Name)
	}()

	var errs error
	msg := " DBG: (srs *streamRPCServer) RemoveStream len(srs.ss.activePeerStreams[pc][req.Name].senders): %d"
	srs.ss.logger.Infof(msg, len(srs.ss.activePeerStreams[pc][req.Name].senders))
	for _, sender := range srs.ss.activePeerStreams[pc][req.Name].senders {
		srs.ss.logger.Infof(" DBG: (srs *streamRPCServer) RemoveStream calling pc.RemoveTrack(sender) on %p", sender)
		errs = multierr.Combine(errs, pc.RemoveTrack(sender))
	}
	if errs != nil {
		srs.ss.logger.Infof(" DBG: (srs *streamRPCServer) RemoveStream errs %s", errs.Error())
		return nil, errs
	}
	srs.ss.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer srs.ss.activeBackgroundWorkers.Done()
		streamToRemove.Stop()
	})

	return &streampb.RemoveStreamResponse{}, nil
}
