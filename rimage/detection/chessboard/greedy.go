package chessboard

import (
	"github.com/golang/geo/r2"
	"go.viam.com/core/rimage/transform"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
	//"image"
)

type ChessGreedyConfiguration struct {
	HomographyAcceptableScaleRatio float64 `json:"scale-ratio"`// acceptable ratio for scale part in estimated homography
	MinPointsNeeded                int     `json:"min_points_needed"`// minimum number of points to deem grid estimation valid
	MaxPointsNeeded                int     `json:"max_points_needed"`// if number of valid points above this, greedy iterations can be stopped
}

var quadI = []r2.Point{{0, 1}, {1, 1}, {1, 0}, {0, 0}}

// Single generates an n-dimensional grid using a single set of values.
// dim specifies the number of dimensions, the entries in x specify the gridded values.
func Single(dim int, x []float64) [][]float64 {
	dims := make([]int, dim)
	for i := range dims {
		dims[i] = len(x)
	}
	sz := size(dims)
	pts := make([][]float64, sz)
	sub := make([]int, dim)
	for i := 0; i < sz; i++ {
		SubFor(sub, i, dims)
		pt := make([]float64, dim)
		for j := range pt {
			pt[j] = x[sub[j]]
		}
		pts[i] = pt
	}
	return pts
}

func size(dims []int) int {
	n := 1
	for _, v := range dims {
		n *= v
	}
	return n
}

// SubFor constructs the multi-dimensional subscript for the input linear index.
// Dims specifies the maximum size in each dimension. SubFor is the converse of
// IdxFor.
//
// If sub is non-nil the result is stored in-place into sub. If it is nil a new
// slice of the appropriate length is allocated.
func SubFor(sub []int, idx int, dims []int) []int {
	for _, v := range dims {
		if v <= 0 {
			panic("bad dims")
		}
	}
	if sub == nil {
		sub = make([]int, len(dims))
	}
	if len(sub) != len(dims) {
		panic("size mismatch")
	}
	if idx < 0 {
		panic("bad index")
	}
	stride := 1
	for i := len(dims) - 1; i >= 1; i-- {
		stride *= dims[i]
	}
	for i := 0; i < len(dims)-1; i++ {
		v := idx / stride
		if v >= dims[i] {
			panic("bad index")
		}
		sub[i] = v
		idx -= v * stride
		stride /= dims[i+1]
	}
	if idx > dims[len(sub)-1] {
		panic("bad dims")
	}
	sub[len(sub)-1] = idx
	return sub
}

// getIdentityGrid returns a n x n 2D grid with coordinates offset,..., offset+n-1,
func getIdentityGrid(n, offset int) []r2.Point {
	// create n x n 2D grid 0...n-1
	x := make([]float64, n)
	floats.Span(x, 0, float64(n-1))
	pts := Single(2, x)

	// add offset
	for _, points := range pts {
		floats.AddConst(float64(offset), points)

	}
	// output slice of r2.Point
	outPoints := make([]r2.Point, 0)
	for i := 0; i < n; i++ {
		pt := r2.Point{pts[0][i], pts[1][i]}
		outPoints = append(outPoints, pt)
	}
	return outPoints
}

// makeChessGrid returns an identity grid and its transformation with homography H
func makeChessGrid(H *mat.Dense, n int) ([]r2.Point, []r2.Point) {
	idealGrid := getIdentityGrid(2 + 2*n, -n)
	grid := transform.ApplyHomography(H, idealGrid)
	return idealGrid, grid
	//return nil, nil
}

func getInitialChessGrid(quad []r2.Point) ([]r2.Point, []r2.Point, *mat.Dense) {
	// order points ccw
	// estimate exact homography
	// make chess grid
	return nil, nil, nil
}
