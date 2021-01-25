package stream

import (
	"context"
	"image"
	"time"

	"github.com/echolabsinc/robotcore/vision"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"gocv.io/x/gocv"
)

func MatSource(ctx context.Context, src vision.MatSource, remoteView gostream.RemoteView, captureInternal time.Duration, logger golog.Logger) {
	gostream.StreamFunc(
		ctx,
		func() image.Image {
			var now time.Time
			if remoteView.Debug() {
				now = time.Now()
			}
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

type ResizeMatSource struct {
	vision.MatSource
	X, Y int
}

func (rms ResizeMatSource) NextColorDepthPair() (gocv.Mat, vision.DepthMap, error) {
	mat, dm, err := rms.MatSource.NextColorDepthPair()
	if err != nil {
		return mat, dm, err
	}
	defer mat.Close()
	out := gocv.NewMatWithSize(rms.X, rms.Y, gocv.MatTypeCV8UC3)

	gocv.Resize(mat, &out, image.Point{rms.X, rms.Y}, 0, 0, gocv.InterpolationCubic)

	return out, dm, err
}

func (rms ResizeMatSource) Close() {
	rms.MatSource.Close()
}
