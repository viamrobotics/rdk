package motionplan

import (
	"context"
	"testing"

	"github.com/golang/geo/r3"

	frame "go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/test"
)

func Test2DPlan(t *testing.T) {
	// Test Map:
	//      - bounds are from (-10, -10) to (10, 10)
	//      - obstacle from (-4, 4) to (4, 10)
	// ------------------------
	// | *      |    |      + |
	// |        |    |        |
	// |        |    |        |
	// |        |    |        |
	// |        ------        |
	// |          *           |
	// |                      |
	// |                      |
	// |                      |
	// ------------------------

	// setup problem parameters
	start := frame.FloatsToInputs([]float64{-9., 9.})
	goal := spatial.PoseToProtobuf(spatial.NewPoseFromPoint(r3.Vector{X: 9, Y: 9, Z: 0}))
	limits := []frame.Limit{{Min: -10, Max: 10}, {Min: -10, Max: 10}}

	// build obstacles
	obstacles := map[string]spatial.Geometry{}
	box, err := spatial.NewBox(spatial.NewPoseFromPoint(r3.Vector{0, 6, 0}), r3.Vector{8, 8, 1})
	test.That(t, err, test.ShouldBeNil)
	obstacles["box"] = box

	// build model
	physicalGeometry, err := spatial.NewBoxCreator(r3.Vector{X: 1, Y: 1, Z: 1}, spatial.NewZeroPose())
	test.That(t, err, test.ShouldBeNil)
	model, err := frame.NewMobileFrame("mobile-base", limits, physicalGeometry)
	test.That(t, err, test.ShouldBeNil)

	// plan
	cbert, err := NewCBiRRTMotionPlanner(model, 1, logger)
	test.That(t, err, test.ShouldBeNil)
	opt := NewDefaultPlannerOptions()
	constraint := NewCollisionConstraintFromFrame(model, obstacles)
	test.That(t, err, test.ShouldBeNil)
	opt.AddConstraint("collision", constraint)
	waypoints, err := cbert.Plan(context.Background(), goal, start, opt)
	test.That(t, err, test.ShouldBeNil)

	obstacle := obstacles["box"]
	workspace, err := spatial.NewBox(spatial.NewZeroPose(), r3.Vector{20, 20, 1})
	test.That(t, err, test.ShouldBeNil)
	for _, waypoint := range waypoints {
		pt := r3.Vector{waypoint[0].Value, waypoint[1].Value, 0}
		collides, err := obstacle.CollidesWith(spatial.NewPoint(pt))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, collides, test.ShouldBeFalse)
		inWorkspace, err := workspace.CollidesWith(spatial.NewPoint(pt))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, inWorkspace, test.ShouldBeTrue)
		logger.Debug("%f\t%f\n", pt.X, pt.Y)
	}
}

func buildModel(t *testing.T) frame.Frame {
	t.Helper()

	return modelFrame
}
