package motionplan

import (
	"context"
	"sort"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/utils"
)

// InverseKinematics defines an interface which, provided with a goal position and seed inputs, will output all found
// solutions to the provided channel until cancelled or otherwise completes.
type InverseKinematicsSolver interface {
	solve(context.Context, chan<- []referenceframe.Input, spatial.Pose, []referenceframe.Input, Metric, int) error
	frame() referenceframe.Frame
	options() *PlannerOptions
}

type ikSolver struct {
	model  referenceframe.Frame
	logger golog.Logger
	opts   *PlannerOptions
}

func (ik *ikSolver) frame() referenceframe.Frame {
	return ik.model
}

func (ik *ikSolver) options() *PlannerOptions {
	return ik.opts
}

func BestIKSolution(
	ctx context.Context,
	ik InverseKinematicsSolver,
	goal spatialmath.Pose,
	seed []referenceframe.Input,
	randseed int,
) (*costNode, error) {
	solutions, err := BestNIKSolutions(ctx, ik, goal, seed, randseed, 1)
	if err != nil {
		return nil, err
	}
	return solutions[0], err
}

func BestNIKSolutions(
	ctx context.Context,
	ik InverseKinematicsSolver,
	goal spatialmath.Pose,
	input []referenceframe.Input,
	randseed int,
	nSolutions int,
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
				if len(solutions) >= nSolutions {
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
