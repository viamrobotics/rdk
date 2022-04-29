/*
Package calibrate uses Zhang's method to estimate intrinsic parameters of a camera.
------------------------------------------------- PLEASE READ ------------------------------------------------------------

This example code demonstrates how to use the calibrate package to estimate intrinsic parameters using Zhang's method
as defined in the paper "A Flexible New Technique for Camera Calibration (Zhang, 1998)." The test data includes three
images corresponding to separate views of the same chessboard. To calibrate a camera, one must have at least four unique
points (on the same plane) per image in at least 3 separate images.

Currently, the parameter estimation varies significantly from that of a Python OpenCV implementation (using calibrateCamera()).
The source of the discretion is unclear.  Running this code on the test data returns the parameters:
   v0: -159.7855277608248,  lam: -0.27235145392528703,  alpha: 1707.468648921812
   beta: 61.57166329492043,  gamma: -813.9634324862932,   u0: -601.9565547382913

Compared to OpenCV which returns:
   vo ~= 381.36 ,  alpha ~= 943.35,   beta ~= 968.31
   gamma = 0,    u0 ~= 622.62

Finally, here are some resources that you may find useful.
	Python implementation of Zhang's: https://kushalvyas.github.io/calib.html
	Youtube lecture on Zhang's: https://www.youtube.com/watch?v=-9He7Nu3u8s
	Great lecture notes on Zhang's: https://engineering.purdue.edu/kak/computervision/ECE661Folder/Lecture19.pdf
	Book on theory: https://people.cs.rutgers.edu/~elgammal/classes/cs534/lectures/CameraCalibration-book-chapter.pdf

*/
package calibrate

import (
	"errors"
	"math"

	"github.com/maorshutman/lm"
	"gonum.org/v1/gonum/mat"
)

// CornersToMatrix takes a list of N corners and places them in a Nx3 matrix
// with homogenous coordinates ([X Y 1] for each corner).
func CornersToMatrix(cc []Corner) mat.Matrix {
	var data []float64
	// var out *mat.Dense
	for _, c := range cc {
		data = append(data, []float64{c.X, c.Y, 1}...)
	}

	out := mat.NewDense(len(cc), 3, data)
	return out.T()
}

/* buildA builds the A matrix for Zhang's method. Reminder that each point pairing from
2D-image(x,y) to 2D-world (X,Y) coordinates gives two equations: ax^T*h=0 and ay^T*h = 0, where
ax^T = [-X, -Y, -1, 0,0,0, xX,xY, x]
ay^T = [0,0,0, -X, -Y, -1, yX,yY, y],
so we use N points and stack these 2 at a time to make A: (2Nx9) matrix

(points should be on the same plane so that plane could be the XY or Z=0 plane).
*/
func buildA(impts, wdpts []Corner) (*mat.Dense, error) {
	var x, y, X, Y float64
	if len(impts) < 4 || len(wdpts) < 4 || len(impts) != len(wdpts) {
		return mat.NewDense(1, 1, nil), errors.New("need at least 4 image and 4 corresponding measured points")
	}
	data := make([]float64, 0)
	for i := range impts {
		x, y, X, Y = impts[i].X, impts[i].Y, wdpts[i].X, wdpts[i].Y
		data = append(data, []float64{-X, -Y, -1, 0, 0, 0, x * X, x * Y, x}...)
		data = append(data, []float64{0, 0, 0, -X, -Y, -1, y * X, y * Y, y}...)
	}
	A := mat.NewDense(2*len(impts), 9, data)
	return A, nil
}

// BuildH uses the A matrix to find the homography matrix H (as a vector)
// such that AH=0. In reality, we approximate this using the SVD of A.
func BuildH(imagepts, worldpts []Corner) mat.Vector {
	A, err := buildA(imagepts, worldpts)
	if err != nil {
		return nil
	}
	svd := mat.SVD{}
	if done := svd.Factorize(A, 6); done {
		var U, V mat.Dense
		svd.UTo(&U)
		svd.VTo(&V)
		sigma := svd.Values(nil)

		// This svd returns V and not V^T, so we will grab the last COLUMN (not row)
		// which corresponds to the smallest eigenvalue (in Sigma)
		h := V.ColView(len(sigma) - 1)
		hvec, ok := h.(*mat.VecDense)
		if !ok {
			return nil
		}
		return hvec
	}
	return nil
}

// ShapeH takes a 9-element vector and forms it into a 3x3 matrix.
func ShapeH(h mat.Vector) *mat.Dense {
	data := []float64{h.AtVec(0), h.AtVec(1), h.AtVec(2), h.AtVec(3), h.AtVec(4), h.AtVec(5), h.AtVec(6), h.AtVec(7), h.AtVec(8)}
	return mat.NewDense(3, 3, data)
}

// Unwrap takes a matrix and forms it into a row*col long vector.
func Unwrap(in *mat.Dense) *mat.VecDense {
	r, c := in.Dims()
	return mat.NewVecDense(r*c, in.RawMatrix().Data)
}

// CheckMul is a testing function that multiplies the two input matrices.
func CheckMul(a, h mat.Matrix) mat.Dense {
	var out mat.Dense
	out.Mul(a, h)
	return out
}

// getVij calculates Vij given column vectors of H (via Zhang's method).
func getVij(hi, hj mat.Vector) *mat.VecDense {
	data := make([]float64, 0)
	data = append(data, []float64{
		hi.AtVec(0) * hj.AtVec(0), hi.AtVec(0)*hj.AtVec(1) + hi.AtVec(1)*hj.AtVec(0), hi.AtVec(1) * hj.AtVec(1),
		hi.AtVec(2)*hj.AtVec(0) + hi.AtVec(0)*hj.AtVec(2), hi.AtVec(2)*hj.AtVec(1) + hi.AtVec(1)*hj.AtVec(2), hi.AtVec(2) * hj.AtVec(2),
	}...)

	return mat.NewVecDense(6, data)
}

// getVFromH uses getVij() to create part of the V matrix from a given homography matrix.
func getVFromH(h mat.Vector) *mat.Dense {
	hh := ShapeH(h)

	// Just do it for 1 H and we can stack them later
	h1 := hh.ColView(0)
	h2 := hh.ColView(1)
	var vv mat.VecDense
	var Vout mat.Dense

	v12 := getVij(h1, h2)
	v11 := getVij(h1, h1)
	v22 := getVij(h2, h2)
	vv.SubVec(v11, v22) // vv = v11 - v22
	Vout.Stack(v12.T(), vv.T())

	return &Vout
}

// GetV uses getVFromH to combine the calculated chunks from different views
// (homographies) into a single V matrix.
func GetV(h1, h2, h3 *mat.VecDense) *mat.Dense {
	var V, W mat.Dense

	// Normalizing homographies (at least here) doesn't change much
	k1, k2, k3 := h1.AtVec(h1.Len()-1), h2.AtVec(h2.Len()-1), h3.AtVec(h3.Len()-1)
	for i := 0; i < h1.Len(); i++ {
		h1.SetVec(i, h1.AtVec(i)/k1)
	}
	for i := 0; i < h2.Len(); i++ {
		h2.SetVec(i, h2.AtVec(i)/k2)
	}
	for i := 0; i < h3.Len(); i++ {
		h3.SetVec(i, h3.AtVec(i)/k3)
	}

	V1, V2, V3 := getVFromH(h1), getVFromH(h2), getVFromH(h3)
	V.Stack(V1, V2)
	W.Stack(&V, V3)

	return &W
}

// BuildBFromV uses V to calculate and return matrix B (as a vector) such that VB = 0.
// In reality, we can only approximate this using the SVD.
func BuildBFromV(v *mat.Dense) (mat.Vector, error) {
	var Bvec mat.Vector
	svd := mat.SVD{}
	if done := svd.Factorize(v, 6); done {
		var uu, vv mat.Dense
		svd.UTo(&uu)
		svd.VTo(&vv)
		sigma := svd.Values(nil)
		Bvec = vv.ColView(len(sigma) - 1)

		return Bvec, nil
	}
	return Bvec, errors.New("couldn't factorize your V")
}

// GetIntrinsicsFromB utilizes the method in Zhang's paper (Apdx B) to directly calculate
// and print out the intrinsic parameters given the B matrix.
func GetIntrinsicsFromB(b mat.Vector) []float64 {
	v0 := (b.AtVec(1)*b.AtVec(3) - b.AtVec(0)*b.AtVec(4)) / (b.AtVec(0)*b.AtVec(2) - b.AtVec(1)*b.AtVec(1))
	lam := b.AtVec(5) - ((b.AtVec(3)*b.AtVec(3) + v0*(b.AtVec(1)*b.AtVec(2)-b.AtVec(0)*b.AtVec(4))) / b.AtVec(0))
	alpha := math.Sqrt(math.Abs(lam / b.AtVec(0)))
	beta := math.Sqrt(math.Abs(lam * b.AtVec(0) / (b.AtVec(0)*b.AtVec(2) - b.AtVec(1)*b.AtVec(1))))
	gamma := -1 * b.AtVec(1) * alpha * alpha * (beta / lam)
	u0 := (gamma * v0 / beta) - (b.AtVec(3) * alpha * alpha / lam)

	return []float64{v0, lam, alpha, beta, gamma, u0}
}

// GetKFromB is supposed to be another method of retrieving the intrinsic parameters. Will
// not work if B can't be subject to a Cholesky decomposition (not positive semi-definite).
// Currently, this method almost never works, so we choose GetIntrinsicsFromB.
func GetKFromB(b mat.Vector) *mat.TriDense {
	// Reshape B (vector) into a SymDense and then get to work
	data := []float64{b.AtVec(0), b.AtVec(1), b.AtVec(3), b.AtVec(1), b.AtVec(2), b.AtVec(4), b.AtVec(3), b.AtVec(4), b.AtVec(5)}
	bb := mat.NewSymDense(3, data)

	var chol mat.Cholesky
	var k mat.TriDense
	_ = chol.Factorize(bb)
	chol.LTo(&k)
	// Now take the inverse transpose and that's it!!!
	k.T()
	if err := k.InverseTri(&k); err != nil {
		return nil
	}
	return &k
}

// The following two functions are for an implementation of the Levenberg-Marquardt algorithm
// which is an optimization method to solve a non-linear least squares problem

// MinClosure is a closure function for the function I want truly want to minimize which is
// |IMPTS - (H * WDPTS)| ^2  . Note that the function to be minimized includes a normalization step.
func MinClosure(dst, x []float64, ip, wp mat.Matrix) func(out, in []float64) {
	return func(dst, x []float64) {
		// x would be the parameters (H)
		// and dst should be the |imagepts - projected world pts|**2
		var out, projected mat.Dense
		h := mat.NewDense(3, 3, x)
		projected.Mul(h, wp)

		// Normalize projected by column before subtracting it
		_, c := projected.Dims()
		for i := 0; i < c; i++ {
			k := projected.At(2, i)
			projected.Set(0, i, projected.At(0, i)/k)
			projected.Set(1, i, projected.At(1, i)/k)
			projected.Set(2, i, projected.At(2, i)/k)
		}

		// got that now do I - projected ... then square it
		out.Sub(ip, &projected)
		for i, d := range out.RawMatrix().Data {
			dst[i] = d * d
		}
	}
}

// DoLM implements the Levenberg-Marquardt algorithm given the homography, image points,
// and world points. The goal is to adjust the homography matrix such that it minimizes the squared
// difference between IMPTS and H * WRLDPTS. Returns the new homography matrix as a 3x3.
func DoLM(h *mat.VecDense, ip, wp mat.Matrix) (*mat.Dense, error) {
	// pass in image and world points to the outside function
	r, c := ip.Dims()
	if len(h.RawVector().Data) != 9 {
		return nil, errors.New("matrix H must have 9 elements")
	}
	minfunc := MinClosure(make([]float64, r*c), h.RawVector().Data, ip, wp)

	jacobian := lm.NumJac{minfunc}
	homogProb := lm.LMProblem{
		Dim:        9,
		Size:       r * c,
		Func:       minfunc,
		Jac:        jacobian.Jac,
		InitParams: h.RawVector().Data,
		Tau:        1e-6,
		Eps1:       1e-8,
		Eps2:       1e-8,
	}

	LMresults, err := lm.LM(homogProb, &lm.Settings{Iterations: 100, ObjectiveTol: 1e-16})
	if err != nil {
		return nil, err
	}
	// fmt.Printf("The results are %v and are of type %T\n", LMresults.X, LMresults)

	return mat.NewDense(3, 3, LMresults.X), nil
}
