// Package server implements gRPC/REST/GUI APIs to control and monitor a robot.
package server

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
	"time"

	"github.com/go-errors/errors"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"go.viam.com/core/action"
	"go.viam.com/core/camera"
	grpcserver "go.viam.com/core/grpc/server"
	"go.viam.com/core/lidar"
	pb "go.viam.com/core/proto/api/v1"
	echopb "go.viam.com/core/proto/rpc/examples/echo/v1"
	"go.viam.com/core/referenceframe"
	"go.viam.com/core/robot"
	echoserver "go.viam.com/core/rpc/examples/echo/server"
	rpcserver "go.viam.com/core/rpc/server"
	"go.viam.com/core/utils"
	"go.viam.com/core/web"

	"github.com/Masterminds/sprig"
	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/edaniels/gostream/codec/x264"
	"go.uber.org/multierr"
	"goji.io"
	"goji.io/pat"
)

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
	options  web.Options
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
		External               bool
		WebRTCEnabled          bool
		WebRTCHost             string
		WebRTCSignalingAddress string
		Actions                []string
		Views                  []View
	}

	temp := Temp{
		Actions: action.AllActionNames(),
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
		cam := theRobot.CameraByName(name)
		cmp := conf.FindComponent(name)
		if cmp != nil && cmp.Attributes.Bool("hide", false) {
			continue
		}

		sources = append(sources, cam)
		names = append(names, name)
	}

	for _, name := range theRobot.LidarNames() {
		device := theRobot.LidarByName(name)
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

func (h *grabAtCameraPositionHandler) doGrab(ctx context.Context, cameraName string, camera camera.Camera, x, y, z float64) error {
	if len(h.app.theRobot.ArmNames()) != 1 {
		return errors.New("robot needs exactly 1 arm to do grabAt")
	}

	armName := h.app.theRobot.ArmNames()[0]
	arm := h.app.theRobot.ArmByName(armName)

	frameLookup, err := h.app.theRobot.FrameLookup(ctx)
	if err != nil {
		return err
	}

	pos, err := referenceframe.FindTranslationChildToParent(ctx, frameLookup, cameraName, armName)
	if err != nil {
		return err
	}

	h.app.logger.Debugf("move - %v %v %v\n", x, y, z)

	h.app.logger.Debugf("pos a: %#v\n", pos)
	pos = referenceframe.OffsetBy(&pb.ArmPosition{X: x, Y: y, Z: z}, pos)
	h.app.logger.Debugf("pos b: %#v\n", pos)

	return arm.MoveToPosition(ctx, pos)
}

func (h *grabAtCameraPositionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	cameraName := pat.Param(r, "camera")
	camera := h.app.theRobot.CameraByName(cameraName)
	if camera == nil {
		http.NotFound(w, r)
		return
	}

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

	err = h.doGrab(ctx, cameraName, camera, x, y, z)
	if err != nil {
		h.app.logger.Errorf("error grabbing: %s", err)
		http.Error(w, fmt.Sprintf("error grabbing: %s", err), http.StatusInternalServerError)
		return
	}

}

// pcdHandler helps serve PCD (point cloud data) files.
type pcdHandler struct {
	app *robotWebApp
}

// ServeHTTP converts a particular camera output to PCD and then serves the data.
// Rendering it is up to the caller.
func (h *pcdHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	camName := pat.Param(r, "name")
	cam := h.app.theRobot.CameraByName(camName)
	if cam == nil {
		http.NotFound(w, r)
		return
	}
	pc, err := cam.NextPointCloud(ctx)
	if err != nil {
		h.app.logger.Errorf("error getting pointcloud: %s", err)
		http.Error(w, fmt.Sprintf("error getting pointcloud: %s", err), http.StatusInternalServerError)
		return
	}
	err = pc.ToPCD(w)
	if err != nil {
		h.app.logger.Debugf("error converting to pcd: %s", err)
		http.Error(w, fmt.Sprintf("error writing pcd: %s", err), http.StatusInternalServerError)
		return
	}
}

// installWeb prepares the given mux to be able to serve the UI for the robot. It also starts some goroutines
// for image processing that can be cleaned up with the returned cleanup function.
func installWeb(ctx context.Context, mux *goji.Mux, theRobot robot.Robot, options web.Options, logger golog.Logger) (func(), error) {
	displaySources, displayNames, err := allSourcesToDisplay(ctx, theRobot)
	if err != nil {
		return nil, err
	}
	views := []gostream.View{}
	var autoCameraTiler *gostream.AutoTiler

	if len(displaySources) != 0 {
		if options.AutoTile {
			config := x264.DefaultViewConfig
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

	mux.Handle(pat.Get("/cameras/:name/data.pcd"), &pcdHandler{app})
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
		utils.PanicCapturingGo(func() {
			close(waitCh)
			gostream.StreamNamedSource(ctx, autoCameraTiler, "Cameras", views[0])
		})
		<-waitCh
	} else {
		for idx, view := range views {
			waitCh := make(chan struct{})
			utils.PanicCapturingGo(func() {
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
func RunWeb(ctx context.Context, theRobot robot.Robot, options web.Options, logger golog.Logger) (err error) {
	defer func() {
		if err != nil && utils.FilterOutError(err, context.Canceled) != nil {
			logger.Errorw("error running web", "error", err)
		}
		err = multierr.Combine(err, utils.TryClose(theRobot))
	}()

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", options.Port))
	if err != nil {
		return err
	}

	rpcOpts := rpcserver.Options{WebRTC: rpcserver.WebRTCOptions{
		Enable:           true,
		Insecure:         options.Insecure,
		SignalingAddress: options.SignalingAddress,
		SignalingHost:    options.Name,
	}}
	rpcServer, err := rpcserver.NewWithOptions(rpcOpts, logger)
	if err != nil {
		return err
	}
	defer func() {
		err = multierr.Combine(err, rpcServer.Stop())
	}()

	if err := rpcServer.RegisterServiceServer(
		ctx,
		&pb.RobotService_ServiceDesc,
		grpcserver.New(theRobot),
		pb.RegisterRobotServiceHandlerFromEndpoint,
	); err != nil {
		return err
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

	httpServer := &http.Server{
		Addr:           listener.Addr().String(),
		ReadTimeout:    10 * time.Second,
		MaxHeaderBytes: 1 << 20,
		Handler:        h2c.NewHandler(mux, &http2.Server{}),
	}

	stopped := make(chan struct{})
	defer func() {
		<-stopped
	}()
	utils.PanicCapturingGo(func() {
		defer func() {
			close(stopped)
		}()
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
	utils.PanicCapturingGo(func() {
		if err := rpcServer.Start(); err != nil {
			theRobot.Logger().Errorw("error starting rpc server", "error", err)
		}
	})

	theRobot.Logger().Debugw("serving", "url", fmt.Sprintf("http://%s", listener.Addr().String()))
	if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}
