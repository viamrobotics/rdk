package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/echolabsinc/robotcore/lidar/rplidar"
	"github.com/echolabsinc/robotcore/robot"
	"github.com/echolabsinc/robotcore/robots/hellorobot"
	"github.com/echolabsinc/robotcore/vision"

	"github.com/edaniels/golog"
)

func main() {
	flag.Parse()

	srcURL := "127.0.0.1"
	if flag.NArg() >= 1 {
		srcURL = flag.Arg(0)
	}

	lidarDevPath := "/dev/hello-lrf"
	if flag.NArg() >= 2 {
		lidarDevPath = flag.Arg(1)
	}

	helloRobot := hellorobot.New()
	helloRobot.Startup()
	defer helloRobot.Stop()

	lidarDev, err := rplidar.NewRPLidar(lidarDevPath)
	if err != nil {
		panic(err)
	}
	lidarDev.Start()

	theRobot := robot.NewBlankRobot()
	theRobot.AddBase(helloRobot.Base(), robot.Component{})
	theRobot.AddCamera(vision.NewIntelServerSource(srcURL, 8181, nil), robot.Component{})
	theRobot.AddLidar(lidarDev, robot.Component{})

	defer theRobot.Close()

	mux := http.NewServeMux()

	webCloser, err := robot.InstallWeb(mux, theRobot)
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
