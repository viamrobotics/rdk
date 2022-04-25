package calibrate

import (
	"errors"
	"fmt"
	"math"

	//"github.com/maorshutman/lm"
	"github.com/maorshutman/lm"
	"gonum.org/v1/gonum/mat"
)

func MatPrint(X mat.Matrix) {
	fa := mat.Formatted(X, mat.Prefix(""), mat.Squeeze())
	fmt.Printf("%v\n", fa)
}

func CornersToMatrix(cc []Corner) mat.Matrix {
	var data []float64
	//var out *mat.Dense
	for _, c := range cc {
		data = append(data, []float64{c.X, c.Y, 1}...)
	}

	out := mat.NewDense(len(cc), 3, data)
	return out.T()
}

/* BuildA builds the A matrix for Zhang's method. Reminder that each point pairing from
2D-image(x,y) to 2D-world (X,Y) coordinates gives two equations: ax^T*h=0 and ay^T*h = 0, where
ax^T = [-X, -Y, -1, 0,0,0, xX,xY, x]
ay^T = [0,0,0, -X, -Y, -1, yX,yY, y],
so we use 4 points and stack these 2 at a time to make A: (8x9) matrix

(points should be on the same plane so that plane could be the XY or Z=0 plane)
*/
func BuildA(impts, wdpts []Corner) (*mat.Dense, error) {
	var x, y, X, Y float64
	if len(impts) < 4 || len(wdpts) < 4 || len(impts) != len(wdpts) {
		return mat.NewDense(1, 1, nil), errors.New("need at least 4 image and 4 corresponding measured points")
	}
	data := make([]float64, 0)
	for i, _ := range impts {
		x, y, X, Y = float64(impts[i].X), float64(impts[i].Y), float64(wdpts[i].X), float64(wdpts[i].Y)
		data = append(data, []float64{-X, -Y, -1, 0, 0, 0, x * X, x * Y, x}...)
		data = append(data, []float64{0, 0, 0, -X, -Y, -1, y * X, y * Y, y}...)
	}
	A := mat.NewDense(2*len(impts), 9, data)
	return A, nil
}

func BuildH(imagepts, worldpts []Corner) mat.Vector {
	var Hvec *mat.VecDense
	A, _ := BuildA(imagepts, worldpts)

	svd := mat.SVD{}
	done := svd.Factorize(A, 6)

	if done {
		var U, V mat.Dense
		svd.UTo(&U)
		svd.VTo(&V)
		sigma := svd.Values(nil)

		//This svd returns V and not V^T, so we will grab the last COLUMN (not row)
		//which corresponds to the smallest eigenvalue (in Sigma)
		Hvec = V.ColView(len(sigma) - 1).(*mat.VecDense)
	}
	/*
		//normalizing
		k := Hvec.AtVec(Hvec.Len() - 1)
		for i := 0; i < Hvec.Len(); i++ {
			Hvec.SetVec(i, Hvec.AtVec(i)/k)
		}
	*/
	return Hvec
}

func CheckH(A, H mat.Matrix) {
	var out mat.Dense
	out.Mul(A, H)
	MatPrint(&out)
}

func ShapeH(H mat.Vector) *mat.Dense {
	data := []float64{H.AtVec(0), H.AtVec(1), H.AtVec(2), H.AtVec(3), H.AtVec(4), H.AtVec(5), H.AtVec(6), H.AtVec(7), H.AtVec(8)}
	return mat.NewDense(3, 3, data)
}

func Unwrap(in *mat.Dense) *mat.VecDense {
	r, c := in.Dims()
	return mat.NewVecDense(r*c, in.RawMatrix().Data)
}

func getVij(hi, hj mat.Vector) *mat.VecDense {
	data := make([]float64, 0)
	data = append(data, []float64{hi.AtVec(0) * hj.AtVec(0), hi.AtVec(0)*hj.AtVec(1) + hi.AtVec(1)*hj.AtVec(0), hi.AtVec(1) * hj.AtVec(1),
		hi.AtVec(2)*hj.AtVec(0) + hi.AtVec(0)*hj.AtVec(2), hi.AtVec(2)*hj.AtVec(1) + hi.AtVec(1)*hj.AtVec(2), hi.AtVec(2) * hj.AtVec(2)}...)

	return mat.NewVecDense(6, data)
}

func GetVFromH(H mat.Vector) *mat.Dense {
	HH := ShapeH(H)

	//Just do it for 1 H and we can stack them later
	h1 := HH.ColView(0)
	h2 := HH.ColView(1)
	var vv mat.VecDense
	var Vout mat.Dense

	v12 := getVij(h1, h2)
	v11 := getVij(h1, h1)
	v22 := getVij(h2, h2)
	vv.SubVec(v11, v22) // vv = v11 - v22
	Vout.Stack(v12.T(), vv.T())

	return &Vout
}

func GetV(H1, H2, H3 *mat.VecDense) *mat.Dense {
	var V, W mat.Dense

	//Normalizing homographies (at least here) doesn't change much
	k1, k2, k3 := H1.AtVec(H1.Len()-1), H2.AtVec(H2.Len()-1), H3.AtVec(H3.Len()-1)
	for i := 0; i < H1.Len(); i++ {
		H1.SetVec(i, H1.AtVec(i)/k1)
	}
	for i := 0; i < H2.Len(); i++ {
		H2.SetVec(i, H2.AtVec(i)/k2)
	}
	for i := 0; i < H3.Len(); i++ {
		H3.SetVec(i, H3.AtVec(i)/k3)
	}

	fmt.Println("H1: ")
	MatPrint(ShapeH(H1))
	fmt.Println("H2: ")
	MatPrint(ShapeH(H2))
	fmt.Println("H3: ")
	MatPrint(ShapeH(H3))

	V1, V2, V3 := GetVFromH(H1), GetVFromH(H2), GetVFromH(H3)
	V.Stack(V1, V2)
	W.Stack(&V, V3)

	return &W
}

func BuildBFromV(V *mat.Dense) (mat.Vector, error) {
	var Bvec mat.Vector
	svd := mat.SVD{}

	done := svd.Factorize(V, 6)

	if done {
		var UU, VV mat.Dense
		svd.UTo(&UU)
		svd.VTo(&VV)
		sigma := svd.Values(nil)
		Bvec = VV.ColView(len(sigma) - 1)

		return Bvec, nil
	}

	return Bvec, errors.New("couldn't factorize your V")
}

func GetIntrinsicsFromB(B mat.Vector) {
	v0 := (B.AtVec(1)*B.AtVec(3) - B.AtVec(0)*B.AtVec(4)) / (B.AtVec(0)*B.AtVec(2) - B.AtVec(1)*B.AtVec(1))
	fmt.Printf("v0: %v\n", v0)
	lam := B.AtVec(5) - ((B.AtVec(3)*B.AtVec(3) + v0*(B.AtVec(1)*B.AtVec(2)-B.AtVec(0)*B.AtVec(4))) / B.AtVec(0))
	fmt.Printf("lam: %v\n", lam)
	alpha := math.Sqrt(math.Abs(lam / B.AtVec(0)))
	fmt.Printf("alpha: %v\n", alpha)
	beta := math.Sqrt(math.Abs(lam * B.AtVec(0) / (B.AtVec(0)*B.AtVec(2) - B.AtVec(1)*B.AtVec(1))))
	fmt.Printf("beta: %v\n", beta)
	gamma := -1 * B.AtVec(1) * alpha * alpha * (beta / lam)
	fmt.Printf("gamma: %v\n", gamma)
	u0 := (gamma * v0 / beta) - (B.AtVec(3) * alpha * alpha / lam)
	fmt.Printf("u0: %v\n", u0)
}

func GetKFromB(B mat.Vector) *mat.TriDense {
	//Reshape B (vector) into a SymDense and then get to work
	data := []float64{B.AtVec(0), B.AtVec(1), B.AtVec(3), B.AtVec(1), B.AtVec(2), B.AtVec(4), B.AtVec(3), B.AtVec(4), B.AtVec(5)}
	BB := mat.NewSymDense(3, data)

	MatPrint(BB)

	var chol mat.Cholesky
	var K mat.TriDense
	if done := chol.Factorize(BB); !done {
		fmt.Println("Didn't factorize")
	}
	chol.LTo(&K)
	//Now take the inverse transpose and that's it!!!
	K.T()
	_ = K.InverseTri(&K)

	return &K
}

//The function I want to minimize is |IMPTS - (H * WDPTS)|**2
func MinClosure(dst, x []float64, I, W mat.Matrix) func(out, in []float64) {

	//define I?
	//define W?
	//naw just pass it in

	return func(dst, x []float64) {
		//x would be the parameters (H)
		//and dst should be the |imagepts - projected world pts|**2
		var out, projected mat.Dense
		H := mat.NewDense(3, 3, x)
		projected.Mul(H, W)

		//Normalize projected by column before subtracting it
		_, c := projected.Dims()
		for i := 0; i < c; i++ {
			k := projected.At(2, i)
			projected.Set(0, i, projected.At(0, i)/k)
			projected.Set(1, i, projected.At(1, i)/k)
			projected.Set(2, i, projected.At(2, i)/k)
		}
		//This might have made things worse... INVESTIGATE LATER

		//got that now do I - projected ... then square it
		out.Sub(I, &projected)

		for i, d := range out.RawMatrix().Data {
			dst[i] = d * d
		}
	}
}

func DoLM(H *mat.VecDense, I, W mat.Matrix) *mat.Dense {
	//pass in image and world points to the outside function
	r, c := I.Dims()
	minfunc := MinClosure(make([]float64, r*c), H.RawVector().Data, I, W)

	jacobian := lm.NumJac{minfunc}
	homogProb := lm.LMProblem{
		Dim:        9,
		Size:       r * c,
		Func:       minfunc,
		Jac:        jacobian.Jac,
		InitParams: H.RawVector().Data,
		Tau:        1e-6,
		Eps1:       1e-8,
		Eps2:       1e-8,
	}

	LMresults, _ := lm.LM(homogProb, &lm.Settings{Iterations: 100, ObjectiveTol: 1e-16})
	fmt.Println()
	//fmt.Printf("The results are %v and are of type %T\n", LMresults.X, LMresults)

	return mat.NewDense(3, 3, LMresults.X)
}
