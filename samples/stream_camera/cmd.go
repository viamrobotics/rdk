package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	_ "net/http/pprof"
	"time"

	"github.com/echolabsinc/robotcore/utils/stream"
	"github.com/echolabsinc/robotcore/vision"
)

func main() {

	go func() {
		log.Println(http.ListenAndServe(":6060", nil))
	}()

	flag.Parse()

	var src vision.MatSource
	var err error

	if flag.NArg() == 0 {
		src, err = vision.NewWebcamSource(0)
		if err != nil {
			panic(err)
		}
	} else {
		src = &vision.HttpSource{"http://" + flag.Arg(0) + "/pic.ppm", ""}
	}

	config := stream.DefaultRemoteViewConfig
	config.Debug = false
	remoteView, err := stream.NewRemoteView(config)
	if err != nil {
		panic(err)
	}

	remoteView.SetOnClickHandler(func(x, y int) {
		println(x, y)
	})

	server := stream.NewRemoteViewServer(5555, remoteView)
	if err := server.Run(context.Background()); err != nil {
		panic(err)
	}

	stream.StreamMatSource(src, remoteView, 33*time.Millisecond)
}
