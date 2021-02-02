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

	"github.com/viamrobotics/robotcore/utils"
	"github.com/viamrobotics/robotcore/vision"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/edaniels/gostream/codec/vpx"
	"github.com/kbinani/screenshot"
)

func main() {

	go func() {
		http.ListenAndServe(":6060", nil)
	}()

	dupe := flag.Int("dupe", 0, "number of times to duplicate image")
	flag.Parse()

	var imgSrc gostream.ImageSource
	var err error

	if flag.NArg() == 0 || flag.Arg(0) == "webcam" {
		imgSrc, err = utils.NewWebcamSource(0)
		if err != nil {
			panic(err)
		}
	} else if flag.Arg(0) != "screen" {
		imgSrc = &vision.HTTPSource{"http://" + flag.Arg(0) + "/pic.ppm", ""}
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

	captureRate := 33 * time.Millisecond
	if flag.Arg(0) == "screen" {
		bounds := screenshot.GetDisplayBounds(0)
		imgSrc = gostream.ImageSourceFunc(func(ctx context.Context) (image.Image, error) {
			return screenshot.CaptureRect(bounds)
		})
	}
	if *dupe != 0 {
		autoTiler := gostream.NewAutoTiler(800, 600, imgSrc)
		for i := 0; i < *dupe; i++ {
			autoTiler.AddSource(imgSrc)
		}
		gostream.StreamSource(cancelCtx, autoTiler, remoteView, captureRate)
	} else {
		gostream.StreamSource(cancelCtx, imgSrc, remoteView, captureRate)
	}
	server.Stop(context.Background())
}
