package main

import (
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jacobsa/go-serial/serial"

	"github.com/echolabsinc/robotcore/rcutil"
	"github.com/echolabsinc/robotcore/utils/stream"
	"github.com/echolabsinc/robotcore/vision"
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

// ------

func findPort() (string, error) {
	for _, possibleFile := range []string{"/dev/ttyTHS0"} {
		_, err := os.Stat(possibleFile)
		if err == nil {
			return possibleFile, nil
		}
	}

	lines, err := rcutil.ExecuteShellCommand("arduino-cli", "board", "list")
	if err != nil {
		return "", err
	}

	for _, l := range lines {
		if strings.Index(l, "Mega") < 0 {
			continue
		}
		return strings.Split(l, " ")[0], nil
	}

	return "", fmt.Errorf("couldn't find an arduino")
}

func getSerialConfig() (serial.OpenOptions, error) {

	options := serial.OpenOptions{
		PortName:        "",
		BaudRate:        9600,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 4,
	}

	portName, err := findPort()
	if err != nil {
		return options, err
	}

	options.PortName = portName

	return options, nil
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

func (r *Rover) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var err error

	speed := 64
	if req.FormValue("speed") != "" {
		speed2, err := strconv.ParseInt(req.FormValue("speed"), 10, 64)
		if err != nil {
			log.Printf("bad speed [%s] %s", req.FormValue("speed"), err)
		}
		speed = int(speed2)
	}

	a := FindAction(req.FormValue("a"))
	if a == nil {
		io.WriteString(w, "unknown action: "+req.FormValue("a"))
		log.Printf("unknown action: %s", req.FormValue("a"))
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
	remoteViews []stream.RemoteView
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
			log.Printf("couldn't reload tempalte: %s", err)
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
	for i, remoteView := range app.remoteViews {
		htmlData := remoteView.HTML(i)
		temp.RemoteViews = append(temp.RemoteViews, RemoteView{
			htmlData.JavaScript,
			htmlData.Body,
		})
	}

	err := app.template.Execute(w, temp)
	if err != nil {
		log.Printf("couldn't execute web page: %s", err)
	}
}

// ---

func main() {
	flag.Parse()
	options, err := getSerialConfig()
	if err != nil {
		log.Fatalf("can't get serial config: %v", err)
	}

	port, err := serial.Open(options)
	if err != nil {
		log.Fatalf("can't option serial port %v", err)
	}
	defer port.Close()

	time.Sleep(1000 * time.Millisecond) // wait for startup?

	fmt.Println("ready")

	rover := Rover{}
	rover.out = port
	defer rover.Stop()

	cameraSrc := vision.NewHttpSourceIntelEliot(flag.Arg(0))
	config := stream.DefaultRemoteViewConfig
	config.Debug = true
	remoteView, err := stream.NewRemoteView(config)
	if err != nil {
		panic(err)
	}

	views := []stream.RemoteView{remoteView}
	app := &WebApp{remoteViews: views}
	err = app.Init()
	if err != nil {
		log.Fatalf("couldn't create web app: %s", err)
	}

	go stream.StreamMatSource(cameraSrc, remoteView, 33*time.Millisecond)

	mux := http.NewServeMux()
	mux.Handle("/api/rover", &rover)

	mux.Handle("/", app)
	for i, view := range views {
		handler := view.Handler(i)
		mux.Handle("/"+handler.Name, handler.Func)
	}

	log.Println("going to listen")
	log.Fatal(http.ListenAndServe(":8080", mux))

}
