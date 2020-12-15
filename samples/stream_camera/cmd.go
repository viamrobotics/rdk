package main

import (
	"context"
	"time"

	"github.com/echolabsinc/robotcore/utils/stream"
	"github.com/echolabsinc/robotcore/vision"
)

// TODO(erd):
// - auto negotiate
// - auto open browser :)?
func main() {
	src, err := vision.NewWebcamSource(0)
	if err != nil {
		panic(err)
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
