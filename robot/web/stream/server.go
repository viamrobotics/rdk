package webstream

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pion/webrtc/v3"
	"github.com/pkg/errors"
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
	r         robot.Robot
	closedCtx context.Context
	closedFn  context.CancelFunc

	mu                      sync.RWMutex
	streamNames             []string
	nameToStreamState       map[string]*state.StreamState
	activePeerStreams       map[*webrtc.PeerConnection]map[string]*peerState
	activeBackgroundWorkers sync.WaitGroup
	isAlive                 bool
}

// NewServer returns a server that will run on the given port and initially starts with the given
// stream.
func NewServer(
	streams []gostream.Stream,
	r robot.Robot,
	logger logging.Logger,
) (*Server, error) {
	closedCtx, closedFn := context.WithCancel(context.Background())
	ss := &Server{
		closedCtx:         closedCtx,
		closedFn:          closedFn,
		r:                 r,
		logger:            logger,
		nameToStreamState: map[string]*state.StreamState{},
		activePeerStreams: map[*webrtc.PeerConnection]map[string]*peerState{},
		isAlive:           true,
	}

	for _, stream := range streams {
		if err := ss.add(stream); err != nil {
			return nil, err
		}
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

// NewStream informs the stream server of new streams that are capable of being streamed.
func (ss *Server) NewStream(config gostream.StreamConfig) (gostream.Stream, error) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if _, ok := ss.nameToStreamState[config.Name]; ok {
		return nil, &StreamAlreadyRegisteredError{config.Name}
	}

	stream, err := gostream.NewStream(config)
	if err != nil {
		return nil, err
	}

	if err = ss.add(stream); err != nil {
		return nil, err
	}

	return stream, nil
}

// ListStreams implements part of the StreamServiceServer.
func (ss *Server) ListStreams(ctx context.Context, req *streampb.ListStreamsRequest) (*streampb.ListStreamsResponse, error) {
	_, span := trace.StartSpan(ctx, "stream::server::ListStreams")
	defer span.End()
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	names := make([]string, 0, len(ss.streamNames))
	for _, name := range ss.streamNames {
		streamState := ss.nameToStreamState[name]
		names = append(names, streamState.Stream.Name())
	}
	return &streampb.ListStreamsResponse{Names: names}, nil
}

// AddStream implements part of the StreamServiceServer.
func (ss *Server) AddStream(ctx context.Context, req *streampb.AddStreamRequest) (*streampb.AddStreamResponse, error) {
	ctx, span := trace.StartSpan(ctx, "stream::server::AddStream")
	defer span.End()
	ss.logger.Warnf("AddStream START %s", req.Name)
	defer ss.logger.Warnf("AddStream END %s", req.Name)
	// Get the peer connection
	pc, ok := rpc.ContextPeerConnection(ctx)
	if !ok {
		return nil, errors.New("can only add a stream over a WebRTC based connection")
	}

	ss.mu.Lock()
	defer ss.mu.Unlock()

	streamStateToAdd, ok := ss.nameToStreamState[req.Name]

	// return error if there is no stream for that camera
	if !ok {
		var availableStreams string
		for n := range ss.nameToStreamState {
			if availableStreams != "" {
				availableStreams += ", "
			}
			availableStreams += fmt.Sprintf("%q", n)
		}
		err := fmt.Errorf("no stream for %q, available streams: %s", req.Name, availableStreams)
		ss.logger.Error(err.Error())
		return nil, err
	}
	// return error if camera is not in resource graph
	if _, err := streamStateToAdd.Camera(); err != nil {
		return nil, err
	}

	// return error if the caller's peer connection is already being sent video data
	if _, ok := ss.activePeerStreams[pc][req.Name]; ok {
		err := errors.New("stream already active")
		ss.logger.Error(err.Error())
		return nil, err
	}
	nameToPeerState, ok := ss.activePeerStreams[pc]
	// if there is no active video data being sent, set up a callback to remove the peer connection from
	// the active streams & stop the stream from doing h264 encode if this is the last peer connection
	// subcribed to the camera's video feed
	// the callback fires when the peer connection state changes & runs the cleanup routine when the
	// peer connection is in a terminal state.
	if !ok {
		nameToPeerState = map[string]*peerState{}
		pc.OnConnectionStateChange(func(peerConnectionState webrtc.PeerConnectionState) {
			ss.logger.Debugf("%s pc.OnConnectionStateChange state: %s", req.Name, peerConnectionState)
			switch peerConnectionState {
			case webrtc.PeerConnectionStateDisconnected,
				webrtc.PeerConnectionStateFailed,
				webrtc.PeerConnectionStateClosed:

				ss.mu.Lock()
				defer ss.mu.Unlock()

				if ss.isAlive {
					// Dan: This conditional closing on `isAlive` is a hack to avoid a data
					// race. Shutting down a robot causes the PeerConnection to be closed
					// concurrently with this `stream.Server`. Thus, `stream.Server.Close` waiting
					// on the `activeBackgroundWorkers` WaitGroup can race with adding a new
					// "worker". Given `Close` is expected to `Stop` remaining streams, we can elide
					// spinning off the below goroutine.
					//
					// Given this is an existing race, I'm choosing to add to the tech
					// debt rather than architect how shutdown should holistically work. Revert this
					// change and run `TestRobotPeerConnect` (double check the test name at PR time)
					// to reproduce the race.
					ss.activeBackgroundWorkers.Add(1)
					utils.PanicCapturingGo(func() {
						defer ss.activeBackgroundWorkers.Done()
						ss.mu.Lock()
						defer ss.mu.Unlock()
						defer delete(ss.activePeerStreams, pc)
						var errs error
						for _, ps := range ss.activePeerStreams[pc] {
							errs = multierr.Combine(errs, ps.streamState.Decrement())
						}
						// We don't want to log this if the streamState was closed (as it only happens if viam-server is terminating)
						if errs != nil && !errors.Is(errs, state.ErrClosed) {
							ss.logger.Errorw("error(s) stopping the streamState", "errs", errs)
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
		ss.activePeerStreams[pc] = nameToPeerState
	}

	ps, ok := nameToPeerState[req.Name]
	// if the active peer stream doesn't have a peerState, add one containing the stream in question
	if !ok {
		ps = &peerState{streamState: streamStateToAdd}
		nameToPeerState[req.Name] = ps
	}

	guard := rutils.NewGuard(func() {
		for _, sender := range ps.senders {
			golog.Global().Infof("calling RemoveTrack on %s pc: %p", sender.Track().StreamID(), pc)
			utils.UncheckedError(pc.RemoveTrack(sender))
		}
	})
	defer guard.OnFail()

	addTrack := func(track webrtc.TrackLocal) error {
		// golog.Global().Infof("calling AddTrack on %s pc: %p", track.StreamID(), pc)
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
			ss.logger.Error(err.Error())
			return nil, err
		}
	}
	// if the stream supports audio, add the audio track
	if trackLocal, haveTrackLocal := streamStateToAdd.Stream.AudioTrackLocal(); haveTrackLocal {
		if err := addTrack(trackLocal); err != nil {
			ss.logger.Error(err.Error())
			return nil, err
		}
	}
	if err := streamStateToAdd.Increment(); err != nil {
		ss.logger.Error(err.Error())
		return nil, err
	}

	guard.Success()
	return &streampb.AddStreamResponse{}, nil
}

// RemoveStream implements part of the StreamServiceServer.
func (ss *Server) RemoveStream(ctx context.Context, req *streampb.RemoveStreamRequest) (*streampb.RemoveStreamResponse, error) {
	ctx, span := trace.StartSpan(ctx, "stream::server::RemoveStream")
	defer span.End()
	ss.logger.Warnf("RemoveStream START %s", req.Name)
	defer ss.logger.Warnf("RemoveStream END %s", req.Name)
	pc, ok := rpc.ContextPeerConnection(ctx)
	if !ok {
		return nil, errors.New("can only remove a stream over a WebRTC based connection")
	}

	ss.mu.Lock()
	defer ss.mu.Unlock()

	streamToRemove, ok := ss.nameToStreamState[req.Name]
	if !ok {
		return nil, fmt.Errorf("no stream for %q", req.Name)
	}
	// return error if camera is not in resource graph
	if _, err := streamToRemove.Camera(); err != nil {
		return nil, err
	}

	if _, ok := ss.activePeerStreams[pc][req.Name]; !ok {
		return nil, errors.New("stream already inactive")
	}

	var errs error
	for _, sender := range ss.activePeerStreams[pc][req.Name].senders {
		// golog.Global().Infof("calling RemoveTrack on %s pc: %p", sender.Track().StreamID(), pc)
		errs = multierr.Combine(errs, pc.RemoveTrack(sender))
	}
	if errs != nil {
		ss.logger.Error(errs.Error())
		return nil, errs
	}

	if err := streamToRemove.Decrement(); err != nil {
		ss.logger.Error(err.Error())
		return nil, err
	}

	delete(ss.activePeerStreams[pc], req.Name)
	return &streampb.RemoveStreamResponse{}, nil
}

// Close closes the Server and waits for spun off goroutines to complete.
func (ss *Server) Close() error {
	ss.closedFn()
	ss.mu.Lock()
	ss.isAlive = false

	var errs error
	for _, name := range ss.streamNames {
		errs = multierr.Combine(errs, ss.nameToStreamState[name].Close())
	}
	if errs != nil {
		ss.logger.Errorf("Stream Server Close > StreamState.Close() errs: %s", errs)
	}
	ss.mu.Unlock()
	ss.activeBackgroundWorkers.Wait()
	return errs
}

func (ss *Server) Camera(stream gostream.Stream) (camera.Camera, error) {
	// Stream names are slightly modified versions of the resource short name
	shortName := resource.SDPTrackNameToShortName(stream.Name())
	cam, err := camera.FromRobot(ss.r, shortName)
	if err != nil {
		return nil, err
	}
	return cam, nil
}

func (ss *Server) monitor(stream gostream.Stream) {
	streamName := stream.Name()
	ss.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		for utils.SelectContextOrWait(ss.closedCtx, time.Second) {
			golog.Global().Infof("%s tick", streamName)
			cam, err := ss.Camera(stream)
			if err == nil {
				golog.Global().Infof("%s calling Properties", cam.Name())
				p, err := cam.Properties(context.Background())
				if err != nil {
					golog.Global().Errorf("%s err: %s", cam.Name(), err.Error())
				}
				golog.Global().Infof("%#v Properties %s", p, cam.Name())
				continue
			}

			golog.Global().Errorf("stream %s has no camera err: %s", streamName, err.Error())
		}
	}, ss.activeBackgroundWorkers.Done)
}

func (ss *Server) add(stream gostream.Stream) error {
	streamName := stream.Name()
	if _, ok := ss.nameToStreamState[streamName]; ok {
		return &StreamAlreadyRegisteredError{streamName}
	}

	newStreamState := state.New(stream, ss.r, ss.logger)
	newStreamState.Init()
	ss.nameToStreamState[streamName] = newStreamState
	ss.streamNames = append(ss.streamNames, streamName)
	ss.startMonitorCameraAvailable()
	ss.monitor(stream)
	return nil
}

// startMonitorCameraAvailable monitors whether or not the camera still exists
// If it no longer exists, it:
// 1. calls RemoveTrack on all peer connection senders that
// 2. decrements the number of active peers on the stream state (which should result in the
// stream state having no subscribers and calling gostream.Stop() or rtppassthrough.Unsubscribe)
// streaming tracks from it,
func (ss *Server) startMonitorCameraAvailable() {
	ss.activeBackgroundWorkers.Add(1)
	f := func() {
		for utils.SelectContextOrWait(ss.closedCtx, monitorCameraInterval) {
			ss.monitorStreamAvailable()
		}
	}
	utils.ManagedGo(f, ss.activeBackgroundWorkers.Done)
}

func (ss *Server) monitorStreamAvailable() {
	ss.logger.Info("monitorStreamAvailable loop START")
	defer ss.logger.Info("monitorStreamAvailable loop END")
	ss.mu.Lock()
	defer ss.mu.Unlock()
	for _, streamState := range ss.nameToStreamState {
		// TODO: confirm this is the correct way to detect that the
		// camera is no longer in the resource graph
		if _, err := streamState.Camera(); err != nil {
			camName := streamState.Stream.Name()
			ss.logger.Info("monitorStreamAvailable camera doesn't exist: %s", camName)
			for pc, peerStateByCamName := range ss.activePeerStreams {
				peerState, ok := peerStateByCamName[camName]
				if ok {
					ss.logger.Info("monitorStreamAvailable unsubscribing: %s", camName)
					var errs error
					for _, sender := range peerState.senders {
						errs = multierr.Combine(errs, pc.RemoveTrack(sender))
					}
					if errs != nil {
						ss.logger.Warn(errs.Error())
					}

					if err := streamState.Decrement(); err != nil {
						ss.logger.Warn(err.Error())
					}
					delete(ss.activePeerStreams[pc], camName)
				}
			}
		}
	}
}
