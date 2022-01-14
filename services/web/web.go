package web

import (
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"net/http/pprof"
	"strings"
	"sync"

	"github.com/Masterminds/sprig"
	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/edaniels/gostream/codec/x264"
	streampb "github.com/edaniels/gostream/proto/stream/v1"
	"github.com/pkg/errors"
	"go.viam.com/utils"
	echopb "go.viam.com/utils/proto/rpc/examples/echo/v1"
	"go.viam.com/utils/rpc"
	echoserver "go.viam.com/utils/rpc/examples/echo/server"
	"goji.io"
	"goji.io/pat"

	"go.viam.com/rdk/action"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc"
	grpcmetadata "go.viam.com/rdk/grpc/metadata/server"
	grpcserver "go.viam.com/rdk/grpc/server"
	"go.viam.com/rdk/metadata/service"
	metadatapb "go.viam.com/rdk/proto/api/service/v1"
	pb "go.viam.com/rdk/proto/api/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
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
		Actions                    []string
		SupportedAuthTypes         []string
	}

	temp := Temp{
		Actions: action.AllActionNames(),
	}

	if app.options.WebRTC {
		temp.WebRTCEnabled = true
		var scheme string
		if app.options.secureSignaling {
			scheme = "https"
		} else {
			scheme = "http"
		}
		// we will rely on pages host and port if we are listening everywhere
		if !(strings.HasPrefix(app.options.SignalingAddress, "0.0.0.0:") ||
			strings.HasPrefix(app.options.SignalingAddress, "[::]:")) {
			temp.WebRTCSignalingAddress = fmt.Sprintf("%s://%s", scheme, app.options.SignalingAddress)
		}
		temp.WebRTCHost = app.options.FQDNs[0]
	}

	for _, handler := range app.options.Auth.Handlers {
		temp.SupportedAuthTypes = append(temp.SupportedAuthTypes, string(handler.Type))
	}

	err := app.template.Execute(w, temp)
	if err != nil {
		app.logger.Debugf("couldn't execute web page: %s", err)
	}
}

// allSourcesToDisplay returns every possible image source that could be viewed from
// the robot.
func allSourcesToDisplay(ctx context.Context, theRobot robot.Robot) ([]gostream.ImageSource, []string, error) {
	sources := []gostream.ImageSource{}
	names := []string{}

	conf, err := theRobot.Config(ctx)
	if err != nil {
		return nil, nil, err
	}

	for _, name := range theRobot.CameraNames() {
		cam, ok := theRobot.CameraByName(name)
		if !ok {
			continue
		}
		cmp := conf.FindComponent(name)
		if cmp != nil && cmp.Attributes.Bool("hide", false) {
			continue
		}

		sources = append(sources, cam)
		names = append(names, name)
	}

	return sources, names, nil
}

var defaultStreamConfig = x264.DefaultStreamConfig

// A Service controls the web server for a robot.
type Service interface {
	// Start starts the web server
	Start(context.Context, Options) error
}

// New returns a new web service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (Service, error) {
	webSvc := &webService{
		r:        r,
		logger:   logger,
		server:   nil,
		services: make(map[resource.Subtype]subtype.Service),
	}
	return webSvc, nil
}

type webService struct {
	mu       sync.Mutex
	r        robot.Robot
	server   rpc.Server
	services map[resource.Subtype]subtype.Service

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
	return svc.update(resources)
}

func (svc *webService) update(resources map[resource.Name]interface{}) error {
	// so group resources by subtype
	groupedResources := make(map[resource.Subtype]map[resource.Name]interface{})
	for n, v := range resources {
		r, ok := groupedResources[n.Subtype]
		if !ok {
			r = make(map[resource.Name]interface{})
		}
		r[n] = v
		groupedResources[n.Subtype] = r
	}

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
	return nil
}

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

func (svc *webService) makeStreamServer(ctx context.Context, theRobot robot.Robot, options Options) (gostream.StreamServer, bool, error) {
	displaySources, displayNames, err := allSourcesToDisplay(ctx, theRobot)
	if err != nil {
		return nil, false, err
	}
	var streams []gostream.Stream
	var autoCameraTiler *gostream.AutoTiler

	if len(displaySources) == 0 {
		noopServer, err := gostream.NewStreamServer(streams...)
		return noopServer, false, err
	}

	if options.AutoTile {
		config := defaultStreamConfig
		config.Name = "Cameras"
		stream, err := gostream.NewStream(config)
		if err != nil {
			return nil, false, err
		}
		streams = append(streams, stream)

		tilerHeight := 480 * len(displaySources)
		autoCameraTiler = gostream.NewAutoTiler(640, tilerHeight)
		autoCameraTiler.SetLogger(svc.logger)
	} else {
		for idx := range displaySources {
			config := x264.DefaultStreamConfig
			config.Name = displayNames[idx]
			view, err := gostream.NewStream(config)
			if err != nil {
				return nil, false, err
			}
			streams = append(streams, view)
		}
	}

	streamServer, err := gostream.NewStreamServer(streams...)
	if err != nil {
		return nil, false, err
	}

	// start background workers
	if autoCameraTiler != nil {
		for _, src := range displaySources {
			autoCameraTiler.AddSource(src)
		}
		waitCh := make(chan struct{})
		svc.activeBackgroundWorkers.Add(1)
		utils.PanicCapturingGo(func() {
			defer svc.activeBackgroundWorkers.Done()
			close(waitCh)
			gostream.StreamSource(ctx, autoCameraTiler, streams[0])
		})
		<-waitCh
	} else {
		for idx, stream := range streams {
			waitCh := make(chan struct{})
			svc.activeBackgroundWorkers.Add(1)
			utils.PanicCapturingGo(func() {
				defer svc.activeBackgroundWorkers.Done()
				close(waitCh)
				gostream.StreamSource(ctx, displaySources[idx], stream)
			})
			<-waitCh
		}
	}

	return streamServer, true, nil
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

// DefaultFQDN is the default name that will be used to identify the
// web server when one is not specified.
const DefaultFQDN = "local"

// RunWeb takes the given robot and options and runs the web server. This function will block
// until the context is done.
func (svc *webService) runWeb(ctx context.Context, options Options) (err error) {
	listener, secure, err := utils.NewPossiblySecureTCPListenerFromFile(
		options.Network.BindAddress,
		options.Network.TLSCertFile,
		options.Network.TLSKeyFile,
	)
	if err != nil {
		return err
	}
	listenerAddr := listener.Addr().String()
	options.secure = secure

	var signalingOpts []rpc.DialOption
	var insecureSignaling bool
	if options.SignalingAddress == "" {
		if !secure {
			signalingOpts = append(signalingOpts, rpc.WithInsecure())
			insecureSignaling = true
		}
	} else {
		_, port, err := net.SplitHostPort(options.SignalingAddress)
		if err != nil {
			return err
		}
		if port != "443" {
			signalingOpts = append(signalingOpts, rpc.WithInsecure())
			insecureSignaling = true
		}
	}
	options.secureSignaling = !insecureSignaling

	if len(options.FQDNs) == 0 {
		options.FQDNs = []string{DefaultFQDN}
	}
	rpcOpts := []rpc.ServerOption{
		rpc.WithWebRTCServerOptions(rpc.WebRTCServerOptions{
			Enable:                    true,
			ExternalSignalingDialOpts: signalingOpts,
			ExternalSignalingAddress:  options.SignalingAddress,
			SignalingHosts:            options.FQDNs,
			Config:                    &grpc.DefaultWebRTCConfiguration,
		}),
	}
	if options.Debug {
		rpcOpts = append(rpcOpts, rpc.WithDebug())
	}
	if len(options.Auth.Handlers) == 0 {
		rpcOpts = append(rpcOpts, rpc.WithUnauthenticated())
	} else {
		authEntities := options.FQDNs
		if !options.Managed {
			// allow authentication for non-unique entities.
			// This eases direct connections via address.
			authEntities = append(authEntities, listenerAddr, options.Network.BindAddress)
		}
		for _, handler := range options.Auth.Handlers {
			switch handler.Type {
			case rpc.CredentialsTypeAPIKey:
				apiKey := handler.Config.String("key")
				if apiKey == "" {
					return errors.Errorf("%q handler requires non-empty API key", rpc.CredentialsTypeAPIKey)
				}
				rpcOpts = append(rpcOpts, rpc.WithAuthHandler(
					rpc.CredentialsTypeAPIKey,
					rpc.MakeSimpleAuthHandler(authEntities, apiKey),
				))
			default:
				return errors.Errorf("do not know how to handle auth for %q", handler.Type)
			}
		}
	}
	rpcServer, err := rpc.NewServer(svc.logger, rpcOpts...)
	if err != nil {
		return err
	}
	svc.server = rpcServer
	if options.SignalingAddress == "" {
		options.SignalingAddress = listenerAddr
	}

	if err := rpcServer.RegisterServiceServer(
		ctx,
		&pb.RobotService_ServiceDesc,
		grpcserver.New(svc.r),
		pb.RegisterRobotServiceHandlerFromEndpoint,
	); err != nil {
		return err
	}

	// if metadata service is in the context, register it
	if s := service.ContextService(ctx); s != nil {
		if err := rpcServer.RegisterServiceServer(
			ctx,
			&metadatapb.MetadataService_ServiceDesc,
			grpcmetadata.New(s),
			metadatapb.RegisterMetadataServiceHandlerFromEndpoint,
		); err != nil {
			return err
		}
	}

	resources := make(map[resource.Name]interface{})
	for _, name := range svc.r.ResourceNames() {
		resource, ok := svc.r.ResourceByName(name)
		if !ok {
			continue
		}
		resources[name] = resource
	}
	if err := svc.update(resources); err != nil {
		return err
	}

	// register every subtype resource grpc service here
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
		if err := rs.RegisterRPCService(ctx, rpcServer, subtypeSvc); err != nil {
			return err
		}
	}

	streamServer, hasStreams, err := svc.makeStreamServer(ctx, svc.r, options)
	if err != nil {
		return err
	}
	if err := rpcServer.RegisterServiceServer(
		ctx,
		&streampb.StreamService_ServiceDesc,
		streamServer.ServiceServer(),
		streampb.RegisterStreamServiceHandlerFromEndpoint,
	); err != nil {
		return err
	}
	if hasStreams {
		// force WebRTC template rendering
		options.WebRTC = true
	}

	if options.Debug {
		if err := rpcServer.RegisterServiceServer(
			ctx,
			&echopb.EchoService_ServiceDesc,
			&echoserver.Server{},
			echopb.RegisterEchoServiceHandlerFromEndpoint,
		); err != nil {
			return err
		}
	}

	mux := goji.NewMux()
	if err := svc.installWeb(mux, svc.r, options); err != nil {
		return err
	}

	if options.Pprof {
		mux.HandleFunc(pat.New("/debug/pprof/"), pprof.Index)
		mux.HandleFunc(pat.New("/debug/pprof/cmdline"), pprof.Cmdline)
		mux.HandleFunc(pat.New("/debug/pprof/profile"), pprof.Profile)
		mux.HandleFunc(pat.New("/debug/pprof/symbol"), pprof.Symbol)
		mux.HandleFunc(pat.New("/debug/pprof/trace"), pprof.Trace)
	}

	mux.Handle(pat.New("/api/*"), http.StripPrefix("/api", rpcServer.GatewayHandler()))
	mux.Handle(pat.New("/*"), rpcServer.GRPCHandler())

	httpServer, err := utils.NewPlainTextHTTP2Server(mux)
	if err != nil {
		return err
	}
	httpServer.Addr = listenerAddr

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
			if err := rpcServer.Stop(); err != nil {
				svc.logger.Errorw("error stopping rpc server", "error", err)
			}
		}()
		if streamServer != nil {
			if err := streamServer.Close(); err != nil {
				svc.logger.Errorw("error closing stream server", "error", err)
			}
		}
	})
	svc.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer svc.activeBackgroundWorkers.Done()
		if err := rpcServer.Start(); err != nil {
			svc.logger.Errorw("error starting rpc server", "error", err)
		}
	})

	var scheme string
	if secure {
		scheme = "https"
	} else {
		scheme = "http"
	}
	svc.logger.Infow("serving", "url", fmt.Sprintf("%s://%s", scheme, listenerAddr))
	svc.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer svc.activeBackgroundWorkers.Done()
		if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			svc.logger.Errorw("error serving rpc server", "error", err)
		}
	})
	return err
}
