package stream

import (
	"context"
	"image"
	"time"

	"github.com/echolabsinc/robotcore/utils"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"gocv.io/x/gocv"
)

func MatSource(ctx context.Context, src utils.MatSource, remoteView gostream.RemoteView, captureInternal time.Duration, logger golog.Logger) {
	gostream.StreamFunc(
		ctx,
		func() image.Image {
			var now time.Time
			if remoteView.Debug() {
				now = time.Now()
			}
			mat, err := src.NextMat()
			if err != nil {
				panic(err)
			}
			defer mat.Close()
			if remoteView.Debug() {
				logger.Debugw("NextMat", "elapsed", time.Since(now))
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
	utils.MatSource
	X, Y int
}

func (rms ResizeMatSource) NextMat() (gocv.Mat, error) {
	mat, err := rms.MatSource.NextMat()
	if err != nil {
		return mat, err
	}
	defer mat.Close()
	out := gocv.NewMatWithSize(rms.X, rms.Y, gocv.MatTypeCV8UC3)

	gocv.Resize(mat, &out, image.Point{rms.X, rms.Y}, 0, 0, gocv.InterpolationCubic)

	return out, err
}

func (rms ResizeMatSource) Close() {
	rms.MatSource.Close()
}
