package transform

import (
	"errors"
	"math"

	"github.com/golang/geo/r2"
	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/mat"
)

// GetEssentialMatrixFromFundamental returns the essential matrix from the fundamental matrix and intrinsics parameters.
func GetEssentialMatrixFromFundamental(k1, k2, f *mat.Dense) (*mat.Dense, error) {
	var essMat, tmp mat.Dense
	tmp.Mul(transposeDense(k2), f)
	essMat.Mul(&tmp, k1)
	// enforce rank 2
	mats := performSVD(&essMat)
	S := eye(3)
	S.Set(2, 2, 0)

	essMat.Mul(mats.U, S)
	essMat.Mul(&essMat, mats.VT)
	return &essMat, nil
}

// DecomposeEssentialMatrix decomposes the Essential matrix into 2 possible 3D rotations and a 3D translation.
func DecomposeEssentialMatrix(essMat *mat.Dense) (*mat.Dense, *mat.Dense, *mat.Dense, error) {
	// svd
	mats := performSVD(essMat)
	// check determinant sign of U and V
	if mat.Det(mats.U) < 0 {
		mats.U.Scale(-1, mats.U)
	}
	if mat.Det(mats.VT) < 0 {
		mats.VT.Scale(-1, mats.VT)
	}
	// create matrix W
	W := mat.NewDense(3, 3, nil)
	W.Set(0, 1, 1)
	W.Set(1, 0, -1)
	W.Set(2, 2, 1)
	// compute possible poses
	var R1, R2 mat.Dense
	// UWV^T
	R1.Mul(mats.U, W)
	R1.Mul(&R1, mats.VT)
	U3 := mats.U.ColView(2)
	t := mat.NewDense(3, 1, []float64{U3.AtVec(0), U3.AtVec(1), U3.AtVec(2)})
	// UW^TV^T
	R2.Mul(mats.U, transposeDense(W))
	R2.Mul(&R2, mats.VT)
	return &R1, &R2, t, nil
}

// Convert2DPointsToHomogeneousPoints converts float64 image coordinates to homogeneous float64 coordinates.
func Convert2DPointsToHomogeneousPoints(pts []r2.Point) []r3.Vector {
	ptsHomogeneous := make([]r3.Vector, len(pts))
	for i, pt := range pts {
		ptsHomogeneous[i] = r3.Vector{
			X: pt.X,
			Y: pt.Y,
			Z: 1,
		}
	}
	return ptsHomogeneous
}

// ComputeFundamentalMatrixAllPoints compute the fundamental matrix from all points.
func ComputeFundamentalMatrixAllPoints(pts1, pts2 []r2.Point, normalize bool) (*mat.Dense, error) {
	if len(pts1) != len(pts2) {
		return nil, errors.New("sets of points pts1 and pts2 must have the same number of elements")
	}
	if len(pts1) < 8 {
		return nil, errors.New("sets of points must have at least 8 elements")
	}
	nPoints := len(pts1)

	var points1, points2 []r2.Point
	var T1, T2 *mat.Dense

	// if normalize, normalize points and get transform
	if normalize {
		points1, T1 = normalizePoints(pts1)
		points2, T2 = normalizePoints(pts2)
	} else {
		points1 = make([]r2.Point, nPoints)
		copy(points1, pts1)
		points2 = make([]r2.Point, nPoints)
		copy(points2, pts2)
		T1 = eye(3)
		T2 = eye(3)
	}

	m := mat.NewDense(nPoints, 9, nil)
	for i := range points1 {
		v1 := points1[i]
		v2 := points2[i]
		row := []float64{
			v2.X * v1.X, v2.X * v1.Y, v2.X,
			v2.Y * v1.X, v2.Y * v1.Y, v2.Y,
			v1.X, v1.Y, 1,
		}
		m.SetRow(i, row)
	}

	// perform SVD on m
	mats1 := performSVD(m)
	V := mats1.V
	lastColV := V.ColView(8)

	// reshape into F
	lastColVdata := make([]float64, 9)
	for i := range lastColVdata {
		lastColVdata[i] = lastColV.AtVec(i)
	}
	F := mat.NewDense(3, 3, lastColVdata)

	// enforce rank 2 of F
	mats2 := performSVD(F)
	S := mats2.S
	S.Set(2, 2, 0)

	// get refined F: U@S@V2^T
	Fhat := mat.NewDense(3, 3, nil)
	Fhat.Mul(mats2.U, S)
	F.Mul(Fhat, mats2.VT)
	// rescale F: T2^T @ F @ T1
	T2T := transposeDense(T2)
	F.Mul(T2T, F)
	F.Mul(F, T1)

	F.Scale(1/F.At(2, 2), F)

	return F, nil
}

// helpers
// normalizePoints normalizes points as described in Multiple View Geometry, Alg 11.1.
func normalizePoints(pts []r2.Point) ([]r2.Point, *mat.Dense) {
	nPoints := len(pts)
	// computer centroid of points
	mu := r2.Point{0, 0}

	for _, pt := range pts {
		mu.X += pt.X
		mu.Y += pt.Y
	}
	mu = mu.Mul(1. / float64(nPoints))
	// compute scale factor
	d := 0.0
	for _, pt := range pts {
		x2 := (pt.X - mu.X) * (pt.X - mu.X)
		y2 := (pt.Y - mu.Y) * (pt.Y - mu.Y)
		d += math.Sqrt(x2+y2) / float64(nPoints)
	}
	scale := math.Sqrt(2) / d
	transformData := []float64{
		scale, 0, -scale * mu.X,
		0, scale, -scale * mu.Y,
		0, 0, 1,
	}
	T := mat.NewDense(3, 3, transformData)
	// apply transform to points
	pointsTransformed := make([]r2.Point, nPoints)
	for i := range pointsTransformed {
		pointsTransformed[i] = r2.Point{scale * (pts[i].X - mu.X), scale * (pts[i].Y - mu.Y)}
	}
	return pointsTransformed, T
}

// mat.Dense utils.
func transposeDense(m *mat.Dense) *mat.Dense {
	nRows, nCols := m.Dims()
	m2 := mat.NewDense(nCols, nRows, nil)
	m3 := m.T()
	m2.Copy(m3)
	return m2
}

// eye create an identity matrix of size nxn.
func eye(n int) *mat.Dense {
	if n <= 0 {
		return nil
	}
	m := mat.NewDense(n, n, nil)
	for i := 0; i < n; i++ {
		m.Set(i, i, 1)
	}
	return m
}

// matsSVD stores the matrices from SVD decomposition.
type matsSVD struct {
	U  *mat.Dense
	V  *mat.Dense
	VT *mat.Dense
	S  *mat.Dense
}

// performSVD performs SVD on inputMatrix and returns matrices U, Sigma and V from the decomposition.
func performSVD(inputMatrix *mat.Dense) *matsSVD {
	var svd mat.SVD
	ok := svd.Factorize(inputMatrix, mat.SVDFull)
	if !ok {
		return nil
	}

	u, v, sigma, vt := &mat.Dense{}, &mat.Dense{}, &mat.Dense{}, &mat.Dense{}

	svd.UTo(u)
	svd.VTo(v)
	vt.CloneFrom(v.T())

	singularValues := svd.Values(nil)
	// firstly create diag matrix. Next fill new sigma matrix with zeros
	sigma.CloneFrom(mat.NewDiagDense(len(singularValues), singularValues))

	return &matsSVD{u, v, vt, sigma}
}
