package servo

import (
	"math"
	"math/rand"
	"slices"
	"testing"

	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/num/quat"
)

func TestPoseError(t *testing.T) {
	q := rotX(0.3) // different axis from the error rotation, so a body-frame error would differ
	p := r3.Vector{X: 0.4, Y: 0.1, Z: 0.2}
	for i, v := range poseError(p, q, p, q) {
		if math.Abs(v) > 1e-12 {
			t.Fatalf("identical poses: e[%d] = %v, want 0", i, v)
		}
	}
	// target is 10 mm further in x and rotated a further 0.1 rad about z
	e := poseError(p.Add(r3.Vector{X: 0.01}), quat.Mul(rotZ(0.1), q), p, q)
	want := [6]float64{0.01, 0, 0, 0, 0, 0.1}
	for i := range e {
		if math.Abs(e[i]-want[i]) > 1e-9 {
			t.Fatalf("e = %v, want %v", e, want)
		}
	}
}

func TestRotVecSmallAngle(t *testing.T) {
	r := rotVec(rotZ(1e-7), quat.Number{Real: 1})
	if math.Abs(r.Z-1e-7) > 1e-12 {
		t.Fatalf("RotVec of 1e-7 rad = %v, want z = 1e-7", r)
	}
}

// Perturbs one joint at a time by 1°. Perturbing all joints at once lets parallel axes cancel,
// which shrinks the true motion until second-order terms dominate the relative error.
func TestJacobianPredictsSmallMotion(t *testing.T) {
	m := loadModel(t, "ur5e")
	rng := rand.New(rand.NewSource(1))
	n := len(m.DoF())
	for trial := 0; trial < 20; trial++ {
		q := make([]float64, n)
		for i := range q {
			q[i] = (rng.Float64()*2 - 1) * 1.5
		}
		J, err := Jacobian(m, q)
		if err != nil {
			t.Fatal(err)
		}
		p1, r1 := mustFK(t, m, q)
		for j := 0; j < n; j++ {
			q2 := slices.Clone(q)
			q2[j] += (math.Pi / 180)
			p2, r2 := mustFK(t, m, q2)
			col := mat.Col(nil, j, J)
			predLin := r3.Vector{X: col[0], Y: col[1], Z: col[2]}.Mul((math.Pi / 180))
			predAng := r3.Vector{X: col[3], Y: col[4], Z: col[5]}.Mul((math.Pi / 180))
			if act := p2.Sub(p1); act.Norm() > 1e-5 { // skip joints whose axis passes through the tool point
				if e := relErr(act, predLin); e > 0.05 {
					t.Errorf("trial %d joint %d: linear prediction off by %.1f%%", trial, j, e*100)
				}
			}
			if e := relErr(rotVec(r2, r1), predAng); e > 0.05 {
				t.Errorf("trial %d joint %d: angular prediction off by %.1f%%", trial, j, e*100)
			}
		}
	}
}

func TestDLSStep(t *testing.T) {
	ones := [6]float64{1, 1, 1, 1, 1, 1}
	dx := [6]float64{0.01, -0.02, 0.03, 0.001, 0, -0.002}

	// Well-conditioned diagonal J: no damping, exact solve dq_i = dx_i / d_i.
	d := []float64{1, 2, 3, 1, 2, 3}
	J := mat.NewDense(6, 6, nil)
	for i, v := range d {
		J.Set(i, i, v)
	}
	dq, lambda, _, err := dlsStep(J, ones, dx, 0.04, 0.1)
	if err != nil || lambda != 0 {
		t.Fatalf("lambda = %v, err = %v; want 0, nil", lambda, err)
	}
	for i := range dq {
		if math.Abs(dq[i]-dx[i]/d[i]) > 1e-12 {
			t.Fatalf("dq = %v, want dx/d", dq)
		}
	}

	// Rank-deficient: column 6 is zero, so damping must kick in and the step stays finite.
	J = mat.NewDense(6, 6, nil)
	for i := 0; i < 5; i++ {
		J.Set(i, i, 1)
	}
	dq, lambda, sigmaMin, err := dlsStep(J, ones, dx, 0.04, 0.1)
	if err != nil {
		t.Fatal(err)
	}
	if sigmaMin != 0 || math.Abs(lambda-0.1) > 1e-12 {
		t.Fatalf("sigmaMin = %v lambda = %v, want 0 and 0.1", sigmaMin, lambda)
	}
	if dq[5] != 0 {
		t.Fatalf("dq[5] = %v, want 0 for the dead joint", dq[5])
	}
	for i := 0; i < 5; i++ {
		if want := dx[i] / 1.01; math.Abs(dq[i]-want) > 1e-12 {
			t.Fatalf("dq[%d] = %v, want %v", i, dq[i], want)
		}
	}

	// Weighted, overdetermined 6x5 J, lambda = 0: compare with a direct (JᵀWJ)⁻¹JᵀWdx using plain W.
	J = mat.NewDense(6, 5, nil)
	for i := 0; i < 5; i++ {
		J.Set(i, i, 1)
	}
	J.Set(5, 0, 1)
	J.Set(5, 1, 1)
	w := [6]float64{4, 1, 2, 1, 3, 0.5}
	W := mat.NewDiagDense(6, w[:])
	var JtW, A mat.Dense
	JtW.Mul(J.T(), W)
	A.Mul(&JtW, J)
	bv := mat.NewVecDense(6, dx[:])
	var rhs, ref mat.VecDense
	rhs.MulVec(&JtW, bv)
	if err := ref.SolveVec(&A, &rhs); err != nil {
		t.Fatal(err)
	}
	dq, lambda, _, err = dlsStep(J, w, dx, 0.04, 0.1)
	if err != nil || lambda != 0 {
		t.Fatalf("weighted: lambda = %v, err = %v; want 0, nil", lambda, err)
	}
	for i := range dq {
		if math.Abs(dq[i]-ref.AtVec(i)) > 1e-12 {
			t.Fatalf("weighted dq = %v, want %v", dq, mat.Formatted(&ref))
		}
	}
}

func TestExpRotInvertsRotVec(t *testing.T) {
	for _, v := range []r3.Vector{{X: 0.3}, {Y: -1.2, Z: 0.4}, {X: 1e-9}, {}} {
		got := rotVec(expRot(v), quat.Number{Real: 1})
		if got.Sub(v).Norm() > 1e-12 {
			t.Errorf("rotVec(expRot(%v)) = %v", v, got)
		}
	}
}

func TestDpinvMatchesClosedForms(t *testing.T) {
	sq := mat.NewDense(2, 2, []float64{2, 1, 1, 3})
	P, lam, _, r, err := dpinv(sq, 0.04, 0.1, true)
	if err != nil || lam != 0 || r != 2 {
		t.Fatalf("square: λ=%v r=%d err=%v", lam, r, err)
	}
	nearVec(t, mulVec(P, []float64{1, 2}), []float64{0.2, 0.6}, 1e-12) // inverse · b

	tall := mat.NewDense(3, 2, []float64{1, 0, 0, 1, 1, 1})
	P, _ = pinv(tall)
	var ata, inv, want mat.Dense
	ata.Mul(tall.T(), tall)
	_ = inv.Inverse(&ata)
	want.Mul(&inv, tall.T())
	if !mat.EqualApprox(P, &want, 1e-12) {
		t.Fatalf("tall: dpinv ≠ (AᵀA)⁻¹Aᵀ")
	}

	wide := mat.NewDense(2, 3, []float64{1, 0, 1, 0, 1, 1})
	P, _ = pinv(wide)
	var aat, inv2, want2 mat.Dense
	aat.Mul(wide, wide.T())
	_ = inv2.Inverse(&aat)
	want2.Mul(wide.T(), &inv2)
	if !mat.EqualApprox(P, &want2, 1e-12) {
		t.Fatalf("wide: dpinv ≠ Aᵀ(AAᵀ)⁻¹")
	}

	zero := mat.NewDense(2, 3, nil)
	P, lam, _, r, err = dpinv(zero, 0.04, 0.1, true)
	if err != nil || r != 0 || lam != 0 || mat.Norm(P, 2) != 0 {
		t.Fatalf("zero: r=%d λ=%v err=%v", r, lam, err)
	}
}

// A rank-deficient system must not blow up, and must give the minimum-norm solution
// (the SO-101 pointing stack has a structural null direction, spec "SO-101 kinematics").
func TestDpinvRankDeficientIsMinimumNorm(t *testing.T) {
	A := mat.NewDense(2, 3, []float64{1, 1, 0, 2, 2, 0}) // rank 1
	P, lam, _, r, err := dpinv(A, 0.04, 0.1, true)
	if err != nil || r != 1 || lam != 0 {
		t.Fatalf("r=%d λ=%v err=%v", r, lam, err)
	}
	x := mulVec(P, []float64{1, 2})
	nearVec(t, x, []float64{0.5, 0.5, 0}, 1e-12) // minimum-norm solution of x+y=1
}

// Chiaverini damping keys on the smallest *retained* singular value.
func TestDpinvDampsOnlyWeakRetainedDirections(t *testing.T) {
	A := mat.NewDense(2, 2, []float64{1, 0, 0, 0.02}) // σ = 1, 0.02 < ε
	_, lam, sr, r, _ := dpinv(A, 0.04, 0.1, true)
	want := 0.1 * math.Sqrt(1-(0.02/0.04)*(0.02/0.04))
	if r != 2 || math.Abs(sr-0.02) > 1e-15 || math.Abs(lam-want) > 1e-12 {
		t.Fatalf("r=%d σr=%v λ=%v, want λ=%v", r, sr, lam, want)
	}
	B := mat.NewDense(2, 2, []float64{1, 0, 0, 1e-9}) // below rank tolerance: dropped, not damped
	_, lam, _, r, err := dpinv(B, 0.04, 0.1, true)
	if err != nil {
		t.Fatal(err)
	}
	if r != 1 || lam != 0 {
		t.Fatalf("near-null direction: r=%d λ=%v, want r=1 λ=0", r, lam)
	}
}

// m < n pose form vs the n×n form at λ > 0 (push-through identity).
func TestDpinvMatchesNormalEquationsWhenDamped(t *testing.T) {
	A := mat.NewDense(2, 3, []float64{1, 0.2, 0, 0, 0.03, 1})
	P, lam, _, _, err := dpinv(A, 2, 0.1, true)
	if err != nil {
		t.Fatal(err)
	} // ε above σmin = 1.0 so λ > 0
	if lam == 0 {
		t.Fatal("want λ > 0")
	}
	var ata mat.Dense
	ata.Mul(A.T(), A)
	for i := 0; i < 3; i++ {
		ata.Set(i, i, ata.At(i, i)+lam*lam)
	}
	var rhs, x mat.VecDense
	rhs.MulVec(A.T(), mat.NewVecDense(2, []float64{0.3, -0.1}))
	if err := x.SolveVec(&ata, &rhs); err != nil {
		t.Fatal(err)
	}
	nearVec(t, mulVec(P, []float64{0.3, -0.1}), x.RawVector().Data, 1e-12)
}

// Each rank-tolerance term is pinned separately: the relative one (rankTol·σmax) and the absolute
// floor (rankAbs), plus the all-below-floor guard.
func TestDpinvRankToleranceTerms(t *testing.T) {
	for _, c := range []struct {
		name string
		a, b float64
		rank int
	}{
		{"relative term", 10, 5e-4, 1},    // 5e-4 > rankAbs but < 1e-4·10
		{"absolute floor", 5e-5, 2e-6, 1}, // 2e-6 > 1e-4·5e-5 but < rankAbs
		{"all below floor", 1e-6, 0, 0},
	} {
		P, _, _, rank, err := dpinv(mat.NewDiagDense(2, []float64{c.a, c.b}), 0.04, 0.1, false)
		if err != nil || rank != c.rank {
			t.Fatalf("%s: rank %d err %v, want %d", c.name, rank, err, c.rank)
		}
		if c.rank == 0 && mat.Norm(P, 1) != 0 {
			t.Fatalf("%s: P not all zeros: %v", c.name, mat.Formatted(P))
		}
	}
}

func TestToolAxisIsRotatedZ(t *testing.T) {
	vnear(t, toolAxis(rotX(math.Pi/2)), r3.Vector{Y: -1}, 1e-12) // z rotated +90° about x → −y
	vnear(t, toolAxis(rotZ(1.0)), r3.Vector{Z: 1}, 1e-12)
}

func TestPointingErrorExactAndBasis(t *testing.T) {
	ac := r3.Vector{X: 1}
	r, u, v := pointingError(ac, ac)
	if r.Norm() != 0 {
		t.Fatalf("matching axes: r = %v", r)
	}
	if math.Abs(u.Dot(ac)) > 1e-12 || math.Abs(v.Dot(ac)) > 1e-12 || math.Abs(u.Dot(v)) > 1e-12 {
		t.Fatal("u, v not orthonormal to a_c")
	}
	at := r3.Vector{X: math.Cos(0.2), Y: math.Sin(0.2)} // 0.2 rad about +z from a_c
	r, _, _ = pointingError(ac, at)
	vnear(t, r, r3.Vector{Z: 0.2}, 1e-12)

	r, u, _ = pointingError(ac, ac.Mul(-1)) // antiparallel
	if math.Abs(r.Norm()-math.Pi) > 1e-12 || math.Abs(r.Normalize().Dot(u)) < 1-1e-12 {
		t.Fatalf("antiparallel: r = %v", r)
	}
	_, u, v = pointingError(r3.Vector{Z: 1}, r3.Vector{Z: 1}) // vertical tool: x̂ fallback
	if math.Abs(u.Norm()-1) > 1e-12 || math.Abs(v.Norm()-1) > 1e-12 {
		t.Fatalf("vertical fallback: u=%v v=%v", u, v)
	}
}

// pointingRows builds J2 = [uᵀJ_ang; vᵀJ_ang] — shared by tests and the loop.
func TestPointingJacobianMatchesFiniteDifference(t *testing.T) {
	m := loadModel(t, "so101")
	J, err := Jacobian(m, so101Qb)
	if err != nil {
		t.Fatal(err)
	}
	_, o0 := mustFK(t, m, so101Qb)
	a0 := toolAxis(o0)
	_, u, v := pointingError(a0, a0) // freeze u, v at q0; target axis = a_c(q0)
	J2 := pointingRows(J, u, v)
	const h = math.Pi / 180
	for j := 0; j < 5; j++ {
		qp, qm := append([]float64(nil), so101Qb...), append([]float64(nil), so101Qb...)
		qp[j] += h
		qm[j] -= h
		_, op := mustFK(t, m, qp)
		_, om := mustFK(t, m, qm)
		rp, _, _ := pointingError(toolAxis(op), a0)
		rm, _, _ := pointingError(toolAxis(om), a0)
		// error is target − current, so d(e2)/dq = −J2
		fd := [2]float64{-(u.Dot(rp) - u.Dot(rm)) / (2 * h), -(v.Dot(rp) - v.Dot(rm)) / (2 * h)}
		for i := 0; i < 2; i++ {
			if a, b := J2.At(i, j), fd[i]; math.Abs(a-b) > 0.05*math.Max(math.Abs(b), 0.01) {
				t.Fatalf("joint %d row %d: J2 = %.4f, finite difference = %.4f", j, i, a, b)
			}
		}
	}
}

// Roll (joint 5 on the SO-101) moves neither position nor pointing: its J2·N1 column is ~0.
func TestSO101RollIsNullForPointing(t *testing.T) {
	m := loadModel(t, "so101")
	J := mustJacobian(t, m, so101Qa)
	_, o := mustFK(t, m, so101Qa)
	_, u, v := pointingError(toolAxis(o), toolAxis(o))
	J2 := pointingRows(J, u, v)
	for i := 0; i < 2; i++ {
		if math.Abs(J2.At(i, 4)) > 1e-5 {
			t.Fatalf("roll column of J2 = %.2e, want ~0", J2.At(i, 4))
		}
	}
}

// Rotating u, v within the plane must not change the priority solve's Δq.
func TestPrioritySolveIsBasisInvariant(t *testing.T) {
	m := loadModel(t, "so101")
	J := mustJacobian(t, m, so101Qb)
	_, o := mustFK(t, m, so101Qb)
	a := toolAxis(o)
	_, u, v := pointingError(a, a)
	c, s := math.Cos(0.7), math.Sin(0.7)
	u2, v2 := u.Mul(c).Add(v.Mul(s)), v.Mul(c).Sub(u.Mul(s))
	dx1 := []float64{0.001, -0.0005, 0.0008}
	e := r3.Vector{}.Add(u.Mul(0.003)).Add(v.Mul(-0.002)) // a pointing step as a vector ⟂ a
	dqA, _, _, err := prioritySolve(J.Slice(0, 3, 0, 5).(*mat.Dense), pointingRows(J, u, v), dx1,
		[]float64{u.Dot(e), v.Dot(e)}, 0.04, 0.1)
	if err != nil {
		t.Fatal(err)
	}
	dqB, _, _, err := prioritySolve(J.Slice(0, 3, 0, 5).(*mat.Dense), pointingRows(J, u2, v2), dx1,
		[]float64{u2.Dot(e), v2.Dot(e)}, 0.04, 0.1)
	if err != nil {
		t.Fatal(err)
	}
	nearVec(t, dqA, dqB, 1e-10)
}

// Mid-workspace SO-101: neither level damps. This does not by itself catch a too-tight relative
// tolerance (the absolute floor masks it); see TestDpinvRankToleranceTerms.
func TestSO101MidWorkspaceIsUndamped(t *testing.T) {
	m := loadModel(t, "so101")
	for _, q := range [][]float64{so101Qa, so101Qb, so101Qc} {
		J := mustJacobian(t, m, q)
		_, o := mustFK(t, m, q)
		a := toolAxis(o)
		_, u, v := pointingError(a, a)
		_, l1, l2, err := prioritySolve(J.Slice(0, 3, 0, 5).(*mat.Dense), pointingRows(J, u, v),
			[]float64{0.001, 0, 0}, []float64{0.002, 0.001}, 0.04, 0.1)
		if err != nil || l1 != 0 || l2 != 0 {
			t.Fatalf("q=%v: λ1=%v λ2=%v err=%v, want 0, 0", q, l1, l2, err)
		}
	}
}

// pointingReach: an azimuth-only pointing error is unreachable on the SO-101 (residual), an
// in-plane one is reachable.
func TestPointingReachSplitsSO101Error(t *testing.T) {
	m := loadModel(t, "so101")
	J := mustJacobian(t, m, so101Qa)
	_, o := mustFK(t, m, so101Qa)
	a := toolAxis(o)
	_, u, v := pointingError(a, a)
	J1, J2 := J.Slice(0, 3, 0, 5).(*mat.Dense), pointingRows(J, u, v)
	// Rotating about base z changes the azimuth; its component ⟂ a is the unreachable direction.
	z := r3.Vector{Z: 1}
	zp := z.Sub(a.Mul(a.Dot(z))).Normalize()
	reach, res, err := pointingReach(J1, J2, [2]float64{u.Dot(zp) * 0.1, v.Dot(zp) * 0.1}, nil, nil, nil)
	if err != nil || res < 0.09 || reach > 0.01 {
		t.Fatalf("azimuth error: reach=%.4f res=%.4f err=%v, want mostly residual", reach, res, err)
	}
	pitch := a.Cross(z).Normalize() // rotation about the horizontal axis ⟂ arm plane: in-plane pitch
	reach, res, _ = pointingReach(J1, J2, [2]float64{u.Dot(pitch) * 0.1, v.Dot(pitch) * 0.1}, nil, nil, nil)
	if reach < 0.09 || res > 0.01 {
		t.Fatalf("pitch error: reach=%.4f res=%.4f, want mostly reachable", reach, res)
	}
}

// With λ1 = 0 the secondary task must not move the tool position: J1·dq = dx1 exactly, and the
// secondary contribution lies in the null space of J1.
func TestPrioritySecondaryLeavesPositionAlone(t *testing.T) {
	m := loadModel(t, "so101")
	J := mustJacobian(t, m, so101Qb)
	_, o := mustFK(t, m, so101Qb)
	a := toolAxis(o)
	_, u, v := pointingError(a, a)
	J1, J2 := J.Slice(0, 3, 0, 5).(*mat.Dense), pointingRows(J, u, v)
	dx1 := []float64{0.001, -0.0005, 0.0008}
	dq, l1, _, err := prioritySolve(J1, J2, dx1, []float64{0.003, -0.002}, 1e-3, 0.1)
	if err != nil || l1 != 0 {
		t.Fatalf("λ1=%v err=%v, want 0", l1, err)
	}
	dq1, _, _, err := prioritySolve(J1, J2, dx1, []float64{0, 0}, 1e-3, 0.1)
	if err != nil {
		t.Fatal(err)
	}
	var pos, sec mat.VecDense
	pos.MulVec(J1, mat.NewVecDense(5, dq))
	nearVec(t, pos.RawVector().Data, dx1, 1e-10)
	diff := make([]float64, 5)
	for i := range diff {
		diff[i] = dq[i] - dq1[i]
	}
	sec.MulVec(J1, mat.NewVecDense(5, diff))
	if mat.Norm(&sec, 2) > 1e-10 {
		t.Fatalf("|J1·(dq-dq1)| = %g, secondary moved position", mat.Norm(&sec, 2))
	}
}

// A joint at its limit whose reducing step would push it outward is locked: the error it alone
// could fix becomes residual. The opposite error direction is unaffected.
func TestPointingReachLocksJointAtLimit(t *testing.T) {
	m := loadModel(t, "so101")
	lim := m.DoF()
	lo, hi := make([]float64, 5), make([]float64, 5)
	for i, l := range lim {
		lo[i], hi[i] = l.Min, l.Max
	}
	q := []float64{0, 0.3, 0.3, hi[3], 0}
	J := mustJacobian(t, m, q)
	_, o := mustFK(t, m, q)
	a := toolAxis(o)
	_, u, v := pointingError(a, a)
	J1, J2 := J.Slice(0, 3, 0, 5).(*mat.Dense), pointingRows(J, u, v)
	pitch := a.Cross(r3.Vector{Z: 1}).Normalize()
	var out, in [2]float64 // out: reducing step needs joint 3 to increase
	for _, s := range []float64{0.1, -0.1} {
		e := [2]float64{u.Dot(pitch) * s, v.Dot(pitch) * s}
		_, _, dq, err := pointingSplit(J1, J2, e)
		if err != nil {
			t.Fatal(err)
		}
		if dq[3] > 0 {
			out = e
		} else {
			in = e
		}
	}
	if out == ([2]float64{}) || in == ([2]float64{}) {
		t.Fatal("joint 3 does not move with the pitch error")
	}
	r0, s0, _ := pointingReach(J1, J2, out, nil, nil, nil)
	r1, s1, err := pointingReach(J1, J2, out, q, lo, hi)
	t.Logf("outward: unlimited reach=%.4f res=%.4f; limit-aware reach=%.4f res=%.4f", r0, s0, r1, s1)
	if err != nil || r1 > 0.2*r0 || s1 < 0.8*math.Hypot(out[0], out[1]) {
		t.Fatalf("locked joint still counted: reach=%.4f res=%.4f", r1, s1)
	}
	r0, s0, _ = pointingReach(J1, J2, in, nil, nil, nil)
	r1, s1, _ = pointingReach(J1, J2, in, q, lo, hi)
	if math.Abs(r1-r0) > 1e-9 || math.Abs(s1-s0) > 1e-9 {
		t.Fatalf("inward error changed: reach %.4f→%.4f res %.4f→%.4f", r0, r1, s0, s1)
	}
}
