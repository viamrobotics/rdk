package transform

import (
	"math"
	"testing"

	"github.com/golang/geo/r2"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"

	"go.viam.com/test"

	"go.viam.com/core/utils"
)

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

// CreateRotationMatrix creates a 2x2 rotation matrix with given angle in radians
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

func TestEstimateHomographyFrom8Points(t *testing.T) {
	h := []float64{7.82502613e-01, -9.71005496e-02, 9.73247024e+00,
		9.71005496e-02, 7.82502613e-01, 7.26666735e+00,
		8.96533720e-04, -9.39239890e-04, 1.00000000e+00,
	}
	homography := mat.NewDense(3, 3, h)
	pts1 := []r2.Point{{0., 0.}, {25., 0.}, {0., 25.}, {25., 25.}}
	pts2 := []r2.Point{{9.7324705, 7.2666674}, {28.65283, 9.481666}, {7.4806085, 27.474358}, {26.896238, 29.288015}}
	H, _ := EstimateExactHomographyFrom8Points(pts1, pts2, false)
	test.That(t, mat.EqualApprox(H, homography, 0.00001), test.ShouldBeTrue)
}

func repeatedSlice(value float64, n int) []float64 {
	arr := make([]float64, n)
	for i := 0; i < n; i++ {
		arr[i] = value
	}
	return arr
}

func TestEstimateLeastSquaresHomography(t *testing.T) {
	// create a rotation as a simple homography
	h := CreateRotationMatrix(math.Pi / 8.)
	// create grid of points
	x := make([]float64, 9)
	floats.Span(x, 0, 200)
	pts1Slice := utils.Single(2, x)
	pts1 := mat.NewDense(len(pts1Slice), len(pts1Slice[0]), nil)
	for i, pt := range pts1Slice {
		pts1.Set(i, 0, pt[0])
		pts1.Set(i, 1, pt[1])
	}
	// rotate point with H
	r, c := pts1.Dims()
	pts2 := mat.NewDense(c, r, nil)
	pts2.Mul(h, pts1.T())
	b := pts2.T()
	pts3 := mat.DenseCopyOf(b)
	// estimate homography with least squares method
	estH, _ := EstimateLeastSquaresHomography(pts1, pts3)
	// check that 2x2 block are close to each other
	test.That(t, estH.At(0, 0), test.ShouldAlmostEqual, h.At(0, 0), 0.001)
	test.That(t, estH.At(0, 1), test.ShouldAlmostEqual, h.At(0, 1), 0.001)
	test.That(t, estH.At(1, 0), test.ShouldAlmostEqual, h.At(1, 0), 0.001)
	test.That(t, estH.At(1, 1), test.ShouldAlmostEqual, h.At(1, 1), 0.001)

	// test translation (2,2)
	pts1Copy := mat.DenseCopyOf(pts1)

	pts4Data := pts1Copy.RawMatrix().Data
	vals := repeatedSlice(2., r*c)
	floats.Add(pts4Data, vals)
	pts4 := mat.NewDense(r, c, pts4Data)
	estH2, _ := EstimateLeastSquaresHomography(pts1, pts4)

	// check that 2x2 block are close to identity
	test.That(t, estH2.At(0, 0), test.ShouldAlmostEqual, 1.0, 0.001)
	test.That(t, estH2.At(0, 1), test.ShouldBeLessThanOrEqualTo, 0.001)
	test.That(t, estH2.At(1, 0), test.ShouldBeLessThanOrEqualTo, 0.001)
	test.That(t, estH2.At(1, 1), test.ShouldAlmostEqual, 1.0, 0.001)
	// check that translation terms are close to sqrt(2) / 4. (from homography decomposition formula)
	test.That(t, estH2.At(0, 2), test.ShouldAlmostEqual, 0.3535533905932738, 0.01)
	test.That(t, estH2.At(1, 2), test.ShouldAlmostEqual, 0.3535533905932738, 0.01)
	// test that translation terms are equal tx = ty
	test.That(t, estH2.At(1, 2), test.ShouldAlmostEqual, estH2.At(0, 2), 0.01)
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

func TestEstimateHomographyRANSAC(t *testing.T) {
	dim := 2
	grid := make([]float64, 8)
	floats.Span(grid, 0, 7)
	pts := Single(dim, grid)
	pts1 := SlicesXsYsToPoints(pts)

	pts2 := make([]r2.Point, len(pts1))
	for i := 0; i < len(pts1); i++ {
		pts2[i] = r2.Point{pts1[i].X + 2, pts1[i].Y + 3}
	}
	h, _, _ := EstimateHomographyRANSAC(pts1, pts2, 0.5, 2000)
	// homography should be close to
	// [[1 0 2],
	//  [0 1 3],
	//  [0 0 1]]
	test.That(t, h.At(0, 0), test.ShouldAlmostEqual, 1, 0.001)
	test.That(t, h.At(1, 1), test.ShouldAlmostEqual, 1, 0.001)
	test.That(t, h.At(0, 1), test.ShouldAlmostEqual, 0, 0.001)
	test.That(t, h.At(1, 0), test.ShouldAlmostEqual, 0, 0.001)
	test.That(t, h.At(0, 2), test.ShouldAlmostEqual, 2, 0.001)
	test.That(t, h.At(1, 2), test.ShouldAlmostEqual, 3, 0.001)
	test.That(t, h.At(2, 0), test.ShouldAlmostEqual, 0, 0.001)
	test.That(t, h.At(2, 1), test.ShouldAlmostEqual, 0, 0.001)
	test.That(t, h.At(2, 2), test.ShouldAlmostEqual, 1, 0.001)
}
