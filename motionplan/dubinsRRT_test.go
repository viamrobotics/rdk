package motionplan

import (
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"

	frame "go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
)

func TestDubinsRRT(t *testing.T) {
	logger := golog.NewTestLogger(t)
	robotGeometry, err := spatial.NewBoxCreator(r3.Vector{X: 1, Y: 1, Z: 1}, spatial.NewZeroPose(), "")
	test.That(t, err, test.ShouldEqual, nil)
	limits := []frame.Limit{{Min: -10, Max: 10}, {Min: -10, Max: 10}}

	// build model
	model, err := frame.NewMobile2DFrame("name", limits, robotGeometry)
	test.That(t, err, test.ShouldEqual, nil)

	// setup planner
	d := Dubins{Radius: 0.6, PointSeparation: 0.1}
	dubins, err := NewDubinsRRTMotionPlanner(model, 1, logger, d)
	test.That(t, err, test.ShouldEqual, nil)

	start := []float64{0, 0, 0}
	goal := []float64{10, 0, 0}

	testDubin := func(obstacleGeometries map[string]spatial.Geometry) bool {
		opt := newBasicPlannerOptions()
		opt.AddConstraint("collision", NewCollisionConstraint(
			dubins.Frame(),
			frame.FloatsToInputs(start[0:2]),
			obstacleGeometries,
			map[string]spatial.Geometry{},
			true,
		))
		o := d.AllPaths(start, goal, false)
		return dubins.checkPath(
			&basicNode{q: frame.FloatsToInputs(start)},
			&basicNode{q: frame.FloatsToInputs(goal)},
			opt,
			&dubinPathAttrManager{nCPU: 1, d: d},
			o[0],
		)
	}

	// case with no obstacles
	test.That(t, testDubin(map[string]spatial.Geometry{}), test.ShouldBeTrue)

	// case with obstacles
	obstacleGeometries := map[string]spatial.Geometry{}
	box, err := spatial.NewBox(spatial.NewPoseFromPoint(
		r3.Vector{X: 5, Y: 0, Z: 0}), // Center of box
		r3.Vector{X: 1, Y: 20, Z: 1}, // Dimensions of box
		"")
	test.That(t, err, test.ShouldEqual, nil)
	obstacleGeometries["1"] = box
	test.That(t, testDubin(obstacleGeometries), test.ShouldBeFalse)
}
