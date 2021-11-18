package web

import (
	"context"
	"fmt"
	"html/template"
	"image"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"sync"

	"github.com/Masterminds/sprig"
	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/edaniels/gostream/codec/x264"
	"github.com/go-errors/errors"

	goutils "go.viam.com/utils"
	echopb "go.viam.com/utils/proto/rpc/examples/echo/v1"
	echoserver "go.viam.com/utils/rpc/examples/echo/server"
	rpcserver "go.viam.com/utils/rpc/server"

	"go.viam.com/core/action"
	"go.viam.com/core/config"
	"go.viam.com/core/grpc"
	grpcmetadata "go.viam.com/core/grpc/metadata/server"
	grpcserver "go.viam.com/core/grpc/server"
	"go.viam.com/core/lidar"
	"go.viam.com/core/metadata/service"
	metadatapb "go.viam.com/core/proto/api/service/v1"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/resource"
	"go.viam.com/core/subtype"

	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/utils"
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

// ResolveSharedDir discovers where the shared assets directory
// is located depending on how the executable was called.
func ResolveSharedDir(target string) string {
	calledBinary, err := os.Executable()
	if err != nil {
		panic(err)
	}

	if target != "" {
		return target
	} else if calledBinary == "/usr/bin/viam-server" {
		if _, err := os.Stat("/usr/share/viam"); !os.IsNotExist(err) {
			return "/usr/share/viam"
		}
	}
	return utils.ResolveFile("web/runtime-shared")
}

// robotWebApp hosts a web server to interact with a robot in addition to hosting
// a gRPC/REST server.
type robotWebApp struct {
	template *template.Template
	views    []gostream.View
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

	if app.options.Debug {
		t, err = t.ParseGlob(fmt.Sprintf("%s/*.html", ResolveSharedDir(app.options.SharedDir)+"/templates"))
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

	type View struct {
		JavaScript string
		Body       string
	}

	type Temp struct {
		External                   bool
		WebRTCEnabled              bool
		WebRTCHost                 string
		WebRTCSignalingAddress     string
		WebRTCAdditionalICEServers []map[string]interface{}
		Actions                    []string
		Views                      []View
	}

	temp := Temp{
		Actions: action.AllActionNames(),
	}

	if app.options.WebRTC {
		temp.WebRTCEnabled = true
		if app.options.Insecure {
			temp.WebRTCSignalingAddress = fmt.Sprintf("http://%s", app.options.SignalingAddress)
		} else {
			temp.WebRTCSignalingAddress = fmt.Sprintf("https://%s", app.options.SignalingAddress)
		}
		temp.WebRTCHost = app.options.Name
	}

	for _, view := range app.views {
		htmlData := view.HTML()
		temp.Views = append(temp.Views, View{
			htmlData.JavaScript,
			htmlData.Body,
		})
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

	for _, name := range theRobot.LidarNames() {
		device, ok := theRobot.LidarByName(name)
		if !ok {
			continue
		}
		cmp := conf.FindComponent(name)
		if cmp != nil && cmp.Attributes.Bool("hide", false) {
			continue
		}

		if err := device.Start(ctx); err != nil {
			return nil, nil, err
		}
		source := lidar.NewImageSource(image.Point{600, 600}, device)

		sources = append(sources, source)
		names = append(names, name)
	}

	return sources, names, nil
}

var defaultViewConfig = x264.DefaultViewConfig

func init() {
	defaultViewConfig.WebRTCConfig = grpc.DefaultWebRTCConfiguration
}

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
	server   rpcserver.Server
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

// installWeb prepares the given mux to be able to serve the UI for the robot. It also starts some goroutines
// for image processing that can be cleaned up with the returned cleanup function.
func (svc *webService) installWeb(ctx context.Context, mux *goji.Mux, theRobot robot.Robot, options Options) (func(), error) {
	displaySources, displayNames, err := allSourcesToDisplay(ctx, theRobot)
	if err != nil {
		return nil, err
	}
	views := []gostream.View{}
	var autoCameraTiler *gostream.AutoTiler

	if len(displaySources) != 0 {
		if options.AutoTile {
			config := defaultViewConfig
			view, err := gostream.NewView(config)
			if err != nil {
				return nil, err
			}
			views = append(views, view)

			tilerHeight := 480 * len(displaySources)
			autoCameraTiler = gostream.NewAutoTiler(640, tilerHeight)
			autoCameraTiler.SetLogger(svc.logger)

		} else {
			for idx := range displaySources {
				config := x264.DefaultViewConfig
				config.StreamNumber = idx
				config.StreamName = displayNames[idx]
				view, err := gostream.NewView(config)
				if err != nil {
					return nil, err
				}
				views = append(views, view)
			}
		}
	}

	app := &robotWebApp{views: views, theRobot: theRobot, logger: svc.logger, options: options}
	if err := app.Init(); err != nil {
		return nil, err
	}

	mux.Handle(pat.Get("/static/*"), http.StripPrefix("/static", http.FileServer(http.Dir(ResolveSharedDir(app.options.SharedDir)+"/static"))))
	mux.Handle(pat.New("/"), app)

	for _, view := range views {
		handler := view.Handler()
		mux.Handle(pat.New("/"+handler.Name), handler.Func)
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
			gostream.StreamNamedSource(ctx, autoCameraTiler, "Cameras", views[0])
		})
		<-waitCh
	} else {
		for idx, view := range views {
			waitCh := make(chan struct{})
			svc.activeBackgroundWorkers.Add(1)
			goutils.PanicCapturingGo(func() {
				defer svc.activeBackgroundWorkers.Done()
				close(waitCh)
				gostream.StreamNamedSource(ctx, displaySources[idx], displayNames[idx], view)
			})
			<-waitCh
		}
	}

	return func() {
		for _, view := range views {
			view.Stop()
		}
	}, nil
}

// RunWeb takes the given robot and options and runs the web server. This function will block
// until the context is done.
func (svc *webService) runWeb(ctx context.Context, options Options) (err error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", options.Port))
	if err != nil {
		return err
	}

	rpcOpts := rpcserver.Options{
		WebRTC: rpcserver.WebRTCOptions{
			Enable:           true,
			Insecure:         options.Insecure,
			SignalingAddress: options.SignalingAddress,
			SignalingHost:    options.Name,
			Config:           &grpc.DefaultWebRTCConfiguration,
		},
		Debug: options.Debug,
	}
	rpcServer, err := rpcserver.NewWithOptions(rpcOpts, svc.logger)
	if err != nil {
		return err
	}
	svc.server = rpcServer
	if options.SignalingAddress == "" {
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
	webCloser, err := svc.installWeb(ctx, mux, svc.r, options)
	if err != nil {
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
		webCloser()
		defer func() {
			if err := httpServer.Shutdown(context.Background()); err != nil {
				svc.logger.Errorw("error shutting down", "error", err)
			}
		}()
		if err := rpcServer.Stop(); err != nil {
			svc.logger.Errorw("error stopping rpc server", "error", err)
		}
	})
	svc.activeBackgroundWorkers.Add(1)
	goutils.PanicCapturingGo(func() {
		defer svc.activeBackgroundWorkers.Done()
		if err := rpcServer.Start(); err != nil {
			svc.logger.Errorw("error starting rpc server", "error", err)
		}
	})

	svc.logger.Debugw("serving", "url", fmt.Sprintf("http://%s", listener.Addr().String()))
	svc.activeBackgroundWorkers.Add(1)
	goutils.PanicCapturingGo(func() {
		defer svc.activeBackgroundWorkers.Done()
		if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			svc.logger.Errorw("error serving rpc server", "error", err)
		}
	})
	return err
}
