package rimage

import (
	"image"
	"math"

	"github.com/golang/geo/r2"
	"github.com/pkg/errors"

	"go.viam.com/rdk/utils"

	"gonum.org/v1/gonum/floats"
)

// NoPoints TODO.
var NoPoints = image.Point{-1, -1}

// Center TODO.
func Center(contour []image.Point, maxDiff int) image.Point {
	if len(contour) == 0 {
		return NoPoints
	}

	x := 0
	y := 0

	for _, p := range contour {
		x += p.X
		y += p.Y
	}

	weightedMiddle := image.Point{x / len(contour), y / len(contour)}

	numPoints := 0
	box := image.Rectangle{image.Point{1000000, 100000}, image.Point{0, 0}}
	for _, p := range contour {
		if utils.AbsInt(p.X-weightedMiddle.X) > maxDiff || utils.AbsInt(p.Y-weightedMiddle.Y) > maxDiff {
			continue
		}

		numPoints++

		if p.X < box.Min.X {
			box.Min.X = p.X
		}
		if p.Y < box.Min.Y {
			box.Min.Y = p.Y
		}

		if p.X > box.Max.X {
			box.Max.X = p.X
		}
		if p.Y > box.Max.Y {
			box.Max.Y = p.Y
		}
	}

	if numPoints == 0 {
		return NoPoints
	}

	avgMiddle := image.Point{(box.Min.X + box.Max.X) / 2, (box.Min.Y + box.Max.Y) / 2}
	// fmt.Printf("%v -> %v  box: %v\n", weightedMiddle, avgMiddle, box)
	return avgMiddle
}

// PointDistance TODO.
func PointDistance(a, b image.Point) float64 {
	x := utils.SquareInt(b.X - a.X)
	x += utils.SquareInt(b.Y - a.Y)
	return math.Sqrt(float64(x))
}

// ArrayToPoints TODO.
func ArrayToPoints(pts []image.Point) []image.Point {
	if len(pts) == 4 {
		return pts
	}

	if len(pts) == 2 {
		r := image.Rectangle{pts[0], pts[1]}
		return []image.Point{
			r.Min,
			{r.Max.X, r.Min.Y},
			r.Max,
			{r.Min.X, r.Max.Y},
		}
	}

	panic(errors.Errorf("invalid number of points passed to ArrayToPoints %d", len(pts)))
	return nil
}

// PointAngle TODO.
func PointAngle(a, b image.Point) float64 {
	x := b.X - a.X
	y := b.Y - a.Y
	return math.Atan2(float64(y), float64(x))
}

// BoundingBox TODO.
func BoundingBox(pts []image.Point) image.Rectangle {
	min := image.Point{math.MaxInt32, math.MaxInt32}
	max := image.Point{0, 0}

	for _, p := range pts {
		if p.X < min.X {
			min.X = p.X
		}
		if p.Y < min.Y {
			min.Y = p.Y
		}

		if p.X > max.X {
			max.X = p.X
		}
		if p.Y > max.Y {
			max.Y = p.Y
		}
	}

	return image.Rectangle{min, max}
}

// AllPointsIn TODO.
func AllPointsIn(size image.Point, pts []image.Point) bool {
	for _, p := range pts {
		if p.X < 0 || p.Y < 0 {
			return false
		}
		if p.X >= size.X {
			return false
		}
		if p.Y >= size.Y {
			return false
		}
	}

	return true
}

// R2PointToImagePoint TODO.
func R2PointToImagePoint(pt r2.Point) image.Point {
	return image.Point{int(math.Round(pt.X)), int(math.Round(pt.Y))}
}

// R2RectToImageRect TODO.
func R2RectToImageRect(rec r2.Rect) image.Rectangle {
	return image.Rectangle{R2PointToImagePoint(rec.Lo()), R2PointToImagePoint(rec.Hi())}
}

// TranslateR2Rect TODO.
func TranslateR2Rect(rect r2.Rect, pt r2.Point) r2.Rect {
	return r2.RectFromCenterSize(rect.Center().Add(pt), rect.Size())
}

// SortPointCounterClockwise sorts a slice of image.Point in counterclockwise order, starting from point closest to -pi
func SortPointCounterClockwise(pts []r2.Point) []r2.Point {
	// create new slice of points
	out := make([]r2.Point, len(pts))
	xs, ys := SliceVecsToXsYs(pts)
	xMin := floats.Min(xs)
	xMax := floats.Max(xs)
	yMin := floats.Min(ys)
	yMax := floats.Max(ys)
	centerX := xMin + (xMax-xMin)/2
	centerY := yMin + (yMax-yMin)/2
	floats.AddConst(-centerX, xs)
	floats.AddConst(-centerY, ys)
	angles := make([]float64, len(pts))
	for i, _ := range xs {
		angles[i] = math.Atan2(ys[i], xs[i])
	}
	inds := make([]int, len(pts))
	floats.Argsort(angles, inds)

	for i := 0; i < len(pts); i++ {
		idx := inds[i]
		x := math.Round(xs[idx] + centerX)
		y := math.Round(ys[idx] + centerY)
		out[i] = r2.Point{X: x, Y: y}
	}
	return out
}

// SliceVecsToXsYs converts a slice of r2.Point to 2 slices floats containing x and y coordinates
func SliceVecsToXsYs(pts []r2.Point) ([]float64, []float64) {
	xs := make([]float64, len(pts))
	ys := make([]float64, len(pts))
	for i, pt := range pts {
		xs[i] = pt.X
		ys[i] = pt.Y
	}
	return xs, ys
}

// SortVectorsCounterClockwise sorts a slice of vectors counterclockwise
func SortVectorsCounterClockwise(pts []r2.Point) []r2.Point {
	xs, ys := SliceVecsToXsYs(pts)
	xMin := floats.Min(xs)
	xMax := floats.Max(xs)
	yMin := floats.Min(ys)
	yMax := floats.Max(ys)
	centerX := xMin + (xMax-xMin)/2
	centerY := yMin + (yMax-yMin)/2
	floats.AddConst(-centerX, xs)
	floats.AddConst(-centerY, ys)
	angles := make([]float64, len(pts))
	for i, _ := range xs {
		angles[i] = math.Atan2(ys[i], xs[i])
	}
	inds := make([]int, len(pts))
	floats.Argsort(angles, inds)
	// create new slice of vecs
	out := make([]r2.Point, len(pts))
	for i := 0; i < len(pts); i++ {
		idx := inds[i]
		x := xs[idx]
		y := ys[idx]
		out[i] = r2.Point{X: x, Y: y}
	}
	return out
}

// AreCollinear returns true if the 3 points a, b and c are collinear
func AreCollinear(a, b, c r2.Point, eps float64) bool {
	val := math.Abs((b.X-a.X)*(c.Y-a.Y) - (c.X-a.X)*(b.Y-a.Y))
	return val < eps
}
