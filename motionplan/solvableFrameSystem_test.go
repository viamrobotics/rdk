package motionplan

import (
	"context"
	"math"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"

	commonpb "go.viam.com/core/proto/api/common/v1"
	frame "go.viam.com/core/referenceframe"
	spatial "go.viam.com/core/spatialmath"
	"go.viam.com/core/utils"
)

func makeTestFS(t *testing.T) *SolvableFrameSystem {
	logger := golog.NewTestLogger(t)
	fs := frame.NewEmptySimpleFrameSystem("test")

	urOffset, err := frame.NewStaticFrame("urOffset", spatial.NewPoseFromPoint(r3.Vector{100, 100, 200}))
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(urOffset, fs.World())
	gantryOffset, err := frame.NewStaticFrame("gantryOffset", spatial.NewPoseFromPoint(r3.Vector{-50, -50, -200}))
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(gantryOffset, fs.World())

	limits := []frame.Limit{{math.Inf(-1), math.Inf(1)}, {math.Inf(-1), math.Inf(1)}}

	gantry, err := frame.NewTranslationalFrame("gantry", []bool{true, true, false}, limits)
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(gantry, gantryOffset)

	modelXarm, err := frame.ParseJSONFile(utils.ResolveFile("robots/xarm/xArm6_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(modelXarm, gantry)

	modelUR5e, err := frame.ParseJSONFile(utils.ResolveFile("robots/universalrobots/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(modelUR5e, urOffset)

	// Note that positive Z is always "forwards". If the position of the arm is such that it is pointing elsewhere,
	// the resulting translation will be similarly oriented
	urCamera, err := frame.NewStaticFrame("urCamera", spatial.NewPoseFromPoint(r3.Vector{0, 0, 30}))
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(urCamera, modelUR5e)

	// Add static frame for the gripper
	xArmVgripper, err := frame.NewStaticFrame("xArmVgripper", spatial.NewPoseFromPoint(r3.Vector{0, 0, 200}))
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(xArmVgripper, modelXarm)

	return NewSolvableFrameSystem(fs, logger)
}

func TestFrameSystemSolver(t *testing.T) {
	solver := makeTestFS(t)
	positions := frame.StartPositions(solver)

	pointXarmGripper := r3.Vector{157., -50, -288}

	transformPoint, err := solver.TransformFrame(positions, solver.GetFrame("xArmVgripper"), solver.GetFrame(frame.World))

	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint.Point().X, test.ShouldAlmostEqual, pointXarmGripper.X)
	test.That(t, transformPoint.Point().Y, test.ShouldAlmostEqual, pointXarmGripper.Y)
	test.That(t, transformPoint.Point().Z, test.ShouldAlmostEqual, pointXarmGripper.Z)

	// Set a goal such that the gantry and arm must both be used to solve
	goal1 := &commonpb.Pose{
		X:     257,
		Y:     2100,
		Z:     -300,
		Theta: 0,
		OX:    0,
		OY:    0,
		OZ:    -1,
	}
	newPos, err := solver.SolvePose(context.Background(), positions, spatial.NewPoseFromProtobuf(goal1), solver.GetFrame("xArmVgripper"), solver.GetFrame(frame.World))
	test.That(t, err, test.ShouldBeNil)
	solvedPose, err := solver.TransformFrame(newPos[len(newPos)-1], solver.GetFrame("xArmVgripper"), solver.GetFrame(frame.World))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, solvedPose.Point().X, test.ShouldAlmostEqual, goal1.X, 0.01)
	test.That(t, solvedPose.Point().Y, test.ShouldAlmostEqual, goal1.Y, 0.01)
	test.That(t, solvedPose.Point().Z, test.ShouldAlmostEqual, goal1.Z, 0.01)

	// Solve such that the ur5 and xArm are pointing at each other, 60mm from gripper to camera
	goal2 := &commonpb.Pose{
		X:     0,
		Y:     0,
		Z:     60,
		Theta: 0,
		OX:    0,
		OY:    0,
		OZ:    -1,
	}
	newPos, err = solver.SolvePose(context.Background(), positions, spatial.NewPoseFromProtobuf(goal2), solver.GetFrame("xArmVgripper"), solver.GetFrame("urCamera"))
	test.That(t, err, test.ShouldBeNil)

	// Both frames should wind up at the goal relative to one another
	solvedPose, err = solver.TransformFrame(newPos[len(newPos)-1], solver.GetFrame("xArmVgripper"), solver.GetFrame("urCamera"))
	test.That(t, err, test.ShouldBeNil)
	solvedPose2, err := solver.TransformFrame(newPos[len(newPos)-1], solver.GetFrame("urCamera"), solver.GetFrame("xArmVgripper"))
	test.That(t, err, test.ShouldBeNil)

	test.That(t, solvedPose.Point().X, test.ShouldAlmostEqual, goal2.X, 0.1)
	test.That(t, solvedPose.Point().Y, test.ShouldAlmostEqual, goal2.Y, 0.1)
	test.That(t, solvedPose.Point().Z, test.ShouldAlmostEqual, goal2.Z, 0.1)
	test.That(t, solvedPose2.Point().X, test.ShouldAlmostEqual, goal2.X, 0.1)
	test.That(t, solvedPose2.Point().Y, test.ShouldAlmostEqual, goal2.Y, 0.1)
	test.That(t, solvedPose2.Point().Z, test.ShouldAlmostEqual, goal2.Z, 0.1)
}

func TestSliceUniq(t *testing.T) {
	solver := makeTestFS(t)
	slice := []frame.Frame{}
	slice = append(slice, solver.GetFrame("urCamera"))
	slice = append(slice, solver.GetFrame("gantryOffset"))
	slice = append(slice, solver.GetFrame("xArmVgripper"))
	slice = append(slice, solver.GetFrame("urCamera"))
	uniqd := uniqInPlaceSlice(slice)
	test.That(t, len(uniqd), test.ShouldEqual, 3)
}

func TestSolverFrame(t *testing.T) {
	// setup solverFrame with start and goal frames
	solver := makeTestFS(t)
	solveFrame := solver.GetFrame("UR5e")
	test.That(t, solveFrame, test.ShouldNotBeNil)
	sFrames, err := solver.TracebackFrame(solveFrame)
	test.That(t, err, test.ShouldBeNil)
	goalFrame := solver.GetFrame("xArm6")
	test.That(t, goalFrame, test.ShouldNotBeNil)
	gFrames, err := solver.TracebackFrame(goalFrame)
	test.That(t, err, test.ShouldBeNil)
	frames := uniqInPlaceSlice(append(sFrames, gFrames...))
	sf := &solverFrame{solveFrame.Name() + "_" + goalFrame.Name(), solver, frames, solveFrame, goalFrame}

	// get the VerboseTransform at the zero position
	inputs := frame.StartPositions(solver)
	poseMap, err := sf.VerboseTransform(sf.mapToSlice(inputs))
	test.That(t, err, test.ShouldBeNil)

	// test that the VerboseTransform outputs the same output as the basic Transform
	poseExpect, err := sf.Transform(sf.mapToSlice(inputs))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatial.AlmostCoincident(poseMap["UR5e:ee_link"], poseExpect), test.ShouldBeTrue)
}
