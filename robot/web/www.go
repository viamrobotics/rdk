package web

import (
	"context"
	"fmt"
	"html/template"
	"image"
	"net"
	"net/http"
	"net/http/pprof"
	"path/filepath"
	"runtime"
	"time"

	apiserver "go.viam.com/robotcore/api/server"
	"go.viam.com/robotcore/lidar"
	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/robot"
	"go.viam.com/robotcore/robot/actions"
	"go.viam.com/robotcore/rpc"
	"go.viam.com/robotcore/utils"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/Masterminds/sprig"
	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/edaniels/gostream/codec/x264"
	"go.uber.org/multierr"
	"goji.io"
	"goji.io/pat"
)

type robotWebApp struct {
	template *template.Template
	views    []gostream.View
	theRobot *robot.Robot
	logger   golog.Logger
}

func (app *robotWebApp) Init() error {
	_, thisFilePath, _, _ := runtime.Caller(0)
	thisDirPath, err := filepath.Abs(filepath.Dir(thisFilePath))
	if err != nil {
		return err
	}
	t, err := template.New("foo").Funcs(template.FuncMap{
		"jsSafe": func(js string) template.JS {
			return template.JS(js)
		},
		"htmlSafe": func(html string) template.HTML {
			return template.HTML(html)
		},
	}).Funcs(sprig.FuncMap()).ParseGlob(fmt.Sprintf("%s/*.html", thisDirPath))
	if err != nil {
		return err
	}
	app.template = t.Lookup("webappindex.html")
	return nil
}

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
		Actions: actions.AllActionNames(),
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

// ---------------

func allSourcesToDisplay(ctx context.Context, theRobot *robot.Robot) ([]gostream.ImageSource, []string, error) {
	sources := []gostream.ImageSource{}
	names := []string{}

	config, err := theRobot.GetConfig(ctx)
	if err != nil {
		return nil, nil, err
	}

	for _, name := range theRobot.CameraNames() {
		cam := theRobot.CameraByName(name)
		cmp := config.FindComponent(name)
		if cmp != nil && cmp.Attributes.GetBool("hide", false) {
			continue
		}

		sources = append(sources, cam)
		names = append(names, name)
	}

	for _, name := range theRobot.LidarDeviceNames() {
		device := theRobot.LidarDeviceByName(name)
		cmp := config.FindComponent(name)
		if cmp != nil && cmp.Attributes.GetBool("hide", false) {
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

// returns a closer func to be called when done
func InstallWeb(ctx context.Context, mux *goji.Mux, theRobot *robot.Robot, options Options, logger golog.Logger) (func(), error) {
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

	app := &robotWebApp{views: views, theRobot: theRobot, logger: logger}
	if err := app.Init(); err != nil {
		return nil, err
	}

	mux.Handle(pat.Get("/static/*"), http.StripPrefix("/static", http.FileServer(http.Dir(utils.ResolveFile("robot/web/frontend/dist")))))
	mux.Handle(pat.New("/"), app)

	for _, view := range views {
		handler := view.Handler()
		mux.Handle(pat.New("/"+handler.Name), handler.Func)
	}

	// start background threads

	if autoCameraTiler != nil {
		for _, src := range displaySources {
			autoCameraTiler.AddSource(src)
		}
		waitCh := make(chan struct{})
		go func() {
			close(waitCh)
			gostream.StreamNamedSource(ctx, autoCameraTiler, "Cameras", views[0])
		}()
		<-waitCh
	} else {
		for idx, view := range views {
			waitCh := make(chan struct{})
			go func() {
				close(waitCh)
				gostream.StreamNamedSource(ctx, displaySources[idx], displayNames[idx], view)
			}()
			<-waitCh
		}
	}

	return func() {
		for _, view := range views {
			view.Stop()
		}
	}, nil

}

// ---

/*
helper if you don't need to customize at all
*/
func RunWeb(ctx context.Context, theRobot *robot.Robot, options Options, logger golog.Logger) error {
	defer theRobot.Close()

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
		apiserver.New(theRobot),
		pb.RegisterRobotServiceHandlerFromEndpoint,
	); err != nil {
		return err
	}

	mux := goji.NewMux()
	webCloser, err := InstallWeb(ctx, mux, theRobot, options, logger)
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

	h2s := &http2.Server{}
	httpServer := &http.Server{
		Addr:           listener.Addr().String(),
		ReadTimeout:    10 * time.Second,
		MaxHeaderBytes: 1 << 20,
		Handler:        h2c.NewHandler(mux, h2s),
	}

	go func() {
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
	}()
	go func() {
		if err := rpcServer.Start(); err != nil {
			theRobot.Logger().Errorw("error starting", "error", err)
		}
	}()

	theRobot.Logger().Debugw("serving", "url", fmt.Sprintf("http://%s", listener.Addr().String()))
	if err := httpServer.Serve(listener); err != http.ErrServerClosed {
		return err
	}

	return nil
}
