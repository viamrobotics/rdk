package support

import (
	"image"
	"image/color"

	"github.com/echolabsinc/robotcore/vision"

	"gocv.io/x/gocv"
)

type WorldViewer struct {
	Room      *SquareRoom
	ViewScale int
}

func (wv *WorldViewer) NextColorDepthPair() (gocv.Mat, vision.DepthMap, error) {
	roomSize, roomSizeScale := wv.Room.Size()
	roomSize *= roomSizeScale

	// TODO(erd): any way to make this really fast? Allocate these in advance in
	// a goroutine? Pool?
	out := gocv.NewMatWithSize(roomSize/wv.ViewScale, roomSize/wv.ViewScale, gocv.MatTypeCV8UC3)

	wv.Room.Mutate(func(room MutableRoom) {
		room.DoNonZero(func(x, y int, _ float64) {
			p := image.Point{x / wv.ViewScale, y / wv.ViewScale}
			gocv.Circle(&out, p, 1, color.RGBA{R: 255}, 1)
		})
	})

	return out, vision.DepthMap{}, nil
}

func (wv *WorldViewer) Close() {
}
