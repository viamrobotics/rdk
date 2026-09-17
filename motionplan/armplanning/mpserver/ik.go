package mpserver

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"sync"
	"time"

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

	// CheckPathFeedback carries diagnostics from the CheckPath call, including the last
	// configuration along the interpolated path that still satisfied all constraints. Only
	// meaningful when CheckPathOK is false.
	CheckPathFeedback armplanning.PathFeedback
}

// limitSeedPoint bounds random restarts on joints with infinite limits. Mirrors
// ik.defaultLimitSeedPoint, which is unexported.
const limitSeedPoint = 999

//nolint:revive
type IKInspectTable struct {
	SeedResults [][]IKInspectCell
	SeedLabels  []string
}

//nolint:revive
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

	linearSchema := pc.GetLinearInputsSchema()
	startLinear, err := linearSchema.GetLinearInputs(segmentStart)
	if err != nil {
		return nil, err
	}

	psc, err := armplanning.NewPlanSegmentContext(ctx, pc, startLinear, segmentGoal)
	if err != nil {
		return nil, err
	}

	ikMinimizingFunc := pc.LinearizeFSMetric(req.PlannerOptions.GetGoalMetric(segmentGoal))

	sss, err := armplanning.NewSolutionSolvingState(ctx, psc, logger)
	if err != nil {
		return nil, err
	}

	//nolint: gosec
	randSeed := rand.New(rand.NewSource(int64(req.PlannerOptions.RandomSeed)))
	errs := make([]error, len(sss.LinearSeeds))

	var ret IKInspectTable
	ret.SeedLabels = sss.SeedDescriptions
	ret.SeedResults = make([][]IKInspectCell, len(sss.LinearSeeds))
	wg := sync.WaitGroup{}
	for seedIdx, seed := range sss.LinearSeeds {
		limits := sss.SeedLimits[seedIdx]
		solver, err := ik.CreateNloptSolver(logger, -1, true, true, time.Second)
		if err != nil {
			errs[seedIdx] = err
			break
		}
		// Drawn here so restart streams follow seed order rather than goroutine scheduling, and so
		// no two goroutines share a *rand.Rand.
		//nolint: gosec
		seedRand := rand.New(rand.NewSource(int64(randSeed.Int())))

		wg.Add(1)
		go func() {
			defer wg.Done()

			// SolveOnce has no internal budget the way Solve does, so a seed that cannot reach the
			// goal would retry forever.
			seedCtx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()

			// SolveOnce optimizes from exactly the configuration it is handed, so the random
			// restarts Solve does internally have to happen here: the seed first, then random
			// configurations. Without them every attempt re-derives the same solution.
			attempt := seed

			cells := make([]IKInspectCell, 0, numSolutions)
			for len(cells) < numSolutions && seedCtx.Err() == nil {
				solution, err := solver.SolveOnce(seedCtx, ikMinimizingFunc, attempt, limits)
				attempt = randomConfiguration(seedRand, limits)
				if err != nil {
					// An attempt that misses the goal threshold is ordinary rather than fatal;
					// Solve would simply have retried from the next random seed.
					continue
				}

				inputs, err := linearSchema.FloatsToInputs(solution)
				if err != nil {
					errs[seedIdx] = err
					break
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
				cells = append(cells, IKInspectCell{
					Cost: pc.ConfigurationDistanceFunc(stepArc) +
						armplanning.NeutralBias(linearSchema.GetLimits(), solution),
					// The solver is built with exact=true, so SolveOnce only returns
					// configurations that already met the goal threshold.
					Exact:             true,
					Inputs:            inputs,
					Valid:             finalStateErr == nil,
					StateError:        finalStateErr,
					CheckPathOK:       pathError == nil,
					CheckPathError:    pathError,
					CheckPathFeedback: pathFeedback,
				})
			}

			for len(cells) < numSolutions {
				cells = append(cells, IKInspectCell{Cost: -1.0})
			}
			ret.SeedResults[seedIdx] = cells
		}()
	}

	wg.Wait()

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	return &ret, nil
}

// randomConfiguration samples uniformly within limits, mirroring the restart sampling Solve does
// internally via ik.generateRandomPositions (unexported). Infinite bounds collapse to a finite
// window so sampling stays meaningful.
func randomConfiguration(randSeed *rand.Rand, limits []referenceframe.Limit) []float64 {
	pos := make([]float64, len(limits))
	for i, limit := range limits {
		lower, upper := limit.Min, limit.Max
		if math.IsInf(lower, -1) {
			lower = -limitSeedPoint
		}
		if math.IsInf(upper, 1) {
			upper = limitSeedPoint
		}
		pos[i] = randSeed.Float64()*math.Abs(upper-lower) + lower
	}
	return pos
}
