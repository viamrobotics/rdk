package vision

import (
	"fmt"
	"image"
	"math"
	"sort"

	"github.com/gonum/stat"

	"gocv.io/x/gocv"
)

var (
	DepthCheckSizeRadius = 35
	MinPieceDepth        = 10.0
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
	if depth.Ptr() != nil && !depth.Empty() {
		warpedDepth = gocv.NewMatWithSize(800, 800, depth.Type())
		gocv.WarpPerspective(depth, &warpedDepth, m, image.Point{800, 800})
	}

	return warped, warpedDepth, nil
}

// returns point in a1, a8, h1, h8 order
func FindChessCorners(img gocv.Mat, debugOut *gocv.Mat) ([]image.Point, error) {
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
	var x = int(chess[0]-'A') * 100
	var y = 100 * (7 - int(chess[1]-'1'))
	return image.Point{x, y}
}

func GetChessPieceHeight(square string, warpedDepth gocv.Mat) float64 {
	if warpedDepth.Cols() != 800 || warpedDepth.Rows() != 800 {
		panic("bad image size pased to GetChessPieceHeight")
	}
	data := []float64{}

	corner := getMinChessCorner(square)
	for x := corner.X + 50 - DepthCheckSizeRadius; x < corner.X+50+DepthCheckSizeRadius; x++ {
		for y := corner.Y + 50 - DepthCheckSizeRadius; y < corner.Y+50+DepthCheckSizeRadius; y++ {
			d := warpedDepth.GetDoubleAt(y, x)
			if d == 0 {
				continue
			}
			data = append(data, d)
		}
	}

	// since there is some noise, let's try and remove the outliers

	mean, stdDev := stat.MeanStdDev(data, nil)

	sort.Float64s(data)
	cleaned := data
	if false {
		cleaned := []float64{}

		for _, x := range data {
			diff := math.Abs(mean - x)
			if diff > 5*stdDev { // this 3 is totally a magic number, is it good?
				continue
			}
			cleaned = append(cleaned, x)
		}
	}

	min := stat.Mean(cleaned[0:10], nil)
	max := stat.Mean(cleaned[len(cleaned)-10:], nil)

	if false {
		fmt.Println(square)

		for _, d := range cleaned[0:5] {
			fmt.Printf("\t %f\n", d)
		}
		fmt.Println("...")
		for _, d := range cleaned[len(cleaned)-5:] {
			fmt.Printf("\t %f\n", d)
		}
	}
	//fmt.Printf("\t %s mean: %f stdDev: %f min: %f max: %f\n", square, mean, stdDev, min, max)

	return max - min
}

func HasPiece(square string, warpedDepth gocv.Mat) bool {
	return GetChessPieceHeight(square, warpedDepth) > MinPieceDepth
}

func GetSquaresWithPieces(warpedDepth gocv.Mat) []string {
	squares := []string{}
	for x := 'A'; x <= 'H'; x++ {
		for y := '1'; y <= '8'; y++ {
			s := string(x) + string(y)
			if HasPiece(s, warpedDepth) {
				squares = append(squares, s)
			}
		}
	}
	return squares
}

func GetSquaresWithNoPieces(warpedDepth gocv.Mat) []string {
	squares := []string{}
	for x := 'A'; x <= 'H'; x++ {
		for y := '1'; y <= '8'; y++ {
			s := string(x) + string(y)
			if !HasPiece(s, warpedDepth) {
				squares = append(squares, s)
			}
		}
	}
	return squares
}

func AnnotateBoard(color, depth gocv.Mat) {
	for x := 'A'; x <= 'H'; x++ {
		for y := '1'; y <= '8'; y++ {
			s := string(x) + string(y)

			p := getMinChessCorner(s)
			p.X += 50
			p.Y += 50

			// draw the box around the points we are using
			a := image.Point{p.X - DepthCheckSizeRadius, p.Y - DepthCheckSizeRadius}
			b := image.Point{p.X + DepthCheckSizeRadius, p.Y - DepthCheckSizeRadius}
			c := image.Point{p.X + DepthCheckSizeRadius, p.Y + DepthCheckSizeRadius}
			d := image.Point{p.X - DepthCheckSizeRadius, p.Y + DepthCheckSizeRadius}
			gocv.Line(&color, a, b, Green.C, 1)
			gocv.Line(&color, b, c, Green.C, 1)
			gocv.Line(&color, c, d, Green.C, 1)
			gocv.Line(&color, a, d, Green.C, 1)

			height := GetChessPieceHeight(s, depth)
			if height > MinPieceDepth {
				gocv.Circle(&color, p, 10, Red.C, 2)
			}

			p.Y -= 20
			gocv.PutText(&color, fmt.Sprintf("%d", int(height)), p, gocv.FontHersheyPlain, 1.2, Green.C, 2)

		}
	}
}
