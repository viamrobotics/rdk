package chess

import (
	"fmt"
	"image"

	"github.com/edaniels/golog"
	"go.viam.com/robotcore/rimage"
)

var (
	DepthCheckSizeRadius = 20
	MinPieceDepth        = 9.9999
)

func warpColorAndDepthToChess(img *rimage.ImageWithDepth, corners []image.Point) (*rimage.ImageWithDepth, error) {
	dst := []image.Point{
		image.Pt(0, 800),
		image.Pt(0, 0),
		image.Pt(800, 800),
		image.Pt(800, 0),
	}

	if len(corners) != 4 {
		return nil, fmt.Errorf("need 4 corners, got %d", len(corners))
	}
	return img.Warp(corners, dst, image.Point{800, 800}), nil
}

// returns point in a1, a8, h1, h8 order
func findChessCorners(img *rimage.ImageWithDepth, logger golog.Logger) (image.Image, []image.Point, error) {
	return FindChessCornersPinkCheat(img, logger)
}

func getMinChessCorner(chess string) image.Point {
	var x = int(chess[0] - 'a')
	var y = (7 - int(chess[1]-'1'))
	if x < 0 || x > 7 || y < 0 || y > 7 {
		panic(fmt.Errorf("bad chess position %s %d %d", chess, x, y))
	}
	return image.Point{x * 100, y * 100}
}

func getChessMiddle(chess string) image.Point {
	p := getMinChessCorner(chess)
	p.X += 50
	p.Y += 50
	return p
}
