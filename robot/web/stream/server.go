package webstream

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/viamrobotics/webrtc/v3"
	"go.opencensus.io/trace"
	"go.uber.org/multierr"
	streampb "go.viam.com/api/stream/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	streamCamera "go.viam.com/rdk/robot/web/stream/camera"
	"go.viam.com/rdk/robot/web/stream/state"
	rutils "go.viam.com/rdk/utils"
)

var monitorCameraInterval = time.Second

type peerState struct {
	streamState *state.StreamState
	senders     []*webrtc.RTPSender
}

// Server implements the gRPC audio/video streaming service.
type Server struct {
	streampb.UnimplementedStreamServiceServer
	logger    logging.Logger
	robot     robot.Robot
	closedCtx context.Context
	closedFn  context.CancelFunc

	mu                      sync.RWMutex
	nameToStreamState       map[string]*state.StreamState
	activePeerStreams       map[*webrtc.PeerConnection]map[string]*peerState
	activeBackgroundWorkers sync.WaitGroup
	isAlive                 bool
}

// NewServer returns a server that will run on the given port and initially starts with the given
// stream.
func NewServer(
	streams []gostream.Stream,
	robot robot.Robot,
	logger logging.Logger,
) (*Server, error) {
	closedCtx, closedFn := context.WithCancel(context.Background())
	server := &Server{
		closedCtx:         closedCtx,
		closedFn:          closedFn,
		robot:             robot,
		logger:            logger,
		nameToStreamState: map[string]*state.StreamState{},
		activePeerStreams: map[*webrtc.PeerConnection]map[string]*peerState{},
		isAlive:           true,
	}

	for _, stream := range streams {
		if err := server.add(stream); err != nil {
			return nil, err
		}
	}
	server.startMonitorCameraAvailable()

	return server, nil
}

// StreamAlreadyRegisteredError indicates that a stream has a name that is already registered on
// the stream server.
type StreamAlreadyRegisteredError struct {
	name string
}

func (e *StreamAlreadyRegisteredError) Error() string {
	return fmt.Sprintf("stream %q already registered", e.name)
}

// NewStream informs the stream server of new streams that are capable of being streamed.
func (server *Server) NewStream(config gostream.StreamConfig) (gostream.Stream, error) {
	server.mu.Lock()
	defer server.mu.Unlock()

	if _, ok := server.nameToStreamState[config.Name]; ok {
		return nil, &StreamAlreadyRegisteredError{config.Name}
	}

	stream, err := gostream.NewStream(config, server.logger)
	if err != nil {
		return nil, err
	}

	if err = server.add(stream); err != nil {
		return nil, err
	}

	return stream, nil
}

// ListStreams implements part of the StreamServiceServer.
func (server *Server) ListStreams(ctx context.Context, req *streampb.ListStreamsRequest) (*streampb.ListStreamsResponse, error) {
	_, span := trace.StartSpan(ctx, "stream::server::ListStreams")
	defer span.End()
	server.mu.RLock()
	defer server.mu.RUnlock()

	names := make([]string, 0, len(server.nameToStreamState))
	for name := range server.nameToStreamState {
		names = append(names, name)
	}
	return &streampb.ListStreamsResponse{Names: names}, nil
}

// AddStream implements part of the StreamServiceServer.
func (server *Server) AddStream(ctx context.Context, req *streampb.AddStreamRequest) (*streampb.AddStreamResponse, error) {
	ctx, span := trace.StartSpan(ctx, "stream::server::AddStream")
	defer span.End()
	// Get the peer connection to the caller.
	pc, ok := rpc.ContextPeerConnection(ctx)
	server.logger.Infow("Adding video stream", "name", req.Name, "peerConn", pc)
	defer server.logger.Warnf("AddStream END %s", req.Name)

	if !ok {
		return nil, errors.New("can only add a stream over a WebRTC based connection")
	}

	server.mu.Lock()
	defer server.mu.Unlock()

	streamStateToAdd, ok := server.nameToStreamState[req.Name]

	// return error if there is no stream for that camera
	if !ok {
		var availableStreams string
		for n := range server.nameToStreamState {
			if availableStreams != "" {
				availableStreams += ", "
			}
			availableStreams += fmt.Sprintf("%q", n)
		}
		err := fmt.Errorf("no stream for %q, available streams: %s", req.Name, availableStreams)
		server.logger.Error(err.Error())
		return nil, err
	}
	// return error if camera is not in resource graph
	if _, err := streamCamera.Camera(server.robot, streamStateToAdd.Stream); err != nil {
		return nil, err
	}

	// return error if the caller's peer connection is already being sent video data
	if _, ok := server.activePeerStreams[pc][req.Name]; ok {
		err := errors.New("stream already active")
		server.logger.Error(err.Error())
		return nil, err
	}
	nameToPeerState, ok := server.activePeerStreams[pc]
	// if there is no active video data being sent, set up a callback to remove the peer connection from
	// the active streams & stop the stream from doing h264 encode if this is the last peer connection
	// subcribed to the camera's video feed
	// the callback fires when the peer connection state changes & runs the cleanup routine when the
	// peer connection is in a terminal state.
	if !ok {
		nameToPeerState = map[string]*peerState{}
		pc.OnConnectionStateChange(func(peerConnectionState webrtc.PeerConnectionState) {
			server.logger.Infof("%s pc.OnConnectionStateChange state: %s", req.Name, peerConnectionState)
			switch peerConnectionState {
			case webrtc.PeerConnectionStateDisconnected,
				webrtc.PeerConnectionStateFailed,
				webrtc.PeerConnectionStateClosed:

				server.mu.Lock()
				defer server.mu.Unlock()

				if server.isAlive {
					// Dan: This conditional closing on `isAlive` is a hack to avoid a data
					// race. Shutting down a robot causes the PeerConnection to be closed
					// concurrently with this `stream.Server`. Thus, `stream.Server.Close` waiting
					// on the `activeBackgroundWorkers` WaitGroup can race with adding a new
					// "worker". Given `Close` is expected to `Stop` remaining streams, we can elide
					// spinning off the below goroutine.
					//
					// Given this is an existing race, I'm choosing to add to the tech debt rather
					// than architect how shutdown should holistically work. Revert this change and
					// run `TestAudioTrackIsNotCreatedForVideoStream` to reproduce the race.
					server.activeBackgroundWorkers.Add(1)
					utils.PanicCapturingGo(func() {
						defer server.activeBackgroundWorkers.Done()
						server.mu.Lock()
						defer server.mu.Unlock()
						defer delete(server.activePeerStreams, pc)
						var errs error
						for _, ps := range server.activePeerStreams[pc] {
							errs = multierr.Combine(errs, ps.streamState.Decrement())
						}
						// We don't want to log this if the streamState was closed (as it only happens if viam-server is terminating)
						if errs != nil && !errors.Is(errs, state.ErrClosed) {
							server.logger.Errorw("error(s) stopping the streamState", "errs", errs)
						}
					})
				}
			case webrtc.PeerConnectionStateConnected,
				webrtc.PeerConnectionStateConnecting,
				webrtc.PeerConnectionStateNew:
				fallthrough
			default:
				return
			}
		})
		server.activePeerStreams[pc] = nameToPeerState
	}

	ps, ok := nameToPeerState[req.Name]
	// if the active peer stream doesn't have a peerState, add one containing the stream in question
	if !ok {
		ps = &peerState{streamState: streamStateToAdd}
		nameToPeerState[req.Name] = ps
	}

	guard := rutils.NewGuard(func() {
		for _, sender := range ps.senders {
			utils.UncheckedError(pc.RemoveTrack(sender))
		}
	})
	defer guard.OnFail()

	addTrack := func(track webrtc.TrackLocal) error {
		sender, err := pc.AddTrack(track)
		if err != nil {
			return err
		}
		ps.senders = append(ps.senders, sender)
		return nil
	}

	// if the stream supports video, add the video track
	if trackLocal, haveTrackLocal := streamStateToAdd.Stream.VideoTrackLocal(); haveTrackLocal {
		if err := addTrack(trackLocal); err != nil {
			server.logger.Error(err.Error())
			return nil, err
		}
	}
	// if the stream supports audio, add the audio track
	if trackLocal, haveTrackLocal := streamStateToAdd.Stream.AudioTrackLocal(); haveTrackLocal {
		if err := addTrack(trackLocal); err != nil {
			server.logger.Error(err.Error())
			return nil, err
		}
	}
	if err := streamStateToAdd.Increment(); err != nil {
		server.logger.Error(err.Error())
		return nil, err
	}

	guard.Success()
	return &streampb.AddStreamResponse{}, nil
}

// RemoveStream implements part of the StreamServiceServer.
func (server *Server) RemoveStream(ctx context.Context, req *streampb.RemoveStreamRequest) (*streampb.RemoveStreamResponse, error) {
	ctx, span := trace.StartSpan(ctx, "stream::server::RemoveStream")
	defer span.End()
	pc, ok := rpc.ContextPeerConnection(ctx)
	server.logger.Infow("Removing video stream", "name", req.Name, "peerConn", pc)
	if !ok {
		return nil, errors.New("can only remove a stream over a WebRTC based connection")
	}

	server.mu.Lock()
	defer server.mu.Unlock()

	streamToRemove, ok := server.nameToStreamState[req.Name]
	// Callers of RemoveStream will continue calling RemoveStream until it succeeds. Retrying on the
	// following "stream not found" errors is not helpful in this goal. Thus we return a success
	// response.
	if !ok {
		return &streampb.RemoveStreamResponse{}, nil
	}

	//nolint:nilerr
	if _, err := streamCamera.Camera(server.robot, streamToRemove.Stream); err != nil {
		return &streampb.RemoveStreamResponse{}, nil
	}

	if _, ok := server.activePeerStreams[pc][req.Name]; !ok {
		return &streampb.RemoveStreamResponse{}, nil
	}

	var errs error
	for _, sender := range server.activePeerStreams[pc][req.Name].senders {
		errs = multierr.Combine(errs, pc.RemoveTrack(sender))
	}
	if errs != nil {
		server.logger.Error(errs.Error())
		return nil, errs
	}

	if err := streamToRemove.Decrement(); err != nil {
		server.logger.Error(err.Error())
		return nil, err
	}

	delete(server.activePeerStreams[pc], req.Name)
	return &streampb.RemoveStreamResponse{}, nil
}

// Close closes the Server and waits for spun off goroutines to complete.
func (server *Server) Close() error {
	server.closedFn()
	server.mu.Lock()
	server.isAlive = false

	var errs error
	for _, streamState := range server.nameToStreamState {
		errs = multierr.Combine(errs, streamState.Close())
	}
	if errs != nil {
		server.logger.Errorf("Stream Server Close > StreamState.Close() errs: %s", errs)
	}
	server.mu.Unlock()
	server.activeBackgroundWorkers.Wait()
	return errs
}

func (server *Server) add(stream gostream.Stream) error {
	streamName := stream.Name()
	if _, ok := server.nameToStreamState[streamName]; ok {
		return &StreamAlreadyRegisteredError{streamName}
	}

	logger := server.logger.Sublogger(streamName)
	newStreamState := state.New(stream, server.robot, logger)
	server.nameToStreamState[streamName] = newStreamState
	return nil
}

// startMonitorCameraAvailable monitors whether or not the camera still exists
// If it no longer exists, it:
// 1. calls RemoveTrack on the senders of all peer connections that called AddTrack on the camera name.
// 2. decrements the number of active peers on the stream state (which should result in the
// stream state having no subscribers and calling gostream.Stop() or rtppaserverthrough.Unsubscribe)
// streaming tracks from it.
func (server *Server) startMonitorCameraAvailable() {
	server.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		for utils.SelectContextOrWait(server.closedCtx, monitorCameraInterval) {
			server.removeMissingStreams()
		}
	}, server.activeBackgroundWorkers.Done)
}

func (server *Server) removeMissingStreams() {
	server.mu.Lock()
	defer server.mu.Unlock()
	for key, streamState := range server.nameToStreamState {
		// Stream names are slightly modified versions of the resource short name
		camName := streamState.Stream.Name()
		shortName := resource.SDPTrackNameToShortName(camName)

		_, err := camera.FromRobot(server.robot, shortName)
		if !resource.IsNotFoundError(err) {
			// Cameras can go through transient states during reconfigure that don't necessarily
			// imply the camera is missing. E.g: *resource.notAvailableError. To double-check we
			// have the right set of exceptions here, we log the error and ignore.
			if err != nil {
				server.logger.Warnw("Error getting camera from robot",
					"camera", camName, "err", err, "errType", fmt.Sprintf("%T", err))
			}
			continue
		}

		// Best effort close any active peer streams. We'll remove from the known streams
		// first. Such that we only try closing/unsubscribing once.
		server.logger.Infow("Camera doesn't exist. Closing its streams",
			"camera", camName, "err", err, "Type", fmt.Sprintf("%T", err))
		delete(server.nameToStreamState, key)

		for pc, peerStateByCamName := range server.activePeerStreams {
			peerState, ok := peerStateByCamName[camName]
			if !ok {
				// There are no known peers for this camera. Do nothing.
				server.logger.Infow("no entry in peer map", "camera", camName)
				continue
			}

			server.logger.Infow("unsubscribing", "camera", camName, "numSenders", len(peerState.senders))
			var errs error
			for _, sender := range peerState.senders {
				errs = multierr.Combine(errs, pc.RemoveTrack(sender))
			}

			if errs != nil {
				server.logger.Warn(errs.Error())
			}

			if err := streamState.Decrement(); err != nil {
				server.logger.Warn(err.Error())
			}
			delete(server.activePeerStreams[pc], camName)
		}
		utils.UncheckedError(streamState.Close())
	}
}
