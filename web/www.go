// Package web provides gRPC/REST/GUI APIs to control and monitor a robot.
package web

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"image"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"go.viam.com/core/action"
	grpcserver "go.viam.com/core/grpc/server"
	"go.viam.com/core/lidar"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/rimage"
	"go.viam.com/core/robot"
	"go.viam.com/core/rpc"
	"go.viam.com/core/utils"

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
	options  Options
}

// Init does template initialization work.
func (app *robotWebApp) Init() error {
	t, err := template.New("foo").Funcs(template.FuncMap{
		"jsSafe": func(js string) template.JS {
			return template.JS(js)
		},
		"htmlSafe": func(html string) template.HTML {
			return template.HTML(html)
		},
	}).Funcs(sprig.FuncMap()).ParseGlob(fmt.Sprintf("%s/*.html", ResolveSharedDir(app.options.SharedDir)+"/templates"))
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
		Actions []string
		Views   []View
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

	img, closer, err := cam.Next(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("error reading camera: %s", err), http.StatusInternalServerError)
		return
	}
	defer closer()

	iwd := rimage.ConvertToImageWithDepth(img)
	pc, err := iwd.ToPointCloud()
	if err != nil {
		http.Error(w, fmt.Sprintf("error converting image to pointcloud: %s", err), http.StatusInternalServerError)
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
func installWeb(ctx context.Context, mux *goji.Mux, theRobot robot.Robot, options Options, logger golog.Logger) (func(), error) {
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
func RunWeb(ctx context.Context, theRobot robot.Robot, options Options, logger golog.Logger) (err error) {
	defer func() {
		err = multierr.Combine(err, utils.TryClose(theRobot))
	}()

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", options.Port))
	if err != nil {
		return err
	}

	rpcServer, err := rpc.NewServer()
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

	utils.PanicCapturingGo(func() {
		<-ctx.Done()
		webCloser()
		defer func() {
			if err := rpcServer.Stop(); err != nil {
				theRobot.Logger().Errorw("error stopping", "error", err)
			}
		}()
		if err := httpServer.Shutdown(context.Background()); err != nil {
			theRobot.Logger().Errorw("error shutting down", "error", err)
		}
	})
	utils.PanicCapturingGo(func() {
		if err := rpcServer.Start(); err != nil {
			theRobot.Logger().Errorw("error starting", "error", err)
		}
	})

	theRobot.Logger().Debugw("serving", "url", fmt.Sprintf("http://%s", listener.Addr().String()))
	if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}
