package web

import (
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"net/http/pprof"
	"sync"

	"github.com/Masterminds/sprig"
	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/edaniels/gostream/codec/x264"
	streampb "github.com/edaniels/gostream/proto/stream/v1"
	"github.com/pkg/errors"

	"go.viam.com/utils"
	goutils "go.viam.com/utils"
	echopb "go.viam.com/utils/proto/rpc/examples/echo/v1"
	"go.viam.com/utils/rpc"
	echoserver "go.viam.com/utils/rpc/examples/echo/server"

	"go.viam.com/core/action"
	"go.viam.com/core/config"
	"go.viam.com/core/grpc"
	grpcmetadata "go.viam.com/core/grpc/metadata/server"
	grpcserver "go.viam.com/core/grpc/server"
	"go.viam.com/core/metadata/service"
	metadatapb "go.viam.com/core/proto/api/service/v1"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/resource"
	"go.viam.com/core/subtype"

	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/web"

	"goji.io"
	"goji.io/pat"
)

// Type is the type of service.
const Type = config.ServiceType("web")

func init() {
	registry.RegisterService(Type, registry.Service{
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
		"jsSafe": func(js string) template.JS {
			return template.JS(js)
		},
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
	}

	temp := Temp{
		Actions: action.AllActionNames(),
	}

	if app.options.WebRTC {
		temp.WebRTCEnabled = true
		if app.options.internalSignaling && !app.options.secure {
			temp.WebRTCSignalingAddress = fmt.Sprintf("http://%s", app.options.SignalingAddress)
		} else {
			temp.WebRTCSignalingAddress = fmt.Sprintf("https://%s", app.options.SignalingAddress)
		}
		temp.WebRTCHost = app.options.Name
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

	// Close attempts to close the web service gracefully
	Close() error
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

// Start starts the web server, will return an error if server is already up
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

func (svc *webService) Close() error {
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
		goutils.PanicCapturingGo(func() {
			defer svc.activeBackgroundWorkers.Done()
			close(waitCh)
			gostream.StreamSource(ctx, autoCameraTiler, streams[0])
		})
		<-waitCh
	} else {
		for idx, stream := range streams {
			waitCh := make(chan struct{})
			svc.activeBackgroundWorkers.Add(1)
			goutils.PanicCapturingGo(func() {
				defer svc.activeBackgroundWorkers.Done()
				close(waitCh)
				gostream.StreamSource(ctx, displaySources[idx], stream)
			})
			<-waitCh
		}
	}

	return streamServer, true, nil
}

// installWeb prepares the given mux to be able to serve the UI for the robot
func (svc *webService) installWeb(ctx context.Context, mux *goji.Mux, theRobot robot.Robot, options Options) error {
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

// RunWeb takes the given robot and options and runs the web server. This function will block
// until the context is done.
func (svc *webService) runWeb(ctx context.Context, options Options) (err error) {
	listener, secure, err := utils.NewPossiblySecureTCPListenerFromFile(
		options.Port, options.TLSCertFile, options.TLSKeyFile)
	if err != nil {
		return err
	}
	options.secure = secure
	humanAddress := fmt.Sprintf("localhost:%d", options.Port)

	var signalingOpts []rpc.DialOption
	if options.SignalingAddress == "" && !secure {
		signalingOpts = append(signalingOpts, rpc.WithInsecure())
	}

	rpcOpts := []rpc.ServerOption{
		rpc.WithWebRTCServerOptions(rpc.WebRTCServerOptions{
			Enable:                    true,
			ExternalSignalingDialOpts: signalingOpts,
			ExternalSignalingAddress:  options.SignalingAddress,
			SignalingHost:             options.Name,
			Config:                    &grpc.DefaultWebRTCConfiguration,
		}),
	}
	if options.Debug {
		rpcOpts = append(rpcOpts, rpc.WithDebug())
	}
	// TODO(erd): later: add command flags to enable auth
	if true {
		rpcOpts = append(rpcOpts, rpc.WithUnauthenticated())
	}
	rpcServer, err := rpc.NewServer(svc.logger, rpcOpts...)
	if err != nil {
		return err
	}
	svc.server = rpcServer
	if options.SignalingAddress == "" {
		options.internalSignaling = true
		options.SignalingAddress = fmt.Sprintf("localhost:%d", options.Port)
	}
	if options.Name == "" {
		options.Name = rpcServer.SignalingHost()
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
	if err := svc.update(ctx, resources); err != nil {
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
		context.Background(),
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
			context.Background(),
			&echopb.EchoService_ServiceDesc,
			&echoserver.Server{},
			echopb.RegisterEchoServiceHandlerFromEndpoint,
		); err != nil {
			return err
		}
	}

	mux := goji.NewMux()
	if err := svc.installWeb(ctx, mux, svc.r, options); err != nil {
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

	httpServer, err := goutils.NewPlainTextHTTP2Server(mux)
	if err != nil {
		return err
	}
	httpServer.Addr = listener.Addr().String()

	svc.activeBackgroundWorkers.Add(1)
	goutils.PanicCapturingGo(func() {
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
	goutils.PanicCapturingGo(func() {
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
	svc.logger.Infow("serving", "url", fmt.Sprintf("%s://%s", scheme, humanAddress))
	svc.activeBackgroundWorkers.Add(1)
	goutils.PanicCapturingGo(func() {
		defer svc.activeBackgroundWorkers.Done()
		if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			svc.logger.Errorw("error serving rpc server", "error", err)
		}
	})
	return err
}
