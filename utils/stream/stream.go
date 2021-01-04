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
				panic(err)
			}
			defer mat.Close()
			if remoteView.Debug() {
				logger.Debugw("NextColorDepthPair", "elapsed", time.Since(now))
			}
			img, err := mat.ToImage()
			if err != nil {
				panic(err)
			}
			return img
		},
		remoteView,
		captureInternal,
	)
}
