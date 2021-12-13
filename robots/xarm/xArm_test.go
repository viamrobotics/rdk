package xarm

import (
	"context"
	"testing"

	pb "go.viam.com/core/proto/api/common/v1"
	frame "go.viam.com/core/referenceframe"
	spatial "go.viam.com/core/spatialmath"
	"go.viam.com/core/utils"

	"go.viam.com/core/motionplan"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

var (
	home7 = frame.FloatsToInputs([]float64{0, 0, 0, 0, 0, 0, 0})
	wbY   = -426.
)

// This will test solving the path to write the word "VIAM" on a whiteboard
func TestWriteViam(t *testing.T) {

	fs := frame.NewEmptySimpleFrameSystem("test")

	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	m, err := frame.ParseJSONFile(utils.ResolveFile("robots/xarm/xArm7_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)

	err = fs.AddFrame(m, fs.World())
	test.That(t, err, test.ShouldBeNil)

	markerOffFrame, err := frame.NewStaticFrame("marker_offset", spatial.NewPoseFromOrientation(r3.Vector{}, &spatial.OrientationVectorDegrees{OY: -1, OZ: 1}))
	test.That(t, err, test.ShouldBeNil)
	markerFrame, err := frame.NewStaticFrame("marker", spatial.NewPoseFromPoint(r3.Vector{0, 0, 160}))
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(markerOffFrame, m)
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(markerFrame, markerOffFrame)
	test.That(t, err, test.ShouldBeNil)

	eraserOffFrame, err := frame.NewStaticFrame("eraser_offset", spatial.NewPoseFromOrientation(r3.Vector{}, &spatial.OrientationVectorDegrees{OY: 1, OZ: 1}))
	test.That(t, err, test.ShouldBeNil)
	eraserFrame, err := frame.NewStaticFrame("eraser", spatial.NewPoseFromPoint(r3.Vector{0, 0, 160}))
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

	seedMap := map[string][]frame.Input{}

	seedMap[m.Name()] = home7
	curPos, err := fs.TransformFrame(seedMap, moveFrame, fs.World())
	test.That(t, err, test.ShouldBeNil)

	steps, err := fss.SolvePose(ctx, seedMap, goal, moveFrame, fs.World())
	test.That(t, err, test.ShouldBeNil)

	validOV := &spatial.OrientationVector{OX: 0, OY: -1, OZ: 0}

	goToGoal := func(seedMap map[string][]frame.Input, goal spatial.Pose) map[string][]frame.Input {

		curPos, _ = fs.TransformFrame(seedMap, moveFrame, fs.World())

		validFunc, gradFunc := motionplan.NewLineConstraintAndGradient(curPos.Point(), goal.Point(), validOV, 0.3, 0.05)
		destGrad := motionplan.NewPoseFlexOVMetric(goal, 0.2)

		// update constraints
		mpFunc := func(f frame.Frame, ncpu int, logger golog.Logger) (motionplan.MotionPlanner, error) {
			// just in case frame changed
			mp, err := motionplan.NewCBiRRTMotionPlanner(f, ncpu, logger)
			test.That(t, err, test.ShouldBeNil)
			opt := motionplan.NewDefaultPlannerOptions()

			opt.SetPathDist(gradFunc)
			opt.SetMetric(destGrad)
			opt.AddConstraint("whiteboard", validFunc)

			mp.SetOptions(opt)

			return mp, err
		}
		fss.SetPlannerGen(mpFunc)

		waysteps, err := fss.SolvePose(ctx, seedMap, goal, moveFrame, fs.World())
		test.That(t, err, test.ShouldBeNil)
		return waysteps[len(waysteps)-1]
	}

	seed := steps[len(steps)-1]
	for _, goal = range viamPoints {
		seed = goToGoal(seed, goal)
	}

}

var viamPoints = []spatial.Pose{
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 480, Y: wbY + 1.5, Z: 595, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 120, Y: wbY + 1.5, Z: 595, OY: -1}),
}
