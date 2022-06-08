package transform

import (
	"github.com/golang/geo/r2"
	"github.com/pkg/errors"
	"gonum.org/v1/gonum/mat"
)

// Homography is a 3x3 matrix used to transform a plane from the perspective of a 2D
// camera to the perspective of another 2D camera.
type Homography struct {
	matrix *mat.Dense
}

// NewHomography creates a Homography from a slice of floats.
func NewHomography(vals []float64) (*Homography, error) {
	if len(vals) != 9 {
		return nil, errors.Errorf("input to NewHomography must have length of 9. Has length of %d", len(vals))
	}
	// TODO(bij): add check for mathematical property of homography
	d := mat.NewDense(3, 3, vals)
	return &Homography{d}, nil
}

// At returns the value of the homography at the given index.
func (h *Homography) At(row, col int) float64 {
	return h.matrix.At(row, col)
}

// Apply will transform the given point according to the homography.
func (h *Homography) Apply(pt r2.Point) r2.Point {
	x := h.At(0, 0)*pt.X + h.At(0, 1)*pt.Y + h.At(0, 2)
	y := h.At(1, 0)*pt.X + h.At(1, 1)*pt.Y + h.At(1, 2)
	z := h.At(2, 0)*pt.X + h.At(2, 1)*pt.Y + h.At(2, 2)
	return r2.Point{X: x / z, Y: y / z}
}

// Inverse inverts the homography. If homography went from color -> depth, Inverse makes it point
// from depth -> color.
func (h *Homography) Inverse() (*Homography, error) {
	var hInv mat.Dense
	if err := hInv.Inverse(h.matrix); err != nil {
		return nil, errors.Wrap(err, "homography is not invertible (but homographies should always be invertible?)")
	}
	return &Homography{&hInv}, nil
}

// EstimateExactHomographyFrom8Points computes the exact homography from 2 sets of 4 matching points
// from Multiple View Geometry. Richard Hartley and Andrew Zisserman. Alg 4.1 p91.
func EstimateExactHomographyFrom8Points(s1, s2 []r2.Point, normalize bool) (*Homography, error) {
	if len(s1) != 4 {
		err := errors.New("slice s1 must have 4 points each")
		return nil, err
	}
	if len(s2) != 4 {
		err := errors.New("slice s2 must have 4 points each")
		return nil, err
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
	a := []float64{
		x1, y1, 1, 0, 0, 0, -X1 * x1, -X1 * y1,
		0, 0, 0, x1, y1, 1, -Y1 * x1, -Y1 * y1,
		x2, y2, 1, 0, 0, 0, -X2 * x2, -X2 * y2,
		0, 0, 0, x2, y2, 1, -Y2 * x2, -Y2 * y2,
		x3, y3, 1, 0, 0, 0, -X3 * x3, -X3 * y3,
		0, 0, 0, x3, y3, 1, -Y3 * x3, -Y3 * y3,
		x4, y4, 1, 0, 0, 0, -X4 * x4, -X4 * y4,
		0, 0, 0, x4, y4, 1, -Y4 * x4, -Y4 * y4,
	}

	// Set matrices with data from slice
	bSlice := []float64{X1, Y1, X2, Y2, X3, Y3, X4, Y4}
	b := mat.NewDense(8, 1, bSlice)

	// If matrix A is invertible, get the least square solution
	if A := mat.NewDense(8, 8, a); mat.Det(A) != 0 {
		// x := mat.NewDense(8, 1, nil)
		// Perform an SVD retaining all singular vectors.
		var svd mat.SVD
		ok := svd.Factorize(A, mat.SVDFull)
		if !ok {
			err := errors.New("failed to factorize A")
			return nil, err
		}

		// Determine the rank of the A matrix with a near zero condition threshold.
		const rcond = 1e-15
		rank := svd.Rank(rcond)
		if rank == 0 {
			err := errors.New("zero rank system")
			return nil, err
		}

		// Find a least-squares solution using the determined parts of the system.
		var x mat.Dense
		svd.SolveTo(&x, b, rank)
		// homography is a 3x3 matrix, with last element =1
		s := make([]float64, len(x.RawMatrix().Data)+1)
		for i, v := range x.RawMatrix().Data {
			s[i] = v
		}
		s[len(x.RawMatrix().Data)] = 1.
		outMat := mat.NewDense(3, 3, s)

		if normalize {
			// de-normalize data
			invNorm1 := mat.NewDense(3, 3, nil)
			err := invNorm1.Inverse(norm1)
			if err != nil {
				return nil, err
			}
			invNorm2 := mat.NewDense(3, 3, nil)
			err = invNorm2.Inverse(norm2)
			if err != nil {
				return nil, err
			}
			var m1, m2, m3 mat.Dense
			m1.Mul(norm1, outMat)
			m2.Mul(&m1, invNorm2)
			m3.Scale(1./m2.At(2, 2), &m2)
			return &Homography{&m3}, nil
		}

		return &Homography{outMat}, nil
	}
	// Otherwise, matrix cannot be inverted; return nothing
	err := errors.New("matrix could not be inverted")
	return nil, err
}

// EstimateHomographyRANSAC estimates a homography from matches of 2 sets of
// points with the RANdom SAmple Consensus method
// from Multiple View Geometry. Richard Hartley and Andrew Zisserman. Alg 4.4 p118.
func EstimateHomographyRANSAC(pts1, pts2 []r2.Point, thresh float64, nMaxIteration int) (*Homography, []int, error) {
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
		for !are4PointsNonCollinear(s1[0], s1[1], s1[2], s1[3]) {
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
				d := geometricDistance(s1[k], s2[k], h.matrix)
				if d < 5. {
					currentInliers = append(currentInliers, k)
				}
			}
			// keep current set of inliers and homography if number of inliers is bigger than before
			if len(currentInliers) > len(maxInliers) {
				maxInliers = currentInliers
				finalH = h.matrix
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
	return &Homography{finalH}, maxInliers, nil
}

// EstimateLeastSquaresHomography estimates an homography from 2 sets of corresponding points.
func EstimateLeastSquaresHomography(pts1, pts2 *mat.Dense) (*Homography, error) {
	nPoints1, _ := pts1.Dims()
	if nPoints1 < 4 {
		err := errors.New("pts1 must have at least 4 points")
		return nil, err
	}
	nPoints2, _ := pts2.Dims()
	if nPoints2 < 4 {
		err := errors.New("pts1 must have at least 4 points")
		return nil, err
	}

	if nPoints1 != nPoints2 {
		err := errors.New("pts1 and pts2 must have the same number of points")
		return nil, err
	}
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
		err := errors.New("failed to factorize A")
		return nil, err
	}
	// Determine the rank of the A matrix with a near zero condition threshold.
	const rcond = 1e-15
	if svd.Rank(rcond) == 0 {
		err := errors.New("zero rank system")
		return nil, err
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
		return nil, err
	}
	invNorm2 := mat.NewDense(3, 3, nil)
	err = invNorm2.Inverse(normalizationMat2)
	if err != nil {
		return nil, err
	}
	m1.Mul(normalizationMat1, H)
	m2.Mul(&m1, invNorm2)
	m3.Scale(1./m2.At(2, 2), &m2)

	return &Homography{&m3}, nil
}

// ApplyHomography applies a homography on a slice of r2.Vec.
func ApplyHomography(h *Homography, pts []r2.Point) []r2.Point {
	outPoints := make([]r2.Point, len(pts))
	for i, pt := range pts {
		outPoints[i] = h.Apply(pt)
	}
	return outPoints
}
