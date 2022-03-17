package motionplan

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/geo/r3"

	frame "go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/test"
)

func Test2DPlan(t *testing.T) {
	// Test Map:
	//      - bounds are from (-10, -10) to (10, 10)
	//      - obstacle from (-4, 0) to (4, 10)
	// ------------------------
	// | *      |    |      + |
	// |        |    |        |
	// |        |    |        |
	// |        |    |        |
	// |        |    |        |
	// |        ------        |
	// |                      |
	// |                      |
	// |                      |
	// |                      |
	// ------------------------

	// setup problem
	start := frame.FloatsToInputs([]float64{-9., 9.})
	goal := spatial.PoseToProtobuf(spatial.NewPoseFromPoint(r3.Vector{X: 9, Y: 9, Z: 0}))
	obstacles := buildObstacles(t)
	model := buildModel(t)

	// plan
	cbert, err := NewCBiRRTMotionPlanner(model, 1, logger)
	test.That(t, err, test.ShouldBeNil)
	opt := &PlannerOptions{metric: NewSquaredNormMetric(), pathDist: NewSquaredNormMetric()}
	constraint := NewCollisionConstraintFromFrame(model, obstacles)
	test.That(t, err, test.ShouldBeNil)
	opt.AddConstraint("collision", constraint)
	waypoints, err := cbert.Plan(context.Background(), goal, start, opt)
	test.That(t, err, test.ShouldBeNil)

	// print waypoints for debugging
	for _, waypoint := range waypoints {
		for _, dim := range waypoint {
			fmt.Printf("%f\t", dim.Value)
		}
		fmt.Printf("\n")
	}
	_ = constraint
}

func buildModel(t *testing.T) frame.Model {
	t.Helper()
	model := frame.NewSimpleModel()
	model.ChangeName("rover")
	physicalGeometry, err := spatial.NewBoxCreator(r3.Vector{X: 1, Y: 1, Z: 1}, spatial.NewZeroPose())
	test.That(t, err, test.ShouldBeNil)
	limits := []frame.Limit{{Min: -10, Max: 10}, {Min: -10, Max: 10}}
	modelFrame, err := frame.NewMobileFrame("base", limits, physicalGeometry)
	test.That(t, err, test.ShouldBeNil)
	model.OrdTransforms = append(model.OrdTransforms, modelFrame)
	return model
}

func buildObstacles(t *testing.T) map[string]spatial.Geometry {
	t.Helper()
	obstacles := map[string]spatial.Geometry{}
	box, err := spatial.NewBox(spatial.NewPoseFromPoint(r3.Vector{0, 5, 0}), r3.Vector{8, 10, 1})
	test.That(t, err, test.ShouldBeNil)
	obstacles["box"] = box
	return obstacles
}
