// Package servo implements the Cartesian servo loop for the builtin motion service.
package servo

import (
	"errors"
	"math"

	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/num/quat"

	"go.viam.com/rdk/referenceframe"
)

var errSVD = errors.New("jacobian SVD failed")

const (
	mmPerM = 1000.0
	fdStep = 1e-6 // rad, central-difference step for the Jacobian
)

// FK returns the end-effector position (m) and orientation in the arm's base frame.
// This is the only place model output is converted from mm to m.
func FK(m referenceframe.Model, q []referenceframe.Input) (r3.Vector, quat.Number, error) {
	p, err := m.Transform(q)
	if err != nil {
		return r3.Vector{}, quat.Number{}, err
	}
	return p.Point().Mul(1 / mmPerM), p.Orientation().Quaternion(), nil
}

// rotVec returns log(a·bᵀ) as a base-frame rotation vector in radians.
// spatialmath.QuatToR3AA is not used because it returns zero below 1e-6 rad,
// which would zero out the Jacobian's angular rows.
func rotVec(a, b quat.Number) r3.Vector {
	q := quat.Mul(a, quat.Conj(b))
	if q.Real < 0 { // shortest rotation
		q = quat.Scale(-1, q)
	}
	s := math.Sqrt(q.Imag*q.Imag + q.Jmag*q.Jmag + q.Kmag*q.Kmag)
	k := 2.0 // small-angle limit of 2·atan2(s, w)/s
	if s > 1e-12 {
		k = 2 * math.Atan2(s, q.Real) / s
	}
	return r3.Vector{X: k * q.Imag, Y: k * q.Jmag, Z: k * q.Kmag}
}

// poseError returns [dx dy dz rx ry rz] (m, rad) from current to target, in the base frame.
func poseError(tp r3.Vector, tq quat.Number, cp r3.Vector, cq quat.Number) [6]float64 {
	d := tp.Sub(cp)
	r := rotVec(tq, cq)
	return [6]float64{d.X, d.Y, d.Z, r.X, r.Y, r.Z}
}

// Jacobian returns the 6×n base-frame Jacobian at q by central differences.
// Rows are (vx, vy, vz, wx, wy, wz) in m/rad and rad/rad.
func Jacobian(m referenceframe.Model, q []referenceframe.Input) (*mat.Dense, error) {
	n := len(q)
	J := mat.NewDense(6, n, nil)
	qp := make([]referenceframe.Input, n)
	qm := make([]referenceframe.Input, n)
	for i := 0; i < n; i++ {
		copy(qp, q)
		copy(qm, q)
		qp[i] += fdStep
		qm[i] -= fdStep
		pp, rp, err := FK(m, qp)
		if err != nil {
			return nil, err
		}
		pm, rm, err := FK(m, qm)
		if err != nil {
			return nil, err
		}
		dp := pp.Sub(pm).Mul(1 / (2 * fdStep))
		dr := rotVec(rp, rm).Mul(1 / (2 * fdStep))
		J.SetCol(i, []float64{dp.X, dp.Y, dp.Z, dr.X, dr.Y, dr.Z})
	}
	return J, nil
}

// dlsStep solves Δq = (JᵀWJ + λ²I)⁻¹ JᵀW Δx with Chiaverini damping on σmin(W^½J).
//
//nolint:gocritic // matrix notation
func dlsStep(J *mat.Dense, w, dx [6]float64, eps, maxDamping float64) (dq []float64, lambda, sigmaMin float64, err error) {
	_, n := J.Dims()
	var wj mat.Dense
	wj.Apply(func(i, _ int, v float64) float64 { return math.Sqrt(w[i]) * v }, J)

	var svd mat.SVD
	if !svd.Factorize(&wj, mat.SVDNone) {
		return nil, 0, 0, errSVD
	}
	vals := svd.Values(nil)
	sigmaMin = vals[len(vals)-1]

	l2 := 0.0
	if sigmaMin < eps {
		r := sigmaMin / eps
		l2 = maxDamping * maxDamping * (1 - r*r)
	}

	var a mat.Dense
	a.Mul(wj.T(), &wj)
	for i := 0; i < n; i++ {
		a.Set(i, i, a.At(i, i)+l2)
	}
	wdx := mat.NewVecDense(6, nil)
	for i := range dx {
		wdx.SetVec(i, math.Sqrt(w[i])*dx[i])
	}
	var b, x mat.VecDense
	b.MulVec(wj.T(), wdx)
	if err := x.SolveVec(&a, &b); err != nil {
		return nil, 0, 0, err
	}
	return append([]float64(nil), x.RawVector().Data...), math.Sqrt(l2), sigmaMin, nil
}

// expRot returns the unit quaternion for base-frame rotation vector v (rad): the inverse of rotVec.
func expRot(v r3.Vector) quat.Number {
	th := v.Norm()
	if th < 1e-12 {
		return quat.Number{Real: 1}
	}
	s := math.Sin(th/2) / th
	return quat.Number{Real: math.Cos(th / 2), Imag: s * v.X, Jmag: s * v.Y, Kmag: s * v.Z}
}

// pinv is dpinv without damping: the plain rank-truncated pseudo-inverse.
//
//nolint:gocritic // matrix notation
func pinv(A mat.Matrix) (*mat.Dense, error) {
	P, _, _, _, err := dpinv(A, 1, 0, false) //nolint:dogsled // only the matrix is wanted
	return P, err
}

// rankTol is the relative singular-value cutoff for dpinv. It is 1e-4, not tighter, because
// so101.urdf/so101.json write angles as rounded constants (3.14159, 1.5708): structurally zero
// singular values come out near 4e-6·σmax (spec "Damped pseudo-inverse with rank tolerance").
const rankTol = 1e-4

// rankAbs is an absolute floor: near the SO-101 elbow singularity σmax itself collapses, and a purely
// relative cutoff would then keep the ≈2e-6 structural zero (flipping the reachable/residual split).
const rankAbs = 1e-5

// dpinv returns the damped pseudo-inverse Σ σᵢ/(σᵢ²+λ²)·vᵢuᵢᵀ of A (m×n → n×m), keeping only
// σᵢ > rankTol·σmax. With damp, λ is Chiaverini damping on σr, the smallest retained singular value.
//
//nolint:gocritic // matrix notation
func dpinv(A mat.Matrix, eps, maxDamping float64, damp bool) (P *mat.Dense, lambda, sigmaR float64, rank int, err error) {
	m, n := A.Dims()
	P = mat.NewDense(n, m, nil)
	var svd mat.SVD
	if !svd.Factorize(A, mat.SVDThin) {
		return nil, 0, 0, 0, errSVD
	}
	vals := svd.Values(nil)
	if len(vals) == 0 || vals[0] <= rankAbs {
		return P, 0, 0, 0, nil
	}
	for _, s := range vals {
		if s > max(rankTol*vals[0], rankAbs) {
			rank++
		}
	}
	sigmaR = vals[rank-1]
	l2 := 0.0
	if damp && sigmaR < eps {
		r := sigmaR / eps
		l2 = maxDamping * maxDamping * (1 - r*r)
	}
	var U, V mat.Dense
	svd.UTo(&U)
	svd.VTo(&V)
	for i := 0; i < rank; i++ {
		var o mat.Dense
		o.Outer(vals[i]/(vals[i]*vals[i]+l2), V.ColView(i), U.ColView(i))
		P.Add(P, &o)
	}
	return P, math.Sqrt(l2), sigmaR, rank, nil
}

// toolAxis returns the tool (flange z) axis of orientation q in the base frame — Viam's
// orientation-vector direction.
func toolAxis(q quat.Number) r3.Vector {
	x, y, z, w := q.Imag, q.Jmag, q.Kmag, q.Real
	return r3.Vector{X: 2 * (x*z + w*y), Y: 2 * (y*z - w*x), Z: 1 - 2*(x*x+y*y)}
}

// rotate turns v by unit quaternion q: q·v·q*.
func rotate(q quat.Number, v r3.Vector) r3.Vector {
	r := quat.Mul(quat.Mul(q, quat.Number{Imag: v.X, Jmag: v.Y, Kmag: v.Z}), quat.Conj(q))
	return r3.Vector{X: r.Imag, Y: r.Jmag, Z: r.Kmag}
}

// pointingError returns the rotation vector r (base frame, ⟂ ac) that turns the current tool axis
// ac onto the target axis at, and an orthonormal pair u, v ⟂ ac.
func pointingError(ac, at r3.Vector) (r, u, v r3.Vector) {
	u = ac.Cross(r3.Vector{Z: 1})
	if u.Norm() < 1e-6 {
		u = ac.Cross(r3.Vector{X: 1})
	}
	u = u.Normalize()
	v = ac.Cross(u)
	c := ac.Cross(at)
	switch {
	case c.Norm() >= 1e-9:
		r = c.Normalize().Mul(math.Atan2(c.Norm(), ac.Dot(at)))
	case ac.Dot(at) < 0:
		r = u.Mul(math.Pi)
	}
	return r, u, v
}

// pointingRows returns J2 = [uᵀJ_ang; vᵀJ_ang] (2×n) from the 6×n Jacobian.
//
//nolint:gocritic // matrix notation
func pointingRows(J *mat.Dense, u, v r3.Vector) *mat.Dense {
	_, n := J.Dims()
	J2 := mat.NewDense(2, n, nil)
	for j := 0; j < n; j++ {
		w := r3.Vector{X: J.At(3, j), Y: J.At(4, j), Z: J.At(5, j)}
		J2.Set(0, j, u.Dot(w))
		J2.Set(1, j, v.Dot(w))
	}
	return J2
}

// nullProjector returns N1 = I − pinv(J1)·J1 with the dpinv rank tolerance.
//
//nolint:gocritic // matrix notation
func nullProjector(J1 mat.Matrix) (*mat.Dense, error) {
	_, n := J1.Dims()
	P, err := pinv(J1)
	if err != nil {
		return nil, err
	}
	N := mat.NewDense(n, n, nil)
	N.Mul(P, J1)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			d := -N.At(i, j)
			if i == j {
				d++
			}
			N.Set(i, j, d)
		}
	}
	return N, nil
}

// prioritySolve returns Δq = dpinv(J1)Δx1 + dpinv(J2·N1)(Δx2 − J2·Δq1), with independent
// Chiaverini damping per level (spec "Pointing mode → Priority solve").
//
//nolint:gocritic // matrix notation
func prioritySolve(J1, J2 *mat.Dense, dx1, dx2 []float64, eps, maxDamping float64) (dq []float64, l1, l2 float64, err error) {
	P1, l1, _, _, err := dpinv(J1, eps, maxDamping, true)
	if err != nil {
		return nil, 0, 0, err
	}
	dq1 := mat.NewVecDense(len(dx1), dx1)
	var q1 mat.VecDense
	q1.MulVec(P1, dq1)
	N1, err := nullProjector(J1)
	if err != nil {
		return nil, 0, 0, err
	}
	var J2N mat.Dense
	J2N.Mul(J2, N1)
	P2, l2, _, _, err := dpinv(&J2N, eps, maxDamping, true)
	if err != nil {
		return nil, 0, 0, err
	}
	var j2q1, rhs, q2 mat.VecDense
	j2q1.MulVec(J2, &q1)
	rhs.SubVec(mat.NewVecDense(len(dx2), dx2), &j2q1)
	q2.MulVec(P2, &rhs)
	q1.AddVec(&q1, &q2)
	return append([]float64(nil), q1.RawVector().Data...), l1, l2, nil
}

// pointingReach splits pointing error e2 (rad) into the part J2·N1 can still reduce and the
// residual it can't: |e2_reach| and |e2 − e2_reach| (spec "Achievable pointing"). A joint within
// limitEps of a limit in [lo, hi] whose reducing step would push it further out is locked (its
// column zeroed in J1 and J2) and the split is recomputed. lo/hi nil: no limits.
//
//nolint:gocritic // matrix notation
func pointingReach(J1, J2 *mat.Dense, e2 [2]float64, q, lo, hi []float64) (reach, residual float64, err error) {
	reach, residual, dq, err := pointingSplit(J1, J2, e2)
	if err != nil || lo == nil || hi == nil {
		return reach, residual, err
	}
	J1, J2 = mat.DenseCopyOf(J1), mat.DenseCopyOf(J2)
	if !lockOutward(J1, J2, dq, q, lo, hi) {
		return reach, residual, nil
	}
	reach, residual, _, err = pointingSplit(J1, J2, e2)
	return reach, residual, err
}

// limitEps: a joint within this of its limit with an outward step is locked (lockOutward), so it
// won't cover that last <0.5 deg toward the limit.
const limitEps = 0.5 * math.Pi / 180 // rad

// lockOutward zeroes (in place) the J1/J2 columns of joints within limitEps of a limit whose step dq
// pushes further out (|dq| > 1e-12). Reports whether any column was zeroed.
//
//nolint:gocritic // matrix notation
func lockOutward(J1, J2 *mat.Dense, dq, q, lo, hi []float64) bool {
	locked := false
	for i, d := range dq {
		if (d > 1e-12 && q[i] >= hi[i]-limitEps) || (d < -1e-12 && q[i] <= lo[i]+limitEps) {
			for r := 0; r < 3; r++ {
				J1.Set(r, i, 0)
			}
			for r := 0; r < 2; r++ {
				J2.Set(r, i, 0)
			}
			locked = true
		}
	}
	return locked
}

// pointingSplit is the limit-blind split; dq is the null-space joint step that would reduce e2.
//
//nolint:gocritic // matrix notation
func pointingSplit(J1, J2 *mat.Dense, e2 [2]float64) (reach, residual float64, dq []float64, err error) {
	N1, err := nullProjector(J1)
	if err != nil {
		return 0, 0, nil, err
	}
	var J2N, proj mat.Dense
	J2N.Mul(J2, N1)
	P, err := pinv(&J2N)
	if err != nil {
		return 0, 0, nil, err
	}
	proj.Mul(&J2N, P)
	e := mat.NewVecDense(2, e2[:])
	var er, q0, q mat.VecDense
	er.MulVec(&proj, e)
	q0.MulVec(P, e)
	q.MulVec(N1, &q0)
	return er.Norm(2), math.Hypot(e2[0]-er.AtVec(0), e2[1]-er.AtVec(1)), q.RawVector().Data, nil
}

// activePosMM / activeAngDeg are the norms of the components whose weight is > 0.
func activePosMM(e, w [6]float64) float64 {
	s := 0.0
	for i := 0; i < 3; i++ {
		if w[i] > 0 {
			s += e[i] * e[i]
		}
	}
	return math.Sqrt(s) * mmPerM
}

func activeAngDeg(e, w [6]float64) float64 {
	s := 0.0
	for i := 3; i < 6; i++ {
		if w[i] > 0 {
			s += e[i] * e[i]
		}
	}
	return math.Sqrt(s) * 180 / math.Pi
}

// capNorm scales v down to length limit, keeping its direction; v is unchanged if already within limit.
func capNorm(v r3.Vector, limit float64) r3.Vector {
	if n := v.Norm(); n > limit {
		return v.Mul(limit / n)
	}
	return v
}
