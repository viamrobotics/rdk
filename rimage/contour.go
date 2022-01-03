package rimage

import (
	"image"
	"image/color"
	"image/png"
	"os"

	"github.com/fogleman/gg"
	"github.com/golang/geo/r2"
	"go.viam.com/utils"
	"gonum.org/v1/gonum/mat"
)

// ContourFloat defines the Contour type that contains 2d float points.
type ContourFloat []r2.Point

// ContourInt defines the Contour type that contains image points (int coordinates).
type ContourInt []image.Point

// ContourPoint is a simple structure for readability.
type ContourPoint struct {
	Point r2.Point
	Idx   int
}

// Border stores the ID of a Border and its type (Hole or Outer).
type Border struct {
	segNum, borderType int
}

// enumeration for Border types.
const (
	Hole = iota + 1
	Outer
)

// CreateHoleBorder creates a Hole Border.
func CreateHoleBorder() Border {
	return Border{
		segNum:     0,
		borderType: Hole,
	}
}

// CreateOuterBorder creates an Outer Border.
func CreateOuterBorder() Border {
	return Border{
		segNum:     0,
		borderType: Outer,
	}
}

// PointMat stores a point in matrix coordinates convention.
type PointMat struct {
	Row, Col int
}

// Set sets a current point to new coordinates (r,c).
func (p *PointMat) Set(r, c int) {
	p.Col = c
	p.Row = r
}

// SetTo sets a current point to new coordinates (r,c).
func (p *PointMat) SetTo(p2 PointMat) {
	p.Col = p2.Col
	p.Row = p2.Row
}

// SamePoint returns true if a point q is equal to the point p.
func (p *PointMat) SamePoint(q *PointMat) bool {
	return p.Row == q.Row && p.Col == q.Col
}

// GetPairOfFarthestPointsContour take an ordered contour and returns the 2 farthest points and their respective
// indices in that contour.
func GetPairOfFarthestPointsContour(points ContourFloat) (ContourPoint, ContourPoint) {
	start := ContourPoint{points[0], 0}
	end := ContourPoint{points[1], 1}
	distMax := 0.0
	for i, p := range points {
		for j, q := range points {
			d := p.Sub(q).Norm()
			if d > distMax {
				start = ContourPoint{p, i}
				end = ContourPoint{q, j}
				distMax = d
			}
		}
	}
	return start, end
}

// IsContourClosed takes a sorted contour, and returns true if the first and last points of it are spatially epsilon-close.
func IsContourClosed(points ContourFloat, eps float64) bool {
	start := points[0]
	end := points[len(points)-1]
	d := end.Sub(start).Norm()

	return d < eps
}

// ReorderClosedContourWithFarthestPoints takes a closed ordered contour and reorders the points in it so that all the
// points from p1 to p2 are in the first part of the contour and points from p2 to p1 in the second part.
func ReorderClosedContourWithFarthestPoints(points ContourFloat) ContourFloat {
	start, end := GetPairOfFarthestPointsContour(points)
	if start.Idx > end.Idx {
		start, end = end, start
	}
	c1 := points[start.Idx:end.Idx]
	c2 := points[end.Idx:]
	c2 = append(c2, points[:start.Idx]...)
	c1 = append(c1, c2...)
	return c1
}

// helpers
// convertImagePointToVec converts an image.Point to a r2.Point.
func convertImagePointToVec(p image.Point) r2.Point {
	return r2.Point{X: float64(p.X), Y: float64(p.Y)}
}

// ConvertContourIntToContourFloat converts a slice of image.Point to a slice of r2.Point.
func ConvertContourIntToContourFloat(pts []image.Point) ContourFloat {
	out := make([]r2.Point, len(pts))
	for i, pt := range pts {
		out[i] = convertImagePointToVec(pt)
	}
	return out
}

// savePNG takes an image.Image and saves it to a png file fn.
func savePNG(fn string, m image.Image) error {
	f, err := os.Create(fn)
	if err != nil {
		return err
	}
	defer utils.UncheckedErrorFunc(f.Close)
	return png.Encode(f, m)
}

// Visualization functions

// DrawContours draws the contours in a black image and saves it in outFile.
func DrawContours(img *mat.Dense, contours [][]image.Point, outFile string) error {
	h, w := img.Dims()
	g := image.NewGray16(image.Rect(0, 0, w, h))

	for i, cnt := range contours {
		for _, pt := range cnt {
			g.SetGray16(pt.X, pt.Y, color.Gray16{uint16(i)})
		}
	}

	if err := savePNG(outFile, g); err != nil {
		return err
	}
	return nil
}

// DrawContoursSimplified draws the simplified polygonal contours in a black image and saves it in outFile.
func DrawContoursSimplified(img *mat.Dense, contours []ContourFloat, outFile string) error {
	h, w := img.Dims()
	dc := gg.NewContext(h, w)
	dc.SetRGB(0, 0, 0)
	dc.Clear()
	for _, cnt := range contours {
		nPoints := len(cnt)
		for i, pt := range cnt {
			x1, y1 := pt.X, pt.Y
			x2, y2 := cnt[(i+1)%nPoints].X, cnt[(i+1)%nPoints].Y
			r := 1.0
			g := 1.0
			b := 1.0
			a := 1.0
			dc.SetRGBA(r, g, b, a)
			dc.SetLineWidth(1)
			dc.DrawLine(y1, x1, y2, x2)
			dc.Stroke()
		}
	}
	return dc.SavePNG(outFile)
}
