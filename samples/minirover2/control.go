package main

import (
	"context"
	"flag"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/echolabsinc/robotcore/robot"
	"github.com/echolabsinc/robotcore/utils/stream"
	"github.com/echolabsinc/robotcore/vision"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/edaniels/gostream/codec/vpx"
)

const (
	PanCenter  = 83
	TiltCenter = 65
)

type Action struct {
	Name           string
	m0, m1, m2, m3 string // the directions for each motor
}

func MakeAction(name string, motorDirections string) Action {
	return Action{
		name,
		string(motorDirections[0]),
		string(motorDirections[1]),
		string(motorDirections[2]),
		string(motorDirections[3]),
	}
}

var (
	Actions = []Action{
		MakeAction("forward", "ffff"),
		MakeAction("backward", "bbbb"),
		MakeAction("stop", "ssss"),
		MakeAction("shift left", "bffb"),
		MakeAction("shift right", "fbbf"),
		MakeAction("spin right", "bfbf"),
		MakeAction("spin left", "fbfb"),
	}
)

func FindAction(name string) *Action {
	for _, a := range Actions {
		if a.Name == name {
			return &a
		}
	}
	return nil
}

func MustFindAction(name string) Action {
	a := FindAction(name)
	if a == nil {
		panic(fmt.Errorf("couldn't find action: %s", name))
	}
	return *a
}

// ------

type Rover struct {
	out      io.Writer
	sendLock sync.Mutex
}

func (r *Rover) Forward(power int) error {
	return r.move(
		"f", "f", "f", "f",
		power, power, power, power)
}

func (r *Rover) Backward(power int) error {
	return r.move(
		"b", "b", "b", "b",
		power, power, power, power)
}

func (r *Rover) Stop() error {
	s := fmt.Sprintf("0s\r" +
		"1s\r" +
		"2s\r" +
		"3s\r")
	return r.sendCommand(s)
}

func (r *Rover) MoveFor(a Action, power int, d time.Duration) error {
	err := r.move(a.m0, a.m1, a.m2, a.m3, power, power, power, power)
	if err != nil {
		return err
	}
	time.Sleep(d)
	return r.Stop()
}

func (r *Rover) Move(a Action, power int) error {
	return r.move(a.m0, a.m1, a.m2, a.m3, power, power, power, power)
}

func (r *Rover) move(a1, a2, a3, a4 string, p1, p2, p3, p4 int) error {
	s := fmt.Sprintf("0%s%d\r"+
		"1%s%d\r"+
		"2%s%d\r"+
		"3%s%d\r", a1, p1, a2, p2, a3, p3, a4, p4)
	return r.sendCommand(s)
}

func (r *Rover) sendCommand(cmd string) error {
	r.sendLock.Lock()
	defer r.sendLock.Unlock()
	_, err := r.out.Write([]byte(cmd))
	return err
}

func (r *Rover) neckCenter() error {
	return r.neckPosition(PanCenter, TiltCenter)
}

func (r *Rover) neckOffset(left int) error {
	return r.neckPosition(PanCenter+(left*-30), TiltCenter-20)
}

func (r *Rover) neckPosition(pan, tilt int) error {
	return r.sendCommand(fmt.Sprintf("n%d\rm%d\r", pan, tilt))
}

func (r *Rover) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var err error

	speed := 64
	if req.FormValue("speed") != "" {
		speed2, err := strconv.ParseInt(req.FormValue("speed"), 10, 64)
		if err != nil {
			golog.Global.Debugf("bad speed [%s] %s", req.FormValue("speed"), err)
		}
		speed = int(speed2)
	}

	a := FindAction(req.FormValue("a"))
	if a == nil {
		io.WriteString(w, "unknown action: "+req.FormValue("a"))
		golog.Global.Debugf("unknown action: %s", req.FormValue("a"))
		return
	}

	err = r.Move(*a, speed)

	if err != nil {
		io.WriteString(w, "err: "+err.Error())
		return
	}

	io.WriteString(w, "done")
}

// ------

type WebApp struct {
	template    *template.Template
	remoteViews []gostream.RemoteView
}

func (app *WebApp) Init() error {
	t, err := template.New("index.html").Funcs(template.FuncMap{
		"jsSafe": func(js string) template.JS {
			return template.JS(js)
		},
		"htmlSafe": func(html string) template.HTML {
			return template.HTML(html)
		},
	}).ParseFiles("index.html")
	if err != nil {
		return err
	}
	app.template = t
	return nil
}

func (app *WebApp) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if true {
		err := app.Init()
		if err != nil {
			golog.Global.Debugf("couldn't reload template: %s", err)
			return
		}
	}

	type RemoteView struct {
		JavaScript string
		Body       string
	}

	type Temp struct {
		Actions     []Action
		RemoteViews []RemoteView
	}

	temp := Temp{Actions: Actions}
	for _, remoteView := range app.remoteViews {
		htmlData := remoteView.HTML()
		temp.RemoteViews = append(temp.RemoteViews, RemoteView{
			htmlData.JavaScript,
			htmlData.Body,
		})
	}

	err := app.template.Execute(w, temp)
	if err != nil {
		golog.Global.Debugf("couldn't execute web page: %s", err)
	}
}

// ---

func driveMyself(rover *Rover, camera vision.MatSource) {
	for {
		mat, dm, err := camera.NextColorDepthPair()
		if err != nil {
			golog.Global.Debugf("error reading camera: %s", err)
			time.Sleep(2000 * time.Millisecond)
			continue
		}
		func() {
			defer mat.Close()
			img, err := vision.NewImage(mat)
			if err != nil {
				golog.Global.Debugf("error parsing image: %s", err)
				return
			}

			pc := vision.PointCloud{dm, img}
			pc, err = pc.CropToDepthData()

			if err != nil || pc.Depth.Width() < 10 || pc.Depth.Height() < 10 {
				golog.Global.Debugf("error getting deth info: %s, backing up", err)
				rover.MoveFor(MustFindAction("backward"), 60, 1500*time.Millisecond)
				return
			}

			points := roverWalk(&pc, nil)
			if points < 100 {
				golog.Global.Debugf("safe to move forward")
				rover.MoveFor(MustFindAction("forward"), 35, 1500*time.Millisecond)
			} else {
				golog.Global.Debugf("not safe, let's spin")
				rover.MoveFor(MustFindAction("spin left"), 60, 600*time.Millisecond)
			}
		}()

	}
}

// ---

func main() {
	flag.Parse()

	srcURL := "127.0.0.1"
	if flag.NArg() >= 1 {
		srcURL = flag.Arg(0)
	}

	port, err := robot.ConnectArduinoSerial("Mega")
	if err != nil {
		golog.Global.Fatalf("can't connecto to arduino: %v", err)
	}
	defer port.Close()

	time.Sleep(1000 * time.Millisecond) // wait for startup?

	golog.Global.Debug("ready")

	rover := Rover{}
	rover.out = port
	defer rover.Stop()

	go func() {
		for {
			time.Sleep(1500 * time.Millisecond)
			rover.neckCenter()
			time.Sleep(1500 * time.Millisecond)
			rover.neckOffset(-1)
			time.Sleep(1500 * time.Millisecond)
			rover.neckOffset(1)
		}
	}()

	realCameraNotFlippedSrc := vision.NewIntelServerSource(srcURL, 8181, nil)
	realCameraSrc := &vision.RotateSource{realCameraNotFlippedSrc}

	cameraSrc := &vision.RotateSource{&vision.HTTPSource{realCameraNotFlippedSrc.ColorURL(), ""}}
	config := vpx.DefaultRemoteViewConfig
	config.Debug = false
	remoteView, err := gostream.NewRemoteView(config)
	if err != nil {
		panic(err)
	}

	views := []gostream.RemoteView{remoteView}
	app := &WebApp{remoteViews: views}
	err = app.Init()
	if err != nil {
		golog.Global.Fatalf("couldn't create web app: %s", err)
	}

	httpServer := &http.Server{
		Addr:           ":8080",
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cancelFunc()
		remoteView.Stop()
		httpServer.Shutdown(context.Background())
	}()

	go stream.MatSource(cancelCtx, cameraSrc, remoteView, 33*time.Millisecond, golog.Global)
	go driveMyself(&rover, realCameraSrc)

	mux := http.NewServeMux()
	mux.Handle("/api/rover", &rover)

	mux.Handle("/", app)
	for _, view := range views {
		handler := view.Handler()
		mux.Handle("/"+handler.Name, handler.Func)
	}

	golog.Global.Debug("going to listen")
	httpServer.Handler = mux
	golog.Global.Fatal(httpServer.ListenAndServe())
}
