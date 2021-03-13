package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/rimage"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/edaniels/gostream/codec/x264"
	"github.com/edaniels/gostream/media"
)

func main() {

	debug := flag.Bool("debug", false, "")
	dump := flag.Bool("dump", false, "dump all camera info")
	format := flag.String("format", "", "")
	path := flag.String("path", "", "")
	pathPattern := flag.String("pathPattern", "", "")

	flag.Parse()

	if *dump {
		all := media.QueryVideoDevices()
		for _, info := range all {
			golog.Global.Debugf("%s", info.ID)
			golog.Global.Debugf("\t labels: %v", info.Labels)
			for _, p := range info.Properties {
				golog.Global.Debugf("\t %v %d x %d", p.FrameFormat, p.Width, p.Height)
			}

		}
		return
	}

	port := 5555
	if flag.NArg() >= 1 {
		portParsed, err := strconv.ParseInt(flag.Arg(1), 10, 32)
		if err != nil {
			golog.Global.Fatal(err)
		}
		port = int(portParsed)
	}

	attrs := api.AttributeMap{}

	if *format != "" {
		attrs["format"] = *format
	}

	if *path != "" {
		attrs["path"] = *path
	}

	if *pathPattern != "" {
		attrs["path_pattern"] = *pathPattern
	}

	if *debug {
		attrs["debug"] = true
	}

	golog.Global.Debugf("attrs: %v", attrs)

	webcam, err := rimage.NewWebcamSource(attrs)
	if err != nil {
		golog.Global.Fatal(err)
	}

	func() {
		img, closer, err := webcam.Next(context.TODO())
		if err != nil {
			golog.Global.Fatal(err)
		}
		defer closer()
		golog.Global.Debugf("image type: %T dimensions: %v", img, img.Bounds())
	}()

	remoteView, err := gostream.NewView(x264.DefaultViewConfig)
	if err != nil {
		golog.Global.Fatal(err)
	}

	server := gostream.NewViewServer(port, remoteView, golog.Global)
	if err := server.Start(); err != nil {
		golog.Global.Fatal(err)
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cancelFunc()
	}()

	gostream.StreamSource(cancelCtx, webcam, remoteView)

	if err := server.Stop(context.Background()); err != nil {
		golog.Global.Error(err)
	}
}
