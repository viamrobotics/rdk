//go:build !no_cgo

package web

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/goccy/go-graphviz"
	"github.com/pkg/errors"
	streampb "go.viam.com/api/stream/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"golang.org/x/exp/slices"

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

	newStream := func(name string) (gostream.Stream, bool, error) {
		// Configure new stream
		config := *svc.opts.streamConfig
		config.Name = name
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
		stream, alreadyRegistered, err := newStream(name)
		if err != nil {
			return err
		} else if alreadyRegistered {
			continue
		}

		svc.startVideoStream(ctx, source, stream)
	}

	for name, source := range svc.audioSources {
		stream, alreadyRegistered, err := newStream(name)
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
		noopServer, err := webstream.NewServer(streams...)
		return &StreamServer{noopServer, false}, err
	}

	addStream := func(streams []gostream.Stream, name string, isVideo bool) ([]gostream.Stream, error) {
		config := *svc.opts.streamConfig
		config.Name = name
		if isVideo {
			config.AudioEncoderFactory = nil

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
			config.VideoEncoderFactory = nil
		}
		stream, err := gostream.NewStream(config)
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

	streamServer, err := webstream.NewServer(streams...)
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
		opts := &webstream.BackoffTuningOptions{
			BaseSleep: 50 * time.Microsecond,
			MaxSleep:  2 * time.Second,
			Cooldown:  5 * time.Second,
		}
		if err := streamFunc(opts); err != nil {
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
		existing, ok := svc.videoSources[validSDPTrackName(name)]
		if ok {
			existing.Swap(cam)
			continue
		}
		newSwapper := gostream.NewHotSwappableVideoSource(cam)
		svc.videoSources[validSDPTrackName(name)] = newSwapper
	}
}

// refreshAudioSources checks and initializes every possible audio source that could be viewed from the robot.
func (svc *webService) refreshAudioSources() {
	for _, name := range audioinput.NamesFromRobot(svc.r) {
		input, err := audioinput.FromRobot(svc.r, name)
		if err != nil {
			continue
		}
		existing, ok := svc.audioSources[validSDPTrackName(name)]
		if ok {
			existing.Swap(input)
			continue
		}
		newSwapper := gostream.NewHotSwappableAudioSource(input)
		svc.audioSources[validSDPTrackName(name)] = newSwapper
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

func (svc *webService) handleVisualizeResourceGraph(w http.ResponseWriter, r *http.Request) {
	localRobot, isLocal := svc.r.(robot.LocalRobot)
	if !isLocal {
		return
	}
	const lookupParam = "history"
	redirectToLatestSnapshot := func() {
		url := *r.URL
		q := r.URL.Query()
		q.Del(lookupParam)
		url.RawQuery = q.Encode()

		http.Redirect(w, r, url.String(), http.StatusSeeOther)
	}

	lookupRawValue := strings.TrimSpace(r.URL.Query().Get(lookupParam))
	var (
		lookup int
		err    error
	)
	switch {
	case lookupRawValue == "":
		lookup = 0
	case lookupRawValue == "0":
		redirectToLatestSnapshot()
		return
	default:
		lookup, err = strconv.Atoi(lookupRawValue)
		if err != nil {
			redirectToLatestSnapshot()
			return
		}
	}

	snapshot, err := localRobot.ExportResourcesAsDot(lookup)
	if snapshot.Count == 0 {
		return
	}
	if err != nil {
		redirectToLatestSnapshot()
		return
	}

	write := func(s string) {
		//nolint:errcheck
		_, _ = w.Write([]byte(s))
	}

	layout := r.URL.Query().Get("layout")
	if layout == "text" {
		write(snapshot.Snapshot.Dot)
		return
	}

	gv := graphviz.New()
	defer func() {
		closeErr := gv.Close()
		if closeErr != nil {
			svc.r.Logger().Warn("failed to close graph visualizer")
		}
	}()

	graph, err := graphviz.ParseBytes([]byte(snapshot.Snapshot.Dot))
	if err != nil {
		return
	}
	if layout != "" {
		gv.SetLayout(graphviz.Layout(layout))
	}

	navButton := func(index int, label string) {
		url := *r.URL
		q := r.URL.Query()
		q.Set(lookupParam, strconv.Itoa(index))
		url.RawQuery = q.Encode()
		var html string
		if index < 0 || index >= snapshot.Count || index == lookup {
			html = fmt.Sprintf(`<a>%s</a>`, label)
		} else {
			html = fmt.Sprintf(`<a href=%q>%s</a>`, url.String(), label)
		}
		write(html)
	}

	// Navigation buttons
	write(`<html><div>`)
	navButton(0, "Latest")
	write(`|`)
	navButton(snapshot.Index-1, "Later")
	// Index counts from 0, but we want to show pages starting from 1
	write(fmt.Sprintf(`| %d / %d |`, snapshot.Index+1, snapshot.Count))
	navButton(snapshot.Index+1, "Earlier")
	write(`|`)
	navButton(snapshot.Count-1, "Earliest")
	write(`</div>`)

	// Snapshot capture timestamp
	write(fmt.Sprintf("<p>%s</p>", snapshot.Snapshot.CreatedAt.Format(time.UnixDate)))

	// HACK: We create a custom writer that removes the first 6 lines of XML written by
	// `gv.Render` - we exclude these lines of XML since they prevent us from adding HTML
	// elements to the rendered HTML. We depend on `gv.Render` calling fxml.Write exactly
	// one time.
	//
	// TODO(RSDK-6797): Parse the html text returned by `gv.Render` using an HTML parser
	// (https://pkg.go.dev/golang.org/x/net/html or equivalent) and remove the nodes that
	// prevent us from adding additional HTML.
	fxml := &filterXML{w: w}
	if err = gv.Render(graph, graphviz.SVG, fxml); err != nil {
		return
	}
	write(`</html>`)
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
