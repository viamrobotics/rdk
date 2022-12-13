package motionplan

import (
	"context"
	"math"
	"runtime"
	"sort"

	"github.com/edaniels/golog"
	"go.viam.com/utils"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// default values for inverse kinematics.
const (
	// If an IK solution scores below this much, return it immediately.
	defaultMinIkScore = 0.

	// Number of IK solutions that should be generated before stopping.
	defaultSolutionsToSeed = 50
)

var defaultNumThreads = runtime.NumCPU() / 2

type ikOptions struct {
	constraintHandler
	extra map[string]interface{}

	// Metric by which to measure nearness to the goal
	metric Metric

	// Solutions that score below this amount are considered "good enough" and returned immediately
	MinScore float64 `json:"min_ik_score"`

	// Max number of ik solutions to consider
	MaxSolutions int `json:"max_ik_solutions"`

	// Number of cpu cores to use
	NumThreads int `json:"num_threads"`
}

func newBasicIKOptions() *ikOptions {
	opts := &ikOptions{
		metric:       NewSquaredNormMetric(),
		MinScore:     defaultMinIkScore,
		MaxSolutions: defaultSolutionsToSeed,
		NumThreads:   defaultNumThreads,
	}

	opts.AddConstraint(defaultJointConstraint, NewJointConstraint(math.Inf(1)))
	return opts
}

func copyIKOptions(toCopy *ikOptions) *ikOptions {
	return &ikOptions{
		constraintHandler: toCopy.constraintHandler,
		extra:             deepAtomicCopyMap(toCopy.extra),
		metric:            toCopy.metric,
		MinScore:          toCopy.MinScore,
		MaxSolutions:      toCopy.MaxSolutions,
		NumThreads:        toCopy.NumThreads,
	}
}

// inverseKinematicsSolver defines an interface which is used to solve inverse kinematics queries.
type inverseKinematicsSolver interface {
	solve(context.Context, chan<- []referenceframe.Input, spatialmath.Pose, []referenceframe.Input, Metric, int) error
	frame() referenceframe.Frame
	options() *ikOptions
}

type ikSolver struct {
	model  referenceframe.Frame
	logger golog.Logger
	opts   *ikOptions
}

func (ik *ikSolver) frame() referenceframe.Frame {
	return ik.model
}

func (ik *ikSolver) options() *ikOptions {
	return ik.opts
}

// NewIKSolver instantiates an InverseKinematicsSolver according to the configuration parameters defined in ikConfig.
func newIKSolver(frame referenceframe.Frame, logger golog.Logger, opt *ikOptions) (inverseKinematicsSolver, error) {
	// infer IK solver to build based on number of threads allowed
	if opt.NumThreads <= 0 {
		opt.NumThreads = defaultNumThreads
	}
	if opt.NumThreads == 1 {
		return newNLOptIKSolver(frame, logger, opt)
	}
	return newEnsembleIKSolver(frame, logger, opt)
}

// BestIKSolutions takes an InverseKinematicsSolver and a goal location and calculates a number of solutions to achieve this goal, scored
// by proximity to some reference input that is also specified by the user.  Finally, a WorldState argument allows users to
// disallow or allow regions of state space through defining obstacles or interaction spaces respectively.
func BestIKSolutions(
	ctx context.Context,
	logger golog.Logger,
	frame referenceframe.Frame,
	fs referenceframe.FrameSystem,
	inputMap map[string][]referenceframe.Input,
	goal spatialmath.Pose,
	worldState *referenceframe.WorldState,
	randseed int,
	ikConfig map[string]interface{},
) ([][]referenceframe.Input, error) {
	manager, err := newPlanManager(logger, fs, frame, inputMap, goal, worldState, randseed, ikConfig)
	if err != nil {
		return nil, err
	}
	opt, err := manager.plannerOptionsFromConfig(nil, goal, ikConfig)
	if err != nil {
		return nil, err
	}
	ik, err := newIKSolver(frame, logger, opt.ikOptions)
	if err != nil {
		return nil, err
	}
	input, err := referenceframe.GetFrameInputs(frame, inputMap)
	if err != nil {
		return nil, err
	}
	getSolutions(ctx, ik, goal, input, randseed)
	return nil, nil
}

func getSolutions(
	ctx context.Context,
	ik inverseKinematicsSolver,
	goal spatialmath.Pose,
	input []referenceframe.Input,
	randseed int,
) ([]*costNode, error) {
	seedPos, err := ik.frame().Transform(input)
	if err != nil {
		return nil, err
	}
	goalPos := fixOvIncrement(goal, seedPos)

	solutionGen := make(chan []referenceframe.Input)
	ikErr := make(chan error, 1)
	defer func() { <-ikErr }()

	ctxWithCancel, cancel := context.WithCancel(ctx)
	defer cancel()

	// Spawn the IK solver to generate solutions until done
	utils.PanicCapturingGo(func() {
		defer close(ikErr)
		ikErr <- ik.solve(ctxWithCancel, solutionGen, goalPos, input, ik.options().metric, randseed)
	})

	solutions := map[float64][]referenceframe.Input{}

	// Solve the IK solver. Loop labels are required because `break` etc in a `select` will break only the `select`.
IK:
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		select {
		case step := <-solutionGen:
			cPass, cScore := ik.options().CheckConstraints(&ConstraintInput{
				StartPos:   seedPos,
				EndPos:     goalPos,
				StartInput: input,
				EndInput:   step,
				Frame:      ik.frame(),
			})
			endPass, _ := ik.options().CheckConstraints(&ConstraintInput{
				StartPos:   goalPos,
				EndPos:     goalPos,
				StartInput: step,
				EndInput:   step,
				Frame:      ik.frame(),
			})

			if cPass && endPass {
				if cScore < ik.options().MinScore && ik.options().MinScore > 0 {
					solutions = map[float64][]referenceframe.Input{}
					solutions[cScore] = step
					// good solution, stopping early
					break IK
				}

				solutions[cScore] = step
				if len(solutions) >= ik.options().MaxSolutions {
					// sufficient solutions found, stopping early
					break IK
				}
			}
			// Skip the return check below until we have nothing left to read from solutionGen
			continue IK
		default:
		}

		select {
		case <-ikErr:
			// If we have a return from the IK solver, there are no more solutions, so we finish processing above
			// until we've drained the channel
			break IK
		default:
		}
	}
	if len(solutions) == 0 {
		return nil, errIKSolve
	}

	keys := make([]float64, 0, len(solutions))
	for k := range solutions {
		keys = append(keys, k)
	}
	sort.Float64s(keys)

	orderedSolutions := make([]*costNode, 0)
	for _, key := range keys {
		orderedSolutions = append(orderedSolutions, newCostNode(solutions[key], key))
	}
	return orderedSolutions, nil
}
