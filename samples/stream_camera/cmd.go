package main

import (
	"context"
	"flag"
	"image"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/echolabsinc/robotcore/utils/log"
	"github.com/echolabsinc/robotcore/utils/stream"
	"github.com/echolabsinc/robotcore/vision"

	"github.com/kbinani/screenshot"
)

func main() {

	go func() {
		http.ListenAndServe(":6060", nil)
	}()

	flag.Parse()

	var src vision.MatSource
	var err error

	if flag.NArg() == 0 {
		src, err = vision.NewWebcamSource(0)
		if err != nil {
			panic(err)
		}
	} else if flag.Arg(0) != "screen" {
		src = &vision.HTTPSource{"http://" + flag.Arg(0) + "/pic.ppm", ""}
	}

	config := stream.DefaultRemoteViewConfig
	config.Debug = false
	remoteView, err := stream.NewRemoteView(config)
	if err != nil {
		panic(err)
	}

	remoteView.SetOnClickHandler(func(x, y int) {
		log.Global.Debugw("got click", "x", x, "y", y)
	})

	server := stream.NewRemoteViewServer(5555, remoteView, log.Global)
	server.Run()

	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cancelFunc()
	}()

	if flag.Arg(0) == "screen" {
		bounds := screenshot.GetDisplayBounds(0)
		stream.StreamFunc(cancelCtx, func() image.Image {
			img, err := screenshot.CaptureRect(bounds)
			if err != nil {
				panic(err)
			}
			return img
		}, remoteView, 33*time.Millisecond)
	} else {
		stream.StreamMatSource(cancelCtx, src, remoteView, 33*time.Millisecond, log.Global)
	}
	server.Stop(context.Background())
}
