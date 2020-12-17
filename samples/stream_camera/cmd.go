package main

import (
	"context"
	"flag"
	"time"

	"github.com/echolabsinc/robotcore/utils/stream"
	"github.com/echolabsinc/robotcore/vision"
)

// TODO(erd):
// - auto negotiate
// - auto open browser :)?
func main() {

	flag.Parse()

	var src vision.MatSource
	var err error

	if flag.NArg() == 0 {
		src, err = vision.NewWebcamSource(0)
		if err != nil {
			panic(err)
		}
	} else {
		src = vision.NewHttpSourceIntelEliot(flag.Arg(0))
	}

	remoteView, err := stream.NewRemoteView(stream.DefaultRemoteViewConfig)
	if err != nil {
		panic(err)
	}

	remoteView.SetOnClickHandler(func(x, y int) {
		println(x, y)
	})

	if err := remoteView.Start(context.Background()); err != nil {
		panic(err)
	}

	stream.StreamMatSource(src, remoteView, 5*time.Millisecond)
}
