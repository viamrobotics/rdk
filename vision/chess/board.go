package chess

import (
	"fmt"
	"image"
	"math"
	"sort"

	"github.com/echolabsinc/robotcore/vision"

	"github.com/edaniels/golog"
	"github.com/gonum/stat"
	"gocv.io/x/gocv"
)

type Board struct {
	color  vision.Image // TODO(erh): should we get rid of
	depth  *vision.DepthMap
	edges  *gocv.Mat
	logger golog.Logger
}

func FindAndWarpBoardFromFilesRoot(root string) (*Board, error) {
	return FindAndWarpBoardFromFiles(root+".png", root+".dat.gz")
}

func FindAndWarpBoardFromFiles(colorFN, depthFN string) (*Board, error) {
	img, err := vision.NewImageFromFile(colorFN)
	if err != nil {
		return nil, err
	}

	dm, err := vision.ParseDepthMap(depthFN)
	if err != nil {
		return nil, err
	}
	dm.Smooth()

	return FindAndWarpBoard(img, dm)
}

func FindAndWarpBoard(color vision.Image, depth *vision.DepthMap) (*Board, error) {
	corners, err := findChessCorners(color, nil)
	if err != nil {
		return nil, err
	}

	if len(corners) != 4 {
		return nil, fmt.Errorf("couldnt find 4 corners, only got %d", len(corners))
	}

	a, b, err := warpColorAndDepthToChess(color, depth, corners)
	if err != nil {
		return nil, err
	}

	edges := gocv.NewMat()
	gocv.Canny(a.MatUnsafe(), &edges, 32, 32) // magic number

	return &Board{a, b, &edges, golog.Global}, nil
}

func (b *Board) Close() {
	b.color.Close()
	b.edges.Close()
}

func (b *Board) SquareCenterHeight(square string, radius int) float64 {
	return b.SquareCenterHeight2(square, radius, false)
}

// return highest delta, average floor height
func (b *Board) SquareCenterHeight2(square string, radius int, matchColor bool) float64 {

	edges := b.SquareCenterEdges(square)
	if edges < 100 {
		return 0
	}

	if edges > 100 {
		radius++
	}

	data := []float64{}

	corner := getMinChessCorner(square)
	centerColor := b.color.ColorHSV(image.Point{corner.X + 50, corner.Y + 50})

	for x := corner.X + 50 - radius; x < corner.X+50+radius; x++ {
		for y := corner.Y + 50 - radius; y < corner.Y+50+radius; y++ {
			d := b.depth.GetDepth(x, y)
			if d == 0 {
				continue
			}
			if matchColor {
				c := b.color.ColorHSV(image.Point{x, y})
				if c.Distance(centerColor) > 1 {
					continue
				}
			}
			data = append(data, float64(d))
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
		cleaned = []float64{}

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
		b.logger.Debug(square)

		for _, d := range cleaned[0:5] {
			b.logger.Debugf("\t %f\n", d)
		}
		b.logger.Debug("...")
		for _, d := range cleaned[len(cleaned)-5:] {
			b.logger.Debugf("\t %f\n", d)
		}

		b.logger.Debugf("\t %s mean: %f stdDev: %f min: %f max: %f diff: %f\n", square, mean, stdDev, min, max, max-min)
	}

	res := max - min

	if res < MinPieceDepth {
		return MinPieceDepth + .5
	}

	return res
}

func (b *Board) SquareCenterEdges(square string) int {

	radius := 25

	num := 0

	corner := getMinChessCorner(square)
	for x := corner.X + 50 - radius; x < corner.X+50+radius; x++ {
		for y := corner.Y + 50 - radius; y < corner.Y+50+radius; y++ {
			d := b.edges.GetUCharAt(y, x)
			//b.logger.Debugf("\t %v\n", d )
			if d == 255 {
				num++
			}
		}
	}

	return num
}

type SquareFunc func(b *Board, square string) error

func (b *Board) Annotate() gocv.Mat {
	out := gocv.NewMat()
	temp := b.color.MatUnsafe()
	temp.CopyTo(&out)

	for x := 'a'; x <= 'h'; x++ {
		for y := '1'; y <= '8'; y++ {
			s := string(x) + string(y)

			p := getChessMiddle(s)

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

			edges := b.SquareCenterEdges(s)

			p.Y -= 20
			gocv.PutText(&out, fmt.Sprintf("%d,%d", int(height), edges), p, gocv.FontHersheyPlain, 1.2, vision.Green.C, 2)

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
			//b.logger.Debugf("%s -> %v\n", s, h)
			if h > 150 {
				b.logger.Debugf("blocked at %s with %v\n", s, h)
				return true
			}

			if h > MinPieceDepth {
				numPieces++
			}
		}
	}

	if numPieces > 32 || numPieces == 0 {
		b.logger.Debugf("blocked b/c numPieces: %d\n", numPieces)
	}

	return false
}
