package servo

import (
	"math"
	"math/rand"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/referenceframe"
)

func TestNewLimitsTightenOnly(t *testing.T) {
	v := 1.0 // rad/s
	a := 2.0 // rad/s²
	model := []referenceframe.Limit{{Min: -3, Max: 3, MaxVelocity: &v, MaxAcceleration: &a}}
	for _, tc := range []struct {
		name    string
		opts    arm.MoveOptions
		wantVel float64
		wantErr string
	}{
		{"model only", arm.MoveOptions{}, 1, ""},
		{"tighter", arm.MoveOptions{MaxVelRads: 0.5}, 0.5, ""},
		{"looser", arm.MoveOptions{MaxVelRads: 1.5}, 0, "above the model"},
		{"per joint", arm.MoveOptions{MaxVelRadsJoints: []float64{0.25}}, 0.25, ""},
		{"per joint beats all", arm.MoveOptions{MaxVelRads: 0.5, MaxVelRadsJoints: []float64{0.25}}, 0.25, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l, err := NewLimits(model, tc.opts, 45)
			if tc.wantErr != "" {
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.wantErr)
				return
			}
			test.That(t, err, test.ShouldBeNil)
			test.That(t, l.Vel[0], test.ShouldAlmostEqual, tc.wantVel)
		})
	}
}

func TestNewLimitsAccelAndLinear(t *testing.T) {
	v, a := 1.0, 2.0
	model := []referenceframe.Limit{{Min: -3, Max: 3, MaxVelocity: &v, MaxAcceleration: &a}}
	l, err := NewLimits(model, arm.MoveOptions{MaxAccRadsJoints: []float64{1.5}}, 90)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, l.Acc[0], test.ShouldAlmostEqual, 1.5)
	test.That(t, math.IsInf(l.MaxLinear, 1), test.ShouldBeTrue)
	test.That(t, l.MaxAngular, test.ShouldAlmostEqual, math.Pi/2)
	_, err = NewLimits(model, arm.MoveOptions{MaxAccRads: 3}, 45)
	test.That(t, err.Error(), test.ShouldContainSubstring, "above the model")

	tcp := 0.3
	l, err = NewLimits(model, arm.MoveOptions{MaxTCPSpeedMPerSec: &tcp}, 45)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, l.MaxLinear, test.ShouldEqual, 0.3)
	l.TightenLinear(0.5)
	test.That(t, l.MaxLinear, test.ShouldEqual, 0.3)
	l.TightenLinear(0)
	test.That(t, l.MaxLinear, test.ShouldEqual, 0.3)
	l.TightenLinear(0.1)
	test.That(t, l.MaxLinear, test.ShouldEqual, 0.1)
}

func TestNewLimitsMissingLimitNeedsCaller(t *testing.T) {
	model := []referenceframe.Limit{{Min: -3, Max: 3}} // no velocity or acceleration
	_, err := NewLimits(model, arm.MoveOptions{}, 45)
	test.That(t, err.Error(), test.ShouldContainSubstring, "joint 0 has no velocity limit")
	_, err = NewLimits(model, arm.MoveOptions{MaxVelRads: 1, MaxAccRads: 2}, 45)
	test.That(t, err, test.ShouldBeNil)
}

// Property: Apply never changes a joint's step by more than a·dt² from the last step, never exceeds v·dt,
// and never crosses a position limit, for random inputs.
func TestApplyRespectsLimits(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	v, a := 1.0, 2.0
	model := []referenceframe.Limit{
		{Min: -1, Max: 1, MaxVelocity: &v, MaxAcceleration: &a},
		{Min: -1, Max: 1, MaxVelocity: &v, MaxAcceleration: &a},
	}
	l, err := NewLimits(model, arm.MoveOptions{}, 45)
	test.That(t, err, test.ShouldBeNil)
	dt := 0.02
	q := []float64{0.9, -0.9}
	last := []float64{0, 0}
	for i := 0; i < 5000; i++ {
		dq := []float64{rng.NormFloat64() * 0.2, rng.NormFloat64() * 0.2}
		step, _ := l.Apply(q, dq, last, dt)
		for j := range step {
			test.That(t, math.Abs(step[j]), test.ShouldBeLessThanOrEqualTo, v*dt+1e-12)
			test.That(t, math.Abs(step[j]-last[j]), test.ShouldBeLessThanOrEqualTo, a*dt*dt+1e-12)
			test.That(t, q[j]+step[j], test.ShouldBeBetweenOrEqual, -1, 1)
			q[j] += step[j]
		}
		last = step
	}
}

func TestApplyPerJointAccel(t *testing.T) {
	v := 1.0
	a0, a1 := 2.0, 0.5
	model := []referenceframe.Limit{
		{Min: -9, Max: 9, MaxVelocity: &v, MaxAcceleration: &a0},
		{Min: -9, Max: 9, MaxVelocity: &v, MaxAcceleration: &a1},
	}
	l, err := NewLimits(model, arm.MoveOptions{}, 45)
	test.That(t, err, test.ShouldBeNil)
	dt := 0.02
	step, by := l.Apply([]float64{0, 0}, []float64{0.01, 0.01}, []float64{0, 0}, dt)
	test.That(t, by, test.ShouldNotEqual, Limited(0))
	// joint 1 binds at a1·dt²; joint 0 is scaled with it, direction kept.
	test.That(t, step[1], test.ShouldAlmostEqual, a1*dt*dt)
	test.That(t, step[0], test.ShouldAlmostEqual, a1*dt*dt)
}

func TestBrakeCapEdgeCases(t *testing.T) {
	c := 0.001
	test.That(t, brakeCap(0, c), test.ShouldEqual, 0)
	test.That(t, brakeCap(-1, c), test.ShouldEqual, 0)
	test.That(t, brakeCap(1, 0), test.ShouldEqual, 0)
	test.That(t, math.IsInf(brakeCap(math.Inf(1), c), 1), test.ShouldBeTrue)
	test.That(t, brakeCap(math.NaN(), c), test.ShouldEqual, 0)
	test.That(t, brakeCap(c/2, c), test.ShouldBeLessThanOrEqualTo, c/2)
}

func TestApplyPastPositionLimit(t *testing.T) {
	v, a := 1.0, 2.0
	model := []referenceframe.Limit{{Min: -1, Max: 1, MaxVelocity: &v, MaxAcceleration: &a}}
	l, err := NewLimits(model, arm.MoveOptions{}, 45)
	test.That(t, err, test.ShouldBeNil)
	dt := 0.02
	q := []float64{1.05}
	step, _ := l.Apply(q, []float64{0.01}, []float64{0}, dt)
	test.That(t, step[0], test.ShouldEqual, 0)
	step, _ = l.Apply(q, []float64{-0.1}, []float64{0}, dt)
	test.That(t, step[0], test.ShouldBeLessThan, 0)
	test.That(t, math.Abs(step[0]), test.ShouldBeLessThanOrEqualTo, v*dt+1e-12)
	test.That(t, math.Abs(step[0]), test.ShouldBeLessThanOrEqualTo, a*dt*dt+1e-12)
}

func TestApplyNilLastIsAtRest(t *testing.T) {
	v, a := 1.0, 2.0
	model := []referenceframe.Limit{
		{Min: -9, Max: 9, MaxVelocity: &v, MaxAcceleration: &a},
		{Min: -9, Max: 9, MaxVelocity: &v, MaxAcceleration: &a},
	}
	l, err := NewLimits(model, arm.MoveOptions{}, 45)
	test.That(t, err, test.ShouldBeNil)
	dt := 0.02
	step, _ := l.Apply([]float64{0, 0}, []float64{5, -5}, nil, dt)
	for j := range step {
		test.That(t, math.Abs(step[j]), test.ShouldBeLessThanOrEqualTo, a*dt*dt+1e-12)
	}
}

func TestApplyReportsWhichLimit(t *testing.T) {
	l := Limits{
		Lo: []float64{-1}, Hi: []float64{1}, Vel: []float64{1}, Acc: []float64{100},
		MaxLinear: math.Inf(1), MaxAngular: 1,
	}
	dt := 0.02
	_, by := l.Apply([]float64{0}, []float64{0.001}, []float64{0.001}, dt)
	test.That(t, by, test.ShouldEqual, Limited(0))
	_, by = l.Apply([]float64{0}, []float64{0.5}, []float64{0.02}, dt) // 25 rad/s against 1 rad/s
	test.That(t, by&LimitedJointSpeed, test.ShouldNotEqual, Limited(0))
	test.That(t, by&LimitedJointPosition, test.ShouldEqual, Limited(0))
	_, by = l.Apply([]float64{0.9999}, []float64{0.02}, []float64{0.02}, dt) // braking at Hi
	test.That(t, by&LimitedJointPosition, test.ShouldNotEqual, Limited(0))
}
