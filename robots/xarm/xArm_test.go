package xarm

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.viam.com/core/config"
	"go.viam.com/core/kinematics"
	pb "go.viam.com/core/proto/api/v1"
	frame "go.viam.com/core/referenceframe"
	spatial "go.viam.com/core/spatialmath"
	"go.viam.com/core/utils"

	"go.viam.com/core/motionplan"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

var home7 = frame.FloatsToInputs([]float64{0, 0, 0, 0, 0, 0, 0})

var wbY = -523.

//~ var wbY = -425.

func TestWrite1(t *testing.T) {

	fs := frame.NewEmptySimpleFrameSystem("test")

	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	m, err := kinematics.ParseJSONFile(utils.ResolveFile("robots/xarm/xArm7_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)

	err = fs.AddFrame(m, fs.World())
	test.That(t, err, test.ShouldBeNil)

	markerOffFrame, err := frame.NewStaticFrame("marker_offset", spatial.NewPoseFromOrientation(r3.Vector{}, &spatial.OrientationVectorDegrees{OY: -1, OZ: 1}))
	markerFrame, err := frame.NewStaticFrame("marker", spatial.NewPoseFromPoint(r3.Vector{0, 0, 160}))
	err = fs.AddFrame(markerOffFrame, m)
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(markerFrame, markerOffFrame)
	test.That(t, err, test.ShouldBeNil)

	eraserOffFrame, err := frame.NewStaticFrame("eraser_offset", spatial.NewPoseFromOrientation(r3.Vector{}, &spatial.OrientationVectorDegrees{OY: 1, OZ: 1}))
	eraserFrame, err := frame.NewStaticFrame("eraser", spatial.NewPoseFromPoint(r3.Vector{0, 0, 160}))
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(eraserOffFrame, m)
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(eraserFrame, eraserOffFrame)
	test.That(t, err, test.ShouldBeNil)

	//~ markerFrame, err := frame.NewStaticFrame("marker", spatial.NewPoseFromPoint(r3.Vector{0, 0, 135}))
	//~ err = fs.AddFrame(markerFrame, m)
	//~ test.That(t, err, test.ShouldBeNil)

	moveFrame := eraserFrame

	// Have to be able to update the motion planner from here
	mpFunc := func(f frame.Frame, logger golog.Logger, ncpu int) (motionplan.MotionPlanner, error) {
		// just in case frame changed
		mp, err := motionplan.NewCBiRRTMotionPlanner(f, logger, 4)
		//~ mp.AddConstraint("officewall", motionplan.DontHitPetersWallConstraint())

		return mp, err
	}

	fss := motionplan.NewSolvableFrameSystem(fs, logger)

	fss.SetPlannerGen(mpFunc)

	arm, err := NewxArm(ctx, config.Component{Host: "10.0.0.98"}, logger, 7)
	// home
	//~ arm.MoveToJointPositions(ctx, frame.InputsToJointPos(home7))

	// draw pos start
	goal := spatial.NewPoseFromProtobuf(&pb.Pose{
		X:  480,
		Y:  wbY,
		Z:  600,
		OY: -1,
	})

	seedMap := map[string][]frame.Input{}

	jPos, err := arm.CurrentJointPositions(ctx)
	seedMap[m.Name()] = frame.JointPosToInputs(jPos)
	curPos, _ := fs.TransformFrame(seedMap, moveFrame, fs.World())

	fmt.Println("curpos", spatial.PoseToProtobuf(curPos))

	steps, err := fss.SolvePose(ctx, seedMap, goal, moveFrame, fs.World())
	test.That(t, err, test.ShouldBeNil)

	fmt.Println("steps, err", steps, err)

	validOV := &spatial.OrientationVector{OX: 0, OY: -1, OZ: 0}

	waypoints := make(chan []frame.Input, 9999)

	goToGoal := func(seedMap map[string][]frame.Input, goal spatial.Pose) map[string][]frame.Input {

		curPos, _ = fs.TransformFrame(seedMap, moveFrame, fs.World())

		validFunc, gradFunc := motionplan.NewLineConstraintAndGradient(curPos.Point(), goal.Point(), validOV)
		destGrad := motionplan.NewPoseFlexOVGradient(goal, 0.15)

		// update constraints
		mpFunc = func(f frame.Frame, logger golog.Logger, ncpu int) (motionplan.MotionPlanner, error) {
			// just in case frame changed
			mp, err := motionplan.NewCBiRRTMotionPlanner(f, logger, 4)
			mp.SetPathDistFunc(gradFunc)
			mp.SetGoalDistFunc(destGrad)
			//~ mp.AddConstraint("officewall", motionplan.DontHitPetersWallConstraint())
			mp.AddConstraint("whiteboard", validFunc)

			return mp, err
		}
		fss.SetPlannerGen(mpFunc)

		waysteps, err := fss.SolvePose(ctx, seedMap, goal, moveFrame, fs.World())
		test.That(t, err, test.ShouldBeNil)
		for _, waystep := range waysteps {
			waypoints <- waystep[m.Name()]
		}
		return waysteps[len(waysteps)-1]
	}

	go func() {
		seed := steps[len(steps)-1]
		for _, goal = range viamPoints {
			seed = goToGoal(seed, goal)
		}
	}()

	for _, step := range steps {
		arm.MoveToJointPositions(ctx, frame.InputsToJointPos(step[m.Name()]))
	}
	for {
		select {
		case waypoint := <-waypoints:
			arm.MoveToJointPositions(ctx, frame.InputsToJointPos(waypoint))
		default:
			time.Sleep(1000 * time.Millisecond)
		}
	}
}

// Write out the word "VIAM"
var viamPoints = []spatial.Pose{
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 440, Y: wbY, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 400, Y: wbY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 400, Y: wbY + 10, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 380, Y: wbY + 10, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 380, Y: wbY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 380, Y: wbY, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 380, Y: wbY + 10, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 360, Y: wbY + 10, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 360, Y: wbY, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 320, Y: wbY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 280, Y: wbY, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 280, Y: wbY + 10, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 340, Y: wbY + 10, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 340, Y: wbY + 10, Z: 550, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 340, Y: wbY, Z: 550, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 300, Y: wbY, Z: 550, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 300, Y: wbY + 10, Z: 550, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 260, Y: wbY + 10, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 260, Y: wbY, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 230, Y: wbY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 200, Y: wbY, Z: 500, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 170, Y: wbY, Z: 600, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 140, Y: wbY, Z: 500, OY: -1}),
}
