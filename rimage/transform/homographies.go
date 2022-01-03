package transform

import (
	"log"
	"math"

	"github.com/gonum/floats"

	"github.com/go-errors/errors"

	"github.com/golang/geo/r2"
	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat"

	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/utils"
)

// ComputeNormalizationMatFromSliceVecs computes the normalization matrix from a slice of vectors
// from Multiple View Geometry. Richard Hartley and Andrew Zisserman. Alg 4.2 p109
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

// EstimateExactHomographyFrom8Points computes the exact homography from 2 sets of 4 matching points
// from Multiple View Geometry. Richard Hartley and Andrew Zisserman. Alg 4.1 p91
func EstimateExactHomographyFrom8Points(s1, s2 []r2.Point, normalize bool) (*mat.Dense, error) {
	if len(s1) != 4 {
		panic("slice s1 must have 4 points each")
	}
	if len(s2) != 4 {
		panic("slice s2 must have 4 points each")
	}
	st1 := s1
	st2 := s2
	norm1 := ComputeNormalizationMatFromSliceVecs(s1)
	norm2 := ComputeNormalizationMatFromSliceVecs(s1)
	if normalize {

		st1 = ApplyNormalizationMat(norm1, s1)
		st2 = ApplyNormalizationMat(norm2, s2)
	}

	x1 := st1[0].X
	y1 := st1[0].Y
	X1 := st2[0].X
	Y1 := st2[0].Y

	x2 := st1[1].X
	y2 := st1[1].Y
	X2 := st2[1].X
	Y2 := st2[1].Y

	x3 := st1[2].X
	y3 := st1[2].Y
	X3 := st2[2].X
	Y3 := st2[2].Y

	x4 := st1[3].X
	y4 := st1[3].Y
	X4 := st2[3].X
	Y4 := st2[3].Y
	// create homography system
	a := []float64{x1, y1, 1, 0, 0, 0, -X1 * x1, -X1 * y1,
		0, 0, 0, x1, y1, 1, -Y1 * x1, -Y1 * y1,
		x2, y2, 1, 0, 0, 0, -X2 * x2, -X2 * y2,
		0, 0, 0, x2, y2, 1, -Y2 * x2, -Y2 * y2,
		x3, y3, 1, 0, 0, 0, -X3 * x3, -X3 * y3,
		0, 0, 0, x3, y3, 1, -Y3 * x3, -Y3 * y3,
		x4, y4, 1, 0, 0, 0, -X4 * x4, -X4 * y4,
		0, 0, 0, x4, y4, 1, -Y4 * x4, -Y4 * y4,
	}

	// Set matrices with data from slice
	A := mat.NewDense(8, 8, a)
	bSlice := []float64{X1, Y1, X2, Y2, X3, Y3, X4, Y4}
	b := mat.NewDense(8, 1, bSlice)

	// If matrix A is invertible, get the least square solution
	if mat.Det(A) != 0 {
		//x := mat.NewDense(8, 1, nil)
		// Perform an SVD retaining all singular vectors.
		var svd mat.SVD
		ok := svd.Factorize(A, mat.SVDFull)
		if !ok {
			log.Fatal("failed to factorize A")
		}

		// Determine the rank of the A matrix with a near zero condition threshold.
		const rcond = 1e-15
		rank := svd.Rank(rcond)
		if rank == 0 {
			log.Fatal("zero rank system")
			return nil, nil
		}

		// Find a least-squares solution using the determined parts of the system.
		var x mat.Dense
		svd.SolveTo(&x, b, rank)
		// homography is a 3x3 matrix, with last element =1
		s := append(x.RawMatrix().Data, 1.)
		outMat := mat.NewDense(3, 3, s)

		if normalize {
			// de-normalize data
			invNorm1 := mat.NewDense(3, 3, nil)
			err := invNorm1.Inverse(norm1)
			if err != nil {
				panic(err)
			}
			invNorm2 := mat.NewDense(3, 3, nil)
			err = invNorm2.Inverse(norm2)
			if err != nil {
				panic(err)
			}
			var m1, m2, m3 mat.Dense
			m1.Mul(norm1, outMat)
			m2.Mul(&m1, invNorm2)
			m3.Scale(1./m2.At(2, 2), &m2)
			return &m3, nil
		}

		return outMat, nil
	}
	// Otherwise, matrix cannot be inverted; return nothing
	return nil, nil
}

// MeanVarianceCol is a helper function to compute the normalization matrix for least squares homography estimation
// It estimates the mean and variance of a column in a *mat.Dense
func MeanVarianceCol(m mat.Matrix, j int) (float64, float64) {
	r, _ := m.Dims()
	col := make([]float64, r)

	mat.Col(col, j, m)
	mean := stat.Mean(col, nil)
	std := stat.StdDev(col, nil)

	return mean, std
}

// getNormalizationMatrix gets the matrix to center the points on (0,0)
func getNormalizationMatrix(pts rimage.TransformationMatrix) *mat.Dense {

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

// EstimateLeastSquaresHomography estimates an homography from 2 sets of corresponding points
func EstimateLeastSquaresHomography(pts1, pts2 rimage.TransformationMatrix) (*mat.Dense, error) {
	normalizationMat1 := getNormalizationMatrix(pts1)
	normalizationMat2 := getNormalizationMatrix(pts2)
	M := make([]float64, 0)
	nRows, nCols := pts1.Dims()
	for i := 0; i < nRows; i++ {
		p1 := mat.NewDense(nCols+1, 1, []float64{pts1.At(i, 0), pts1.At(i, 1), 1.})
		p2 := mat.NewDense(nCols+1, 1, []float64{pts2.At(i, 0), pts2.At(i, 1), 1.})
		p1.Mul(normalizationMat1, p1)
		p2.Mul(normalizationMat2, p2)
		currentSlice1 := []float64{
			p1.At(0, 0), p1.At(1, 0), 1,
			0., 0., 0.,
			-p1.At(0, 0) * p2.At(0, 0), -p1.At(1, 0) * p2.At(0, 0), -p2.At(0, 0),
		}
		M = append(M, currentSlice1...)
		currentSlice2 := []float64{
			0., 0., 0.,
			p1.At(0, 0), p1.At(1, 0), 1,

			-p1.At(0, 0) * p2.At(1, 0), -p1.At(1, 0) * p2.At(1, 0), -p2.At(1, 0),
		}
		M = append(M, currentSlice2...)

	}
	m := mat.NewDense(2*nRows, 9, M)
	var svd mat.SVD
	ok := svd.Factorize(m, mat.SVDFull)
	if !ok {
		log.Fatal("failed to factorize A")
	}
	// Determine the rank of the A matrix with a near zero condition threshold.
	const rcond = 1e-15
	rank := svd.Rank(rcond)
	if rank == 0 {
		log.Fatal("zero rank system")
	}
	var V, m1, m2, m3 mat.Dense
	svd.VTo(&V)
	L := V.ColView(8)
	var l mat.VecDense
	l.CloneFromVec(L)
	H := mat.NewDense(3, 3, l.RawVector().Data)
	invNorm1 := mat.NewDense(3, 3, nil)
	err := invNorm1.Inverse(normalizationMat1)
	if err != nil {
		panic(err)
	}
	invNorm2 := mat.NewDense(3, 3, nil)
	err = invNorm2.Inverse(normalizationMat2)
	if err != nil {
		panic(err)
	}
	m1.Mul(normalizationMat1, H)
	m2.Mul(&m1, invNorm2)
	m3.Scale(1./m2.At(2, 2), &m2)

	return &m3, nil

}

// SelectFourPointPairs randomly selects 4 pairs of points in two point slices of the same length
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

// geometricDistance computes the distance of point1 transformed by the homography h 1->2 to point 2
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

// Are4PointsNonCollinear returns true if 4 points are not collinear 3 by 3
func Are4PointsNonCollinear(p1, p2, p3, p4 r2.Point) bool {
	return !rimage.AreCollinear(p1, p2, p3, 0.01) && !rimage.AreCollinear(p2, p3, p4, 0.01) &&
		!rimage.AreCollinear(p1, p3, p4, 0.01) && !rimage.AreCollinear(p1, p2, p4, 0.01)
}

// EstimateHomographyRANSAC estimates a homography from matches of 2 sets of
// points with the RANdom SAmple Consensus method
// from Multiple View Geometry. Richard Hartley and Andrew Zisserman. Alg 4.4 p118
func EstimateHomographyRANSAC(pts1, pts2 []r2.Point, thresh float64, nMaxIteration int) (*mat.Dense, []int, error) {
	// test len(pts1)==len(pts2)
	// test len(pts1) > 4
	maxInliers := make([]int, 0, len(pts1))
	finalH := mat.NewDense(3, 3, nil)
	// RANSAC iterations
	for i := 0; i < nMaxIteration; i++ {
		// select 4 random matches
		s1, s2, err := SelectFourPointPairs(pts1, pts2)
		if err != nil {
			return nil, nil, err
		}
		for !Are4PointsNonCollinear(s1[0], s1[1], s1[2], s1[3]) {
			s1, s2, err = SelectFourPointPairs(pts1, pts2)
			if err != nil {
				return nil, nil, err
			}
		}

		// estimate exact homography from these 4 matches
		h, err := EstimateExactHomographyFrom8Points(s1, s2, false)
		if err != nil {
			return nil, nil, err
		}
		if h != nil {
			// compute inliers
			currentInliers := make([]int, 0, len(pts1))
			for k := 0; k < 4; k++ {
				d := geometricDistance(s1[k], s2[k], h)
				if d < 5. {
					currentInliers = append(currentInliers, k)
				}
			}
			// keep current set of inliers and homography if number of inliers is bigger than before
			if len(currentInliers) > len(maxInliers) {
				maxInliers = currentInliers
				finalH = h
			}
			// if the current homography has a number of inliers that exceeds a certain ratio of points in matches,
			// iterations can be stopped - the homography estimation is accurate enough
			nReasonable := int(float64(len(pts1)) * thresh)
			if len(currentInliers) > nReasonable {
				break
			}
		} else {
			continue
		}
	}
	return finalH, maxInliers, nil
}

// ApplyHomography applies a homography on a slice of r2.Vec
func ApplyHomography(H rimage.Matrix, pts []r2.Point) []r2.Point {
	outPoints := make([]r2.Point, len(pts))
	for i, pt := range pts {
		x := H.At(0, 0)*pt.X + H.At(0, 1)*pt.Y + H.At(0, 2)
		y := H.At(1, 0)*pt.X + H.At(1, 1)*pt.Y + H.At(1, 2)
		z := H.At(2, 0)*pt.X + H.At(2, 1)*pt.Y + H.At(2, 2)
		outPoints[i] = r2.Point{X: x / z, Y: y / z}
	}
	return outPoints
}

// ApplyNormalizationMat applies a normalization matrix to a slice of r2.Point
func ApplyNormalizationMat(H rimage.Matrix, pts []r2.Point) []r2.Point {
	outPoints := make([]r2.Point, len(pts))
	for i, pt := range pts {
		x := H.At(0, 0)*pt.X + H.At(0, 1)*pt.Y + H.At(0, 2)
		y := H.At(1, 0)*pt.X + H.At(1, 1)*pt.Y + H.At(1, 2)
		//z := H.At(2, 0)*pt.X + H.At(2, 1)*pt.Y + H.At(2, 2)
		outPoints[i] = r2.Point{X: x, Y: y}
	}
	return outPoints
}
