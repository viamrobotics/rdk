package web

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"image/jpeg"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/Masterminds/sprig"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/edaniels/gostream/codec/x264"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/board"
	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/robot"
	"go.viam.com/robotcore/robot/actions"
)

type robotWebApp struct {
	template *template.Template
	views    []gostream.View
	theRobot *robot.Robot
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
			golog.Global.Debugf("couldn't reload template: %s", err)
			return
		}
	}

	type View struct {
		JavaScript string
		Body       string
	}

	type Temp struct {
		Views    []View
		Bases    []string
		Arms     []string
		Grippers []string
		Boards   []board.Config
	}

	temp := Temp{
		Bases:    app.theRobot.BaseNames(),
		Arms:     app.theRobot.ArmNames(),
		Grippers: app.theRobot.GripperNames(),
	}

	for _, name := range app.theRobot.BoardNames() {
		b := app.theRobot.BoardByName(name)
		temp.Boards = append(temp.Boards, b.GetConfig())
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
		golog.Global.Debugf("couldn't execute web page: %s", err)
	}
}

// ---------------

func InstallWebBase(mux *http.ServeMux, theBase api.Base) {

	mux.Handle("/api/base", &apiCall{func(r *http.Request) (map[string]interface{}, error) {
		millisPerSec := 500.0 // TODO(erh): this is proably the wrong default
		if r.FormValue("speed") != "" {
			speed2, err := strconv.ParseFloat(r.FormValue("speed"), 64)
			if err != nil {
				return nil, err
			}
			millisPerSec = speed2
		}

		s := r.FormValue("stop")
		d := r.FormValue("distanceMillis")
		a := r.FormValue("angle")

		var err error

		if s == "t" || s == "true" {
			err = theBase.Stop(r.Context())
		} else if d != "" {
			d2, err2 := strconv.ParseInt(d, 10, 64)
			if err2 != nil {
				return nil, err2
			}

			err = theBase.MoveStraight(r.Context(), int(d2), millisPerSec, false)
		} else if a != "" {
			a2, err2 := strconv.ParseInt(a, 10, 64)
			if err2 != nil {
				return nil, err2
			}

			// TODO(erh): fix speed
			err = theBase.Spin(r.Context(), float64(a2), 64, false)
		} else {
			return nil, fmt.Errorf("no stop, distanceMillis, angle given")
		}

		if err != nil {
			return nil, fmt.Errorf("erorr moving %s", err)
		}

		return nil, nil

	}})
}

type apiMethod func(r *http.Request) (map[string]interface{}, error)

type apiCall struct {
	theMethod apiMethod
}

func (ac *apiCall) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	res, err := ac.theMethod(r)
	if err != nil {
		golog.Global.Warnf("error in api call: %s", err)
		res = map[string]interface{}{"err": err.Error()}
	}

	if res == nil {
		res = map[string]interface{}{"ok": true}
	}

	js, err := json.Marshal(res)
	if err != nil {
		golog.Global.Warnf("cannot marshal json: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js) //nolint
}

func InstallActions(mux *http.ServeMux, theRobot *robot.Robot) {
	mux.Handle("/api/action", &apiCall{func(r *http.Request) (map[string]interface{}, error) {
		name := r.FormValue("name")
		switch name {
		case "RandomWalk":
			go actions.RandomWalk(theRobot, 60)
			return map[string]interface{}{"started": true}, nil
		case "ResetBox":
			go actions.ResetBox(theRobot, 4)
			return map[string]interface{}{"started": true}, nil
		default:
			return nil, fmt.Errorf("unknown action name [%s]", name)
		}
	}})
}

func InstallWebArms(mux *http.ServeMux, theRobot *robot.Robot) {
	mux.Handle("/api/arm", &apiCall{func(r *http.Request) (map[string]interface{}, error) {
		mode := r.FormValue("mode")
		if mode == "" {
			mode = "grid"
		}
		action := r.FormValue("action")

		arm := theRobot.ArmByName(r.FormValue("name"))
		if arm == nil {
			return nil, fmt.Errorf("no arm with name (%s)", r.FormValue("name"))
		}

		if mode == "grid" {

			where, err := arm.CurrentPosition()
			if err != nil {
				return nil, err
			}

			changed := false
			for _, n := range []string{"x", "y", "z", "rx", "ry", "rz"} {
				if r.FormValue(n) == "" {
					continue
				}

				val, err := strconv.ParseFloat(r.FormValue(n), 64)
				if err != nil {
					return nil, fmt.Errorf("bad value for:%s [%s]", n, r.FormValue(n))
				}

				if action == "abs" {
					switch n {
					case "x":
						where.X = val / 1000
					case "y":
						where.Y = val / 1000
					case "z":
						where.Z = val / 1000
					case "rx":
						where.Rx = val
					case "ry":
						where.Ry = val
					case "rz":
						where.Rz = val
					}
				} else if action == "inc" {
					switch n {
					case "x":
						where.X += val / 1000
					case "y":
						where.Y += val / 1000
					case "z":
						where.Z += val / 1000
					case "rx":
						where.Rx += val
					case "ry":
						where.Ry += val
					case "rz":
						where.Rz += val
					}
				}

				changed = true
			}

			if changed {
				err = arm.MoveToPosition(where)
				if err != nil {
					return nil, err
				}
			}

			return map[string]interface{}{
				"x":  int64(where.X * 1000),
				"y":  int64(where.Y * 1000),
				"z":  int64(where.Z * 1000),
				"rx": where.Rx,
				"ry": where.Ry,
				"rz": where.Rz,
			}, nil
		} else if mode == "joint" {
			current, err := arm.CurrentJointPositions()
			if err != nil {
				return nil, err
			}

			changes := false
			if action == "inc" {
				for i := 0; i < len(current.Degrees); i++ {
					temp := r.FormValue(fmt.Sprintf("j%d", i))
					if temp == "" {
						continue
					}
					val, err := strconv.ParseFloat(temp, 64)
					if err != nil {
						return nil, err
					}
					current.Degrees[i] += val
					changes = true
				}
			} else if action == "abs" {
				for i := 0; i < len(current.Degrees); i++ {
					temp := r.FormValue(fmt.Sprintf("j%d", i))
					if temp == "" {
						continue
					}
					val, err := strconv.ParseFloat(temp, 64)
					if err != nil {
						return nil, err
					}
					current.Degrees[i] = val
					changes = true
				}
			}

			if changes {
				err = arm.MoveToJointPositions(current)
				if err != nil {
					return nil, err
				}
			}

			return map[string]interface{}{"joints": current.Degrees}, nil

		}

		return nil, fmt.Errorf("invalid mode [%s]", mode)

	}})
}

func InstallWebGrippers(mux *http.ServeMux, theRobot *robot.Robot) {
	mux.Handle("/api/gripper", &apiCall{func(r *http.Request) (map[string]interface{}, error) {
		name := r.FormValue("name")
		gripper := theRobot.GripperByName(name)
		if gripper == nil {
			return nil, fmt.Errorf("no gripper with that name %s", name)
		}

		var err error
		res := map[string]interface{}{}

		action := r.FormValue("action")
		switch action {
		case "open":
			err = gripper.Open()
		case "grab":
			var grabbed bool
			grabbed, err = gripper.Grab()
			res["grabbed"] = grabbed
		default:
			err = fmt.Errorf("bad action: (%s)", action)
		}

		if err != nil {
			return nil, fmt.Errorf("gripper error: %s", err)
		}
		return res, nil
	}})
}

func InstallSimpleCamera(mux *http.ServeMux, theRobot *robot.Robot) {
	theFunc := func(w http.ResponseWriter, r *http.Request) {
		name := r.FormValue("name")
		camera := theRobot.CameraByName(name)
		if camera == nil {
			http.Error(w, "bad camera name", http.StatusBadRequest)
			return
		}

		img, release, err := camera.Next(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer release()

		//TODO(erh): choice of encoding

		w.Header().Set("Content-Type", "image/jpeg")
		err = jpeg.Encode(w, img, nil)
		if err != nil {
			golog.Global.Debugf("error encoding jpeg: %s", err)
		}
	}
	mux.HandleFunc("/api/camera", theFunc)
	mux.HandleFunc("/api/camera/", theFunc)

}

func installBoard(mux *http.ServeMux, b board.Board) {
	cfg := b.GetConfig()

	mux.Handle("/api/board/"+cfg.Name+"/motor", &apiCall{func(r *http.Request) (map[string]interface{}, error) {
		name := r.FormValue("name")
		theMotor := b.Motor(name)
		if theMotor == nil {
			return nil, fmt.Errorf("unknown motor: %s", r.FormValue("name"))
		}

		speed, err := strconv.ParseFloat(r.FormValue("s"), 64)
		if err != nil {
			return nil, err
		}

		rVal := 0.0
		if r.FormValue("r") != "" {
			rVal, err = strconv.ParseFloat(r.FormValue("r"), 64)
			if err != nil {
				return nil, err
			}
		}

		if rVal == 0 {
			return map[string]interface{}{}, theMotor.Go(board.DirectionFromString(r.FormValue("d")), byte(speed))
		}

		return map[string]interface{}{}, theMotor.GoFor(board.DirectionFromString(r.FormValue("d")), speed, rVal)
	}})

	mux.Handle("/api/board/"+cfg.Name+"/servo", &apiCall{func(r *http.Request) (map[string]interface{}, error) {
		name := r.FormValue("name")
		theServo := b.Servo(name)
		if theServo == nil {
			return nil, fmt.Errorf("unknown servo: %s", r.FormValue("name"))
		}

		var angle int64
		var err error

		if r.FormValue("angle") != "" {
			angle, err = strconv.ParseInt(r.FormValue("angle"), 10, 64)
		} else if r.FormValue("delta") != "" {
			var d int64
			d, err = strconv.ParseInt(r.FormValue("delta"), 10, 64)
			angle = int64(theServo.Current()) + d
		} else {
			err = fmt.Errorf("need to specify angle or delta")
		}

		if err != nil {
			return nil, err
		}

		return nil, theServo.Move(uint8(angle))

	}})

	mux.Handle("/api/board/"+cfg.Name, &apiCall{func(r *http.Request) (map[string]interface{}, error) {
		analogs := map[string]int{}
		for _, a := range cfg.Analogs {
			res, err := b.AnalogReader(a.Name).Read()
			if err != nil {
				return nil, fmt.Errorf("couldn't read %s: %s", a.Name, err)
			}
			analogs[a.Name] = res
		}

		digitalInterrupts := map[string]int{}
		for _, di := range cfg.DigitalInterrupts {
			res := b.DigitalInterrupt(di.Name).Value()
			digitalInterrupts[di.Name] = int(res)
		}

		servos := map[string]int{}
		for _, di := range cfg.Servos {
			res := b.Servo(di.Name).Current()
			servos[di.Name] = int(res)
		}

		return map[string]interface{}{
			"analogs":           analogs,
			"digitalInterrupts": digitalInterrupts,
			"servos":            servos,
		}, nil
	}})

	mux.Handle("/api/board", &apiCall{func(r *http.Request) (map[string]interface{}, error) {
		return nil, fmt.Errorf("unknown")
	}})
}

func InstallBoards(mux *http.ServeMux, theRobot *robot.Robot) {
	for _, name := range theRobot.BoardNames() {
		installBoard(mux, theRobot.BoardByName(name))
	}
}

// ---------------

func allSourcesToDisplay(theRobot *robot.Robot) ([]gostream.ImageSource, []string) {
	sources := []gostream.ImageSource{}
	names := []string{}

	for _, name := range theRobot.CameraNames() {
		cam := theRobot.CameraByName(name)
		cmp := theRobot.GetConfig().FindComponent(name)
		if cmp.Attributes.GetBool("hide", false) {
			continue
		}

		sources = append(sources, cam)
		names = append(names, name)
	}

	for _, name := range theRobot.LidarDeviceNames() {
		device := theRobot.LidarDeviceByName(name)
		cmp := theRobot.GetConfig().FindComponent(name)
		if cmp.Attributes.GetBool("hide", false) {
			continue
		}

		source := gostream.ResizeImageSource{lidar.NewImageSource(device), 800, 600}

		sources = append(sources, source)
		names = append(names, name)
	}

	return sources, names
}

// returns a closer func to be called when done
func InstallWeb(ctx context.Context, mux *http.ServeMux, theRobot *robot.Robot, options Options) (func(), error) {
	displaySources, displayNames := allSourcesToDisplay(theRobot)
	views := []gostream.View{}
	var autoCameraTiler *gostream.AutoTiler

	if len(displaySources) != 0 {
		if options.AutoTile {
			config := x264.DefaultViewConfig
			view, err := gostream.NewView(config)
			if err != nil {
				return nil, err
			}
			view.SetOnClickHandler(func(x, y int) {
				golog.Global.Debugw("click", "x", x, "y", y)
			})
			views = append(views, view)

			tilerHeight := 480 * len(displaySources)
			autoCameraTiler = gostream.NewAutoTiler(640, tilerHeight)
			autoCameraTiler.SetLogger(golog.Global)

		} else {
			for idx := range displaySources {
				config := x264.DefaultViewConfig
				config.StreamNumber = idx
				config.StreamName = displayNames[idx]
				view, err := gostream.NewView(config)
				if err != nil {
					return nil, err
				}
				view.SetOnClickHandler(func(x, y int) {
					golog.Global.Debugw("click", "x", x, "y", y)
				})
				views = append(views, view)
			}
		}
	}

	app := &robotWebApp{views: views, theRobot: theRobot}
	err := app.Init()
	if err != nil {
		return nil, err
	}

	// install routes
	for _, name := range theRobot.BaseNames() {
		InstallWebBase(mux, theRobot.BaseByName(name))
	}

	InstallWebArms(mux, theRobot)

	InstallWebGrippers(mux, theRobot)

	InstallActions(mux, theRobot)

	InstallSimpleCamera(mux, theRobot)

	InstallBoards(mux, theRobot)

	mux.Handle("/", app)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))

	for _, view := range views {
		handler := view.Handler()
		mux.Handle("/"+handler.Name, handler.Func)
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
func RunWeb(theRobot *robot.Robot, options Options) error {
	defer theRobot.Close(context.Background())
	mux := http.NewServeMux()

	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	webCloser, err := InstallWeb(cancelCtx, mux, theRobot, options)
	if err != nil {
		return err
	}

	const port = 8080
	httpServer := &http.Server{
		Addr:           fmt.Sprintf(":%d", port),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
		Handler:        mux,
	}

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cancelFunc()
		webCloser()
		if err := httpServer.Shutdown(context.Background()); err != nil {
			golog.Global.Errorw("error shutting down", "error", err)
		}
	}()

	golog.Global.Debugw("going to listen", "addr", fmt.Sprintf("http://localhost:%d", port), "port", port)
	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		golog.Global.Fatal(err)
	}

	return nil
}
