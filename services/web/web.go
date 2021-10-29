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
	"strconv"
	"sync"

	"github.com/Masterminds/sprig"
	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/edaniels/gostream/codec/x264"
	"github.com/go-errors/errors"
	"github.com/golang/geo/r3"

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
	robotimpl "go.viam.com/core/robot/impl"

	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/spatialmath"
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

// grabAtCameraPositionHandler moves a gripper towards an object
type grabAtCameraPositionHandler struct {
	app *robotWebApp
}

// TODO: make doGrab a service
func (h *grabAtCameraPositionHandler) doGrab(ctx context.Context, cameraName string, x, y, z float64) (bool, error) {
	r := h.app.theRobot
	// get gripper component
	if len(r.GripperNames()) != 1 {
		return false, errors.New("robot needs exactly 1 arm for doGrab")
	}
	gripperName := r.GripperNames()[0]
	gripper, ok := r.GripperByName(gripperName)
	if !ok {
		return false, fmt.Errorf("failed to find gripper %q", gripperName)
	}
	// do gripper movement
	err := gripper.Open(ctx)
	if err != nil {
		return false, err
	}
	cameraPoint := r3.Vector{x, y, z}
	cameraPose := spatialmath.NewPoseFromPoint(cameraPoint)
	err = robotimpl.MoveGripper(ctx, r, cameraPose, cameraName)
	if err != nil {
		return false, err
	}
	return gripper.Grab(ctx)
}

func (h *grabAtCameraPositionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	cameraName := pat.Param(r, "camera")

	xString := pat.Param(r, "x")
	yString := pat.Param(r, "y")
	zString := pat.Param(r, "z")
	if xString == "" || yString == "" || zString == "" {
		http.NotFound(w, r)
		return
	}

	x, err := strconv.ParseFloat(xString, 64)
	if err != nil {
		http.Error(w, fmt.Sprintf("error parsing x (%s): %s", xString, err), http.StatusBadRequest)
		return
	}

	y, err := strconv.ParseFloat(yString, 64)
	if err != nil {
		http.Error(w, fmt.Sprintf("error parsing y (%s): %s", yString, err), http.StatusBadRequest)
		return
	}

	z, err := strconv.ParseFloat(zString, 64)
	if err != nil {
		http.Error(w, fmt.Sprintf("error parsing z (%s): %s", zString, err), http.StatusBadRequest)
		return
	}

	didGrab, err := h.doGrab(ctx, cameraName, x, y, z)
	if err != nil {
		h.app.logger.Errorf("error grabbing: %s", err)
		http.Error(w, fmt.Sprintf("error grabbing: %s", err), http.StatusInternalServerError)
		return
	}
	if !didGrab {
		h.app.logger.Error("failed to grab anything")
	}

}

var defaultViewConfig = x264.DefaultViewConfig

func init() {
	defaultViewConfig.WebRTCConfig = grpc.DefaultWebRTCConfiguration
}

// A Service controls the web server for a robot.
type Service interface {
	// Update updates the web service when the robot has changed. Not Reconfigure because this should happen at a different point in the
	// lifecycle.
	Update(resources map[resource.Name]interface{}) error

	// Close attempts to close the web service gracefully
	Close() error
}

// New returns a new web service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (Service, error) {
	options := ContextOptions(ctx)
	if options == (Options{}) {
		options = NewOptions()
	}
	if err := RunWeb(ctx, r, options, logger); err != nil {
		return nil, err
	}

	_, cancelFunc := context.WithCancel(context.Background())
	webSvc := &webService{
		r:          r,
		logger:     logger,
		cancelFunc: cancelFunc,
	}
	return webSvc, nil
}

type webService struct {
	mu sync.RWMutex
	r  robot.Robot

	logger                  golog.Logger
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

func (svc *webService) Update(resources map[resource.Name]interface{}) error {
	// TODO: update itself properly
	svc.mu.Lock()
	defer svc.mu.Unlock()
	return nil
}

func (svc *webService) Close() error {
	svc.cancelFunc()
	svc.activeBackgroundWorkers.Wait()
	return nil
}

// installWeb prepares the given mux to be able to serve the UI for the robot. It also starts some goroutines
// for image processing that can be cleaned up with the returned cleanup function.
func installWeb(ctx context.Context, mux *goji.Mux, theRobot robot.Robot, options Options, logger golog.Logger) (func(), error) {
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
			autoCameraTiler.SetLogger(logger)

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

	app := &robotWebApp{views: views, theRobot: theRobot, logger: logger, options: options}
	if err := app.Init(); err != nil {
		return nil, err
	}

	mux.Handle(pat.Get("/grab_at_camera_position/:camera/:x/:y/:z"), &grabAtCameraPositionHandler{app})
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
		goutils.PanicCapturingGo(func() {
			close(waitCh)
			gostream.StreamNamedSource(ctx, autoCameraTiler, "Cameras", views[0])
		})
		<-waitCh
	} else {
		for idx, view := range views {
			waitCh := make(chan struct{})
			goutils.PanicCapturingGo(func() {
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
func RunWeb(ctx context.Context, theRobot robot.Robot, options Options, logger golog.Logger) (err error) {
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
	rpcServer, err := rpcserver.NewWithOptions(rpcOpts, logger)
	if err != nil {
		return err
	}
	if options.SignalingAddress == "" {
		options.SignalingAddress = fmt.Sprintf("localhost:%d", options.Port)
	}
	if options.Name == "" {
		options.Name = rpcServer.SignalingHost()
	}

	if err := rpcServer.RegisterServiceServer(
		ctx,
		&pb.RobotService_ServiceDesc,
		grpcserver.New(theRobot),
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

	// TODO: register individual grpc services

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
	webCloser, err := installWeb(ctx, mux, theRobot, options, logger)
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

	goutils.PanicCapturingGo(func() {
		<-ctx.Done()
		webCloser()
		defer func() {
			if err := httpServer.Shutdown(context.Background()); err != nil {
				theRobot.Logger().Errorw("error shutting down", "error", err)
			}
		}()
		if err := rpcServer.Stop(); err != nil {
			theRobot.Logger().Errorw("error stopping rpc server", "error", err)
		}
	})
	goutils.PanicCapturingGo(func() {
		if err := rpcServer.Start(); err != nil {
			theRobot.Logger().Errorw("error starting rpc server", "error", err)
		}
	})

	theRobot.Logger().Debugw("serving", "url", fmt.Sprintf("http://%s", listener.Addr().String()))
	goutils.PanicCapturingGo(func() {
		if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			theRobot.Logger().Errorw("error serving rpc server", "error", err)
		}
	})
	return err
}
