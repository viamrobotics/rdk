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

// TestInspectIKSharedSolverRace exercises InspectIK on a minimal single-arm scene. It is expected
// to trip the race detector under `make test-go`.
//
// InspectIK creates one ik.NloptIK and starts one solver goroutine per seed, but never waits for
// a goroutine before starting the next one: its sync.WaitGroup is never Wait()ed on, and
// cancelling the per-seed context only stops NloptIK.Solve at its next loop-condition check, so
// the in-flight nlopt.Optimize call still completes. Two goroutines therefore end up inside Solve
// on the same solver, which both re-seeds (ik.rng.Seed) and draws from (generateRandomPositions)
// the solver's *rand.Rand -- a field documented in motionplan/ik/nlopt.go as being
// single-goroutine within a NloptIK.
//
// Asking for a single solution per seed is what makes the overlap reliable rather than
// timing-dependent: the read loop is satisfied by the solver's first send and moves on, while that
// goroutine is guaranteed to touch the shared rng at least once more (generateRandomPositions runs
// at the tail of every Solve iteration, after the send) with no happens-before edge to the next
// goroutine's ik.rng.Seed.
func TestInspectIKSharedSolverRace(t *testing.T) {
	model, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/xarm6.json"), "xarm6")
	test.That(t, err, test.ShouldBeNil)

	fs := referenceframe.NewEmptyFrameSystem("")
	test.That(t, fs.AddFrame(model, fs.World()), test.ShouldBeNil)

	startInputs := referenceframe.FrameSystemInputs{"xarm6": make([]referenceframe.Input, 6)}
	goalPoses := referenceframe.FrameSystemPoses{
		"xarm6": referenceframe.NewPoseInFrame(
			referenceframe.World,
			spatialmath.NewPose(r3.Vector{X: 200, Y: 100, Z: 300}, &spatialmath.OrientationVector{OZ: -1}),
		),
	}

	req := &armplanning.PlanRequest{
		FrameSystem:    fs,
		StartState:     armplanning.NewPlanState(nil, startInputs),
		Goals:          []*armplanning.PlanState{armplanning.NewPlanState(goalPoses, nil)},
		PlannerOptions: armplanning.NewBasicPlannerOptions(),
	}

	// InspectIK dereferences req.PlannerOptions without going through validatePlanRequest, and
	// NewBasicPlannerOptions is what that validation would have installed.
	table, err := mpserver.InspectIK(
		context.Background(),
		logging.NewBlankLogger("ik-race-test"),
		req,
		startInputs,
		goalPoses,
		1,
	)
	test.That(t, err, test.ShouldBeNil)

	// Overlapping goroutines require more than one seed; NewSolutionSolvingState always derives
	// several from a single-arm request, but assert it so the test cannot silently stop covering
	// the concurrent path.
	test.That(t, len(table.SeedResults), test.ShouldBeGreaterThan, 1)
}
