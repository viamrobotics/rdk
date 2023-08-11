// Package web provides gRPC/REST/GUI APIs to control and monitor a robot.
package web

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"math"
	"net"
	"net/http"
	"net/http/pprof"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Masterminds/sprig"
	"github.com/NYTimes/gziphandler"
	"github.com/edaniels/golog"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/pkg/errors"
	"github.com/rs/cors"
	"github.com/viamrobotics/gostream"
	streampb "github.com/viamrobotics/gostream/proto/stream/v1"
	"go.opencensus.io/trace"
	pb "go.viam.com/api/robot/v1"
	"go.viam.com/utils"
	echopb "go.viam.com/utils/proto/rpc/examples/echo/v1"
	"go.viam.com/utils/rpc"
	echoserver "go.viam.com/utils/rpc/examples/echo/server"
	"goji.io"
	"goji.io/pat"
	googlegrpc "google.golang.org/grpc"

	"go.viam.com/rdk/components/audioinput"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	grpcserver "go.viam.com/rdk/robot/server"
	weboptions "go.viam.com/rdk/robot/web/options"
	webstream "go.viam.com/rdk/robot/web/stream"
	rutils "go.viam.com/rdk/utils"
	"go.viam.com/rdk/web"
)

// SubtypeName is a constant that identifies the internal web resource subtype string.
const SubtypeName = "web"

// API is the fully qualified API for the internal web service.
var API = resource.APINamespaceRDKInternal.WithServiceType(SubtypeName)

// InternalServiceName is used to refer to/depend on this service internally.
var InternalServiceName = resource.NewName(API, "builtin")

// defaultMethodTimeout is the default context timeout for all inbound gRPC
// methods used when no deadline is set on the context.
var defaultMethodTimeout = 10 * time.Minute

// robotWebApp hosts a web server to interact with a robot in addition to hosting
// a gRPC/REST server.
type robotWebApp struct {
	template *template.Template
	theRobot robot.Robot
	logger   golog.Logger
	options  weboptions.Options
}

// Init does template initialization work.
func (app *robotWebApp) Init() error {
	var err error

	t := template.New("foo").Funcs(template.FuncMap{
		//nolint:gosec
		"jsSafe": func(js string) template.JS {
			return template.JS(js)
		},
		//nolint:gosec
		"htmlSafe": func(html string) template.HTML {
			return template.HTML(html)
		},
	}).Funcs(sprig.FuncMap())

	if app.options.SharedDir != "" {
		t, err = t.ParseGlob(fmt.Sprintf("%s/*.html", app.options.SharedDir+"/templates"))
	} else {
		t, err = t.ParseFS(web.AppFS, "runtime-shared/templates/*.html")
	}

	if err != nil {
		return err
	}
	app.template = t.Lookup("webappindex.html")
	return nil
}

// AppTemplateData is used to render the remote control page.
type AppTemplateData struct {
	WebRTCEnabled          bool                   `json:"webrtc_enabled"`
	WebRTCSignalingAddress string                 `json:"webrtc_signaling_address"`
	Env                    string                 `json:"env"`
	Host                   string                 `json:"host"`
	StaticHost             string                 `json:"static_host"`
	SupportedAuthTypes     []string               `json:"supported_auth_types"`
	AuthEntity             string                 `json:"auth_entity"`
	BakedAuth              map[string]interface{} `json:"baked_auth"`
}

// ServeHTTP serves the UI.
func (app *robotWebApp) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if true {
		err := app.Init()
		if err != nil {
			app.logger.Debugf("couldn't reload template: %s", err)
			return
		}
	}

	var data AppTemplateData
	data.StaticHost = app.options.StaticHost

	if err := r.ParseForm(); err != nil {
		app.logger.Debugw("failed to parse form", "error", err)
	}

	if os.Getenv("ENV") == "development" {
		data.Env = "development"
	} else {
		data.Env = "production"
	}

	data.Host = app.options.FQDN
	if app.options.WebRTC && r.Form.Get("grpc") != "true" {
		data.WebRTCEnabled = true
	}

	if app.options.Managed && hasManagedAuthHandlers(app.options.Auth.Handlers) {
		data.BakedAuth = map[string]interface{}{
			"authEntity": app.options.BakedAuthEntity,
			"creds":      app.options.BakedAuthCreds,
		}
	} else {
		for _, handler := range app.options.Auth.Handlers {
			data.SupportedAuthTypes = append(data.SupportedAuthTypes, string(handler.Type))
		}
	}

	err := app.template.Execute(w, data)
	if err != nil {
		app.logger.Debugf("couldn't execute web page: %s", err)
	}
}

// Two known auth handlers (LocationSecret, WebOauth).
func hasManagedAuthHandlers(handlers []config.AuthHandlerConfig) bool {
	hasLocationSecretHandler := false
	for _, h := range handlers {
		if h.Type == rutils.CredentialsTypeRobotLocationSecret {
			hasLocationSecretHandler = true
		}
	}

	if len(handlers) == 1 && hasLocationSecretHandler {
		return true
	}

	return false
}

// validSDPTrackName returns a valid SDP video/audio track name as defined in RFC 4566 (https://www.rfc-editor.org/rfc/rfc4566)
// where track names should not include colons.
func validSDPTrackName(name string) string {
	return strings.ReplaceAll(name, ":", "+")
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

// A Service controls the web server for a robot.
type Service interface {
	resource.Resource

	// Start starts the web server
	Start(context.Context, weboptions.Options) error

	// Stop stops the main web service (but leaves module server socket running.)
	Stop()

	// StartModule starts the module server socket.
	StartModule(context.Context) error

	// Returns the address and port the web service listens on.
	Address() string

	// Returns the unix socket path the module server listens on.
	ModuleAddress() string
}

// StreamServer manages streams and displays.
type StreamServer struct {
	// Server serves streams
	Server gostream.StreamServer
	// HasStreams is true if service has streams that require a WebRTC connection.
	HasStreams bool
}

// New returns a new web service for the given robot.
func New(r robot.Robot, logger golog.Logger, opts ...Option) Service {
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

	mu                      sync.Mutex
	r                       robot.Robot
	rpcServer               rpc.Server
	modServer               rpc.Server
	streamServer            *StreamServer
	services                map[resource.API]resource.APIResourceCollection[resource.Resource]
	opts                    options
	addr                    string
	modAddr                 string
	logger                  golog.Logger
	cancelCtx               context.Context
	cancelFunc              func()
	isRunning               bool
	activeBackgroundWorkers sync.WaitGroup

	videoSources map[string]gostream.HotSwappableVideoSource
	audioSources map[string]gostream.HotSwappableAudioSource
}

var internalWebServiceName = resource.NewName(
	resource.APINamespaceRDKInternal.WithServiceType("web"),
	"builtin",
)

func (svc *webService) Name() resource.Name {
	return internalWebServiceName
}

// Start starts the web server, will return an error if server is already up.
func (svc *webService) Start(ctx context.Context, o weboptions.Options) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if svc.isRunning {
		return errors.New("web server already started")
	}
	svc.isRunning = true
	cancelCtx, cancelFunc := context.WithCancel(ctx)

	svc.cancelCtx = cancelCtx
	svc.cancelFunc = cancelFunc

	if err := svc.runWeb(svc.cancelCtx, o); err != nil {
		if svc.cancelFunc != nil {
			svc.cancelFunc()
		}
		svc.isRunning = false
		return err
	}
	return nil
}

// RunWeb starts the web server on the robot with web options and blocks until we cancel the context.
func RunWeb(ctx context.Context, r robot.LocalRobot, o weboptions.Options, logger golog.Logger) (err error) {
	defer func() {
		if err != nil {
			err = utils.FilterOutError(err, context.Canceled)
			if err != nil {
				logger.Errorw("error running web", "error", err)
			}
		}
	}()

	if err := r.StartWeb(ctx, o); err != nil {
		return err
	}
	<-ctx.Done()
	return ctx.Err()
}

// RunWebWithConfig starts the web server on the robot with a robot config and blocks until we cancel the context.
func RunWebWithConfig(ctx context.Context, r robot.LocalRobot, cfg *config.Config, logger golog.Logger) error {
	o, err := weboptions.FromConfig(cfg)
	if err != nil {
		return err
	}
	return RunWeb(ctx, r, o, logger)
}

// Address returns the address the service is listening on.
func (svc *webService) Address() string {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	return svc.addr
}

// ModuleAddress returns the unix socket path the module server is listening on.
func (svc *webService) ModuleAddress() string {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	return svc.modAddr
}

// StartModule starts the grpc module server.
func (svc *webService) StartModule(ctx context.Context) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if svc.modServer != nil {
		return errors.New("module service already started")
	}

	var lis net.Listener
	var addr string
	if err := module.MakeSelfOwnedFilesFunc(func() error {
		dir, err := os.MkdirTemp("", "viam-module-*")
		if err != nil {
			return errors.WithMessage(err, "module startup failed")
		}
		addr = filepath.ToSlash(filepath.Join(dir, "parent.sock"))
		if runtime.GOOS == "windows" {
			// on windows, we need to craft a good enough looking URL for gRPC which
			// means we need to take out the volume which will have the current drive
			// be used. In a client server relationship for windows dialing, this must
			// be known. That is, if this is a multi process UDS, then for the purposes
			// of dialing without any resolver modifications to gRPC, they must initially
			// agree on using the same drive.
			addr = addr[2:]
		}
		if err := module.CheckSocketAddressLength(addr); err != nil {
			return err
		}
		svc.modAddr = addr
		lis, err = net.Listen("unix", addr)
		if err != nil {
			return errors.WithMessage(err, "failed to listen")
		}
		return nil
	}); err != nil {
		return err
	}
	var (
		unaryInterceptors  []googlegrpc.UnaryServerInterceptor
		streamInterceptors []googlegrpc.StreamServerInterceptor
	)

	unaryInterceptors = append(unaryInterceptors, ensureTimeoutUnaryInterceptor)

	opManager := svc.r.OperationManager()
	unaryInterceptors = append(unaryInterceptors, opManager.UnaryServerInterceptor)
	streamInterceptors = append(streamInterceptors, opManager.StreamServerInterceptor)
	// TODO(PRODUCT-343): Add session manager interceptors

	svc.modServer = module.NewServer(unaryInterceptors, streamInterceptors)
	if err := svc.modServer.RegisterServiceServer(ctx, &pb.RobotService_ServiceDesc, grpcserver.New(svc.r)); err != nil {
		return err
	}
	if err := svc.refreshResources(); err != nil {
		return err
	}
	if err := svc.initAPIResourceCollections(ctx, true); err != nil {
		return err
	}

	svc.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer svc.activeBackgroundWorkers.Done()
		svc.logger.Debugw("module server listening", "socket path", lis.Addr())
		defer utils.UncheckedErrorFunc(func() error { return os.RemoveAll(filepath.Dir(addr)) })
		if err := svc.modServer.Serve(lis); err != nil {
			svc.logger.Errorw("failed to serve module service", "error", err)
		}
	})
	return nil
}

func (svc *webService) refreshResources() error {
	resources := make(map[resource.Name]resource.Resource)
	for _, name := range svc.r.ResourceNames() {
		resource, err := svc.r.ResourceByName(name)
		if err != nil {
			continue
		}
		resources[name] = resource
	}
	return svc.updateResources(resources)
}

// Update updates the web service when the robot has changed.
func (svc *webService) Reconfigure(ctx context.Context, deps resource.Dependencies, _ resource.Config) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if err := svc.updateResources(deps); err != nil {
		return err
	}
	if svc.cancelCtx == nil {
		svc.logger.Debug("web server not started, skip web service reconfiguration")
		return nil
	}
	return svc.addNewStreams(svc.cancelCtx)
}

func (svc *webService) updateResources(resources map[resource.Name]resource.Resource) error {
	// so group resources by API
	groupedResources := make(map[resource.API]map[resource.Name]resource.Resource)
	for n, v := range resources {
		r, ok := groupedResources[n.API]
		if !ok {
			r = make(map[resource.Name]resource.Resource)
		}
		r[n] = v
		groupedResources[n.API] = r
	}

	apiRegs := resource.RegisteredAPIs()
	for a, v := range groupedResources {
		apiResColl, ok := svc.services[a]
		// TODO(RSDK-144): register new service if it doesn't currently exist
		if !ok {
			reg, ok := apiRegs[a]
			var apiResColl resource.APIResourceCollection[resource.Resource]
			if ok {
				apiResColl = reg.MakeEmptyCollection()
			} else {
				// Log a warning here to remind users to register their APIs. Do not warn if the resource is internal to the RDK or
				// the resource is handled by a remote with a possibly separate API registration. Modular resources will
				// have API registrations already and should not reach this point in the method.
				if a.Type.Namespace != resource.APINamespaceRDKInternal {
					for n := range v {
						if !n.ContainsRemoteNames() {
							svc.logger.Warnw(
								"missing registration for api, resources with this API will be unreachable through a client", "api", n.API)
							break
						}
					}
				}
				continue
			}

			if err := apiResColl.ReplaceAll(v); err != nil {
				return err
			}
			svc.services[a] = apiResColl
		} else {
			if err := apiResColl.ReplaceAll(v); err != nil {
				return err
			}
		}
	}

	return nil
}

// Stop stops the main web service prior to actually closing (it leaves the module server running.)
func (svc *webService) Stop() {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if svc.cancelFunc != nil {
		svc.cancelFunc()
	}
	svc.isRunning = false
}

// Close closes a webService via calls to its Cancel func.
func (svc *webService) Close(ctx context.Context) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	var err error
	if svc.cancelFunc != nil {
		svc.cancelFunc()
	}
	svc.isRunning = false
	if svc.modServer != nil {
		err = svc.modServer.Stop()
	}
	svc.activeBackgroundWorkers.Wait()
	return err
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
		var registeredError *gostream.StreamAlreadyRegisteredError
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
		noopServer, err := gostream.NewStreamServer(streams...)
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

	streamServer, err := gostream.NewStreamServer(streams...)
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
	svc.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer svc.activeBackgroundWorkers.Done()
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

func (svc *webService) startVideoStream(ctx context.Context, source gostream.VideoSource, stream gostream.Stream) {
	// Honor ctx that may be coming from a Reconfigure.
	ctxWithJPEGHint := gostream.WithMIMETypeHint(ctx, rutils.WithLazyMIMEType(rutils.MimeTypeJPEG))
	svc.startStream(func(opts *webstream.BackoffTuningOptions) error {
		streamVideoCtx, _ := utils.MergeContext(svc.cancelCtx, ctxWithJPEGHint)
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

// installWeb prepares the given mux to be able to serve the UI for the robot.
func (svc *webService) installWeb(mux *goji.Mux, theRobot robot.Robot, options weboptions.Options) error {
	app := &robotWebApp{theRobot: theRobot, logger: svc.logger, options: options}
	if err := app.Init(); err != nil {
		return err
	}

	var staticDir http.FileSystem
	if app.options.SharedDir != "" {
		staticDir = http.Dir(app.options.SharedDir + "/static")
	} else {
		embedFS, err := fs.Sub(web.AppFS, "runtime-shared/static")
		if err != nil {
			return err
		}
		matches, err := fs.Glob(embedFS, "*.js")
		if err != nil {
			return err
		}
		if len(matches) == 0 {
			svc.logger.Warnw("Couldn't find any static files when running RDK. Make sure to run 'make build-web' - using staticrc.viam.com")
			app.options.StaticHost = "https://staticrc.viam.com"
		}
		staticDir = http.FS(embedFS)
	}
	mux.Handle(pat.Get("/static/*"), gziphandler.GzipHandler(http.StripPrefix("/static", http.FileServer(staticDir))))
	mux.Handle(pat.New("/"), app)

	return nil
}

// runWeb takes the given robot and options and runs the web server. This function will
// block until the context is done.
func (svc *webService) runWeb(ctx context.Context, options weboptions.Options) (err error) {
	if options.Network.BindAddress != "" && options.Network.Listener != nil {
		return errors.New("may only set one of network bind address or listener")
	}
	listener := options.Network.Listener

	if listener == nil {
		listener, err = net.Listen("tcp", options.Network.BindAddress)
		if err != nil {
			return err
		}
	}

	listenerTCPAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return errors.Errorf("expected *net.TCPAddr but got %T", listener.Addr())
	}

	options.Secure = options.Network.TLSConfig != nil || options.Network.TLSCertFile != ""
	if options.SignalingAddress == "" && !options.Secure {
		options.SignalingDialOpts = append(options.SignalingDialOpts, rpc.WithInsecure())
	}

	svc.addr = listenerTCPAddr.String()
	if options.FQDN == "" {
		options.FQDN, err = rpc.InstanceNameFromAddress(svc.addr)
		if err != nil {
			return err
		}
	}

	rpcOpts, err := svc.initRPCOptions(listenerTCPAddr, options)
	if err != nil {
		return err
	}

	svc.rpcServer, err = rpc.NewServer(svc.logger, rpcOpts...)
	if err != nil {
		return err
	}

	if options.SignalingAddress == "" {
		options.SignalingAddress = svc.addr
	}

	if err := svc.rpcServer.RegisterServiceServer(
		ctx,
		&pb.RobotService_ServiceDesc,
		grpcserver.New(svc.r),
		pb.RegisterRobotServiceHandlerFromEndpoint,
	); err != nil {
		return err
	}

	if err := svc.refreshResources(); err != nil {
		return err
	}
	if err := svc.initAPIResourceCollections(ctx, false); err != nil {
		return err
	}

	svc.streamServer, err = svc.makeStreamServer(ctx)
	if err != nil {
		return err
	}
	if err := svc.rpcServer.RegisterServiceServer(
		ctx,
		&streampb.StreamService_ServiceDesc,
		svc.streamServer.Server.ServiceServer(),
		streampb.RegisterStreamServiceHandlerFromEndpoint,
	); err != nil {
		return err
	}
	if svc.streamServer.HasStreams {
		// force WebRTC template rendering
		options.WebRTC = true
	}

	if options.Debug {
		if err := svc.rpcServer.RegisterServiceServer(
			ctx,
			&echopb.EchoService_ServiceDesc,
			&echoserver.Server{},
			echopb.RegisterEchoServiceHandlerFromEndpoint,
		); err != nil {
			return err
		}
	}

	httpServer, err := svc.initHTTPServer(listenerTCPAddr, options)
	if err != nil {
		return err
	}

	// Serve

	svc.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer svc.activeBackgroundWorkers.Done()
		<-ctx.Done()
		defer func() {
			if err := httpServer.Shutdown(context.Background()); err != nil {
				svc.logger.Errorw("error shutting down", "error", err)
			}
		}()
		defer func() {
			if err := svc.rpcServer.Stop(); err != nil {
				svc.logger.Errorw("error stopping rpc server", "error", err)
			}
		}()
		if svc.streamServer.Server != nil {
			if err := svc.streamServer.Server.Close(); err != nil {
				svc.logger.Errorw("error closing stream server", "error", err)
			}
		}
	})
	svc.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer svc.activeBackgroundWorkers.Done()
		if err := svc.rpcServer.Start(); err != nil {
			svc.logger.Errorw("error starting rpc server", "error", err)
		}
	})

	var scheme string
	if options.Secure {
		scheme = "https"
	} else {
		scheme = "http"
	}
	if strings.HasPrefix(svc.addr, "[::]") {
		svc.addr = fmt.Sprintf("0.0.0.0:%d", listenerTCPAddr.Port)
	}
	listenerURL := fmt.Sprintf("%s://%s", scheme, svc.addr)
	var urlFields []interface{}
	if options.LocalFQDN == "" {
		urlFields = append(urlFields, "url", listenerURL)
	} else {
		localURL := fmt.Sprintf("%s://%s:%d", scheme, options.LocalFQDN, listenerTCPAddr.Port)
		urlFields = append(urlFields, "url", localURL, "alt_url", listenerURL)
	}
	svc.logger.Infow("serving", urlFields...)

	svc.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer svc.activeBackgroundWorkers.Done()
		var serveErr error
		if options.Secure {
			serveErr = httpServer.ServeTLS(listener, options.Network.TLSCertFile, options.Network.TLSKeyFile)
		} else {
			serveErr = httpServer.Serve(listener)
		}
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			svc.logger.Errorw("error serving http", "error", serveErr)
		}
	})
	return err
}

// Initialize RPC Server options.
func (svc *webService) initRPCOptions(listenerTCPAddr *net.TCPAddr, options weboptions.Options) ([]rpc.ServerOption, error) {
	hosts := options.GetHosts(listenerTCPAddr)
	rpcOpts := []rpc.ServerOption{
		rpc.WithAuthIssuer(options.FQDN),
		rpc.WithAuthAudience(options.FQDN),
		rpc.WithInstanceNames(hosts.Names...),
		rpc.WithWebRTCServerOptions(rpc.WebRTCServerOptions{
			Enable:                    true,
			EnableInternalSignaling:   true,
			ExternalSignalingDialOpts: options.SignalingDialOpts,
			ExternalSignalingAddress:  options.SignalingAddress,
			ExternalSignalingHosts:    hosts.External,
			InternalSignalingHosts:    hosts.Internal,
			Config:                    &grpc.DefaultWebRTCConfiguration,
			OnPeerAdded:               options.WebRTCOnPeerAdded,
			OnPeerRemoved:             options.WebRTCOnPeerRemoved,
		}),
	}
	var unaryInterceptors []googlegrpc.UnaryServerInterceptor

	unaryInterceptors = append(unaryInterceptors, ensureTimeoutUnaryInterceptor)

	if options.Debug {
		rpcOpts = append(rpcOpts, rpc.WithDebug())
		unaryInterceptors = append(unaryInterceptors, func(
			ctx context.Context,
			req interface{},
			info *googlegrpc.UnaryServerInfo,
			handler googlegrpc.UnaryHandler,
		) (interface{}, error) {
			ctx, span := trace.StartSpan(ctx, fmt.Sprintf("%v", req))
			defer span.End()

			return handler(ctx, req)
		})
	}

	if options.Network.TLSConfig != nil {
		rpcOpts = append(rpcOpts, rpc.WithInternalTLSConfig(options.Network.TLSConfig))
	}

	authOpts, err := svc.initAuthHandlers(listenerTCPAddr, options)
	if err != nil {
		return nil, err
	}
	rpcOpts = append(rpcOpts, authOpts...)

	var streamInterceptors []googlegrpc.StreamServerInterceptor

	opManager := svc.r.OperationManager()
	sessManagerInts := svc.r.SessionManager().ServerInterceptors()
	if sessManagerInts.UnaryServerInterceptor != nil {
		unaryInterceptors = append(unaryInterceptors, sessManagerInts.UnaryServerInterceptor)
	}
	unaryInterceptors = append(unaryInterceptors, opManager.UnaryServerInterceptor)

	if sessManagerInts.StreamServerInterceptor != nil {
		streamInterceptors = append(streamInterceptors, sessManagerInts.StreamServerInterceptor)
	}
	streamInterceptors = append(streamInterceptors, opManager.StreamServerInterceptor)

	rpcOpts = append(
		rpcOpts,
		rpc.WithUnknownServiceHandler(svc.foreignServiceHandler),
	)

	unaryInterceptor := grpc_middleware.ChainUnaryServer(unaryInterceptors...)
	streamInterceptor := grpc_middleware.ChainStreamServer(streamInterceptors...)
	rpcOpts = append(rpcOpts,
		rpc.WithUnaryServerInterceptor(unaryInterceptor),
		rpc.WithStreamServerInterceptor(streamInterceptor),
	)

	return rpcOpts, nil
}

// Initialize authentication handler options.
func (svc *webService) initAuthHandlers(listenerTCPAddr *net.TCPAddr, options weboptions.Options) ([]rpc.ServerOption, error) {
	rpcOpts := []rpc.ServerOption{}

	if options.Managed && len(options.Auth.Handlers) == 1 {
		if options.BakedAuthEntity == "" || options.BakedAuthCreds.Type == "" {
			return nil, errors.New("expected baked in local UI credentials since managed")
		}
	}

	if len(options.Auth.Handlers) == 0 {
		rpcOpts = append(rpcOpts, rpc.WithUnauthenticated())
	} else {
		listenerAddr := listenerTCPAddr.String()
		hosts := options.GetHosts(listenerTCPAddr)
		authEntities := make([]string, len(hosts.Internal))
		copy(authEntities, hosts.Internal)
		if !options.Managed {
			// allow authentication for non-unique entities.
			// This eases direct connections via address.
			addIfNotFound := func(toAdd string) []string {
				for _, ent := range authEntities {
					if ent == toAdd {
						return authEntities
					}
				}
				return append(authEntities, toAdd)
			}
			if options.FQDN != listenerAddr {
				authEntities = addIfNotFound(listenerAddr)
			}
			if listenerTCPAddr.IP.IsLoopback() {
				// plus localhost alias
				authEntities = addIfNotFound(weboptions.LocalHostWithPort(listenerTCPAddr))
			}
		}
		if options.Secure && len(options.Auth.TLSAuthEntities) != 0 {
			rpcOpts = append(rpcOpts, rpc.WithTLSAuthHandler(options.Auth.TLSAuthEntities))
		}
		for _, handler := range options.Auth.Handlers {
			switch handler.Type {
			case rpc.CredentialsTypeAPIKey:
				apiKeys := handler.Config.StringSlice("keys")
				if len(apiKeys) == 0 {
					apiKey := handler.Config.String("key")
					if apiKey == "" {
						return nil, errors.Errorf("%q handler requires non-empty API key or keys", handler.Type)
					}
					apiKeys = []string{apiKey}
				}
				rpcOpts = append(rpcOpts, rpc.WithAuthHandler(
					handler.Type,
					rpc.MakeSimpleMultiAuthHandler(authEntities, apiKeys),
				))
			case rutils.CredentialsTypeRobotLocationSecret:
				locationSecrets := handler.Config.StringSlice("secrets")
				if len(locationSecrets) == 0 {
					secret := handler.Config.String("secret")
					if secret == "" {
						return nil, errors.Errorf("%q handler requires non-empty secret", handler.Type)
					}
					locationSecrets = []string{secret}
				}

				rpcOpts = append(rpcOpts, rpc.WithAuthHandler(
					handler.Type,
					rpc.MakeSimpleMultiAuthHandler(authEntities, locationSecrets),
				))
			case rpc.CredentialsTypeExternal:
			default:
				return nil, errors.Errorf("do not know how to handle auth for %q", handler.Type)
			}
		}
	}

	if options.Auth.ExternalAuthConfig != nil {
		rpcOpts = append(rpcOpts, rpc.WithExternalAuthJWKSetTokenVerifier(
			options.Auth.ExternalAuthConfig.ValidatedKeySet,
		))
	}

	return rpcOpts, nil
}

// Register every API resource grpc service here.
func (svc *webService) initAPIResourceCollections(ctx context.Context, mod bool) error {
	// TODO (RSDK-144): only register necessary services
	apiRegs := resource.RegisteredAPIs()
	for s, rs := range apiRegs {
		apiResColl, ok := svc.services[s]
		if !ok {
			apiResColl = rs.MakeEmptyCollection()
			svc.services[s] = apiResColl
		}

		server := svc.rpcServer
		if mod {
			server = svc.modServer
		}
		if err := rs.RegisterRPCService(ctx, server, apiResColl); err != nil {
			return err
		}
	}
	return nil
}

// Initialize HTTP server.
func (svc *webService) initHTTPServer(listenerTCPAddr *net.TCPAddr, options weboptions.Options) (*http.Server, error) {
	mux, err := svc.initMux(options)
	if err != nil {
		return nil, err
	}

	httpServer, err := utils.NewPossiblySecureHTTPServer(mux, utils.HTTPServerOptions{
		Secure:         options.Secure,
		MaxHeaderBytes: rpc.MaxMessageSize,
		Addr:           listenerTCPAddr.String(),
	})
	if err != nil {
		return httpServer, err
	}
	httpServer.TLSConfig = options.Network.TLSConfig.Clone()

	return httpServer, nil
}

// Initialize multiplexer between http handlers.
func (svc *webService) initMux(options weboptions.Options) (*goji.Mux, error) {
	mux := goji.NewMux()
	if err := svc.installWeb(mux, svc.r, options); err != nil {
		return nil, err
	}

	if options.Pprof {
		mux.HandleFunc(pat.New("/debug/pprof/"), pprof.Index)
		mux.HandleFunc(pat.New("/debug/pprof/cmdline"), pprof.Cmdline)
		mux.HandleFunc(pat.New("/debug/pprof/profile"), pprof.Profile)
		mux.HandleFunc(pat.New("/debug/pprof/symbol"), pprof.Symbol)
		mux.HandleFunc(pat.New("/debug/pprof/trace"), pprof.Trace)
	}

	prefix := "/viam"
	addPrefix := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := prefix + r.URL.Path
			rp := prefix + r.URL.RawPath
			if len(p) > len(r.URL.Path) && (r.URL.RawPath == "" || len(rp) > len(r.URL.RawPath)) {
				r2 := new(http.Request)
				*r2 = *r
				r2.URL = new(url.URL)
				*r2.URL = *r.URL
				r2.URL.Path = p
				r2.URL.RawPath = rp
				h.ServeHTTP(w, r2)
			} else {
				http.NotFound(w, r)
			}
		})
	}

	// for urls with /api, add /viam to the path so that it matches with the paths defined in protobuf.
	corsHandler := cors.AllowAll()
	mux.Handle(pat.New("/api/*"), corsHandler.Handler(addPrefix(svc.rpcServer.GatewayHandler())))
	mux.Handle(pat.New("/*"), corsHandler.Handler(svc.rpcServer.GRPCHandler()))

	return mux, nil
}

func (svc *webService) foreignServiceHandler(srv interface{}, stream googlegrpc.ServerStream) error {
	method, ok := googlegrpc.MethodFromServerStream(stream)
	if !ok {
		return grpc.UnimplementedError
	}
	subType, methodDesc, err := robot.TypeAndMethodDescFromMethod(svc.r, method)
	if err != nil {
		return err
	}

	firstMsg := dynamic.NewMessage(methodDesc.GetInputType())

	if err := stream.RecvMsg(firstMsg); err != nil {
		return err
	}

	resource, fqName, err := robot.ResourceFromProtoMessage(svc.r, firstMsg, subType.API)
	if err != nil {
		svc.logger.Errorw("unable to route foreign message", "error", err)
		return err
	}

	if fqName.ContainsRemoteNames() {
		firstMsg.SetFieldByName("name", fqName.PopRemote().ShortName())
	}

	foreignRes, ok := resource.(*grpc.ForeignResource)
	if !ok {
		svc.logger.Errorf("expected resource to be a foreign RPC resource but was %T", foreignRes)
		return grpc.UnimplementedError
	}

	foreignClient := foreignRes.NewStub()

	// see https://github.com/fullstorydev/grpcurl/blob/76bbedeed0ec9b6e09ad1e1cb88fffe4726c0db2/invoke.go
	switch {
	case methodDesc.IsClientStreaming() && methodDesc.IsServerStreaming():

		ctx, cancel := context.WithCancel(stream.Context())
		defer cancel()

		bidiStream, err := foreignClient.InvokeRpcBidiStream(ctx, methodDesc)
		if err != nil {
			return err
		}

		var wg sync.WaitGroup
		var sendErr atomic.Pointer[error]

		defer wg.Wait()

		wg.Add(1)
		utils.PanicCapturingGo(func() {
			defer wg.Done()

			var err error
			for err == nil {
				msg := dynamic.NewMessage(methodDesc.GetInputType())
				if err = stream.RecvMsg(msg); err != nil {
					if errors.Is(err, io.EOF) {
						err = bidiStream.CloseSend()
						break
					}
					cancel()
					break
				}
				// remove a remote from the name if needed
				if fqName.ContainsRemoteNames() {
					msg.SetFieldByName("name", fqName.PopRemote().ShortName())
				}
				err = bidiStream.SendMsg(msg)
			}

			if err != nil {
				sendErr.Store(&err)
			}
		})

		for {
			resp, err := bidiStream.RecvMsg()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					return err
				}
				break
			}

			if err := stream.SendMsg(resp); err != nil {
				cancel()
				return err
			}
		}

		wg.Wait()
		if err := sendErr.Load(); err != nil && !errors.Is(*err, io.EOF) {
			return *err
		}

		return nil
	case methodDesc.IsClientStreaming():
		clientStream, err := foreignClient.InvokeRpcClientStream(stream.Context(), methodDesc)
		if err != nil {
			return err
		}

		for {
			msg := dynamic.NewMessage(methodDesc.GetInputType())
			if err := stream.RecvMsg(msg); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}

				return err
			}
			if fqName.ContainsRemoteNames() {
				msg.SetFieldByName("name", fqName.PopRemote().ShortName())
			}
			if err := clientStream.SendMsg(msg); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return err
			}
		}
		resp, err := clientStream.CloseAndReceive()
		if err != nil {
			return err
		}
		return stream.SendMsg(resp)
	case methodDesc.IsServerStreaming():
		secondMsg := dynamic.NewMessage(methodDesc.GetInputType())
		if err := stream.RecvMsg(secondMsg); err == nil {
			return errors.Errorf(
				"method %q is a server-streaming RPC, but request data contained more than 1 message",
				methodDesc.GetFullyQualifiedName())
		} else if !errors.Is(err, io.EOF) {
			return err
		}

		serverStream, err := foreignClient.InvokeRpcServerStream(stream.Context(), methodDesc, firstMsg)
		if err != nil {
			return err
		}

		for {
			resp, err := serverStream.RecvMsg()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					return err
				}
				break
			}
			if err := stream.SendMsg(resp); err != nil {
				return err
			}
		}

		return nil
	default:
		invokeResp, err := foreignClient.InvokeRpc(stream.Context(), methodDesc, firstMsg)
		if err != nil {
			return err
		}
		return stream.SendMsg(invokeResp)
	}
}

// ensureTimeoutUnaryInterceptor sets a default timeout on the context if one is
// not already set. To be called as the first unary server interceptor.
func ensureTimeoutUnaryInterceptor(ctx context.Context, req interface{},
	info *googlegrpc.UnaryServerInfo, handler googlegrpc.UnaryHandler,
) (interface{}, error) {
	if _, deadlineSet := ctx.Deadline(); !deadlineSet {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultMethodTimeout)
		defer cancel()
	}

	return handler(ctx, req)
}
