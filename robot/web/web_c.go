//go:build !no_cgo || android

package web

import (
	"bytes"
	"context"
	"math"
	"net/http"
	"runtime"
	"slices"
	"sync"

	"github.com/pkg/errors"
	streampb "go.viam.com/api/stream/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/audioinput"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	weboptions "go.viam.com/rdk/robot/web/options"
	webstream "go.viam.com/rdk/robot/web/stream"
	rutils "go.viam.com/rdk/utils"
)

// StreamServer manages streams and displays.
type StreamServer struct {
	// Server serves streams
	Server *webstream.Server
	// HasStreams is true if service has streams that require a WebRTC connection.
	HasStreams bool
}

// New returns a new web service for the given robot.
func New(r robot.Robot, logger logging.Logger, opts ...Option) Service {
	var wOpts options
	for _, opt := range opts {
		opt.apply(&wOpts)
	}
	webSvc := &webService{
		Named:        InternalServiceName.AsNamed(),
		r:            r,
		logger:       logger,
		rpcServer:    nil,
		streamServer: nil,
		services:     map[resource.API]resource.APIResourceCollection[resource.Resource]{},
		opts:         wOpts,
		videoSources: map[string]gostream.HotSwappableVideoSource{},
		audioSources: map[string]gostream.HotSwappableAudioSource{},
	}
	return webSvc
}

type webService struct {
	resource.Named

	mu           sync.Mutex
	r            robot.Robot
	rpcServer    rpc.Server
	modServer    rpc.Server
	streamServer *StreamServer
	services     map[resource.API]resource.APIResourceCollection[resource.Resource]
	opts         options
	addr         string
	modAddr      string
	logger       logging.Logger
	cancelCtx    context.Context
	cancelFunc   func()
	isRunning    bool
	webWorkers   sync.WaitGroup
	modWorkers   sync.WaitGroup

	videoSources map[string]gostream.HotSwappableVideoSource
	audioSources map[string]gostream.HotSwappableAudioSource
}

func (svc *webService) streamInitialized() bool {
	return svc.streamServer != nil && svc.streamServer.Server != nil
}

func (svc *webService) addNewStreams(ctx context.Context) error {
	if !svc.streamInitialized() {
		return nil
	}
	svc.refreshVideoSources()
	svc.refreshAudioSources()
	if svc.opts.streamConfig == nil {
		if len(svc.videoSources) != 0 || len(svc.audioSources) != 0 {
			svc.logger.Debug("not starting streams due to no stream config being set")
		}
		return nil
	}

	newStream := func(name string, isVideo bool) (gostream.Stream, bool, error) {
		// Configure new stream
		config := gostream.StreamConfig{
			Name: name,
		}

		if isVideo {
			config.VideoEncoderFactory = svc.opts.streamConfig.VideoEncoderFactory
		} else {
			config.AudioEncoderFactory = svc.opts.streamConfig.AudioEncoderFactory
		}
		stream, err := svc.streamServer.Server.NewStream(config)

		// Skip if stream is already registered, otherwise raise any other errors
		var registeredError *webstream.StreamAlreadyRegisteredError
		if errors.As(err, &registeredError) {
			return nil, true, nil
		} else if err != nil {
			return nil, false, err
		}

		if !svc.streamServer.HasStreams {
			svc.streamServer.HasStreams = true
		}
		return stream, false, nil
	}

	for name, source := range svc.videoSources {
		const isVideo = true
		stream, alreadyRegistered, err := newStream(name, isVideo)
		if err != nil {
			return err
		} else if alreadyRegistered {
			continue
		}

		svc.startVideoStream(ctx, source, stream)
	}

	for name, source := range svc.audioSources {
		const isVideo = false
		stream, alreadyRegistered, err := newStream(name, isVideo)
		if err != nil {
			return err
		} else if alreadyRegistered {
			continue
		}

		svc.startAudioStream(ctx, source, stream)
	}

	return nil
}

func (svc *webService) makeStreamServer(ctx context.Context) (*StreamServer, error) {
	svc.refreshVideoSources()
	svc.refreshAudioSources()
	var streams []gostream.Stream
	var streamTypes []bool

	if svc.opts.streamConfig == nil || (len(svc.videoSources) == 0 && len(svc.audioSources) == 0) {
		if len(svc.videoSources) != 0 || len(svc.audioSources) != 0 {
			svc.logger.Debug("not starting streams due to no stream config being set")
		}
		noopServer, err := webstream.NewServer(streams, svc.r, svc.logger.Sublogger("stream"))
		return &StreamServer{noopServer, false}, err
	}

	addStream := func(streams []gostream.Stream, name string, isVideo bool) ([]gostream.Stream, error) {
		config := gostream.StreamConfig{
			Name: name,
		}
		if isVideo {
			config.VideoEncoderFactory = svc.opts.streamConfig.VideoEncoderFactory

			// set TargetFrameRate to the framerate of the video source if available
			props, err := svc.videoSources[name].MediaProperties(ctx)
			if err != nil {
				svc.logger.Warnw("failed to get video source properties", "name", name, "error", err)
			} else if props.FrameRate > 0.0 {
				// round float up to nearest int
				config.TargetFrameRate = int(math.Ceil(float64(props.FrameRate)))
			}
			// default to 60fps if the video source doesn't have a framerate
			if config.TargetFrameRate == 0 {
				config.TargetFrameRate = 60
			}

			if runtime.GOOS == "windows" {
				// TODO(RSDK-1771): support video on windows
				svc.logger.Warnw("not starting video stream since not supported on Windows yet", "name", name)
				return streams, nil
			}
		} else {
			config.AudioEncoderFactory = svc.opts.streamConfig.AudioEncoderFactory
		}
		stream, err := gostream.NewStream(config, svc.logger)
		if err != nil {
			return streams, err
		}
		return append(streams, stream), nil
	}
	for name := range svc.videoSources {
		var err error
		streams, err = addStream(streams, name, true)
		if err != nil {
			return nil, err
		}
		streamTypes = append(streamTypes, true)
	}
	for name := range svc.audioSources {
		var err error
		streams, err = addStream(streams, name, false)
		if err != nil {
			return nil, err
		}
		streamTypes = append(streamTypes, false)
	}

	streamServer, err := webstream.NewServer(streams, svc.r, svc.logger.Sublogger("stream"))
	if err != nil {
		return nil, err
	}

	for idx, stream := range streams {
		if streamTypes[idx] {
			svc.startVideoStream(ctx, svc.videoSources[stream.Name()], stream)
		} else {
			svc.startAudioStream(ctx, svc.audioSources[stream.Name()], stream)
		}
	}

	return &StreamServer{streamServer, true}, nil
}

func (svc *webService) startStream(streamFunc func(opts *webstream.BackoffTuningOptions) error) {
	waitCh := make(chan struct{})
	svc.webWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer svc.webWorkers.Done()
		close(waitCh)
		if err := streamFunc(&webstream.BackoffTuningOptions{}); err != nil {
			if utils.FilterOutError(err, context.Canceled) != nil {
				svc.logger.Errorw("error streaming", "error", err)
			}
		}
	})
	<-waitCh
}

func (svc *webService) propertiesFromStream(ctx context.Context, stream gostream.Stream) (camera.Properties, error) {
	res, err := svc.r.ResourceByName(camera.Named(stream.Name()))
	if err != nil {
		return camera.Properties{}, err
	}

	cam, ok := res.(camera.Camera)
	if !ok {
		return camera.Properties{}, errors.Errorf("cannot convert resource (type %T) to type (%T)", res, camera.Camera(nil))
	}

	return cam.Properties(ctx)
}

func (svc *webService) startVideoStream(ctx context.Context, source gostream.VideoSource, stream gostream.Stream) {
	svc.startStream(func(opts *webstream.BackoffTuningOptions) error {
		streamVideoCtx, _ := utils.MergeContext(svc.cancelCtx, ctx)
		// Use H264 for cameras that support it; but do not override upstream values.
		if props, err := svc.propertiesFromStream(ctx, stream); err == nil && slices.Contains(props.MimeTypes, rutils.MimeTypeH264) {
			streamVideoCtx = gostream.WithMIMETypeHint(streamVideoCtx, rutils.WithLazyMIMEType(rutils.MimeTypeH264))
		}

		return webstream.StreamVideoSource(streamVideoCtx, source, stream, opts, svc.logger)
	})
}

func (svc *webService) startAudioStream(ctx context.Context, source gostream.AudioSource, stream gostream.Stream) {
	svc.startStream(func(opts *webstream.BackoffTuningOptions) error {
		// Merge ctx that may be coming from a Reconfigure.
		streamAudioCtx, _ := utils.MergeContext(svc.cancelCtx, ctx)
		return webstream.StreamAudioSource(streamAudioCtx, source, stream, opts, svc.logger)
	})
}

// refreshVideoSources checks and initializes every possible video source that could be viewed from the robot.
func (svc *webService) refreshVideoSources() {
	for _, name := range camera.NamesFromRobot(svc.r) {
		cam, err := camera.FromRobot(svc.r, name)
		if err != nil {
			continue
		}
		existing, ok := svc.videoSources[cam.Name().SDPTrackName()]
		if ok {
			existing.Swap(cam)
			continue
		}
		newSwapper := gostream.NewHotSwappableVideoSource(cam)
		svc.videoSources[cam.Name().SDPTrackName()] = newSwapper
	}
}

// refreshAudioSources checks and initializes every possible audio source that could be viewed from the robot.
func (svc *webService) refreshAudioSources() {
	for _, name := range audioinput.NamesFromRobot(svc.r) {
		input, err := audioinput.FromRobot(svc.r, name)
		if err != nil {
			continue
		}
		existing, ok := svc.audioSources[input.Name().SDPTrackName()]
		if ok {
			existing.Swap(input)
			continue
		}
		newSwapper := gostream.NewHotSwappableAudioSource(input)
		svc.audioSources[input.Name().SDPTrackName()] = newSwapper
	}
}

// Update updates the web service when the robot has changed.
func (svc *webService) Reconfigure(ctx context.Context, deps resource.Dependencies, _ resource.Config) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if err := svc.updateResources(deps); err != nil {
		return err
	}
	if !svc.isRunning {
		return nil
	}
	return svc.addNewStreams(svc.cancelCtx)
}

func (svc *webService) closeStreamServer() {
	if svc.streamServer.Server != nil {
		if err := svc.streamServer.Server.Close(); err != nil {
			svc.logger.Errorw("error closing stream server", "error", err)
		}
	}
}

func (svc *webService) initStreamServer(ctx context.Context, options *weboptions.Options) error {
	var err error
	svc.streamServer, err = svc.makeStreamServer(ctx)
	if err != nil {
		return err
	}
	if err := svc.rpcServer.RegisterServiceServer(
		ctx,
		&streampb.StreamService_ServiceDesc,
		svc.streamServer.Server,
		streampb.RegisterStreamServiceHandlerFromEndpoint,
	); err != nil {
		return err
	}
	if svc.streamServer.HasStreams {
		// force WebRTC template rendering
		options.WebRTC = true
	}
	return nil
}

type filterXML struct {
	called bool
	w      http.ResponseWriter
}

func (fxml *filterXML) Write(bs []byte) (int, error) {
	if fxml.called {
		return 0, errors.New("cannot write more than once")
	}
	lines := bytes.Split(bs, []byte("\n"))
	// HACK: these lines are XML Document Type Definition strings
	lines = lines[6:]
	bs = bytes.Join(lines, []byte("\n"))
	n, err := fxml.w.Write(bs)
	if err == nil {
		fxml.called = true
	}
	return n, err
}
