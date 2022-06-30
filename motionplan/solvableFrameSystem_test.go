package motionplan

import (
	"context"
	"math"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	frame "go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func makeTestFS(t *testing.T) *SolvableFrameSystem {
	t.Helper()
	logger := golog.NewTestLogger(t)
	fs := frame.NewEmptySimpleFrameSystem("test")

	urOffset, err := frame.NewStaticFrame("urOffset", spatial.NewPoseFromPoint(r3.Vector{100, 100, 200}))
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(urOffset, fs.World())
	gantryOffset, err := frame.NewStaticFrame("gantryOffset", spatial.NewPoseFromPoint(r3.Vector{-50, -50, -200}))
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(gantryOffset, fs.World())

	gantryX, err := frame.NewTranslationalFrame("gantryX", r3.Vector{1, 0, 0}, frame.Limit{math.Inf(-1), math.Inf(1)})
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(gantryX, gantryOffset)
	gantryY, err := frame.NewTranslationalFrame("gantryY", r3.Vector{0, 1, 0}, frame.Limit{math.Inf(-1), math.Inf(1)})
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(gantryY, gantryX)

	modelXarm, err := frame.ParseModelJSONFile(utils.ResolveFile("component/arm/xarm/xarm6_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(modelXarm, gantryY)

	modelUR5e, err := frame.ParseModelJSONFile(utils.ResolveFile("component/arm/universalrobots/ur5e.json"), "")
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
	pointXarmGripper := spatial.NewPoseFromPoint(r3.Vector{157., -50, -288})
	transformPoint, err := solver.Transform(positions, frame.NewPoseInFrame("xArmVgripper", spatial.NewZeroPose()), frame.World)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatial.PoseAlmostCoincident(transformPoint.(*frame.PoseInFrame).Pose(), pointXarmGripper), test.ShouldBeTrue)

	// Set a goal such that the gantry and arm must both be used to solve
	goal1 := spatial.NewPoseFromProtobuf(&commonpb.Pose{
		X:     257,
		Y:     2100,
		Z:     -300,
		Theta: 0,
		OX:    0,
		OY:    0,
		OZ:    -1,
	})
	newPos, err := solver.SolvePose(
		context.Background(),
		positions,
		goal1,
		"xArmVgripper",
		frame.World,
	)
	test.That(t, err, test.ShouldBeNil)
	solvedPose, err := solver.Transform(newPos[len(newPos)-1], frame.NewPoseInFrame("xArmVgripper", spatial.NewZeroPose()), frame.World)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatial.PoseAlmostCoincidentEps(solvedPose.(*frame.PoseInFrame).Pose(), goal1, 0.01), test.ShouldBeTrue)

	// Solve such that the ur5 and xArm are pointing at each other, 60mm from gripper to camera
	goal2 := spatial.NewPoseFromProtobuf(&commonpb.Pose{
		X:     0,
		Y:     0,
		Z:     60,
		Theta: 0,
		OX:    0,
		OY:    0,
		OZ:    -1,
	})
	newPos, err = solver.SolvePose(
		context.Background(),
		positions,
		goal2,
		"xArmVgripper",
		"urCamera",
	)
	test.That(t, err, test.ShouldBeNil)

	// Both frames should wind up at the goal relative to one another
	solvedPose, err = solver.Transform(newPos[len(newPos)-1], frame.NewPoseInFrame("xArmVgripper", spatial.NewZeroPose()), "urCamera")
	test.That(t, err, test.ShouldBeNil)
	solvedPose2, err := solver.Transform(newPos[len(newPos)-1], frame.NewPoseInFrame("urCamera", spatial.NewZeroPose()), "xArmVgripper")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatial.PoseAlmostCoincidentEps(solvedPose.(*frame.PoseInFrame).Pose(), goal2, 0.1), test.ShouldBeTrue)
	test.That(t, spatial.PoseAlmostCoincidentEps(solvedPose2.(*frame.PoseInFrame).Pose(), goal2, 0.1), test.ShouldBeTrue)
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
	solver := makeTestFS(t)
	goalFrame, err := frame.NewStaticFrame("goal", spatial.NewPoseFromPoint(r3.Vector{100, 100, 200}))
	test.That(t, err, test.ShouldBeNil)
	solver.AddFrame(goalFrame, solver.World())
	sFrames, err := solver.TracebackFrame(goalFrame)
	test.That(t, err, test.ShouldBeNil)
	solveFrame := solver.GetFrame("UR5e")
	test.That(t, solveFrame, test.ShouldNotBeNil)
	gFrames, err := solver.TracebackFrame(solveFrame)
	test.That(t, err, test.ShouldBeNil)
	frames := uniqInPlaceSlice(append(sFrames, gFrames...))
	sf := &solverFrame{"", solver, frames, solveFrame, goalFrame}

	// get the Geometry at the zero position and test that the output is correct
	inputs := sf.mapToSlice(frame.StartPositions(solver))
	tf, err := sf.Transform(inputs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatial.R3VectorAlmostEqual(tf.Point(), r3.Vector{-817.2, -232.9, 62.8}, 1e-8), test.ShouldBeTrue)
	geometries, _ := sf.Geometries(inputs)
	test.That(t, geometries, test.ShouldNotBeNil)
	jointExpected := spatial.NewPoseFromPoint(r3.Vector{-425, 0, 162.5})
	linkOffset := spatial.NewPoseFromPoint(r3.Vector{-190, 0, 0})
	poseExpect := spatial.Compose(spatial.Compose(jointExpected, linkOffset), spatial.NewPoseFromPoint(r3.Vector{100, 100, 200}))
	test.That(t, geometries.FrameName(), test.ShouldResemble, frame.World)
	test.That(t, spatial.PoseAlmostCoincident(geometries.Geometries()["UR5e:forearm_link"].Pose(), poseExpect), test.ShouldBeTrue)
}
