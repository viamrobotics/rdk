package xarm

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/motionplan"
	pb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

var (
	home7 = referenceframe.FloatsToInputs([]float64{0, 0, 0, 0, 0, 0, 0})
	wbY   = -426.
)

// This will test solving the path to write the word "VIAM" on a whiteboard.
func TestWriteViam(t *testing.T) {
	fs := referenceframe.NewEmptySimpleFrameSystem("test")

	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	m, err := referenceframe.ParseJSONFile(utils.ResolveFile("component/arm/xarm/xArm7_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)

	err = fs.AddFrame(m, fs.World())
	test.That(t, err, test.ShouldBeNil)

	markerOffFrame, err := referenceframe.NewStaticFrame(
		"marker_offset",
		spatial.NewPoseFromOrientation(r3.Vector{}, &spatial.OrientationVectorDegrees{OY: -1, OZ: 1}),
	)
	test.That(t, err, test.ShouldBeNil)
	markerFrame, err := referenceframe.NewStaticFrame("marker", spatial.NewPoseFromPoint(r3.Vector{0, 0, 160}))
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(markerOffFrame, m)
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(markerFrame, markerOffFrame)
	test.That(t, err, test.ShouldBeNil)

	eraserOffFrame, err := referenceframe.NewStaticFrame(
		"eraser_offset",
		spatial.NewPoseFromOrientation(r3.Vector{}, &spatial.OrientationVectorDegrees{OY: 1, OZ: 1}),
	)
	test.That(t, err, test.ShouldBeNil)
	eraserFrame, err := referenceframe.NewStaticFrame("eraser", spatial.NewPoseFromPoint(r3.Vector{0, 0, 160}))
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(eraserOffFrame, m)
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(eraserFrame, eraserOffFrame)
	test.That(t, err, test.ShouldBeNil)

	moveFrame := eraserFrame
	fss := motionplan.NewSolvableFrameSystem(fs, logger)

	// draw pos start
	goal := spatial.NewPoseFromProtobuf(&pb.Pose{
		X:  230,
		Y:  wbY + 10,
		Z:  600,
		OY: -1,
	})

	seedMap := map[string][]referenceframe.Input{}

	seedMap[m.Name()] = home7
	curPos, err := fs.TransformFrame(seedMap, moveFrame, fs.World())
	test.That(t, err, test.ShouldBeNil)

	steps, err := fss.SolvePose(ctx, seedMap, goal, moveFrame, fs.World())
	test.That(t, err, test.ShouldBeNil)

	validOV := &spatial.OrientationVector{OX: 0, OY: -1, OZ: 0}

	goToGoal := func(seedMap map[string][]referenceframe.Input, goal spatial.Pose) map[string][]referenceframe.Input {
		curPos, _ = fs.TransformFrame(seedMap, moveFrame, fs.World())

		validFunc, gradFunc := motionplan.NewLineConstraint(curPos.Point(), goal.Point(), validOV, 0.3, 0.05)
		destGrad := motionplan.NewPoseFlexOVMetric(goal, 0.2)

		opt := motionplan.NewDefaultPlannerOptions()
		opt.SetPathDist(gradFunc)
		opt.SetMetric(destGrad)
		opt.AddConstraint("whiteboard", validFunc)

		waysteps, err := fss.SolvePoseWithOptions(ctx, seedMap, goal, moveFrame, fs.World(), opt)
		test.That(t, err, test.ShouldBeNil)
		return waysteps[len(waysteps)-1]
	}

	seed := steps[len(steps)-1]
	for _, goal = range viamPoints {
		seed = goToGoal(seed, goal)
	}
}

var viamPoints = []spatial.Pose{
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 200, Y: wbY + 1.5, Z: 595, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 120, Y: wbY + 1.5, Z: 595, OY: -1}),
}
