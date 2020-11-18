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

func (b *Board) depthAt(p image.Point) float64 {
	return b.depth.GetDoubleAt(p.Y, p.X)
}

// return highest delta, average floor height
func (b *Board) SquareCenterHeight(square string, radius int) float64 {
	data := []float64{}

	corner := getMinChessCorner(square)
	for x := corner.X + 50 - radius; x < corner.X+50+radius; x++ {
		for y := corner.Y + 50 - radius; y < corner.Y+50+radius; y++ {
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

/*
func (b *Board) HasPiece(square string) bool {
	height, _ := b.SquareCenterHeight(square)
	return height > MinPieceDepth
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
*/

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

			height := b.SquareCenterHeight(s, DepthCheckSizeRadius)
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
	numPieces := 0
	for x := 'a'; x <= 'h'; x++ {
		for y := '1'; y <= '8'; y++ {
			s := string(x) + string(y)
			h := b.SquareCenterHeight(s, DepthCheckSizeRadius)
			//fmt.Printf("%s -> %v\n", s, h)
			if h > 160 {
				fmt.Printf("blocked at %s with %v\n", s, h)
				return true
			}

			if h > MinPieceDepth {
				numPieces++
			}
		}
	}

	if numPieces > 32 || numPieces == 0 {
		fmt.Printf("blocked b/c numPieces: %d\n", numPieces)
	}

	return false
}
