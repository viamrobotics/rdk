package transform

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.viam.com/core/utils"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/spatial/r2"
)

// CreateRotationMatrix creates a 2x2 rotation matrix with given angle in radians
func CreateRotationMatrix(angle float64) *mat.Dense {
	r := mat.NewDense(2, 2, nil)
	r.Set(0, 0, math.Cos(angle))
	r.Set(0, 1, -math.Sin(angle))
	r.Set(1, 0, math.Sin(angle))
	r.Set(1, 1, math.Cos(angle))

	return r
}

func TestEstimateHomographyFrom8Points(t *testing.T) {
	h := []float64{7.82502613e-01, -9.71005496e-02, 9.73247024e+00,
		9.71005496e-02, 7.82502613e-01, 7.26666735e+00,
		8.96533720e-04, -9.39239890e-04, 1.00000000e+00,
	}
	homography := mat.NewDense(3, 3, h)
	pts1 := []r2.Vec{r2.Vec{0., 0.}, r2.Vec{25., 0.}, r2.Vec{0., 25.}, r2.Vec{25., 25.}}
	pts2 := []r2.Vec{r2.Vec{9.7324705, 7.2666674}, r2.Vec{28.65283, 9.481666}, r2.Vec{7.4806085, 27.474358}, r2.Vec{26.896238, 29.288015}}
	H, _ := EstimateExactHomographyFrom8Points(pts1, pts2)
	assert.True(t, mat.EqualApprox(H, homography, 0.00001))
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
	pts1 := utils.Single(2, x)
	// rotate point with H
	r, c := pts1.Dims()
	pts2 := mat.NewDense(c, r, nil)
	pts2.Mul(h, pts1.T())
	b := pts2.T()
	pts3 := mat.DenseCopyOf(b)
	// estimate homography with least squares method
	estH, _ := EstimateLeastSquaresHomography(pts1, pts3)
	// check that 2x2 block are close to each other
	assert.InEpsilon(t, estH.At(0, 0), h.At(0, 0), 0.001)
	assert.InEpsilon(t, estH.At(0, 1), h.At(0, 1), 0.001)
	assert.InEpsilon(t, estH.At(1, 0), h.At(1, 0), 0.001)
	assert.InEpsilon(t, estH.At(1, 1), h.At(1, 1), 0.001)

	// test translation (2,2)
	pts1Copy := mat.DenseCopyOf(pts1)

	pts4Data := pts1Copy.RawMatrix().Data
	vals := repeatedSlice(2., r*c)
	floats.Add(pts4Data, vals)
	pts4 := mat.NewDense(r, c, pts4Data)
	estH2, _ := EstimateLeastSquaresHomography(pts1, pts4)

	// check that 2x2 block are close to identity
	assert.InEpsilon(t, estH2.At(0, 0), 1., 0.001)
	assert.LessOrEqual(t, math.Abs(estH2.At(0, 1)), 0.001)
	assert.LessOrEqual(t, math.Abs(estH2.At(1, 0)), 0.001)
	assert.InEpsilon(t, estH2.At(1, 1), 1., 0.001)
	// check that translation terms are close to sqrt(2) / 4. (from homography decomposition formula)
	assert.InEpsilon(t, estH2.At(0, 2), 0.3535533905932738, 0.01)
	assert.InEpsilon(t, estH2.At(1, 2), 0.3535533905932738, 0.01)
	// test that translation terms are equal tx = ty
	assert.InEpsilon(t, estH2.At(1, 2), estH2.At(0, 2), 0.01)

}

func TestGeometricDistance(t *testing.T) {
	pt1 := r2.Vec{0, 0}
	pt2 := r2.Vec{1, 0}
	// h = Id, distance should be 1
	h1 := mat.NewDense(3, 3, nil)
	h1.Set(0, 0, 1)
	h1.Set(1, 1, 1)
	h1.Set(2, 2, 1)
	d1 := geometricDistance(pt1, pt2, h1)
	assert.Equal(t, d1, 1.0)
	// rotation -pi/2
	h2 := mat.NewDense(3, 3, nil)
	h2.Set(0, 1, 1)
	h2.Set(1, 0, -1)
	h2.Set(2, 2, 1)
	d2 := geometricDistance(pt1, pt2, h2)
	assert.Equal(t, d2, 1.0)
	// rotation -pi/2
	h3 := mat.NewDense(3, 3, nil)
	h3.Set(0, 1, 1)
	h3.Set(1, 0, -1)
	h3.Set(2, 2, 1)
	pt3 := r2.Vec{1, 0}
	d3 := geometricDistance(pt3, pt2, h3)
	assert.Equal(t, d3, 1.4142135623730951)
	//fmt.Println(d3)
}

func TestEstimateHomographyRANSAC(t *testing.T) {
	dim := 2
	grid := make([]float64, 8)
	floats.Span(grid, 0, 7)
	pts1 := utils.Single(dim, grid)

	r, c := pts1.Dims()
	fmt.Println(r, c)
	fmt.Println(pts1)
	//pts2 := mat.NewDense(r,c, nil)
	//for i := 0; i<r;i++{..
	//
	//		pts2.Set(i, 0, pts1.At(i,0)+2)
	//		pts2.Set(i, 1, pts1.At(i,0)+3)
	//
	//}
	//h, _, _ := EstimateHomographyRANSAC()
}
