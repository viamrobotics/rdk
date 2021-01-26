package support

import (
	"image"
	"image/color"
	"sync"

	"github.com/echolabsinc/robotcore/vision"

	"github.com/james-bowman/sparse"
	"gocv.io/x/gocv"
)

type WorldViewer struct {
	RoomPoints   *sparse.DOK
	RoomPointsMu *sync.Mutex
	Scale        int
}

func (wv *WorldViewer) NextColorDepthPair() (gocv.Mat, vision.DepthMap, error) {
	x, y := wv.RoomPoints.Dims()
	// TODO(erd): any way to make this really fast? Allocate these in advance in
	// a goroutine? Pool?
	out := gocv.NewMatWithSize(x/wv.Scale, y/wv.Scale, gocv.MatTypeCV8UC3)

	wv.RoomPointsMu.Lock()
	defer wv.RoomPointsMu.Unlock()
	wv.RoomPoints.DoNonZero(func(x, y int, _ float64) {
		p := image.Point{x / wv.Scale, y / wv.Scale}
		gocv.Circle(&out, p, 1, color.RGBA{R: 255}, 1)
	})

	return out, vision.DepthMap{}, nil
}

func (wv *WorldViewer) Close() {
}
