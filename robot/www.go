package robot

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/edaniels/gostream/codec/vpx"

	"github.com/echolabsinc/robotcore/base"
	"github.com/echolabsinc/robotcore/utils/stream"
)

type robotWebApp struct {
	template    *template.Template
	remoteViews []gostream.RemoteView
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
	}).ParseGlob(fmt.Sprintf("%s/*.html", thisDirPath))
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

	type RemoteView struct {
		JavaScript string
		Body       string
	}

	type Temp struct {
		RemoteViews []RemoteView
	}

	temp := Temp{}
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

// ---------------

func InstallWebBase(mux *http.ServeMux, theBase base.Base) {

	mux.HandleFunc("/api/base", func(w http.ResponseWriter, req *http.Request) {
		speed := 64 // TODO(erh): this is proably the wrong default
		if req.FormValue("speed") != "" {
			speed2, err := strconv.ParseInt(req.FormValue("speed"), 10, 64)
			if err != nil {
				http.Error(w, fmt.Sprintf("bad speed [%s] %s", req.FormValue("speed"), err), http.StatusBadRequest)
				return
			}
			speed = int(speed2)
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
				http.Error(w, fmt.Sprintf("bad distance [%s] %s", d, err2), http.StatusBadRequest)
				return
			}

			err = theBase.MoveStraight(int(d2), speed, false)
		} else if a != "" {
			a2, err2 := strconv.ParseInt(a, 10, 64)
			if err2 != nil {
				http.Error(w, fmt.Sprintf("bad angle [%s] %s", d, err2), http.StatusBadRequest)
				return
			}

			err = theBase.Spin(int(a2), speed, false)
		} else {
			http.Error(w, "no stop, distanceMM, angle given", http.StatusBadRequest)
			return
		}

		if err != nil {
			http.Error(w, fmt.Sprintf("erorr moving %s", err), http.StatusInternalServerError)
		} else {
			_, err = io.WriteString(w, "ok")
			if err != nil {
				panic("impossible")
			}
		}

	})
}

// ---------------

func InstallWeb(mux *http.ServeMux, theRobot *Robot) (func(), error) {
	if len(theRobot.Bases) != 1 {
		return nil, fmt.Errorf("robot.InstallWeb robot needs exactly one base right now")
	}

	views := []gostream.RemoteView{}

	// set up camera streams
	for idx := range theRobot.Cameras {
		config := vpx.DefaultRemoteViewConfig
		config.Debug = false
		config.StreamNumber = idx
		remoteView, err := gostream.NewRemoteView(config)
		if err != nil {
			return nil, err
		}
		remoteView.SetOnClickHandler(func(x, y int) {
			golog.Global.Debugw("click", "x", x, "y", y)
		})

		views = append(views, remoteView)
	}

	app := &robotWebApp{remoteViews: views}
	err := app.Init()
	if err != nil {
		return nil, err
	}

	// install routes

	InstallWebBase(mux, theRobot.Bases[0])
	mux.Handle("/", app)

	for _, view := range views {
		handler := view.Handler()
		mux.Handle("/"+handler.Name, handler.Func)
	}

	// start background threads

	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	for idx, remoteView := range views {
		go stream.MatSource(cancelCtx, theRobot.Cameras[idx], remoteView, 33*time.Millisecond, golog.Global)
	}

	return func() {
		cancelFunc()
		for _, v := range views {
			v.Stop()
		}
	}, nil

}
