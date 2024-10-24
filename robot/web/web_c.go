//go:build !no_cgo || android

package web

import (
	"bytes"
	"context"
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
	webstream "go.viam.com/rdk/robot/web/stream"
	rutils "go.viam.com/rdk/utils"
)

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
	streamServer *webstream.Server
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
	return svc.streamServer != nil

}

func (svc *webService) addNewStreams(ctx context.Context) error {
	if !svc.streamInitialized() {
		svc.logger.Warn("not starting streams because the stream server is not initialized")
		return nil
	}

	// Refreshing sources will walk the robot resources for anything implementing the camera and
	// audioinput APIs and mutate the `svc.videoSources` and `svc.audioSources` maps.
	svc.refreshVideoSources()
	svc.refreshAudioSources()

	if svc.opts.streamConfig == nil {
		// The `streamConfig` dictates the video and audio encoder libraries to use. We can't do
		// much if none are present.
		if len(svc.videoSources) != 0 || len(svc.audioSources) != 0 {
			svc.logger.Warn("not starting streams due to no stream config being set")
		}
		return nil
	}

	for name := range svc.videoSources {
		if runtime.GOOS == "windows" {
			// TODO(RSDK-1771): support video on windows
			svc.logger.Warn("video streaming not supported on Windows yet")
			break
		}
		// We walk the updated set of `videoSources` and ensure all of the sources are "created" and
		// "started".
		config := gostream.StreamConfig{
			Name:                name,
			VideoEncoderFactory: svc.opts.streamConfig.VideoEncoderFactory,
		}
		// Call `createStream`. `createStream` is responsible for first checking if the stream
		// already exists. If it does, it skips creating a new stream and we continue to the next source.
		//
		// TODO(RSDK-9079) Add reliable framerate fetcher for stream videosources
		stream, alreadyRegistered, err := svc.createStream(config, name)
		if err != nil {
			return err
		} else if alreadyRegistered {
			continue
		}
		svc.startVideoStream(ctx, svc.videoSources[name], stream)
	}

	for name := range svc.audioSources {
		// Similarly, we walk the updated set of `audioSources` and ensure all of the audio sources
		// are "created" and "started". `createStream` and `startAudioStream` have the same
		// behaviors as described above for video streams.
		config := gostream.StreamConfig{
			Name:                name,
			AudioEncoderFactory: svc.opts.streamConfig.AudioEncoderFactory,
		}
		stream, alreadyRegistered, err := svc.createStream(config, name)
		if err != nil {
			return err
		} else if alreadyRegistered {
			continue
		}
		svc.startAudioStream(ctx, svc.audioSources[name], stream)
	}

	return nil
}

func (svc *webService) createStream(config gostream.StreamConfig, name string) (gostream.Stream, bool, error) {
	stream, err := svc.streamServer.NewStream(config)
	// Skip if stream is already registered, otherwise raise any other errors
	var registeredError *webstream.StreamAlreadyRegisteredError
	if errors.As(err, &registeredError) {
		svc.logger.Debugw("stream already registered", "name", name)
		return nil, true, nil
	} else if err != nil {
		return nil, false, err
	}
	return stream, false, err
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
	if svc.streamInitialized() {
		if err := svc.streamServer.Close(); err != nil {
			svc.logger.Errorw("error closing stream server", "error", err)
		}
	}
}

func (svc *webService) initStreamServer(ctx context.Context) error {
	svc.streamServer = webstream.NewServer(svc.r, svc.logger)
	if err := svc.addNewStreams(ctx); err != nil {
		return err
	}
	if err := svc.rpcServer.RegisterServiceServer(
		ctx,
		&streampb.StreamService_ServiceDesc,
		svc.streamServer,
		streampb.RegisterStreamServiceHandlerFromEndpoint,
	); err != nil {
		return err
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
