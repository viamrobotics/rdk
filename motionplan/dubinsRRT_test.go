package motionplan

import (
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"

	frame "go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
)

func TestCheckPathCollision(t *testing.T) {
	logger := golog.NewTestLogger(t)
	robotGeometry, err := spatial.NewBoxCreator(r3.Vector{X: 1, Y: 1, Z: 1}, spatial.NewZeroPose())
	test.That(t, err, test.ShouldEqual, nil)
	limits := []frame.Limit{{Min: -10, Max: 10}, {Min: -10, Max: 10}}

	// build model
	model, err := frame.NewMobile2DFrame("name", limits, robotGeometry)
	test.That(t, err, test.ShouldEqual, nil)

	// setup planner
	d := Dubins{Radius: 0.6, PointSeparation: 0.1}
	mp, err := NewDubinsRRTMotionPlanner(model, 1, logger, d)
	test.That(t, err, test.ShouldEqual, nil)

	dubins, ok := mp.(*DubinsRRTMotionPlanner)
	test.That(t, ok, test.ShouldEqual, true)

	obstacleGeometries := map[string]spatial.Geometry{}
	box, err := spatial.NewBox(spatial.NewPoseFromPoint(
		r3.Vector{X: 5, Y: 0, Z: 0}), // Center of box
		r3.Vector{X: 1, Y: 20, Z: 1}) // Dimensions of box
	test.That(t, err, test.ShouldEqual, nil)
	obstacleGeometries["1"] = box

	opt := NewDefaultPlannerOptions()
	opt.AddConstraint("collision", NewCollisionConstraint(dubins.Frame(), obstacleGeometries, map[string]spatial.Geometry{}))

	dm := &dubinPathAttrManager{
		nCPU: 1,
		d:    d,
	}

	start := make([]float64, 3)
	goal := make([]float64, 3)
	goal[0] = 10
	o := d.AllPaths(start, goal, false)

	startInputs := make([]frame.Input, 3)
	startInputs[0] = frame.Input{0}
	startInputs[1] = frame.Input{0}
	startInputs[2] = frame.Input{0}
	goalInputs := make([]frame.Input, 3)
	goalInputs[0] = frame.Input{10}
	goalInputs[1] = frame.Input{0}
	goalInputs[2] = frame.Input{0}
	isValid := dubins.checkPath(&configuration{inputs: startInputs},
		&configuration{inputs: goalInputs},
		opt,
		dm,
		o[0],
	)
	test.That(t, isValid, test.ShouldEqual, false)
}

func TestCheckPathNoCollision(t *testing.T) {
	logger := golog.NewTestLogger(t)
	robotGeometry, err := spatial.NewBoxCreator(r3.Vector{X: 1, Y: 1, Z: 1}, spatial.NewZeroPose())
	test.That(t, err, test.ShouldEqual, nil)
	limits := []frame.Limit{{Min: -10, Max: 10}, {Min: -10, Max: 10}}

	// build model
	model, err := frame.NewMobile2DFrame("name", limits, robotGeometry)
	test.That(t, err, test.ShouldEqual, nil)

	// setup planner
	d := Dubins{Radius: 0.6, PointSeparation: 0.1}
	mp, err := NewDubinsRRTMotionPlanner(model, 1, logger, d)
	test.That(t, err, test.ShouldEqual, nil)

	dubins, ok := mp.(*DubinsRRTMotionPlanner)
	test.That(t, ok, test.ShouldEqual, true)

	obstacleGeometries := map[string]spatial.Geometry{}

	opt := NewDefaultPlannerOptions()
	opt.AddConstraint("collision", NewCollisionConstraint(dubins.Frame(), obstacleGeometries, map[string]spatial.Geometry{}))

	dm := &dubinPathAttrManager{
		nCPU: 1,
		d:    d,
	}

	start := make([]float64, 3)
	goal := make([]float64, 3)
	goal[0] = 10
	o := d.AllPaths(start, goal, false)

	startInputs := make([]frame.Input, 3)
	startInputs[0] = frame.Input{0}
	startInputs[1] = frame.Input{0}
	startInputs[2] = frame.Input{0}
	goalInputs := make([]frame.Input, 3)
	goalInputs[0] = frame.Input{10}
	goalInputs[1] = frame.Input{0}
	goalInputs[2] = frame.Input{0}
	isValid := dubins.checkPath(&configuration{inputs: startInputs},
		&configuration{inputs: goalInputs},
		opt,
		dm,
		o[0],
	)
	test.That(t, isValid, test.ShouldEqual, true)
}
