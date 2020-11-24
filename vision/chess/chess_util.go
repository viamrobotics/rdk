package chess

import (
	"fmt"
	"image"
	"math"

	"gocv.io/x/gocv"

	"github.com/echolabsinc/robotcore/vision"
)

var (
	DepthCheckSizeRadius = 20
	MinPieceDepth        = 9.9999
)

func _distance(a image.Point, b image.Point) int {
	return int(math.Sqrt(math.Pow(float64(b.X-a.X), 2) + math.Pow(float64(b.Y-a.Y), 2)))
}

func warpColorAndDepthToChess(color gocv.Mat, depth vision.DepthMap, corners []image.Point) (gocv.Mat, vision.DepthMap, error) {
	dst := []image.Point{
		image.Pt(0, 800),
		image.Pt(0, 0),
		image.Pt(800, 800),
		image.Pt(800, 0),
	}

	m := gocv.GetPerspectiveTransform(corners, dst)
	defer m.Close()

	warped := gocv.NewMat()
	gocv.WarpPerspective(color, &warped, m, image.Point{800, 800})

	var warpedDepth vision.DepthMap
	if depth.Width() > 0 {
		dm := depth.ToMat()
		defer dm.Close()
		dm2 := gocv.NewMatWithSize(800, 800, dm.Type())
		gocv.WarpPerspective(dm, &dm2, m, image.Point{800, 800})
		warpedDepth = vision.NewDepthMapFromMat(dm2)
	}

	return warped, warpedDepth, nil
}

// returns point in a1, a8, h1, h8 order
func findChessCorners(img gocv.Mat, debugOut *gocv.Mat) ([]image.Point, error) {
	return FindChessCornersPinkCheat(img, debugOut)
}

func getMinChessCorner(chess string) image.Point {
	var x = int(chess[0] - 'a')
	var y = (7 - int(chess[1]-'1'))
	if x < 0 || x > 7 || y < 0 || y > 7 {
		panic(fmt.Errorf("bad chess position %s %d %d", chess, x, y))
	}
	return image.Point{x * 100, y * 100}
}
