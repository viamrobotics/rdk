package stream

import (
	"context"
	"image"
	"time"

	"github.com/echolabsinc/robotcore/vision"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
)

func MatSource(ctx context.Context, src vision.MatSource, remoteView gostream.RemoteView, captureInternal time.Duration, logger golog.Logger) {
	gostream.StreamFunc(
		ctx,
		func() image.Image {
			now := time.Now()
			mat, _, err := src.NextColorDepthPair()
			if err != nil {
				panic(err) // TODO(erd): don't panic... bones, sinking like stones
			}
			defer mat.Close()
			if remoteView.Debug() {
				logger.Debugw("NextColorDepthPair", "elapsed", time.Since(now))
			}
			img, err := mat.ToImage()
			if err != nil {
				panic(err) // TODO(erd): don't panic
			}
			return img
		},
		remoteView,
		captureInternal,
	)
}
