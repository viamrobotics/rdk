package chess

import (
	"fmt"
	"image"

	"gocv.io/x/gocv"
)

var (
	DepthCheckSizeRadius = 35
	MinPieceDepth        = 10.0
)

func warpColorAndDepthToChess(color, depth gocv.Mat, corners []image.Point) (gocv.Mat, gocv.Mat, error) {
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

	warpedDepth := gocv.Mat{}
	if depth.Ptr() != nil && !depth.Empty() {
		warpedDepth = gocv.NewMatWithSize(800, 800, depth.Type())
		gocv.WarpPerspective(depth, &warpedDepth, m, image.Point{800, 800})
	}

	return warped, warpedDepth, nil
}

// returns point in a1, a8, h1, h8 order
func findChessCorners(img gocv.Mat, debugOut *gocv.Mat) ([]image.Point, error) {
	mine := debugOut == nil
	out := gocv.Mat{}
	if mine {
		out = gocv.NewMat()
		defer out.Close()
		debugOut = &out
	}

	return FindChessCornersPinkCheat(img, debugOut)
}

func getMinChessCorner(chess string) image.Point {
	var x = int(chess[0] - 'A')
	var y = (7 - int(chess[1]-'1'))
	if x < 0 || x > 7 || y < 0 || y > 7 {
		panic(fmt.Errorf("bad chess position %s %d %d", chess, x, y))
	}
	return image.Point{x * 100, y * 100}
}
