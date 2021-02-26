package chess

import (
	"fmt"
	"image"
	"math"
	"sort"

	"go.viam.com/robotcore/utils"
	"go.viam.com/robotcore/vision"

	"github.com/edaniels/golog"
	"github.com/fogleman/gg"
	"github.com/gonum/stat"
)

const (
	EdgeThreshold = 100
)

type Board struct {
	color  vision.Image // TODO(erh): should we get rid of
	depth  *vision.DepthMap
	edges  *image.Gray
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
	_, corners, err := findChessCorners(color)
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

	edges, err := utils.SimpleEdgeDetection(a.Image(), .01, 3.0)
	if err != nil {
		return nil, err
	}
	return &Board{a, b, edges, golog.Global}, nil
}

func (b *Board) SquareCenterHeight(square string, radius int) float64 {
	return b.SquareCenterHeight2(square, radius, false)
}

// return highest delta, average floor height
func (b *Board) SquareCenterHeight2(square string, radius int, matchColor bool) float64 {

	edges := b.SquareCenterEdges(square)
	//fmt.Printf("%s edges: %v\n", square, edges)
	if edges < EdgeThreshold {
		return 0
	}

	if edges > EdgeThreshold {
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
	corner := getMinChessCorner(square)
	center := image.Point{corner.X + 50, corner.Y + 50}

	return utils.CountBrightSpots(b.edges, center, 25, 255)
}

type SquareFunc func(b *Board, square string) error

func (b *Board) WriteDebugImages(prefix string) error {
	err := utils.WriteImageToFile(prefix+"-color.png", b.color.Image())
	if err != nil {
		return err
	}

	err = utils.WriteImageToFile(prefix+"-edges.png", b.edges)
	if err != nil {
		return err
	}

	i := b.Annotate()
	err = utils.WriteImageToFile(prefix+"-annotations.png", i)
	if err != nil {
		return err
	}

	return nil
}

func (b *Board) Annotate() image.Image {
	dc := gg.NewContextForImage(b.color.Image())

	for x := 'a'; x <= 'h'; x++ {
		for y := '1'; y <= '8'; y++ {
			s := string(x) + string(y)

			p := getChessMiddle(s)

			// draw the box around the points we are using
			c1 := image.Point{p.X - DepthCheckSizeRadius, p.Y - DepthCheckSizeRadius}
			c2 := image.Point{p.X + DepthCheckSizeRadius, p.Y - DepthCheckSizeRadius}
			c3 := image.Point{p.X + DepthCheckSizeRadius, p.Y + DepthCheckSizeRadius}
			c4 := image.Point{p.X - DepthCheckSizeRadius, p.Y + DepthCheckSizeRadius}
			dc.SetColor(utils.Green.C)
			dc.DrawLine(float64(c1.X), float64(c1.Y), float64(c2.X), float64(c2.Y))
			dc.SetLineWidth(1)
			dc.Stroke()
			dc.DrawLine(float64(c2.X), float64(c2.Y), float64(c3.X), float64(c3.Y))
			dc.SetLineWidth(1)
			dc.Stroke()
			dc.DrawLine(float64(c3.X), float64(c3.Y), float64(c4.X), float64(c4.Y))
			dc.SetLineWidth(1)
			dc.Stroke()
			dc.DrawLine(float64(c1.X), float64(c1.Y), float64(c4.X), float64(c4.Y))
			dc.SetLineWidth(1)
			dc.Stroke()

			height := b.SquareCenterHeight(s, DepthCheckSizeRadius)
			if height > MinPieceDepth {
				dc.DrawCircle(float64(p.X), float64(p.Y), 10)
				dc.SetColor(utils.Red.C)
				dc.Fill()
			}

			edges := b.SquareCenterEdges(s)

			p.Y -= 20
			utils.DrawString(dc, fmt.Sprintf("%d,%d", int(height), edges), p, utils.Green.C, 12)

		}
	}

	return dc.Image()
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
