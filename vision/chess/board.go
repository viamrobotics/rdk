package chess

import (
	"fmt"
	"image"
	"math"
	"sort"

	"github.com/edaniels/golog"
	"github.com/fogleman/gg"
	"github.com/gonum/stat"
	"github.com/pkg/errors"

	"go.viam.com/rdk/rimage"
)

// EdgeThreshold TODO.
const EdgeThreshold = 100

// Board TODO.
type Board struct {
	color  *rimage.Image
	depth  *rimage.DepthMap
	edges  *image.Gray
	logger golog.Logger
}

// FindAndWarpBoardFromFilesRoot TODO.
func FindAndWarpBoardFromFilesRoot(root string, aligned bool, logger golog.Logger) (*Board, error) {
	return FindAndWarpBoardFromFiles(root+".png", root+".dat.gz", aligned, logger)
}

// FindAndWarpBoardFromFiles TODO.
func FindAndWarpBoardFromFiles(colorFN, depthFN string, aligned bool, logger golog.Logger) (*Board, error) {
	color, err := rimage.NewImageFromFile(colorFN)
	if err != nil {
		return nil, err
	}
	depth, err := rimage.ParseDepthMap(depthFN)
	if err != nil {
		return nil, err
	}

	return FindAndWarpBoard(color, depth, logger)
}

// FindAndWarpBoard TODO.
func FindAndWarpBoard(col *rimage.Image, dm *rimage.DepthMap, logger golog.Logger) (*Board, error) {
	_, corners, err := findChessCorners(col, logger)
	if err != nil {
		return nil, err
	}

	if len(corners) != 4 {
		return nil, errors.Errorf("couldnt find 4 corners, only got %d", len(corners))
	}

	color, depth, err := warpColorAndDepthToChess(col, dm, corners)
	if err != nil {
		return nil, err
	}

	edges, err := rimage.SimpleEdgeDetection(color, .01, 3.0)
	if err != nil {
		return nil, err
	}
	return &Board{color, depth, edges, logger}, nil
}

// SquareCenterHeight TODO.
func (b *Board) SquareCenterHeight(square string, radius int) float64 {
	return b.SquareCenterHeight2(square, radius, false)
}

// SquareCenterHeight2 TODO
// return highes
// SquareCenterHeight2 TODOt delta, average floor height.
func (b *Board) SquareCenterHeight2(square string, radius int, matchColor bool) float64 {
	edges := b.SquareCenterEdges(square)
	// fmt.Printf("%s edges: %v\n", square, edges)
	if edges < EdgeThreshold {
		return 0
	}

	if edges > EdgeThreshold {
		radius++
	}

	data := []float64{}

	corner := getMinChessCorner(square)
	centerColor := b.color.Get(image.Point{corner.X + 50, corner.Y + 50})

	for x := corner.X + 50 - radius; x < corner.X+50+radius; x++ {
		for y := corner.Y + 50 - radius; y < corner.Y+50+radius; y++ {
			d := b.depth.GetDepth(x, y)
			if d == 0 {
				continue
			}
			if matchColor {
				c := b.color.Get(image.Point{x, y})
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

// SquareCenterEdges TODO.
func (b *Board) SquareCenterEdges(square string) int {
	corner := getMinChessCorner(square)
	center := image.Point{corner.X + 50, corner.Y + 50}

	return rimage.CountBrightSpots(b.edges, center, 25, 255)
}

// SquareFunc TODO.
type SquareFunc func(b *Board, square string) error

// WriteDebugImages TODO.
func (b *Board) WriteDebugImages(prefix string) error {
	err := b.color.WriteTo(prefix + "-color.png")
	if err != nil {
		return err
	}

	err = rimage.WriteImageToFile(prefix+"-edges.png", b.edges)
	if err != nil {
		return err
	}

	i := b.Annotate()
	err = rimage.WriteImageToFile(prefix+"-annotations.png", i)
	if err != nil {
		return err
	}

	return nil
}

// Annotate TODO.
func (b *Board) Annotate() image.Image {
	dc := gg.NewContextForImage(b.color)

	for x := 'a'; x <= 'h'; x++ {
		for y := '1'; y <= '8'; y++ {
			s := string(x) + string(y)

			p := getChessMiddle(s)

			// draw the box around the points we are using
			c1 := image.Point{p.X - DepthCheckSizeRadius, p.Y - DepthCheckSizeRadius}
			c2 := image.Point{p.X + DepthCheckSizeRadius, p.Y - DepthCheckSizeRadius}
			c3 := image.Point{p.X + DepthCheckSizeRadius, p.Y + DepthCheckSizeRadius}
			c4 := image.Point{p.X - DepthCheckSizeRadius, p.Y + DepthCheckSizeRadius}
			dc.SetColor(rimage.Green)
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
				dc.SetColor(rimage.Red)
				dc.Fill()
			}

			edges := b.SquareCenterEdges(s)

			p.Y -= 20
			rimage.DrawString(dc, fmt.Sprintf("%d,%d", int(height), edges), p, rimage.Green, 12)
		}
	}

	return dc.Image()
}

// IsBoardBlocked TODO.
func (b *Board) IsBoardBlocked() bool {
	numPieces := 0
	for x := 'a'; x <= 'h'; x++ {
		for y := '1'; y <= '8'; y++ {
			s := string(x) + string(y)
			h := b.SquareCenterHeight(s, DepthCheckSizeRadius)
			// b.logger.Debugf("%s -> %v\n", s, h)
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
