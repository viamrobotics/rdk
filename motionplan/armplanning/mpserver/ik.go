package mpserver

import (
	"context"
	"math/rand"
	"time"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/armplanning"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
)

// IKInspectCell describes a single IK solution emitted by one nlopt thread, scored and validated the
// same way getSolutions would score and validate it.
type IKInspectCell struct {
	// Cost is IK score, but without any "neutral bias".
	Cost float64
	// Exact is true when the solver considered the goal met (GoalDist below the goal threshold).
	Exact bool
	// Inputs is the solution configuration.
	Inputs *referenceframe.LinearInputs

	// Valid is true when the configuration itself passes all state constraints (no self-collision,
	// no obstacle collision, within bounds, ...). When false, StateError explains why.
	Valid      bool
	StateError error

	// CheckPathOK is true when the straight-line interpolation from the start configuration to this
	// solution passes all constraints. Only meaningful when Valid is true. When false, CheckPathError
	// explains why.
	CheckPathOK    bool
	CheckPathError error
}

type IKInspectTable struct {
	Rows [][]IKInspectCell
}

func InspectIK(ctx context.Context, logger logging.Logger,
	req *armplanning.PlanRequest,
	segmentStart referenceframe.FrameSystemInputs,
	segmentGoal referenceframe.FrameSystemPoses,
	numSolutions int,
) (*IKInspectTable, error) {
	var meta armplanning.PlanMeta
	pc, err := armplanning.NewPlanContext(ctx, logger, req, &meta)
	if err != nil {
		return nil, err
	}

	startLinear, err := pc.GetLinearInputsSchema().GetLinearInputs(segmentStart)
	if err != nil {
		return nil, err
	}

	psc, err := armplanning.NewPlanSegmentContext(ctx, pc, startLinear, segmentGoal)
	if err != nil {
		return nil, err
	}

	solver, err := ik.CreateNloptSolver(logger, -1, true, true, time.Minute)
	if err != nil {
		return nil, err
	}

	randSeed := rand.New(rand.NewSource(int64(req.PlannerOptions.RandomSeed)))
	minimizingFunc := pc.LinearizeFSMetric(req.PlannerOptions.GetGoalMetric(segmentGoal))
	retChan := make(chan *ik.Solution, 10)

	sss, err := armplanning.NewSolutionSolvingState(ctx, psc, logger)
	if err != nil {
		return nil, err
	}

	var seeds [][]float64 = sss.LinearSeeds
	var limits [][]referenceframe.Limit = sss.SeedLimits

	ctxWithCancel, cancel := context.WithCancel(ctx)
	var solveErr error
	go func() {
		_, _, solveErr = solver.Solve(ctxWithCancel, retChan, nil,
			seeds, limits, minimizingFunc, randSeed.Int())
		cancel()
	}()

	var ret IKInspectTable
	rowIdx := len(ret.Rows)
	ret.Rows = append(ret.Rows, make([]IKInspectCell, 0, 10))
	for len(ret.Rows[rowIdx]) < 10 {
		select {
		case <-ctxWithCancel.Done():
			// Solver error
			return nil, solveErr
		case solution := <-retChan:
			cells := &ret.Rows[rowIdx]
			inputs, err := pc.GetLinearInputsSchema().FloatsToInputs(solution.Configuration)
			if err != nil {
				return nil, err
			}

			*cells = append(*cells, IKInspectCell{
				Cost:        solution.Score,
				Exact:       solution.Exact,
				Inputs:      inputs,
				Valid:       true,
				CheckPathOK: true,
			})
		}
	}
	cancel()

	return &ret, nil
}
