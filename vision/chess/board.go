package chess

import (
	"fmt"
	"image"
	"math"
	"sort"

	"github.com/gonum/stat"

	"gocv.io/x/gocv"

	"github.com/echolabsinc/robotcore/vision"
)

type Board struct {
	color gocv.Mat
	depth gocv.Mat
}

func FindAndWarpBoardFromFiles(colorFN, depthFN string) (*Board, error) {
	img := gocv.IMRead(colorFN, gocv.IMReadUnchanged)
	dm, err := vision.ParseDepthMap(depthFN)
	if err != nil {
		return nil, err
	}

	return FindAndWarpBoard(img, dm.ToMat())
}

func FindAndWarpBoard(color, depth gocv.Mat) (*Board, error) {
	corners, err := findChessCorners(color, nil)
	if err != nil {
		return nil, err
	}

	a, b, err := warpColorAndDepthToChess(color, depth, corners)
	if err != nil {
		return nil, err
	}

	return &Board{a, b}, nil
}

func (b *Board) PieceHeight(square string) float64 {
	data := []float64{}

	corner := getMinChessCorner(square)
	for x := corner.X + 50 - DepthCheckSizeRadius; x < corner.X+50+DepthCheckSizeRadius; x++ {
		for y := corner.Y + 50 - DepthCheckSizeRadius; y < corner.Y+50+DepthCheckSizeRadius; y++ {
			d := b.depth.GetDoubleAt(y, x)
			if d == 0 {
				continue
			}
			data = append(data, d)
		}
	}

	if len(data) < 30 {
		return 0
	}

	// since there is some noise, let's try and remove the outliers

	mean, stdDev := stat.MeanStdDev(data, nil)

	sort.Float64s(data)
	cleaned := data
	if false {
		cleaned := []float64{}

		for _, x := range data {
			diff := math.Abs(mean - x)
			if diff > 6*stdDev { // this 3 is totally a magic number, is it good?
				continue
			}
			cleaned = append(cleaned, x)
		}
	}

	min := stat.Mean(cleaned[0:10], nil)
	max := stat.Mean(cleaned[len(cleaned)-10:], nil)

	if false && square == "e1" {
		fmt.Println(square)

		for _, d := range cleaned[0:5] {
			fmt.Printf("\t %f\n", d)
		}
		fmt.Println("...")
		for _, d := range cleaned[len(cleaned)-5:] {
			fmt.Printf("\t %f\n", d)
		}

		fmt.Printf("\t %s mean: %f stdDev: %f min: %f max: %f diff: %f\n", square, mean, stdDev, min, max, max-min)
	}

	return max - min
}

func (b *Board) HasPiece(square string) bool {
	return b.PieceHeight(square) > MinPieceDepth
}

func (b *Board) GetSquaresWithPieces() []string {
	squares := []string{}
	for x := 'a'; x <= 'h'; x++ {
		for y := '1'; y <= '8'; y++ {
			s := string(x) + string(y)
			if b.HasPiece(s) {
				squares = append(squares, s)
			}
		}
	}
	return squares
}

func (b *Board) GetSquaresWithNoPieces() []string {
	squares := []string{}
	for x := 'a'; x <= 'h'; x++ {
		for y := '1'; y <= '8'; y++ {
			s := string(x) + string(y)
			if !b.HasPiece(s) {
				squares = append(squares, s)
			}
		}
	}
	return squares
}

func (b *Board) Annotate() gocv.Mat {
	out := gocv.NewMat()
	b.color.CopyTo(&out)

	for x := 'a'; x <= 'h'; x++ {
		for y := '1'; y <= '8'; y++ {
			s := string(x) + string(y)

			p := getMinChessCorner(s)
			p.X += 50
			p.Y += 50

			// draw the box around the points we are using
			c1 := image.Point{p.X - DepthCheckSizeRadius, p.Y - DepthCheckSizeRadius}
			c2 := image.Point{p.X + DepthCheckSizeRadius, p.Y - DepthCheckSizeRadius}
			c3 := image.Point{p.X + DepthCheckSizeRadius, p.Y + DepthCheckSizeRadius}
			c4 := image.Point{p.X - DepthCheckSizeRadius, p.Y + DepthCheckSizeRadius}
			gocv.Line(&out, c1, c2, vision.Green.C, 1)
			gocv.Line(&out, c2, c3, vision.Green.C, 1)
			gocv.Line(&out, c3, c4, vision.Green.C, 1)
			gocv.Line(&out, c1, c4, vision.Green.C, 1)

			height := b.PieceHeight(s)
			if height > MinPieceDepth {
				gocv.Circle(&out, p, 10, vision.Red.C, 2)
			}

			p.Y -= 20
			gocv.PutText(&out, fmt.Sprintf("%d", int(height)), p, gocv.FontHersheyPlain, 1.2, vision.Green.C, 2)

		}
	}

	return out
}

func (b *Board) IsBoardBlocked() bool {
	data := []float64{}

	for x := 0; x < b.depth.Cols(); x++ {
		for y := 0; y < b.depth.Rows(); y++ {
			d := b.depth.GetDoubleAt(y, x)
			if d == 0 {
				continue
			}
			data = append(data, d)

		}
	}

	mean, stdDev := stat.MeanStdDev(data, nil)
	sort.Float64s(data)
	//fmt.Printf("IsBoardBlocked mean: %f stdDev: %f [ %f %f ... %f %f ]\n", mean, stdDev, data[0], data[1], data[len(data)-2], data[len(data)-1])

	return data[len(data)-1]-data[0] > 200
}
