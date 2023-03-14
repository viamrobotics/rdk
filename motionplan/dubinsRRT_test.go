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
	robotGeometry, err := spatial.NewBox(spatial.NewZeroPose(), r3.Vector{X: 1, Y: 1, Z: 1}, "")
	test.That(t, err, test.ShouldEqual, nil)
	limits := []frame.Limit{{Min: -10, Max: 10}, {Min: -10, Max: 10}}

	// build model
	model, err := frame.NewMobile2DFrame("name", limits, robotGeometry)
	test.That(t, err, test.ShouldEqual, nil)

	// add it to a frame system
	fs := frame.NewEmptySimpleFrameSystem("test")
	err = fs.AddFrame(model, fs.Frame(frame.World))
	test.That(t, err, test.ShouldEqual, nil)

	// setup planner
	d := Dubins{Radius: 0.6, PointSeparation: 0.1}
	dubins, err := NewDubinsRRTMotionPlanner(model, 1, logger, d)
	test.That(t, err, test.ShouldEqual, nil)

	start := []float64{0, 0, 0}
	goal := []float64{10, 0, 0}

	testDubin := func(worldState *frame.WorldState) bool {
		opt := newBasicPlannerOptions()
		collisionConstraint, err := newObstacleConstraint(dubins.Frame(), fs, worldState, frame.StartPositions(fs), nil, true)
		if err != nil {
			return false
		}
		opt.AddConstraint("collision", collisionConstraint)
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
	test.That(t, testDubin(&frame.WorldState{}), test.ShouldBeTrue)

	// case with obstacles
	box, err := spatial.NewBox(spatial.NewPoseFromPoint(
		r3.Vector{X: 5, Y: 0, Z: 0}), // Center of box
		r3.Vector{X: 1, Y: 20, Z: 1}, // Dimensions of box
		"")
	test.That(t, err, test.ShouldEqual, nil)
	obstacleGeometries := []spatial.Geometry{box}
	worldState := &frame.WorldState{Obstacles: []*frame.GeometriesInFrame{frame.NewGeometriesInFrame(frame.World, obstacleGeometries)}}
	test.That(t, testDubin(worldState), test.ShouldBeFalse)
}
