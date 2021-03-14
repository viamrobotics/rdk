package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/robot"
	"go.viam.com/robotcore/robot/web"
	"go.viam.com/robotcore/robots/hellorobot"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
)

func main() {
	flag.Parse()

	srcURL := "127.0.0.1"
	if flag.NArg() >= 1 {
		srcURL = flag.Arg(0)
	}

	lidarDevAddr := "ws://127.0.0.1:4444"
	if flag.NArg() >= 2 {
		lidarDevAddr = flag.Arg(1)
	}

	if err := runRobot(srcURL, lidarDevAddr); err != nil {
		golog.Global.Fatal(err)
	}
}

func runRobot(srcURL, lidarDevAddr string) (err error) {
	helloRobot, err := hellorobot.New()
	if err != nil {
		return err
	}
	if err := helloRobot.Startup(); err != nil {
		return err
	}
	defer func() {
		err = multierr.Combine(err, helloRobot.Stop())
	}()

	lidarDev, err := lidar.NewWSDevice(context.Background(), lidarDevAddr)
	if err != nil {
		return err
	}
	if err := lidarDev.Start(context.Background()); err != nil {
		return err
	}

	theRobot := robot.NewBlankRobot()
	robotBase, err := helloRobot.Base()
	if err != nil {
		return err
	}
	theRobot.AddBase(robotBase, api.Component{})
	theRobot.AddCamera(rimage.NewIntelServerSource(srcURL, 8181, nil), api.Component{})
	theRobot.AddLidar(lidarDev, api.Component{})

	defer func() {
		err = multierr.Combine(err, theRobot.Close(context.Background()))
	}()

	mux := http.NewServeMux()

	httpServer := &http.Server{
		Addr:           ":8080",
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
		Handler:        mux,
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	webCloser, err := web.InstallWeb(cancelCtx, mux, theRobot, web.NewOptions())
	if err != nil {
		return err
	}

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		// TODO(erd): for some reason, the signal is never received and
		// seems related to python. come back to this later, maybe.
		<-c
		cancelFunc()
		webCloser()
		if err := httpServer.Shutdown(context.Background()); err != nil {
			golog.Global.Errorw("error shutting down", "error", err)
		}
	}()

	golog.Global.Debug("going to listen")
	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}
