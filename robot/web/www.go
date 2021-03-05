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

	"go.viam.com/robotcore/base"
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

	temp := Temp{}

	for _, b := range app.theRobot.Bases {
		temp.Bases = append(temp.Bases, app.theRobot.ComponentFor(b).Name)
	}

	for _, a := range app.theRobot.Arms {
		temp.Arms = append(temp.Arms, app.theRobot.ComponentFor(a).Name)
	}

	for _, g := range app.theRobot.Grippers {
		temp.Grippers = append(temp.Grippers, app.theRobot.ComponentFor(g).Name)
	}

	for _, b := range app.theRobot.Boards {
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

func InstallWebBase(mux *http.ServeMux, theBase base.Device) {

	mux.Handle("/api/base", &apiCall{func(req *http.Request) (map[string]interface{}, error) {
		mmPerSec := 500.0 // TODO(erh): this is proably the wrong default
		if req.FormValue("speed") != "" {
			speed2, err := strconv.ParseFloat(req.FormValue("speed"), 64)
			if err != nil {
				return nil, err
			}
			mmPerSec = speed2
		}

		s := req.FormValue("stop")
		d := req.FormValue("distanceMM")
		a := req.FormValue("angle")

		var err error

		if s == "t" || s == "true" {
			err = theBase.Stop()
		} else if d != "" {
			d2, err2 := strconv.ParseInt(d, 10, 64)
			if err2 != nil {
				return nil, err2
			}

			err = theBase.MoveStraight(int(d2), mmPerSec, false)
		} else if a != "" {
			a2, err2 := strconv.ParseInt(a, 10, 64)
			if err2 != nil {
				return nil, err2
			}

			// TODO(erh): fix speed
			err = theBase.Spin(float64(a2), 64, false)
		} else {
			return nil, fmt.Errorf("no stop, distanceMM, angle given")
		}

		if err != nil {
			return nil, fmt.Errorf("erorr moving %s", err)
		}

		return nil, nil

	}})
}

type apiMethod func(req *http.Request) (map[string]interface{}, error)

type apiCall struct {
	theMethod apiMethod
}

func (ac *apiCall) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	res, err := ac.theMethod(req)
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
	mux.Handle("/api/action", &apiCall{func(req *http.Request) (map[string]interface{}, error) {
		name := req.FormValue("name")
		switch name {
		case "RandomWalk":
			go actions.RandomWalk(theRobot, 60)
			return map[string]interface{}{"started": true}, nil
		default:
			return nil, fmt.Errorf("unknown action name [%s]", name)
		}
	}})
}

func InstallWebArms(mux *http.ServeMux, theRobot *robot.Robot) {
	mux.Handle("/api/arm", &apiCall{func(req *http.Request) (map[string]interface{}, error) {
		mode := req.FormValue("mode")
		if mode == "" {
			mode = "grid"
		}
		action := req.FormValue("action")
		armNumber := 0

		if req.FormValue("num") != "" {
			arm2, err2 := strconv.ParseInt(req.FormValue("num"), 10, 64)
			if err2 != nil {
				return nil, fmt.Errorf("bad value for arm")
			}
			armNumber = int(arm2)
		}

		if armNumber < 0 || armNumber >= len(theRobot.Arms) {
			return nil, fmt.Errorf("not a valid arm number")
		}

		arm := theRobot.Arms[armNumber]

		if mode == "grid" {

			where, err := arm.CurrentPosition()
			if err != nil {
				return nil, err
			}

			changed := false
			for _, n := range []string{"x", "y", "z", "rx", "ry", "rz"} {
				if req.FormValue(n) == "" {
					continue
				}

				val, err := strconv.ParseFloat(req.FormValue(n), 64)
				if err != nil {
					return nil, fmt.Errorf("bad value for:%s [%s]", n, req.FormValue(n))
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
					temp := req.FormValue(fmt.Sprintf("j%d", i))
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
					temp := req.FormValue(fmt.Sprintf("j%d", i))
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
	mux.HandleFunc("/api/gripper", func(w http.ResponseWriter, req *http.Request) {
		gripper := 0

		if req.FormValue("num") != "" {
			g2, err := strconv.ParseInt(req.FormValue("num"), 10, 64)
			if err != nil {
				http.Error(w, "bad value for num", http.StatusBadRequest)
				return
			}
			gripper = int(g2)
		}

		if gripper < 0 || gripper >= len(theRobot.Grippers) {
			http.Error(w, "not a valid gripper number", http.StatusBadRequest)
			return
		}

		var err error

		action := req.FormValue("action")
		switch action {
		case "open":
			err = theRobot.Grippers[gripper].Open()
		case "grab":
			var res bool
			res, err = theRobot.Grippers[gripper].Grab()
			w.Header().Add("grabbed", fmt.Sprintf("%v", res))
		default:
			err = fmt.Errorf("bad action: (%s)", action)
		}

		if err != nil {
			golog.Global.Debugf("gripper error: %s", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

	})
}

func InstallSimpleCamera(mux *http.ServeMux, theRobot *robot.Robot) {
	theFunc := func(w http.ResponseWriter, req *http.Request) {
		num := 0
		if req.FormValue("num") != "" {
			num2, err := strconv.ParseInt(req.FormValue("num"), 10, 64)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			num = int(num2)
		}

		// TODO(erh): search by name

		if num >= len(theRobot.Cameras) {
			http.Error(w, "invalid camera number", http.StatusBadRequest)
			return
		}

		img, release, err := theRobot.Cameras[num].Next(context.TODO())
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

	mux.Handle("/api/board/"+cfg.Name+"/motor", &apiCall{func(req *http.Request) (map[string]interface{}, error) {
		name := req.FormValue("name")
		theMotor := b.Motor(name)
		if theMotor == nil {
			return nil, fmt.Errorf("unknown motor: %s", req.FormValue("name"))
		}

		speed, err := strconv.ParseFloat(req.FormValue("s"), 64)
		if err != nil {
			return nil, err
		}

		r := 0.0
		if req.FormValue("r") != "" {
			r, err = strconv.ParseFloat(req.FormValue("r"), 64)
			if err != nil {
				return nil, err
			}
		}

		if r == 0 {
			return map[string]interface{}{}, theMotor.Go(board.DirectionFromString(req.FormValue("d")), byte(speed))
		}

		return map[string]interface{}{}, theMotor.GoFor(board.DirectionFromString(req.FormValue("d")), speed, r, false)
	}})

	mux.Handle("/api/board/"+cfg.Name+"/servo", &apiCall{func(req *http.Request) (map[string]interface{}, error) {
		name := req.FormValue("name")
		theServo := b.Servo(name)
		if theServo == nil {
			return nil, fmt.Errorf("unknown servo: %s", req.FormValue("name"))
		}

		var angle int64
		var err error

		if req.FormValue("angle") != "" {
			angle, err = strconv.ParseInt(req.FormValue("angle"), 10, 64)
		} else if req.FormValue("delta") != "" {
			var d int64
			d, err = strconv.ParseInt(req.FormValue("delta"), 10, 64)
			angle = int64(theServo.Current()) + d
		} else {
			err = fmt.Errorf("need to specify angle or delta")
		}

		if err != nil {
			return nil, err
		}

		return nil, theServo.Move(uint8(angle))

	}})

	mux.Handle("/api/board/"+cfg.Name, &apiCall{func(req *http.Request) (map[string]interface{}, error) {
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

	mux.Handle("/api/board", &apiCall{func(req *http.Request) (map[string]interface{}, error) {
		return nil, fmt.Errorf("unknown")
	}})
}

func InstallBoards(mux *http.ServeMux, theRobot *robot.Robot) {
	for _, b := range theRobot.Boards {
		installBoard(mux, b)
	}
}

// ---------------

func InstallWeb(mux *http.ServeMux, theRobot *robot.Robot) (func(), error) {
	if len(theRobot.Bases) > 1 {
		return nil, fmt.Errorf("robot.InstallWeb robot can't have morem than 1 base right now")
	}

	var view gostream.View
	if len(theRobot.Cameras) != 0 || len(theRobot.LidarDevices) != 0 {
		config := x264.DefaultViewConfig
		var err error
		view, err = gostream.NewView(config)
		if err != nil {
			return nil, err
		}
		view.SetOnClickHandler(func(x, y int) {
			golog.Global.Debugw("click", "x", x, "y", y)
		})
	}

	var views []gostream.View
	if view != nil {
		views = append(views, view)
	}
	app := &robotWebApp{views: views, theRobot: theRobot}
	err := app.Init()
	if err != nil {
		return nil, err
	}

	// install routes
	if len(theRobot.Bases) > 0 {
		InstallWebBase(mux, theRobot.Bases[0])
	}

	InstallWebArms(mux, theRobot)

	InstallWebGrippers(mux, theRobot)

	InstallActions(mux, theRobot)

	InstallSimpleCamera(mux, theRobot)

	InstallBoards(mux, theRobot)

	mux.Handle("/", app)

	if view != nil {
		handler := view.Handler()
		mux.Handle("/"+handler.Name, handler.Func)
	}

	// start background threads

	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	autoCameraTiler := gostream.NewAutoTiler(1280, 720)
	autoCameraTiler.SetLogger(golog.Global)
	if len(theRobot.Cameras) > 0 {
		for _, cam := range theRobot.Cameras {
			autoCameraTiler.AddSource(cam)
		}
		waitCh := make(chan struct{})
		go func() {
			close(waitCh)
			gostream.StreamNamedSource(cancelCtx, autoCameraTiler, "Cameras", view)
		}()
		<-waitCh
	}

	for idx := range theRobot.LidarDevices {
		name := fmt.Sprintf("LIDAR %d", idx+1)
		go gostream.StreamNamedSource(cancelCtx, gostream.ResizeImageSource{lidar.NewImageSource(theRobot.LidarDevices[idx]), 800, 600}, name, view)
	}

	return func() {
		cancelFunc()
		if view != nil {
			view.Stop()
		}
	}, nil

}

// ---

/*
helper if you don't need to customize at all
*/
func RunWeb(theRobot *robot.Robot) error {
	defer theRobot.Close(context.TODO())
	mux := http.NewServeMux()

	webCloser, err := InstallWeb(mux, theRobot)
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
		webCloser()
		httpServer.Shutdown(context.Background()) //nolint
	}()

	golog.Global.Debugw("going to listen", "addr", fmt.Sprintf("http://localhost:%d", port), "port", port)
	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		golog.Global.Fatal(err)
	}

	return nil
}
