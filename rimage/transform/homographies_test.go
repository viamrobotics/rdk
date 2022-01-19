package transform

import (
	"math"
	"testing"

	"github.com/golang/geo/r2"
	"go.viam.com/test"
	"gonum.org/v1/gonum/mat"
)

// CreateRotationMatrix creates a 2x2 rotation matrix with given angle in radians.
func CreateRotationMatrix(angle float64) *mat.Dense {
	r := mat.NewDense(2, 2, nil)
	r.Set(0, 0, math.Cos(angle))
	r.Set(0, 1, -math.Sin(angle))
	r.Set(1, 0, math.Sin(angle))
	r.Set(1, 1, math.Cos(angle))

	return r
}

func SlicesXsYsToPoints(points [][]float64) []r2.Point {
	pts := make([]r2.Point, len(points))
	for i, pt := range points {
		x := pt[0]
		y := pt[1]
		pts[i] = r2.Point{x, y}
	}
	return pts
}

func repeatedSlice(value float64, n int) []float64 {
	arr := make([]float64, n)
	for i := 0; i < n; i++ {
		arr[i] = value
	}
	return arr
}

func TestGeometricDistance(t *testing.T) {
	pt1 := r2.Point{0, 0}
	pt2 := r2.Point{1, 0}
	// h = Id, distance should be 1
	h1 := mat.NewDense(3, 3, nil)
	h1.Set(0, 0, 1)
	h1.Set(1, 1, 1)
	h1.Set(2, 2, 1)
	d1 := geometricDistance(pt1, pt2, h1)
	test.That(t, d1, test.ShouldEqual, 1.0)
	// rotation -pi/2
	h2 := mat.NewDense(3, 3, nil)
	h2.Set(0, 1, 1)
	h2.Set(1, 0, -1)
	h2.Set(2, 2, 1)
	d2 := geometricDistance(pt1, pt2, h2)
	test.That(t, d2, test.ShouldEqual, 1.0)
	// rotation -pi/2
	h3 := mat.NewDense(3, 3, nil)
	h3.Set(0, 1, 1)
	h3.Set(1, 0, -1)
	h3.Set(2, 2, 1)
	pt3 := r2.Point{1, 0}
	d3 := geometricDistance(pt3, pt2, h3)
	test.That(t, d3, test.ShouldEqual, 1.4142135623730951)
}
