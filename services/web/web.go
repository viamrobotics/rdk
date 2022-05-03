package web

import (
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"net/http/pprof"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/sprig"
	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/edaniels/gostream/codec/x264"
	streampb "github.com/edaniels/gostream/proto/stream/v1"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"go.uber.org/multierr"
	"go.viam.com/utils"
	"go.viam.com/utils/perf"
	echopb "go.viam.com/utils/proto/rpc/examples/echo/v1"
	"go.viam.com/utils/rpc"
	echoserver "go.viam.com/utils/rpc/examples/echo/server"
	"goji.io"
	"goji.io/pat"
	"golang.org/x/net/http2/h2c"
	googlegrpc "google.golang.org/grpc"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc"
	grpcserver "go.viam.com/rdk/grpc/server"
	pb "go.viam.com/rdk/proto/api/robot/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
	rutils "go.viam.com/rdk/utils"
	"go.viam.com/rdk/web"
)

// SubtypeName is the name of the type of service.
const SubtypeName = resource.SubtypeName("web")

// Subtype is a constant that identifies the web service resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

// Name is the WebService's typed resource name.
var Name = resource.NameFromSubtype(Subtype, "")

// FromRobot retrieves the web service of a robot.
func FromRobot(r robot.Robot) (Service, error) {
	resource, err := r.ResourceByName(Name)
	if err != nil {
		return nil, err
	}
	web, ok := resource.(Service)
	if !ok {
		return nil, rutils.NewUnimplementedInterfaceError("web.Service", resource)
	}
	return web, nil
}

// RunWeb starts the web server on the web service with web options and blocks until we close it.
func RunWeb(ctx context.Context, r robot.Robot, o Options, logger golog.Logger) (err error) {
	defer func() {
		if err != nil {
			err = utils.FilterOutError(err, context.Canceled)
			if err != nil {
				logger.Errorw("error running web", "error", err)
			}
		}
		err = multierr.Combine(err, utils.TryClose(ctx, r))
	}()
	svc, err := FromRobot(r)
	if err != nil {
		return err
	}
	if err := svc.Start(ctx, o); err != nil {
		return err
	}
	<-ctx.Done()
	return ctx.Err()
}

// RunWebWithConfig starts the web server on the web service with a robot config and blocks until we close it.
func RunWebWithConfig(ctx context.Context, r robot.Robot, cfg *config.Config, logger golog.Logger) error {
	o, err := OptionsFromConfig(cfg)
	if err != nil {
		return err
	}
	return RunWeb(ctx, r, o, logger)
}

func init() {
	registry.RegisterService(Subtype, registry.Service{
		Constructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			return New(ctx, r, c, logger)
		},
	},
	)
}

// robotWebApp hosts a web server to interact with a robot in addition to hosting
// a gRPC/REST server.
type robotWebApp struct {
	template *template.Template
	theRobot robot.Robot
	logger   golog.Logger
	options  Options
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

	type Temp struct {
		External                   bool
		WebRTCEnabled              bool
		WebRTCHost                 string
		WebRTCSignalingAddress     string
		WebRTCAdditionalICEServers []map[string]interface{}
		SupportedAuthTypes         []string
		BakedAuth                  map[string]interface{}
	}

	var temp Temp

	if err := r.ParseForm(); err != nil {
		app.logger.Debugw("failed to parse form", "error", err)
	}

	if app.options.WebRTC && r.Form.Get("grpc") != "true" {
		temp.WebRTCEnabled = true
		temp.WebRTCHost = app.options.FQDN
	}

	if app.options.Managed && len(app.options.Auth.Handlers) == 1 {
		temp.BakedAuth = map[string]interface{}{
			"authEntity": app.options.BakedAuthEntity,
			"creds":      app.options.BakedAuthCreds,
		}
	} else {
		for _, handler := range app.options.Auth.Handlers {
			temp.SupportedAuthTypes = append(temp.SupportedAuthTypes, string(handler.Type))
		}
	}

	err := app.template.Execute(w, temp)
	if err != nil {
		app.logger.Debugf("couldn't execute web page: %s", err)
	}
}

// allSourcesToDisplay returns every possible image source that could be viewed from
// the robot.
func allSourcesToDisplay(theRobot robot.Robot) map[string]gostream.ImageSource {
	sources := make(map[string]gostream.ImageSource)

	// TODO (RDK-133): allow users to determine what to stream.
	for _, name := range camera.NamesFromRobot(theRobot) {
		cam, err := camera.FromRobot(theRobot, name)
		if err != nil {
			continue
		}

		sources[name] = cam
	}

	return sources
}

var defaultStreamConfig = x264.DefaultStreamConfig

// A Service controls the web server for a robot.
type Service interface {
	// Start starts the web server
	Start(context.Context, Options) error
}

// StreamServer manages streams and displays.
type StreamServer struct {
	// Server serves streams
	Server gostream.StreamServer
	// ImagesSources to stream from by name
	ImagesSources map[string]gostream.ImageSource
}

// HasStreams is true if service has streams that require a WebRTC connection.
func (ss *StreamServer) HasStreams() bool {
	return len(ss.ImagesSources) > 0
}

// New returns a new web service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (Service, error) {
	webSvc := &webService{
		r:            r,
		logger:       logger,
		rpcServer:    nil,
		streamServer: nil,
		services:     make(map[resource.Subtype]subtype.Service),
	}
	return webSvc, nil
}

type webService struct {
	mu           sync.Mutex
	r            robot.Robot
	rpcServer    rpc.Server
	streamServer *StreamServer
	services     map[resource.Subtype]subtype.Service

	logger                  golog.Logger
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

// Start starts the web server, will return an error if server is already up.
func (svc *webService) Start(ctx context.Context, o Options) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if svc.cancelFunc != nil {
		return errors.New("web server already started")
	}
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	svc.cancelFunc = cancelFunc

	return svc.runWeb(cancelCtx, o)
}

// Update updates the web service when the robot has changed. Not Reconfigure because this should happen at a different point in the
// lifecycle.
func (svc *webService) Update(ctx context.Context, resources map[resource.Name]interface{}) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	return svc.update(ctx, resources)
}

func (svc *webService) update(ctx context.Context, resources map[resource.Name]interface{}) error {
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
		// TODO: as part of #272, register new service if it doesn't currently exist
		if !ok {
			subtypeSvc, err := subtype.New(v)
			if err != nil {
				return err
			}
			svc.services[s] = subtypeSvc
		} else {
			if err := subtypeSvc.Replace(v); err != nil {
				return err
			}
		}
	}

	// update streams
	err := svc.addNewStreams(ctx, svc.r)
	if err != nil {
		return err
	}

	return nil
}

// Close closes a webService via calls to its Cancel func.
func (svc *webService) Close(ctx context.Context) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if svc.cancelFunc != nil {
		svc.cancelFunc()
		svc.cancelFunc = nil
	}
	svc.activeBackgroundWorkers.Wait()
	return nil
}

// TODO: use in makeStreamServer as iterator pattern?
func (svc *webService) addNewStreams(ctx context.Context, theRobot robot.Robot) error {
	// TODO: check if stream service and server are initialized?
	if svc.streamServer == nil || svc.streamServer.Server == nil {
		return nil
	}
	sources := allSourcesToDisplay(theRobot)

	for name, source := range sources {
		// Check if stream already exists for named image source
		if _, ok := svc.streamServer.ImagesSources[name]; ok {
			continue
		}

		// Configure new stream
		config := defaultStreamConfig
		config.Name = name
		view, err := gostream.NewStream(config)
		if err != nil {
			return err
		}

		// Add stream server
		if err := svc.streamServer.Server.AddStream(view); err != nil {
			return err
		}

		// Stream
		svc.startStream(ctx, source, view)
	}

	return nil
}

func (svc *webService) makeStreamServer(ctx context.Context, theRobot robot.Robot) (*StreamServer, error) {
	sources := allSourcesToDisplay(theRobot)
	var streams []gostream.Stream

	if len(sources) == 0 {
		noopServer, err := gostream.NewStreamServer(streams...)
		return &StreamServer{noopServer, sources}, err
	}

	for name := range sources {
		config := defaultStreamConfig
		config.Name = name
		view, err := gostream.NewStream(config)
		if err != nil {
			return nil, err
		}
		streams = append(streams, view)
	}

	streamServer, err := gostream.NewStreamServer(streams...)
	if err != nil {
		return nil, err
	}

	for _, stream := range streams {
		svc.startStream(ctx, sources[stream.Name()], stream)
	}

	return &StreamServer{streamServer, sources}, nil
}

func (svc *webService) startStream(ctx context.Context, source gostream.ImageSource, stream gostream.Stream) {
	waitCh := make(chan struct{})
	svc.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer svc.activeBackgroundWorkers.Done()
		close(waitCh)
		gostream.StreamSource(ctx, source, stream)
	})
	<-waitCh
}

type ssStreamContextWrapper struct {
	googlegrpc.ServerStream
	ctx context.Context
}

func (w ssStreamContextWrapper) Context() context.Context {
	return w.ctx
}

// installWeb prepares the given mux to be able to serve the UI for the robot.
func (svc *webService) installWeb(mux *goji.Mux, theRobot robot.Robot, options Options) error {
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

	mux.Handle(pat.Get("/static/*"), http.StripPrefix("/static", http.FileServer(staticDir)))
	mux.Handle(pat.New("/"), app)

	return nil
}

// runWeb takes the given robot and options and runs the web server. This function will
// block until the context is done.
func (svc *webService) runWeb(ctx context.Context, options Options) (err error) {
	listener, err := net.Listen("tcp", options.Network.BindAddress)
	if err != nil {
		return err
	}

	listenerTCPAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return errors.Errorf("expected *net.TCPAddr but got %T", listener.Addr())
	}

	options.secure = options.Network.TLSConfig != nil || options.Network.TLSCertFile != ""
	if options.SignalingAddress == "" && !options.secure {
		options.SignalingDialOpts = append(options.SignalingDialOpts, rpc.WithInsecure())
	}

	listenerAddr := listenerTCPAddr.String()
	if options.FQDN == "" {
		options.FQDN, err = rpc.InstanceNameFromAddress(listenerAddr)
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
		options.SignalingAddress = listenerAddr
	}

	if err := svc.rpcServer.RegisterServiceServer(
		ctx,
		&pb.RobotService_ServiceDesc,
		grpcserver.New(svc.r),
		pb.RegisterRobotServiceHandlerFromEndpoint,
	); err != nil {
		return err
	}

	if err := svc.initResources(ctx); err != nil {
		return err
	}

	if err := svc.initSubtypeServices(ctx); err != nil {
		return err
	}

	svc.streamServer, err = svc.makeStreamServer(ctx, svc.r)
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
	if svc.streamServer.HasStreams() {
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
	if options.secure {
		scheme = "https"
	} else {
		scheme = "http"
	}
	if strings.HasPrefix(listenerAddr, "[::]") {
		listenerAddr = fmt.Sprintf("0.0.0.0:%d", listenerTCPAddr.Port)
	}
	listenerURL := fmt.Sprintf("%s://%s", scheme, listenerAddr)
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
		if options.secure {
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
func (svc *webService) initRPCOptions(listenerTCPAddr *net.TCPAddr, options Options) ([]rpc.ServerOption, error) {
	hosts := options.GetHosts(listenerTCPAddr)
	rpcOpts := []rpc.ServerOption{
		rpc.WithInstanceNames(hosts.names...),
		rpc.WithWebRTCServerOptions(rpc.WebRTCServerOptions{
			Enable:                    true,
			EnableInternalSignaling:   true,
			ExternalSignalingDialOpts: options.SignalingDialOpts,
			ExternalSignalingAddress:  options.SignalingAddress,
			ExternalSignalingHosts:    hosts.external,
			InternalSignalingHosts:    hosts.internal,
			Config:                    &grpc.DefaultWebRTCConfiguration,
		}),
	}
	if options.Debug {
		trace.RegisterExporter(perf.NewNiceLoggingSpanExporter())
		trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})

		rpcOpts = append(rpcOpts,
			rpc.WithDebug(),
			rpc.WithUnaryServerInterceptor(func(
				ctx context.Context,
				req interface{},
				info *googlegrpc.UnaryServerInfo,
				handler googlegrpc.UnaryHandler,
			) (interface{}, error) {
				ctx, span := trace.StartSpan(ctx, fmt.Sprintf("%v", req))
				defer span.End()

				return handler(ctx, req)
			}),
		)
	}
	if options.Network.TLSConfig != nil {
		rpcOpts = append(rpcOpts, rpc.WithInternalTLSConfig(options.Network.TLSConfig))
	}

	authOpts, err := svc.initAuthHandlers(listenerTCPAddr, options)
	if err != nil {
		return nil, err
	}
	rpcOpts = append(rpcOpts, authOpts...)

	rpcOpts = append(
		rpcOpts,
		rpc.WithUnaryServerInterceptor(func(
			ctx context.Context,
			req interface{},
			info *googlegrpc.UnaryServerInfo,
			handler googlegrpc.UnaryHandler,
		) (interface{}, error) {
			ctx, done := svc.r.OperationManager().Create(ctx, info.FullMethod, req)
			defer done()
			return handler(ctx, req)
		}),
	)

	rpcOpts = append(
		rpcOpts,
		rpc.WithStreamServerInterceptor(func(
			srv interface{},
			ss googlegrpc.ServerStream,
			info *googlegrpc.StreamServerInfo,
			handler googlegrpc.StreamHandler,
		) error {
			ctx, done := svc.r.OperationManager().Create(ss.Context(), info.FullMethod, nil)
			defer done()
			return handler(srv, &ssStreamContextWrapper{ss, ctx})
		}),
	)

	return rpcOpts, nil
}

// Initialize authentication handler options.
func (svc *webService) initAuthHandlers(listenerTCPAddr *net.TCPAddr, options Options) ([]rpc.ServerOption, error) {
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
		authEntities := make([]string, len(hosts.internal))
		copy(authEntities, hosts.internal)
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
				authEntities = addIfNotFound(localHostWithPort(listenerTCPAddr))
			}
		}
		if options.secure && len(options.Auth.TLSAuthEntities) != 0 {
			rpcOpts = append(rpcOpts, rpc.WithTLSAuthHandler(options.Auth.TLSAuthEntities, nil))
		}
		for _, handler := range options.Auth.Handlers {
			switch handler.Type {
			case rpc.CredentialsTypeAPIKey:
				apiKey := handler.Config.String("key")
				if apiKey == "" {
					return nil, errors.Errorf("%q handler requires non-empty API key", handler.Type)
				}
				rpcOpts = append(rpcOpts, rpc.WithAuthHandler(
					handler.Type,
					rpc.MakeSimpleAuthHandler(authEntities, apiKey),
				))
			case rutils.CredentialsTypeRobotLocationSecret:
				secret := handler.Config.String("secret")
				if secret == "" {
					return nil, errors.Errorf("%q handler requires non-empty secret", handler.Type)
				}
				rpcOpts = append(rpcOpts, rpc.WithAuthHandler(
					handler.Type,
					rpc.MakeSimpleAuthHandler(authEntities, secret),
				))
			default:
				return nil, errors.Errorf("do not know how to handle auth for %q", handler.Type)
			}
		}
	}

	return rpcOpts, nil
}

// Populate subtype services with robot resources.
func (svc *webService) initResources(ctx context.Context) error {
	resources := make(map[resource.Name]interface{})
	for _, name := range svc.r.ResourceNames() {
		resource, err := svc.r.ResourceByName(name)
		if err != nil {
			continue
		}

		resources[name] = resource
	}
	if err := svc.update(ctx, resources); err != nil {
		return err
	}

	return nil
}

// Register every subtype resource grpc service here.
func (svc *webService) initSubtypeServices(ctx context.Context) error {
	// TODO: only register necessary services (#272)
	subtypeConstructors := registry.RegisteredResourceSubtypes()
	for s, rs := range subtypeConstructors {
		if rs.RegisterRPCService == nil {
			continue
		}
		subtypeSvc, ok := svc.services[s]
		if !ok {
			newSvc, err := subtype.New(make(map[resource.Name]interface{}))
			if err != nil {
				return err
			}
			subtypeSvc = newSvc
			svc.services[s] = newSvc
		}
		if err := rs.RegisterRPCService(ctx, svc.rpcServer, subtypeSvc); err != nil {
			return err
		}
	}

	return nil
}

// Initialize HTTP server.
func (svc *webService) initHTTPServer(listenerTCPAddr *net.TCPAddr, options Options) (*http.Server, error) {
	mux, err := svc.initMux(options)
	if err != nil {
		return nil, err
	}

	httpServer := &http.Server{
		ReadTimeout:    10 * time.Second,
		MaxHeaderBytes: rpc.MaxMessageSize,
		TLSConfig:      options.Network.TLSConfig.Clone(),
	}
	httpServer.Addr = listenerTCPAddr.String()
	httpServer.Handler = mux

	if !options.secure {
		http2Server, err := utils.NewHTTP2Server()
		if err != nil {
			return nil, err
		}
		httpServer.RegisterOnShutdown(func() {
			utils.UncheckedErrorFunc(http2Server.Close)
		})
		httpServer.Handler = h2c.NewHandler(httpServer.Handler, http2Server.HTTP2)
	}

	return httpServer, nil
}

// Initialize multiplexer between http handlers.
func (svc *webService) initMux(options Options) (*goji.Mux, error) {
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
	mux.Handle(pat.New("/api/*"), addPrefix(svc.rpcServer.GatewayHandler()))
	mux.Handle(pat.New("/*"), svc.rpcServer.GRPCHandler())

	return mux, nil
}
