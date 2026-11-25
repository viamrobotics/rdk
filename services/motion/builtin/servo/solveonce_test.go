package servo

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/spatialmath"
)

func solveOnceOrSkip(t *testing.T) Solver {
	t.Helper()
	s, err := newSolveOnceSolver(logging.NewTestLogger(t))
	if err != nil {
		t.Skipf("SolveOnce unavailable on this build: %v", err)
	}
	return s
}

// stepBox runs on every build (no NLopt needed).
func TestStepBoxNeverCollapses(t *testing.T) {
	l := Limits{Lo: []float64{-1, -1}, Hi: []float64{1, 1}, Vel: []float64{1, 1}, Acc: []float64{4, 4}}
	// {1.1, -1.1} is beyond Hi + v·dt, the case where the box collapses and must be widened.
	for _, q := range [][]float64{{1, -1}, {1 + 1e-9, -1 - 1e-9}, {0, 0}, {1.1, -1.1}} {
		box := stepBox(q, l, 0.02)
		for j, b := range box {
			test.That(t, b.Max, test.ShouldBeGreaterThan, b.Min)
			test.That(t, b.Min, test.ShouldBeGreaterThanOrEqualTo, l.Lo[j]-1e-6)
			test.That(t, b.Max, test.ShouldBeLessThanOrEqualTo, l.Hi[j]+1e-6)
		}
	}
}

func TestSolveOnceStepStaysInVelocityBox(t *testing.T) {
	s := solveOnceOrSkip(t)
	m := loadModel(t, "ur5e")
	l := testLimits(t, m)
	q := []float64{0, -1.2, 1.2, -1.5, -1.57, 0}
	start, _ := m.Transform(q)
	far := spatialmath.Compose(start, spatialmath.NewPoseFromPoint(r3.Vector{X: 300}))
	dq, info, err := s.Step(StepInput{Model: m, Q: q, Target: far, Gain: 2, Dt: 0.02, Limits: l})
	test.That(t, err, test.ShouldBeNil)
	for j := range dq {
		test.That(t, math.Abs(dq[j]), test.ShouldBeLessThanOrEqualTo, l.Vel[j]*0.02+1e-9)
	}
	t.Logf("solveonce step: %d evals, %v", info.Evals, info.SolveTime)
	test.That(t, info.Evals, test.ShouldBeGreaterThan, 0)
}

func TestSolversRejectShortLimits(t *testing.T) {
	m := loadModel(t, "ur5e")
	q := []float64{0, -1.2, 1.2, -1.5, -1.57, 0}
	in := StepInput{Model: m, Q: q, Target: spatialmath.NewZeroPose(), Gain: 1, Dt: 0.02, Limits: Limits{MaxLinear: 1, MaxAngular: 1}}
	for name, s := range solversUnderTest(t) {
		_, _, err := s.Step(in)
		test.That(t, err, test.ShouldNotBeNil)
		t.Log(name, err)
	}
}
