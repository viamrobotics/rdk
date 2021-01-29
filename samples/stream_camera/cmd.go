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

	"github.com/echolabsinc/robotcore/utils"
	"github.com/echolabsinc/robotcore/utils/stream"
	"github.com/echolabsinc/robotcore/vision"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/edaniels/gostream/codec/vpx"
	"github.com/kbinani/screenshot"
)

func main() {

	go func() {
		http.ListenAndServe(":6060", nil)
	}()

	flag.Parse()

	var src utils.MatSource
	var err error

	if flag.NArg() == 0 {
		src, err = utils.NewWebcamSource(0)
		if err != nil {
			panic(err)
		}
	} else if flag.Arg(0) != "screen" {
		src = &vision.HTTPSource{"http://" + flag.Arg(0) + "/pic.ppm", ""}
	}

	config := vpx.DefaultRemoteViewConfig
	config.Debug = false
	remoteView, err := gostream.NewRemoteView(config)
	if err != nil {
		panic(err)
	}

	remoteView.SetOnClickHandler(func(x, y int) {
		golog.Global.Debugw("got click", "x", x, "y", y)
	})

	server := gostream.NewRemoteViewServer(5555, remoteView, golog.Global)
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
		gostream.StreamFunc(cancelCtx, func() image.Image {
			img, err := screenshot.CaptureRect(bounds)
			if err != nil {
				panic(err)
			}
			return img
		}, remoteView, 33*time.Millisecond)
	} else {
		stream.MatSource(cancelCtx, src, remoteView, 33*time.Millisecond, golog.Global)
	}
	server.Stop(context.Background())
}
