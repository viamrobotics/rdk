// Package web provides gRPC/REST/GUI APIs to control and monitor a robot.
package web

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"io/fs"
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
	"github.com/edaniels/gostream"
	streampb "github.com/edaniels/gostream/proto/stream/v1"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/pkg/errors"
	"github.com/rs/cors"
	"go.opencensus.io/trace"
	pb "go.viam.com/api/robot/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/jwks"
	echopb "go.viam.com/utils/proto/rpc/examples/echo/v1"
	"go.viam.com/utils/rpc"
	echoserver "go.viam.com/utils/rpc/examples/echo/server"
	"go.viam.com/utils/rpc/oauth"
	"goji.io"
	"goji.io/pat"
	googlegrpc "google.golang.org/grpc"

	"go.viam.com/rdk/components/audioinput"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	grpcserver "go.viam.com/rdk/robot/server"
	weboptions "go.viam.com/rdk/robot/web/options"
	webstream "go.viam.com/rdk/robot/web/stream"
	"go.viam.com/rdk/subtype"
	rutils "go.viam.com/rdk/utils"
	"go.viam.com/rdk/web"
)

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
	External                   bool                     `json:"external"`
	WebRTCEnabled              bool                     `json:"webrtc_enabled"`
	WebRTCSignalingAddress     string                   `json:"webrtc_signaling_address"`
	WebRTCAdditionalICEServers []map[string]interface{} `json:"webrtc_additional_ice_servers"`
	Env                        string                   `json:"env"`
	Host                       string                   `json:"host"`
	SupportedAuthTypes         []string                 `json:"supported_auth_types"`
	AuthEntity                 string                   `json:"auth_entity"`
	BakedAuth                  map[string]interface{}   `json:"baked_auth"`
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
	hasWebOAuthHandler := false
	for _, h := range handlers {
		if h.Type == rutils.CredentialsTypeRobotLocationSecret {
			hasLocationSecretHandler = true
		}

		if h.Type == oauth.CredentialsTypeOAuthWeb {
			hasWebOAuthHandler = true
		}
	}

	if len(handlers) == 2 && hasLocationSecretHandler && hasWebOAuthHandler {
		return true
	}

	return false
}

// allVideoSourcesToDisplay returns every possible video source that could be viewed from
// the robot.
func allVideoSourcesToDisplay(theRobot robot.Robot) map[string]gostream.VideoSource {
	sources := make(map[string]gostream.VideoSource)

	for _, name := range camera.NamesFromRobot(theRobot) {
		cam, err := camera.FromRobot(theRobot, name)
		if err != nil {
			continue
		}

		sources[name] = cam
	}

	return sources
}

// allAudioSourcesToDisplay returns every possible audio source that could be listened to from
// the robot.
func allAudioSourcesToDisplay(theRobot robot.Robot) map[string]gostream.AudioSource {
	sources := make(map[string]gostream.AudioSource)

	for _, name := range audioinput.NamesFromRobot(theRobot) {
		input, err := audioinput.FromRobot(theRobot, name)
		if err != nil {
			continue
		}

		sources[name] = input
	}

	return sources
}

// A Service controls the web server for a robot.
type Service interface {
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

	// Close closes the web server
	Close() error
}

// StreamServer manages streams and displays.
type StreamServer struct {
	// Server serves streams
	Server gostream.StreamServer
	// HasStreams is true if service has streams that require a WebRTC connection.
	HasStreams bool
}

// New returns a new web service for the given robot.
func New(ctx context.Context, r robot.Robot, logger golog.Logger, opts ...Option) Service {
	var wOpts options
	for _, opt := range opts {
		opt.apply(&wOpts)
	}
	webSvc := &webService{
		r:            r,
		logger:       logger,
		rpcServer:    nil,
		streamServer: nil,
		services:     make(map[resource.Subtype]subtype.Service),
		opts:         wOpts,
	}
	return webSvc
}

type webService struct {
	mu           sync.Mutex
	r            robot.Robot
	rpcServer    rpc.Server
	modServer    rpc.Server
	streamServer *StreamServer
	services     map[resource.Subtype]subtype.Service
	opts         options
	addr         string
	modAddr      string

	logger                  golog.Logger
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

// Start starts the web server, will return an error if server is already up.
func (svc *webService) Start(ctx context.Context, o weboptions.Options) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if svc.cancelFunc != nil {
		return errors.New("web server already started")
	}
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	svc.cancelFunc = cancelFunc

	if err := svc.runWeb(cancelCtx, o); err != nil {
		cancelFunc()
		svc.cancelFunc = nil
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
	if err := svc.initResources(); err != nil {
		return err
	}
	if err := svc.initSubtypeServices(ctx, true); err != nil {
		return err
	}

	svc.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer svc.activeBackgroundWorkers.Done()
		defer utils.UncheckedErrorFunc(func() error { return os.Remove(addr) })
		svc.logger.Debugw("module server listening", "socket path", lis.Addr())
		defer utils.UncheckedErrorFunc(func() error { return os.RemoveAll(filepath.Dir(addr)) })
		if err := svc.modServer.Serve(lis); err != nil {
			svc.logger.Errorw("failed to serve module service", "error", err)
		}
	})
	return nil
}

// Update updates the web service when the robot has changed. Not Reconfigure because
// this should happen at a different point in the lifecycle.
func (svc *webService) Update(ctx context.Context, resources map[resource.Name]interface{}) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if err := svc.updateResources(resources); err != nil {
		return err
	}
	return svc.addNewStreams(ctx)
}

func (svc *webService) updateResources(resources map[resource.Name]interface{}) error {
	// so group resources by subtype
	groupedResources := make(map[resource.Subtype]map[resource.Name]interface{})
	components := make(map[resource.Name]interface{})
	for n, v := range resources {
		r, ok := groupedResources[n.Subtype]
		if !ok {
			r = make(map[resource.Name]interface{})
		}
		r[n] = v
		groupedResources[n.Subtype] = r
		if n.Subtype.Type.ResourceType == resource.ResourceTypeComponent {
			components[n] = v
		}
	}
	groupedResources[generic.Subtype] = components

	for s, v := range groupedResources {
		subtypeSvc, ok := svc.services[s]
		// TODO(RSDK-144): register new service if it doesn't currently exist
		if !ok {
			subtypeSvc, err := subtype.New(v)
			if err != nil {
				return err
			}
			svc.services[s] = subtypeSvc
		} else {
			if err := subtypeSvc.ReplaceAll(v); err != nil {
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
		svc.cancelFunc = nil
	}
}

// Close closes a webService via calls to its Cancel func.
func (svc *webService) Close() error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	var err error
	if svc.cancelFunc != nil {
		svc.cancelFunc()
		svc.cancelFunc = nil
	}
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
	videoSources := allVideoSourcesToDisplay(svc.r)
	audioSources := allAudioSourcesToDisplay(svc.r)
	if svc.opts.streamConfig == nil {
		if len(videoSources) != 0 || len(audioSources) != 0 {
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

	for name, source := range videoSources {
		stream, alreadyRegistered, err := newStream(name)
		if err != nil {
			return err
		} else if alreadyRegistered {
			continue
		}

		svc.startImageStream(ctx, source, stream)
	}

	for name, source := range audioSources {
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
	videoSources := allVideoSourcesToDisplay(svc.r)
	audioSources := allAudioSourcesToDisplay(svc.r)
	var streams []gostream.Stream
	var streamTypes []bool

	if svc.opts.streamConfig == nil || (len(videoSources) == 0 && len(audioSources) == 0) {
		if len(videoSources) != 0 || len(audioSources) != 0 {
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
	for name := range videoSources {
		var err error
		streams, err = addStream(streams, name, true)
		if err != nil {
			return nil, err
		}
		streamTypes = append(streamTypes, true)
	}
	for name := range audioSources {
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
			svc.startImageStream(ctx, videoSources[stream.Name()], stream)
		} else {
			svc.startAudioStream(ctx, audioSources[stream.Name()], stream)
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

func (svc *webService) startImageStream(ctx context.Context, source gostream.VideoSource, stream gostream.Stream) {
	ctxWithJPEGHint := gostream.WithMIMETypeHint(ctx, rutils.WithLazyMIMEType(rutils.MimeTypeJPEG))
	svc.startStream(func(opts *webstream.BackoffTuningOptions) error {
		return webstream.StreamVideoSource(ctxWithJPEGHint, source, stream, opts)
	})
}

func (svc *webService) startAudioStream(ctx context.Context, source gostream.AudioSource, stream gostream.Stream) {
	svc.startStream(func(opts *webstream.BackoffTuningOptions) error {
		return webstream.StreamAudioSource(ctx, source, stream, opts)
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

	if err := svc.initResources(); err != nil {
		return err
	}

	if err := svc.initSubtypeServices(ctx, false); err != nil {
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
			rpcOpts = append(rpcOpts, rpc.WithTLSAuthHandler(options.Auth.TLSAuthEntities, nil))
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
			case oauth.CredentialsTypeOAuthWeb:
				webOauthOptions := oauth.WebOAuthOptions{
					AllowedAudiences: handler.WebOAuthConfig.AllowedAudiences,
					KeyProvider:      jwks.NewStaticJWKKeyProvider(handler.WebOAuthConfig.ValidatedKeySet),
					EntityVerifier: func(ctx context.Context, entity string) (interface{}, error) {
						// For webauth handlers we assume some user subject/entity. For consistency with the app
						// pass the entity as the email to the custom entity info set in the context.
						return map[string]interface{}{"email": entity}, nil
					},
					Logger: svc.logger,
				}

				rpcOpts = append(rpcOpts, oauth.WithWebOAuthTokenAuthHandler(webOauthOptions))
			default:
				return nil, errors.Errorf("do not know how to handle auth for %q", handler.Type)
			}
		}
	}

	return rpcOpts, nil
}

// Populate subtype services with robot resources.
func (svc *webService) initResources() error {
	resources := make(map[resource.Name]interface{})
	for _, name := range svc.r.ResourceNames() {
		resource, err := svc.r.ResourceByName(name)
		if err != nil {
			continue
		}

		resources[name] = resource
	}
	if err := svc.updateResources(resources); err != nil {
		return err
	}

	return nil
}

// Register every subtype resource grpc service here.
func (svc *webService) initSubtypeServices(ctx context.Context, mod bool) error {
	// TODO: only register necessary services (#272)
	subtypeConstructors := registry.RegisteredResourceSubtypes()
	for s, rs := range subtypeConstructors {
		subtypeSvc, ok := svc.services[s]
		if !ok {
			newSvc, err := subtype.New(make(map[resource.Name]interface{}))
			if err != nil {
				return err
			}
			subtypeSvc = newSvc
			svc.services[s] = newSvc
		}

		if rs.RegisterRPCService != nil {
			server := svc.rpcServer
			if mod {
				server = svc.modServer
			}
			if err := rs.RegisterRPCService(ctx, server, subtypeSvc); err != nil {
				return err
			}
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

	resource, fqName, err := robot.ResourceFromProtoMessage(svc.r, firstMsg, subType.Subtype)
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
		var sendErr atomic.Value

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
				sendErr.Store(err)
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
		if err, ok := sendErr.Load().(error); ok && !errors.Is(err, io.EOF) {
			return err
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
