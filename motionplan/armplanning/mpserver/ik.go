// nolint // This is a self-contained program. Most lint errors do not help find bugs.
package mpserver

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"braces.dev/errtrace"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/armplanning"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
)

// IKInspectCell describes a single IK solution emitted from one seed, scored and validated the
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
	Rows       [][]IKInspectCell
	SeedLabels []string
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
		return nil, errtrace.Wrap(err)
	}

	linearSchema := pc.GetLinearInputsSchema()
	startLinear, err := linearSchema.GetLinearInputs(segmentStart)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	psc, err := armplanning.NewPlanSegmentContext(ctx, pc, startLinear, segmentGoal)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	solver, err := ik.CreateNloptSolver(logger, -1, true, true, time.Second)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	randSeed := rand.New(rand.NewSource(int64(req.PlannerOptions.RandomSeed)))
	ikMinimizingFunc := pc.LinearizeFSMetric(req.PlannerOptions.GetGoalMetric(segmentGoal))
	retChan := make(chan *ik.Solution, 10)

	sss, err := armplanning.NewSolutionSolvingState(ctx, psc, logger)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	var ret IKInspectTable
	ret.SeedLabels = sss.SeedDescriptions
	for seedIdx, seed := range sss.LinearSeeds {
		seeds := [][]float64{seed}
		limits := [][]referenceframe.Limit{sss.SeedLimits[seedIdx]}

		ctxWithCancel, cancel := context.WithCancel(ctx)
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			_, _, _ = solver.Solve(ctxWithCancel, retChan, nil,
				seeds, limits, ikMinimizingFunc, randSeed.Int())
			cancel()
			wg.Done()
		}()

		rowIdx := len(ret.Rows)
		ret.Rows = append(ret.Rows, make([]IKInspectCell, 0, 10))
		cells := &ret.Rows[rowIdx]
		for len(ret.Rows[rowIdx]) < 10 {
			select {
			case <-ctxWithCancel.Done():
				// Solver error
				*cells = append(*cells, IKInspectCell{Cost: -1.0})
			case solution := <-retChan:
				inputs, err := linearSchema.FloatsToInputs(solution.Configuration)
				if err != nil {
					return nil, errtrace.Wrap(err)
				}

				_, finalStateErr := psc.Checker.CheckStateFSConstraints(ctx, &motionplan.StateFS{
					Configuration: inputs,
					FS:            req.FrameSystem,
				})

				var pathFeedback armplanning.PathFeedback
				pathError := psc.CheckPath(ctx, startLinear, inputs, false, &pathFeedback)

				stepArc := &motionplan.SegmentFS{
					StartConfiguration: startLinear,
					EndConfiguration:   inputs,
					FS:                 req.FrameSystem,
				}
				*cells = append(*cells, IKInspectCell{
					Cost: pc.ConfigurationDistanceFunc(stepArc) +
						armplanning.NeutralBias(linearSchema.GetLimits(), solution.Configuration),
					Exact:          solution.Exact,
					Inputs:         inputs,
					Valid:          finalStateErr == nil,
					StateError:     finalStateErr,
					CheckPathOK:    pathError == nil,
					CheckPathError: pathError,
				})
			}
		}
		cancel()
	}

	return &ret, nil
}
