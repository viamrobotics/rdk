package main

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// armPlan builds a two waypoint plan for one frame, from joints startJoint to endJoint, whose end
// pose sits endX millimeters along x.
func armPlan(startJoint, endJoint, endX float64) motionplan.Plan {
	path := motionplan.Path{
		{"arm": referenceframe.NewPoseInFrame("world", spatialmath.NewPoseFromPoint(r3.Vector{}))},
		{"arm": referenceframe.NewPoseInFrame("world", spatialmath.NewPoseFromPoint(r3.Vector{X: endX}))},
	}
	traj := motionplan.Trajectory{
		{"arm": {startJoint}},
		{"arm": {endJoint}},
	}

	return motionplan.NewSimplePlan(path, traj)
}

func TestCompareToRecorded(t *testing.T) {
	recorded := armPlan(0, 1, 100)

	t.Run("no recorded response", func(t *testing.T) {
		test.That(t, compareToRecorded(nil, recorded, nil, nil), test.ShouldBeNil)
	})

	t.Run("same plan agrees", func(t *testing.T) {
		div := compareToRecorded(recorded, armPlan(0, 1, 100), nil, nil)

		test.That(t, div.diverged(), test.ShouldBeFalse)
	})

	t.Run("the same pose reached with different joints diverges", func(t *testing.T) {
		div := compareToRecorded(recorded, armPlan(0, 1.2, 100), nil, nil)

		test.That(t, div.diverged(), test.ShouldBeTrue)
		test.That(t, div.endJoint, test.ShouldAlmostEqual, 0.2)
		test.That(t, div.endJointFrame, test.ShouldEqual, "arm")
	})

	t.Run("a joint difference under the threshold agrees", func(t *testing.T) {
		div := compareToRecorded(recorded, armPlan(0, 1.01, 100), nil, nil)

		test.That(t, div.diverged(), test.ShouldBeFalse)
	})

	t.Run("much longer travel diverges", func(t *testing.T) {
		div := compareToRecorded(recorded, armPlan(0, 3, 100), nil, nil)

		test.That(t, div.diverged(), test.ShouldBeTrue)
	})

	t.Run("a plan that does not move diverges", func(t *testing.T) {
		div := compareToRecorded(recorded, armPlan(1, 1, 100), nil, nil)

		test.That(t, div.diverged(), test.ShouldBeTrue)
	})

	t.Run("start drift is measured but is not a divergence", func(t *testing.T) {
		recordedStart := referenceframe.FrameSystemInputs{"arm": {0}}
		actualStart := referenceframe.FrameSystemInputs{"arm": {0.5}}
		div := compareToRecorded(recorded, armPlan(0.5, 1, 100), recordedStart, actualStart)

		test.That(t, div.startDrift, test.ShouldAlmostEqual, 0.5)
		test.That(t, div.diverged(), test.ShouldBeFalse)
	})
}
