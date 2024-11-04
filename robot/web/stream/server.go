package webstream

import (
	"context"
	"fmt"
	"image"
	"runtime"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/viamrobotics/webrtc/v3"
	"go.opencensus.io/trace"
	"go.uber.org/multierr"
	streampb "go.viam.com/api/stream/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/audioinput"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	streamCamera "go.viam.com/rdk/robot/web/stream/camera"
	"go.viam.com/rdk/robot/web/stream/state"
	rutils "go.viam.com/rdk/utils"
)

const (
	frameTimeout          = 200 * time.Millisecond
	monitorCameraInterval = time.Second
)

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

	streamConfig gostream.StreamConfig
	videoSources map[string]gostream.HotSwappableVideoSource
	audioSources map[string]gostream.HotSwappableAudioSource
}

// resolution holds the width and height of a video stream.
// We use int32 to match the resolution type in the proto.
type resolution struct {
	width, height int32
}

// NewServer returns a server that will run on the given port and initially starts with the given
// stream.
func NewServer(
	robot robot.Robot,
	streamConfig gostream.StreamConfig,
	logger logging.Logger,
) *Server {
	closedCtx, closedFn := context.WithCancel(context.Background())
	server := &Server{
		closedCtx:         closedCtx,
		closedFn:          closedFn,
		robot:             robot,
		logger:            logger,
		nameToStreamState: map[string]*state.StreamState{},
		activePeerStreams: map[*webrtc.PeerConnection]map[string]*peerState{},
		isAlive:           true,
		streamConfig:      streamConfig,
		videoSources:      map[string]gostream.HotSwappableVideoSource{},
		audioSources:      map[string]gostream.HotSwappableAudioSource{},
	}
	server.startMonitorCameraAvailable()
	return server
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

	// return error if the stream name is not registered
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

	// return error if resource is neither a camera nor audioinput
	_, isCamErr := streamCamera.Camera(server.robot, streamStateToAdd.Stream)
	_, isAudioErr := audioinput.FromRobot(server.robot, resource.SDPTrackNameToShortName(streamStateToAdd.Stream.Name()))
	if isCamErr != nil && isAudioErr != nil {
		return nil, errors.Errorf("stream is neither a camera nor audioinput. streamName: %v", streamStateToAdd.Stream)
	}

	// return error if the caller's peer connection is already being sent stream data
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

	shortName := resource.SDPTrackNameToShortName(streamToRemove.Stream.Name())
	_, isAudioResourceErr := audioinput.FromRobot(server.robot, shortName)
	_, isCameraResourceErr := streamCamera.Camera(server.robot, streamToRemove.Stream)

	if isAudioResourceErr != nil && isCameraResourceErr != nil {
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

// GetStreamOptions implements part of the StreamServiceServer. It returns the available dynamic resolutions
// for a given stream name. The resolutions are scaled down from the original resolution in the camera
// properties.
func (server *Server) GetStreamOptions(
	ctx context.Context,
	req *streampb.GetStreamOptionsRequest,
) (*streampb.GetStreamOptionsResponse, error) {
	if req.Name == "" {
		return nil, errors.New("stream name is required")
	}
	cam, err := camera.FromRobot(server.robot, req.Name)
	if err != nil {
		return nil, err
	}
	// If the camera properties do not have intrinsic parameters,
	// we need to sample a frame to get the width and height.
	var width, height int
	camProps, err := cam.Properties(ctx)
	if err != nil || camProps.IntrinsicParams == nil || camProps.IntrinsicParams.Width == 0 || camProps.IntrinsicParams.Height == 0 {
		server.logger.Debug("width and height not found in camera properties")
		width, height, err = server.sampleFrameSize(ctx, cam)
		if err != nil {
			return nil, fmt.Errorf("failed to sample frame size: %w", err)
		}
	} else {
		width, height = camProps.IntrinsicParams.Width, camProps.IntrinsicParams.Height
	}
	scaledResolutions := server.generateResolutions(int32(width), int32(height))
	resolutions := make([]*streampb.Resolution, 0, 5)
	for _, res := range scaledResolutions {
		resolutions = append(resolutions, &streampb.Resolution{
			Height: res.height,
			Width:  res.width,
		})
	}
	return &streampb.GetStreamOptionsResponse{
		Resolutions: resolutions,
	}, nil
}

// generateResolutions takes the original width and height of an image and returns
// a list of the original resolution with 4 smaller width/height options that maintain
// the same aspect ratio.
func (server *Server) generateResolutions(width, height int32) []resolution {
	resolutions := []resolution{
		{width: width, height: height},
	}
	// We use integer division to get the scaled width and height. Fractions are truncated
	// to the nearest integer. This means that the scaled width and height may not match the
	// original aspect ratio exactly if source dimensions are odd.
	for i := 0; i < 4; i++ {
		width /= 2
		height /= 2
		resolutions = append(resolutions, resolution{width: width, height: height})
		server.logger.Debugf("scaled resolution %d: %dx%d", i, width, height)
	}
	return resolutions
}

// sampleFrameSize takes in a camera.Camera, starts a stream, attempts to
// pull a frame using Stream.Next, and returns the width and height.
func (server *Server) sampleFrameSize(ctx context.Context, cam camera.Camera) (int, int, error) {
	server.logger.Debug("sampling frame size")
	stream, err := cam.Stream(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer func() {
		if cerr := stream.Close(ctx); cerr != nil {
			server.logger.Error("failed to close stream:", cerr)
		}
	}()
	// Attempt to get a frame from the stream with a maximum of 5 retries.
	// Each attempt waits for a maximum of 200ms before timing out.
	var frame image.Image
	var release func()
	for i := 0; i < 5; i++ {
		timeoutCtx, cancel := context.WithTimeout(ctx, frameTimeout)
		frame, release, err = stream.Next(timeoutCtx)
		cancel() // Do not defer cancel to avoid context leak.
		if err == nil {
			break
		}
		server.logger.Warnf("failed to get frame, retrying... (%d/5)", i+1)
	}
	if release != nil {
		defer release()
	}
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get frame after 3 attempts: %w", err)
	}
	return frame.Bounds().Dx(), frame.Bounds().Dy(), nil
}

// AddNewStreams adds new video and audio streams to the server using the updated set of video and
// audio sources. It refreshes the sources, checks for a valid stream configuration, and starts
// the streams if applicable.
func (server *Server) AddNewStreams(ctx context.Context) error {
	// Refreshing sources will walk the robot resources for anything implementing the camera and
	// audioinput APIs and mutate the `svc.videoSources` and `svc.audioSources` maps.
	server.refreshVideoSources()
	server.refreshAudioSources()

	if server.streamConfig == (gostream.StreamConfig{}) {
		// The `streamConfig` dictates the video and audio encoder libraries to use. We can't do
		// much if none are present.
		if len(server.videoSources) != 0 || len(server.audioSources) != 0 {
			server.logger.Warn("not starting streams due to no stream config being set")
		}
		return nil
	}

	for name := range server.videoSources {
		if runtime.GOOS == "windows" {
			// TODO(RSDK-1771): support video on windows
			server.logger.Warn("video streaming not supported on Windows yet")
			break
		}
		// We walk the updated set of `videoSources` and ensure all of the sources are "created" and
		// "started".
		config := gostream.StreamConfig{
			Name:                name,
			VideoEncoderFactory: server.streamConfig.VideoEncoderFactory,
		}
		// Call `createStream`. `createStream` is responsible for first checking if the stream
		// already exists. If it does, it skips creating a new stream and we continue to the next source.
		//
		// TODO(RSDK-9079) Add reliable framerate fetcher for stream videosources
		stream, alreadyRegistered, err := server.createStream(config, name)
		if err != nil {
			return err
		} else if alreadyRegistered {
			continue
		}
		server.startVideoStream(ctx, server.videoSources[name], stream)
	}

	for name := range server.audioSources {
		// Similarly, we walk the updated set of `audioSources` and ensure all of the audio sources
		// are "created" and "started". `createStream` and `startAudioStream` have the same
		// behaviors as described above for video streams.
		config := gostream.StreamConfig{
			Name:                name,
			AudioEncoderFactory: server.streamConfig.AudioEncoderFactory,
		}
		stream, alreadyRegistered, err := server.createStream(config, name)
		if err != nil {
			return err
		} else if alreadyRegistered {
			continue
		}
		server.startAudioStream(ctx, server.audioSources[name], stream)
	}

	return nil
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
		if _, err := audioinput.FromRobot(server.robot, shortName); err == nil {
			// `nameToStreamState` can contain names for both camera and audio resources. Leave the
			// stream in place if its an audio resource.
			continue
		}

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

// refreshVideoSources checks and initializes every possible video source that could be viewed from the robot.
func (server *Server) refreshVideoSources() {
	for _, name := range camera.NamesFromRobot(server.robot) {
		cam, err := camera.FromRobot(server.robot, name)
		if err != nil {
			continue
		}
		existing, ok := server.videoSources[cam.Name().SDPTrackName()]
		if ok {
			existing.Swap(cam)
			continue
		}
		newSwapper := gostream.NewHotSwappableVideoSource(cam)
		server.videoSources[cam.Name().SDPTrackName()] = newSwapper
	}
}

// refreshAudioSources checks and initializes every possible audio source that could be viewed from the robot.
func (server *Server) refreshAudioSources() {
	for _, name := range audioinput.NamesFromRobot(server.robot) {
		input, err := audioinput.FromRobot(server.robot, name)
		if err != nil {
			continue
		}
		existing, ok := server.audioSources[input.Name().SDPTrackName()]
		if ok {
			existing.Swap(input)
			continue
		}
		newSwapper := gostream.NewHotSwappableAudioSource(input)
		server.audioSources[input.Name().SDPTrackName()] = newSwapper
	}
}

func (server *Server) createStream(config gostream.StreamConfig, name string) (gostream.Stream, bool, error) {
	stream, err := server.NewStream(config)
	// Skip if stream is already registered, otherwise raise any other errors
	registeredError := &StreamAlreadyRegisteredError{}
	if errors.As(err, &registeredError) {
		server.logger.Debugf("%s stream already registered", name)
		return nil, true, nil
	} else if err != nil {
		return nil, false, err
	}
	return stream, false, err
}

func (server *Server) startStream(streamFunc func(opts *BackoffTuningOptions) error) {
	waitCh := make(chan struct{})
	server.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer server.activeBackgroundWorkers.Done()
		close(waitCh)
		if err := streamFunc(&BackoffTuningOptions{}); err != nil {
			if utils.FilterOutError(err, context.Canceled) != nil {
				server.logger.Errorw("error streaming", "error", err)
			}
		}
	})
	<-waitCh
}

func (server *Server) startVideoStream(ctx context.Context, source gostream.VideoSource, stream gostream.Stream) {
	server.startStream(func(opts *BackoffTuningOptions) error {
		streamVideoCtx, _ := utils.MergeContext(server.closedCtx, ctx)
		return streamVideoSource(streamVideoCtx, source, stream, opts, server.logger)
	})
}

func (server *Server) startAudioStream(ctx context.Context, source gostream.AudioSource, stream gostream.Stream) {
	server.startStream(func(opts *BackoffTuningOptions) error {
		// Merge ctx that may be coming from a Reconfigure.
		streamAudioCtx, _ := utils.MergeContext(server.closedCtx, ctx)
		return streamAudioSource(streamAudioCtx, source, stream, opts, server.logger)
	})
}
