package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.viam.com/robotcore/robot"
	"go.viam.com/robotcore/robot/web"
	"go.viam.com/robotcore/robots/hellorobot"
	"go.viam.com/robotcore/vision"

	"github.com/edaniels/golog"
	rplidarws "github.com/viamrobotics/rplidar/ws"
)

func main() {
	flag.Parse()

	srcURL := "127.0.0.1"
	if flag.NArg() >= 1 {
		srcURL = flag.Arg(0)
	}

	lidarDevAddr := "127.0.0.1:4444"
	if flag.NArg() >= 2 {
		lidarDevAddr = flag.Arg(1)
	}

	helloRobot := hellorobot.New()
	helloRobot.Startup()
	defer helloRobot.Stop()

	lidarDev, err := rplidarws.NewDevice(context.Background(), lidarDevAddr)
	if err != nil {
		panic(err)
	}
	if err := lidarDev.Start(context.Background()); err != nil {
		panic(err)
	}

	theRobot := robot.NewBlankRobot()
	theRobot.AddBase(helloRobot.Base(), robot.Component{})
	theRobot.AddCamera(vision.NewIntelServerSource(srcURL, 8181, nil), robot.Component{})
	theRobot.AddLidar(lidarDev, robot.Component{})

	defer func() {
		if err := theRobot.Close(context.Background()); err != nil {
			panic(err)
		}
	}()

	mux := http.NewServeMux()

	webCloser, err := web.InstallWeb(mux, theRobot)
	if err != nil {
		panic(err)
	}

	httpServer := &http.Server{
		Addr:           ":8080",
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
		httpServer.Shutdown(context.Background())
	}()

	golog.Global.Debug("going to listen")
	golog.Global.Fatal(httpServer.ListenAndServe())
}
