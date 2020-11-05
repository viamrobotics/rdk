package vision

import (
	"image"

	"gocv.io/x/gocv"
)

func WarpColorAndDepthToChess(color, depth gocv.Mat, corners []image.Point) (gocv.Mat, gocv.Mat, error) {
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
	if !depth.Empty() {
		warpedDepth = gocv.NewMatWithSize(800, 800, depth.Type())
		gocv.WarpPerspective(depth, &warpedDepth, m, image.Point{800, 800})
	}

	return warped, warpedDepth, nil
}

// returns point in a1, a8, h1, h8 order
func FindChessCorners(img gocv.Mat) ([]image.Point, error) {
	a1Corner := image.Point{145, 45}
	a8Corner := image.Point{520, 52}
	h1Corner := image.Point{125, 440}
	h8Corner := image.Point{545, 440}

	return []image.Point{a1Corner, a8Corner, h1Corner, h8Corner}, nil
}
