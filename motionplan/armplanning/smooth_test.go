package armplanning

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

type smoothNodeJSON struct {
	Inputs referenceframe.FrameSystemInputs `json:"inputs"`
}

func loadTestSmoothNodes(t testing.TB) []*node {
	t.Helper()
	data, err := os.ReadFile("data/smooth-nodes.json")
	test.That(t, err, test.ShouldBeNil)

	var raw []smoothNodeJSON
	test.That(t, json.Unmarshal(data, &raw), test.ShouldBeNil)

	nodes := make([]*node, len(raw))
	for i, r := range raw {
		nodes[i] = &node{
			inputs: r.Inputs.ToLinearInputs(),
		}
	}
	return nodes
}

func TestSmoothPlans1(t *testing.T) {
	t.Parallel()
	testSmoothNodes := loadTestSmoothNodes(t)
	test.That(t, len(testSmoothNodes), test.ShouldEqual, 62)

	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	req, err := ReadRequestFromFile("data/wine-crazy-touch.json")
	test.That(t, err, test.ShouldBeNil)
	req.myTestOptions.doNotCloseObstacles = true

	pc, err := NewPlanContext(ctx, logger, req, &PlanMeta{})
	test.That(t, err, test.ShouldBeNil)

	psc, err := NewPlanSegmentContext(ctx, pc, req.StartState.LinearConfiguration(), req.Goals[0].Poses())
	test.That(t, err, test.ShouldBeNil)

	// Convert []*node to []referenceframe.FrameSystemInputs for smoothPath
	inputSlice := make([]*referenceframe.LinearInputs, len(testSmoothNodes))
	for i, n := range testSmoothNodes {
		inputSlice[i] = n.inputs
	}

	nodes, _, err := smoothPath(ctx, psc, inputSlice)
	test.That(t, err, test.ShouldBeNil)

	for idx, n := range nodes {
		logger.Infof("%d : %v", idx, n.Get("arm-left"))
	}
	// Smoothing reduces 62 waypoints to 3, then addCloseObstacleWaypoints adds 5 more
	// where the path comes within 5mm of obstacles
	test.That(t, len(nodes), test.ShouldEqual, 3)
}

func TestSmoothMultiArms(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	fs, startJoints, goalJoints, req := nudgeBlockedScene(t)
	idle, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/xarm6.json"), "idle")
	test.That(t, err, test.ShouldBeNil)

	mount, err := referenceframe.NewStaticFrame("idle-mount", spatialmath.NewPoseFromPoint(r3.Vector{X: 2000}))
	test.That(t, err, test.ShouldBeNil)

	req.StartState = NewPlanState(nil, referenceframe.FrameSystemInputs{"arm": startJoints})
	req.Goals = []*PlanState{NewPlanState(nil, referenceframe.FrameSystemInputs{"arm": goalJoints})}

	plan, _, err := PlanMotion(ctx, logger.Sublogger("origPlanning"), req)
	test.That(t, err, test.ShouldBeNil)

	// The purpose of the "nudgeBlockedScene" is requiring the arm to work around a slight
	// obstacle. Resulting in an intermediate waypoint that cannot be removed/smoothed.
	traj := plan.Trajectory()
	test.That(t, len(traj), test.ShouldBeGreaterThanOrEqualTo, 3)

	// Alter the framesystem to include a new arm.
	err = fs.AddFrame(mount, fs.World())
	test.That(t, err, test.ShouldBeNil)

	err = fs.AddFrame(idle, mount)
	test.That(t, err, test.ShouldBeNil)

	// Also rebuild the plan request and contexts to include this arm's start/end states.
	req.StartState = NewPlanState(nil, referenceframe.FrameSystemInputs{"arm": startJoints, "idle": startJoints})
	req.Goals = []*PlanState{NewPlanState(nil, referenceframe.FrameSystemInputs{"arm": goalJoints, "idle": startJoints})}

	planMeta := PlanMeta{}
	pc, err := NewPlanContext(ctx, logger.Sublogger("smoothing"), req, &planMeta)
	test.That(t, err, test.ShouldBeNil)

	goalsAsPoses, err := req.Goals[0].ComputePoses(ctx, fs)
	test.That(t, err, test.ShouldBeNil)

	psc, err := NewPlanSegmentContext(ctx, pc, req.StartState.LinearConfiguration(), goalsAsPoses)
	test.That(t, err, test.ShouldBeNil)

	// Fabricte a solution with the real arm solution, and an unnecessary movement for the "idle"
	// arm.
	//
	// Notably, the all-0 value for this is different from the `startJoints` values. Resulting in an
	// unsmoothed motion that can benefit from smoothing.
	unnecessaryStep := make([]referenceframe.Input, len(startJoints))
	test.That(t, unnecessaryStep, test.ShouldNotResemble, startJoints)
	for trajIdx, _ := range traj {
		if trajIdx == 0 || trajIdx+1 == len(traj) {
			// Declare that the idle arm starts and ends at the same position.
			traj[trajIdx]["idle"] = startJoints
		} else {
			traj[trajIdx]["idle"] = unnecessaryStep
		}
	}

	trajAsLinearInputs := make([]*referenceframe.LinearInputs, len(traj))
	for trajIdx, fsi := range traj {
		trajAsLinearInputs[trajIdx] = fsi.ToLinearInputs()
	}

	smoothingFails := smoothPathSimple(ctx, psc, trajAsLinearInputs)
	for stepIdx, step := range smoothingFails {
		// The above call may have removed some waypoints as the moving arm did not need
		// them. However, the idle arm will continue to go through its existing unnecessary
		// positions for waypoints unable to be removed.
		if stepIdx == 0 || stepIdx+1 == len(smoothingFails) {
			test.That(t, step.Get("idle"), test.ShouldResemble, startJoints)
		} else {
			test.That(t, step.Get("idle"), test.ShouldResemble, unnecessaryStep)
		}
	}

	// Rename the variable for legitibility -- the function modifies in place.
	smoothingSucceeds := smoothingFails
	// But if we do an arm-aware pass, the idle arm should stay in its start/goal position.
	smoothMultiArms(ctx, psc, smoothingSucceeds)
	for _, step := range smoothingSucceeds {
		test.That(t, step.Get("idle"), test.ShouldResemble, startJoints)
	}
}

func BenchmarkSmoothPlans1(b *testing.B) {
	// go test -bench Smooth -benchtime 5s -cpuprofile cpu.out && go tool pprof cpu.out
	testSmoothNodes := loadTestSmoothNodes(b)
	test.That(b, len(testSmoothNodes), test.ShouldEqual, 62)

	ctx := context.Background()
	logger := logging.NewTestLogger(b)

	req, err := ReadRequestFromFile("data/wine-crazy-touch.json")
	req.myTestOptions.doNotCloseObstacles = true
	test.That(b, err, test.ShouldBeNil)

	pc, err := NewPlanContext(ctx, logger, req, &PlanMeta{})
	test.That(b, err, test.ShouldBeNil)

	psc, err := NewPlanSegmentContext(ctx, pc, req.StartState.LinearConfiguration(), req.Goals[0].Poses())
	test.That(b, err, test.ShouldBeNil)

	// Convert []*node to []referenceframe.FrameSystemInputs for smoothPath
	inputSlice := make([]*referenceframe.LinearInputs, len(testSmoothNodes))
	for i, n := range testSmoothNodes {
		inputSlice[i] = n.inputs
	}

	b.ResetTimer()
	for b.Loop() {
		nodes, _, err := smoothPath(ctx, psc, inputSlice)
		test.That(b, err, test.ShouldBeNil)
		test.That(b, len(nodes), test.ShouldEqual, 5)
	}
}
