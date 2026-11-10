//go:build !no_cgo && !windows

package mpserver_test

import (
	"context"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/armplanning"
	"go.viam.com/rdk/motionplan/armplanning/mpserver"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

const xarm6Frame = "xarm6"

func xarm6FrameSystem(t *testing.T) (*referenceframe.FrameSystem, referenceframe.Model) {
	t.Helper()

	model, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/xarm6.json"), xarm6Frame)
	test.That(t, err, test.ShouldBeNil)

	fs := referenceframe.NewEmptyFrameSystem("")
	test.That(t, fs.AddFrame(model, fs.World()), test.ShouldBeNil)

	return fs, model
}

func inspectRequest(
	fs *referenceframe.FrameSystem,
	start referenceframe.FrameSystemInputs,
	goal spatialmath.Pose,
) (*armplanning.PlanRequest, referenceframe.FrameSystemPoses) {
	goalPoses := referenceframe.FrameSystemPoses{
		xarm6Frame: referenceframe.NewPoseInFrame(referenceframe.World, goal),
	}
	// InspectIK reads req.PlannerOptions without going through validatePlanRequest, which is what
	// would otherwise install the defaults.
	req := &armplanning.PlanRequest{
		FrameSystem:    fs,
		StartState:     armplanning.NewPlanState(nil, start),
		Goals:          []*armplanning.PlanState{armplanning.NewPlanState(goalPoses, nil)},
		PlannerOptions: armplanning.NewBasicPlannerOptions(),
	}
	return req, goalPoses
}

func homeInputs() referenceframe.FrameSystemInputs {
	return referenceframe.FrameSystemInputs{xarm6Frame: make([]referenceframe.Input, 6)}
}

// TestInspectIKSharedSolverRace guards the per-seed join in InspectIK: every seed reuses one
// ik.NloptIK, whose *rand.Rand is single-goroutine within a NloptIK, so a seed's solver goroutine
// must be joined before the next seed starts.
//
// numSolutions is 1 because that is what makes the overlap reliable rather than timing-dependent:
// the read loop is satisfied by the solver's first send and moves on while that goroutine is still
// running, and generateRandomPositions touches the shared rng at the tail of every Solve iteration.
func TestInspectIKSharedSolverRace(t *testing.T) {
	fs, _ := xarm6FrameSystem(t)
	start := homeInputs()
	req, goalPoses := inspectRequest(fs, start,
		spatialmath.NewPose(r3.Vector{X: 200, Y: 100, Z: 300}, &spatialmath.OrientationVector{OZ: -1}))

	table, err := mpserver.InspectIK(context.Background(), logging.NewBlankLogger("ik-race-test"), req, start, goalPoses, 1)
	test.That(t, err, test.ShouldBeNil)

	// More than one seed is what puts two solver goroutines in play; assert it so the test cannot
	// silently stop covering the concurrent path.
	test.That(t, len(table.SeedResults), test.ShouldBeGreaterThan, 1)
	for _, row := range table.SeedResults {
		test.That(t, len(row), test.ShouldEqual, 1)
	}
}
