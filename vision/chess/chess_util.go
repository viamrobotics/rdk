package chess

import (
	"fmt"
	"image"
	"math"

	"gocv.io/x/gocv"
)

var (
	DepthCheckSizeRadius = 20
	MinPieceDepth        = 10.0
)

func _distance(a image.Point, b image.Point) int {
	return int(math.Sqrt(math.Pow(float64(b.X-a.X), 2) + math.Pow(float64(b.Y-a.Y), 2)))
}

func warpColorAndDepthToChess(color, depth gocv.Mat, corners []image.Point) (gocv.Mat, gocv.Mat, error) {
	if false {
		fmt.Println(_distance(corners[0], corners[1]))
		fmt.Println(_distance(corners[1], corners[3]))
		fmt.Println(_distance(corners[3], corners[2]))
		fmt.Println(_distance(corners[2], corners[0]))
	}

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
