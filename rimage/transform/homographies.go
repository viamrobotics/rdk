package transform

import (
	"math"

	"github.com/golang/geo/r2"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat"

	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/utils"
)

// ComputeNormalizationMatFromSliceVecs computes the normalization matrix from a slice of vectors
// from Multiple View Geometry. Richard Hartley and Andrew Zisserman. Alg 4.2 p109.
func ComputeNormalizationMatFromSliceVecs(pts []r2.Point) *mat.Dense {
	out := mat.NewDense(3, 3, nil)
	xs, ys := rimage.SliceVecsToXsYs(pts)
	avgX := stat.Mean(xs, nil)
	avgY := stat.Mean(ys, nil)
	diffX := make([]float64, len(xs))
	copy(diffX, xs)
	floats.AddConst(avgX, diffX)
	diffY := make([]float64, len(ys))
	copy(diffY, ys)
	floats.AddConst(avgY, diffY)
	norms := make([]float64, len(xs))

	for i := 0; i < len(pts); i++ {
		norms[i] = math.Sqrt(diffX[i]*diffX[i] + diffY[i]*diffY[i])
	}
	scaleFactor := math.Sqrt(2) / stat.Mean(norms, nil)
	out.Set(0, 0, scaleFactor)
	out.Set(0, 2, -scaleFactor*avgX)
	out.Set(1, 1, scaleFactor)
	out.Set(1, 2, -scaleFactor*avgY)
	out.Set(2, 2, 1)
	return out
}

// MeanVarianceCol is a helper function to compute the normalization matrix for least squares homography estimation
// It estimates the mean and variance of a column in a *mat.Dense.
func MeanVarianceCol(m mat.Matrix, j int) (float64, float64) {
	r, _ := m.Dims()
	col := make([]float64, r)

	mat.Col(col, j, m)
	mean := stat.Mean(col, nil)
	std := stat.StdDev(col, nil)

	return mean, std
}

// getNormalizationMatrix gets the matrix to center the points on (0,0).
func getNormalizationMatrix(pts *mat.Dense) *mat.Dense {
	avgX, stdX := MeanVarianceCol(pts, 0)
	avgY, stdY := MeanVarianceCol(pts, 1)

	sX := math.Sqrt(2. / stdX)
	sY := math.Sqrt(2. / stdY)
	data := []float64{sX, 0., -sX * avgX, 0., sY, -sY * avgY, 0, 0, 1}
	outMat := mat.NewDense(3, 3, data)
	//	[
	//	[s_x, 0, -s_x * avg_x],
	//	[0, s_y, -s_y * avg_y],
	//	[0, 0, 1]
	//]
	return outMat
}

// SelectFourPointPairs randomly selects 4 pairs of points in two point slices of the same length.
func SelectFourPointPairs(p1, p2 []r2.Point) ([]r2.Point, []r2.Point, error) {
	if len(p1) != len(p2) {
		err := errors.New("p1 and p2 should have the same length")
		return nil, nil, err
	}
	indices, err := utils.SelectNIndicesWithoutReplacement(4, len(p1))
	if err != nil {
		return nil, nil, err
	}

	// create 2 4-point-slices of selected indices
	s1 := make([]r2.Point, 0, 4)
	s2 := make([]r2.Point, 0, 4)
	for _, idx := range indices {
		s1 = append(s1, p1[idx])
		s2 = append(s2, p2[idx])
	}
	return s1, s2, nil
}

// geometricDistance computes the distance of point1 transformed by the homography h 1->2 to point 2.
func geometricDistance(p1, p2 r2.Point, h mat.Matrix) float64 {
	pt1 := mat.NewDense(3, 1, []float64{p1.X, p1.Y, 1.0})
	pt2 := mat.NewDense(3, 1, []float64{p2.X, p2.Y, 1.0})
	pt2Tilde := mat.NewDense(3, 1, nil)
	pt2Tilde.Mul(h, pt1)
	pt2Tilde.Scale(1./pt2Tilde.At(2, 0), pt2Tilde)

	p := r3.Vector{X: pt2.At(0, 0), Y: pt2.At(1, 0), Z: pt2.At(2, 0)}
	q := r3.Vector{X: pt2Tilde.At(0, 0), Y: pt2Tilde.At(1, 0), Z: pt2Tilde.At(2, 0)}
	errVec := p.Sub(q)

	return errVec.Norm()
}

// are4PointsNonCollinear returns true if 4 points are not collinear 3 by 3.
func are4PointsNonCollinear(p1, p2, p3, p4 r2.Point) bool {
	return !rimage.AreCollinear(p1, p2, p3, 0.01) && !rimage.AreCollinear(p2, p3, p4, 0.01) &&
		!rimage.AreCollinear(p1, p3, p4, 0.01) && !rimage.AreCollinear(p1, p2, p4, 0.01)
}

// ApplyNormalizationMat applies a normalization matrix to a slice of r2.Point.
func ApplyNormalizationMat(h *mat.Dense, pts []r2.Point) []r2.Point {
	outPoints := make([]r2.Point, len(pts))
	for i, pt := range pts {
		x := h.At(0, 0)*pt.X + h.At(0, 1)*pt.Y + h.At(0, 2)
		y := h.At(1, 0)*pt.X + h.At(1, 1)*pt.Y + h.At(1, 2)
		// z := h.At(2, 0)*pt.X + h.At(2, 1)*pt.Y + h.At(2, 2)
		outPoints[i] = r2.Point{X: x, Y: y}
	}
	return outPoints
}
