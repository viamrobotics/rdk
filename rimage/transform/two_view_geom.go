package transform

import (
	"errors"
	"fmt"
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
	fmt.Println("E1 : ", mat.Formatted(&essMat))
	// enforce rank 2
	U, _, VT, _ := performSVD(&essMat)
	S := eye(3)
	S.Set(2, 2, 0)

	essMat.Mul(U, S)
	essMat.Mul(&essMat, VT)
	return &essMat, nil
}

// DecomposeEssentialMatrix decomposes the Essential matrix into 2 possible 3D rotations and a 3D translation
func DecomposeEssentialMatrix(essMat *mat.Dense) (*mat.Dense, *mat.Dense, *mat.Dense, error) {
	// svd
	U, _, VT, _ := performSVD(essMat)
	// check determinant sign of U and V
	if mat.Det(U) < 0 {
		U.Scale(-1, U)
	}
	if mat.Det(VT) < 0 {
		VT.Scale(-1, VT)
	}
	// create matrix W
	W := mat.NewDense(3, 3, nil)
	W.Set(0, 1, 1)
	W.Set(1, 0, -1)
	W.Set(2, 2, 1)
	// compute possible poses
	var R1, R2 mat.Dense
	// UWV^T
	R1.Mul(U, W)
	R1.Mul(&R1, VT)
	U3 := U.ColView(2)
	t := mat.NewDense(3, 1, []float64{U3.AtVec(0), U3.AtVec(1), U3.AtVec(2)})
	// UW^TV^T
	R2.Mul(U, transposeDense(W))
	R2.Mul(&R2, VT)
	return &R1, &R2, t, nil
}

// Convert2DPointsToHomogeneousPoints converts float64 image coordinates to homogeneous float64 coordinates
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

// GetFundamentalMatrixInliers returns the points in pts1 and pts2 that F accurately transforms
func GetFundamentalMatrixInliers(pts1, pts2 []r2.Point, F *mat.Dense, threshold float64) ([]r2.Point, []r2.Point, []int) {
	indices := make([]int, 0, len(pts1))
	inliers1 := make([]r2.Point, 0, len(pts1))
	inliers2 := make([]r2.Point, 0, len(pts1))
	for i := range pts1 {

		d := evaluateFundamentalMatrix(pts1[i], pts2[i], F)
		//fmt.Println(d)
		if d < threshold {
			indices = append(indices, i)
			inliers1 = append(inliers1, pts1[i])
			inliers2 = append(inliers2, pts2[i])
		}
	}
	return inliers1, inliers2, indices
}

// EightPointAlgorithm performs the 8-point algorithm
func EightPointAlgorithm(pts1, pts2 []r2.Point, normalize bool) (*mat.Dense, error) {
	if len(pts1) != 8 {
		return nil, errors.New("sets of points pts1 must have 8 elements")
	}
	if len(pts2) != 8 {
		return nil, errors.New("sets of points pts2 must have 8 elements")
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
	//M := make([]float64, 9*9)
	//A := mat.NewSymDense(9, nil)
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
	//_, v, _, _ := performSVD(m)
	//lastColV := v.ColView(7)
	lastColV := solveLeastSquaresSVD(m)

	// reshape into F
	//lastColVdata := make([]float64, 9)
	lastColVdata := lastColV.RawMatrix().Data
	//for i := range lastColVdata {
	//	lastColVdata[i] = lastColV.At(i, 0)
	//}
	F := mat.NewDense(3, 3, lastColVdata)

	// enforce rank 2 of F
	U, _, V2T, S := performSVD(F)

	// get refined F
	F.Mul(U, S)
	F.Mul(F, V2T)
	// rescale F: T2^T @ F @ T1
	F.Mul(F, T1)
	T2T := transposeDense(T2)
	F.Mul(T2T, F)
	F.Scale(1/F.At(2, 2), F)

	return F, nil
}

// ComputeFundamentalMatrixAllPoints compute the fundamental matrix from all points
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
	lastColV := solveLeastSquaresSVD(m)

	// reshape into F
	lastColVdata := lastColV.RawMatrix().Data

	F := mat.NewDense(3, 3, lastColVdata)

	// enforce rank 2 of F
	U, _, V2T, S := performSVD(F)
	S.Set(2, 2, 0)

	// get refined F: U@S@V2^T
	Fhat := mat.NewDense(3, 3, nil)
	Fhat.Mul(U, S)
	F.Mul(Fhat, V2T)
	// rescale F: T2^T @ F @ T1
	T2T := transposeDense(T2)
	F.Mul(T2T, F)
	F.Mul(F, T1)

	F.Scale(1/F.At(2, 2), F)

	return F, nil
}

// evaluateFundamentalMatrix computes the error made by the estimated fundamental matrix between two points
// error = (x2^T @ F @ x1)^2 / (norm(F@x1)^2 + norm(F^T@x2)^2)
func evaluateFundamentalMatrix(p1, p2 r2.Point, F *mat.Dense) float64 {
	// compute error numerator
	var res1, res2 mat.Dense
	v1 := mat.NewDense(3, 1, []float64{p1.X, p1.Y, 1})
	v2 := mat.NewDense(1, 3, []float64{p2.X, p2.Y, 1})
	res1.Mul(F, v1)
	res2.Mul(v2, &res1)
	num := res2.At(0, 0) * res2.At(0, 0)
	// compute error denominator
	u2 := mat.NewDense(3, 1, []float64{p2.X, p2.Y, 1})
	FT := transposeDense(F)
	var fx1, ftx2 mat.Dense
	fx1.Mul(F, v1)
	ftx2.Mul(FT, u2)
	denom := mat.Norm(&fx1, 2)*mat.Norm(&fx1, 2) + mat.Norm(&ftx2, 2)*mat.Norm(&ftx2, 2)
	return num / denom
}

// helpers
// normalizePoints normalizes points as described in Multiple View Geometry, Alg 11.1
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

// mat.Dense utils
func transposeDense(m *mat.Dense) *mat.Dense {
	nRows, nCols := m.Dims()
	m2 := mat.NewDense(nCols, nRows, nil)
	m3 := m.T()
	m2.Copy(m3)
	return m2
}

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

// performSVD performs SVD on inputMatrix and returns matrices U, Sigma and V from the decomposition
func performSVD(inputMatrix *mat.Dense) (*mat.Dense, *mat.Dense, *mat.Dense, *mat.Dense) {
	var svd mat.SVD
	ok := svd.Factorize(inputMatrix, mat.SVDFull)
	if !ok {
		return nil, nil, nil, nil
	}

	u, v, sigma, vt := &mat.Dense{}, &mat.Dense{}, &mat.Dense{}, &mat.Dense{}

	svd.UTo(u)
	svd.VTo(v)
	vt.CloneFrom(v.T())

	singularValues := svd.Values(nil)
	// firstly create diag matrix. Next fill new sigma matrix with zeros
	sigma.CloneFrom(mat.NewDiagDense(len(singularValues), singularValues))

	return u, v, vt, sigma
}

func solveLeastSquaresSVD(inputMatrix *mat.Dense) *mat.Dense {
	var svd mat.SVD
	ok := svd.Factorize(inputMatrix, mat.SVDFull)
	if !ok {
		return nil
	}
	// Determine the rank of the A matrix with a near zero condition threshold.
	const rcond = 1e-15
	rank := svd.Rank(rcond)
	if rank == 0 {
		return nil
	}
	nRows, _ := inputMatrix.Dims()

	b := mat.NewDense(nRows, 1, nil)

	// Find a least-squares solution using the determined parts of the system.
	var x mat.Dense
	svd.SolveTo(&x, b, rank)
	return &x
}
