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

func warpColorAndDepthToChess(color vision.Image, depth vision.DepthMap, corners []image.Point) (vision.Image, vision.DepthMap, error) {
	dst := []image.Point{
		image.Pt(0, 800),
		image.Pt(0, 0),
		image.Pt(800, 800),
		image.Pt(800, 0),
	}

	pc := vision.PointCloud{depth, color}
	pc2, err := pc.Warp(corners, dst, image.Point{800, 800})
	return pc2.Color, pc2.Depth, err
}

// returns point in a1, a8, h1, h8 order
func findChessCorners(img vision.Image, debugOut *gocv.Mat) ([]image.Point, error) {
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
