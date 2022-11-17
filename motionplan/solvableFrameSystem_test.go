package motionplan

import (
	"context"
	"math"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"

	frame "go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func makeTestFS(t *testing.T) *SolvableFrameSystem {
	t.Helper()
	logger := golog.NewDebugLogger("test")
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

	modelXarm, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/xarm/xarm6_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(modelXarm, gantryY)

	modelUR5e, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/universalrobots/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(modelUR5e, urOffset)

	// Note that positive Z is always "forwards". If the position of the arm is such that it is pointing elsewhere,
	// the resulting translation will be similarly oriented
	urCamera, err := frame.NewStaticFrame("urCamera", spatial.NewPoseFromPoint(r3.Vector{0, 0, 30}))
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(urCamera, modelUR5e)

	// Add static frame for the gripper
	bc, _ := spatial.NewBoxCreator(r3.Vector{200, 200, 200}, spatial.NewPoseFromPoint(r3.Vector{Z: 100}), "")
	xArmVgripper, err := frame.NewStaticFrameWithGeometry("xArmVgripper", spatial.NewPoseFromPoint(r3.Vector{Z: 200}), bc)
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
	goal1 := spatial.NewPoseFromOrientation(r3.Vector{X: 257, Y: 2100, Z: -300}, &spatial.OrientationVectorDegrees{OZ: -1})
	newPos, err := solver.SolvePose(
		context.Background(),
		positions,
		frame.NewPoseInFrame(frame.World, goal1),
		"xArmVgripper",
	)
	test.That(t, err, test.ShouldBeNil)
	solvedPose, err := solver.Transform(newPos[len(newPos)-1], frame.NewPoseInFrame("xArmVgripper", spatial.NewZeroPose()), frame.World)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatial.PoseAlmostCoincidentEps(solvedPose.(*frame.PoseInFrame).Pose(), goal1, 0.01), test.ShouldBeTrue)

	// Solve such that the ur5 and xArm are pointing at each other, 60mm from gripper to camera
	goal2 := spatial.NewPoseFromOrientation(r3.Vector{Z: 60}, &spatial.OrientationVectorDegrees{OZ: -1})
	newPos, err = solver.SolvePose(
		context.Background(),
		positions,
		frame.NewPoseInFrame("urCamera", goal2),
		"xArmVgripper",
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
	slice = append(slice, solver.Frame("urCamera"))
	slice = append(slice, solver.Frame("gantryOffset"))
	slice = append(slice, solver.Frame("xArmVgripper"))
	slice = append(slice, solver.Frame("urCamera"))
	uniqd := uniqInPlaceSlice(slice)
	test.That(t, len(uniqd), test.ShouldEqual, 3)
}

func TestSolverFrameGeometries(t *testing.T) {
	solver := makeTestFS(t)
	sFrames, err := solver.TracebackFrame(solver.Frame("xArmVgripper"))
	test.That(t, err, test.ShouldBeNil)
	sf, err := newSolverFrame(solver, sFrames, frame.World, frame.StartPositions(solver))
	test.That(t, err, test.ShouldBeNil)

	position, err := sf.planSingleWaypoint(
		context.Background(),
		sf.sliceToMap(make([]frame.Input, len(sf.DoF()))),
		spatial.NewPoseFromPoint(r3.Vector{300, 300, 100}),
		nil,
		nil,
	)
	test.That(t, err, test.ShouldBeNil)
	// visualization.VisualizePlan(context.Background(), position, sf, nil)
	gf, _ := sf.Geometries(position[len(position)-1])
	test.That(t, gf, test.ShouldNotBeNil)
	gripperCenter := gf.Geometries()["xArmVgripper"].Pose().Point()
	test.That(t, spatial.R3VectorAlmostEqual(gripperCenter, r3.Vector{300, 300, 0}, 1e-2), test.ShouldBeTrue)
}

func TestMovementWithGripper(t *testing.T) {
	// TODO(rb): move these tests to a separate repo eventually, as they take up too much time for general CI pipeline
	t.Skip()

	// setup solverFrame and planning query
	solver := makeTestFS(t)
	solver.RemoveFrame(solver.Frame("urOffset"))
	sFrames, err := solver.TracebackFrame(solver.Frame("xArmVgripper"))
	test.That(t, err, test.ShouldBeNil)
	sf, err := newSolverFrame(solver, sFrames, frame.World, frame.StartPositions(solver))
	test.That(t, err, test.ShouldBeNil)
	goal := spatial.NewPoseFromOrientation(r3.Vector{500, 0, -300}, &spatial.OrientationVector{OZ: -1})
	zeroPosition := sf.sliceToMap(make([]frame.Input, len(sf.DoF())))

	// linearly plan with the gripper
	motionConfig := make(map[string]interface{})
	motionConfig["motion_profile"] = LinearMotionProfile
	solution, err := sf.planSingleWaypoint(context.Background(), zeroPosition, goal, nil, motionConfig)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, solution, test.ShouldNotBeNil)

	// plan around the obstacle with the gripper
	obstacle, err := spatial.NewBox(spatial.NewPoseFromPoint(r3.Vector{300, 0, -400}), r3.Vector{50, 500, 500}, "")
	test.That(t, err, test.ShouldBeNil)
	geometries := make(map[string]spatial.Geometry)
	geometries["obstacle"] = obstacle
	obstacles := []*commonpb.GeometriesInFrame{frame.GeometriesInFrameToProtobuf(frame.NewGeometriesInFrame(frame.World, geometries))}
	worldState := &commonpb.WorldState{Obstacles: obstacles}
	solution, err = sf.planSingleWaypoint(context.Background(), zeroPosition, goal, worldState, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, solution, test.ShouldNotBeNil)

	// plan with end of arm with gripper attached - this will fail
	sFrames, err = solver.TracebackFrame(solver.Frame("xArm6"))
	test.That(t, err, test.ShouldBeNil)
	sf, err = newSolverFrame(solver, sFrames, frame.World, frame.StartPositions(solver))
	test.That(t, err, test.ShouldBeNil)
	goal = spatial.NewPoseFromOrientation(r3.Vector{500, 0, -100}, &spatial.OrientationVector{OZ: -1})
	zeroPosition = sf.sliceToMap(make([]frame.Input, len(sf.DoF())))
	_, err = sf.planSingleWaypoint(context.Background(), zeroPosition, goal, worldState, motionConfig)
	test.That(t, err, test.ShouldNotBeNil)

	// remove linear constraint and try again
	solution, err = sf.planSingleWaypoint(context.Background(), zeroPosition, goal, worldState, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, solution, test.ShouldNotBeNil)

	// remove gripper and try with linear constraint
	solver.RemoveFrame(solver.Frame("xArmVgripper"))
	sFrames, err = solver.TracebackFrame(solver.Frame("xArm6"))
	test.That(t, err, test.ShouldBeNil)
	sf, err = newSolverFrame(solver, sFrames, frame.World, frame.StartPositions(solver))
	test.That(t, err, test.ShouldBeNil)
	zeroPosition = sf.sliceToMap(make([]frame.Input, len(sf.DoF())))
	solution, err = sf.planSingleWaypoint(context.Background(), zeroPosition, goal, worldState, motionConfig)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, solution, test.ShouldNotBeNil)
}
