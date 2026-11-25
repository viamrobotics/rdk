package servo

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/spatialmath"
)

// Both solvers must pass this; solveOnceSolver is skipped on builds without NLopt.
func solversUnderTest(t testing.TB) map[string]Solver {
	t.Helper()
	solvers := map[string]Solver{"dls": newDLSSolver()}
	if s, err := newSolveOnceSolver(logging.NewTestLogger(t)); err == nil {
		solvers["solveonce"] = s
	}
	return solvers
}

// run steps the solver at 50 Hz through Limits.Apply and returns the final joints.
func run(t *testing.T, s Solver, in StepInput, ticks int) []float64 {
	t.Helper()
	q := append([]float64(nil), in.Q...)
	last := make([]float64, len(q))
	for i := 0; i < ticks; i++ {
		in.Q = q
		dq, _, err := s.Step(in)
		test.That(t, err, test.ShouldBeNil)
		step, _ := in.Limits.Apply(q, dq, last, in.Dt)
		for j := range q {
			q[j] += step[j]
		}
		last = step
	}
	return q
}

func TestSolverConvergesOnReachableTarget(t *testing.T) {
	for name, s := range solversUnderTest(t) {
		t.Run(name, func(t *testing.T) {
			m := loadModel(t, "ur5e")
			q := []float64{0, -1.2, 1.2, -1.5, -1.57, 0}
			start, err := m.Transform(q)
			test.That(t, err, test.ShouldBeNil)
			target := spatialmath.Compose(start, spatialmath.NewPoseFromPoint(r3.Vector{X: 30, Y: -20, Z: 10}))
			l := testLimits(t, m)
			q = run(t, s, StepInput{Model: m, Q: q, Target: target, Gain: 2, Dt: 0.02, Limits: l}, 300) // 6 s at 50 Hz
			got, _ := m.Transform(q)
			test.That(t, got.Point().Distance(target.Point()), test.ShouldBeLessThan, 1.0) // mm
		})
	}
}

// Ported from diff-ik TestSO101ReachablePointingConverges.
func TestSolverPointingConvergesSO101(t *testing.T) {
	for name, s := range solversUnderTest(t) {
		t.Run(name, func(t *testing.T) {
			m := loadModel(t, "so101")
			l := testLimits(t, m)
			for _, q0 := range [][]float64{so101Qa, so101Qb, so101Qc} {
				qt := []float64{q0[0] + 0.12, q0[1] - 0.1, q0[2] + 0.15, q0[3] - 0.15, q0[4] + 0.1}
				target, err := m.Transform(qt)
				test.That(t, err, test.ShouldBeNil)
				q := run(t, s, StepInput{Model: m, Q: q0, Target: target, Gain: 2, Dt: 0.02, Task: TaskPointing, Limits: l}, 300)
				p, o := mustFK(t, m, q)
				tp, to := mustFK(t, m, qt)
				test.That(t, p.Sub(tp).Norm()*mmPerM, test.ShouldBeLessThan, 2.0)
				r, _, _ := pointingError(toolAxis(o), toolAxis(to))
				test.That(t, r.Norm()*180/math.Pi, test.ShouldBeLessThan, 2.0)
			}
		})
	}
}

func TestSolverSingularStaysFinite(t *testing.T) {
	for name, s := range solversUnderTest(t) {
		t.Run(name, func(t *testing.T) {
			m := loadModel(t, "ur5e")
			q := []float64{0, -1.2, 1.2, -1.5, 0, 0} // wrist 2 at zero: wrist axes align
			start, err := m.Transform(q)
			test.That(t, err, test.ShouldBeNil)
			target := spatialmath.Compose(start, spatialmath.NewPose(r3.Vector{X: 5}, &spatialmath.OrientationVector{OZ: 1, Theta: 0.3}))
			dq, info, err := s.Step(StepInput{Model: m, Q: q, Target: target, Gain: 2, Dt: 0.02, Limits: testLimits(t, m)})
			test.That(t, err, test.ShouldBeNil)
			for _, v := range dq {
				test.That(t, math.IsNaN(v) || math.IsInf(v, 0), test.ShouldBeFalse)
			}
			if name == "dls" { // damping is a DLS notion
				test.That(t, info.Damping, test.ShouldBeGreaterThan, 0)
			}
		})
	}
}

func TestDLSRejectsMoreThanSixJoints(t *testing.T) {
	m := loadModel(t, "ur5e")
	q := make([]float64, 7)
	in := StepInput{Model: m, Q: q, Target: spatialmath.NewZeroPose(), Gain: 1, Dt: 0.02, Limits: Limits{MaxLinear: 1, MaxAngular: 1}}
	_, _, err := newDLSSolver().Step(in)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "7-joint arms are not supported (max 6)")
}

func TestTickTwistReportsTCPCap(t *testing.T) {
	m := loadModel(t, "ur5e")
	q := []float64{0, -1.2, 1.2, -1.5, -1.57, 0}
	start, err := m.Transform(q)
	test.That(t, err, test.ShouldBeNil)
	target := spatialmath.Compose(start, spatialmath.NewPoseFromPoint(r3.Vector{X: 100}))
	for _, s := range solversUnderTest(t) {
		for _, tc := range []struct {
			maxLinear float64
			want      bool
		}{{0.05, true}, {math.Inf(1), false}} {
			l := testLimits(t, m)
			l.MaxLinear = tc.maxLinear
			_, info, err := s.Step(StepInput{Model: m, Q: q, Target: target, Gain: 2, Dt: 0.02, Limits: l})
			test.That(t, err, test.ShouldBeNil)
			test.That(t, info.TCPCapped, test.ShouldEqual, tc.want)
		}
	}
}
