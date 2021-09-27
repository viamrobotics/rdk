package kinematics

import (
	"context"
	"math"
	"math/rand"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"
	"github.com/go-nlopt/nlopt"
	"go.uber.org/multierr"

	pb "go.viam.com/core/proto/api/v1"
	frame "go.viam.com/core/referenceframe"
	spatial "go.viam.com/core/spatialmath"
)

// NloptIK TODO
type NloptIK struct {
	id            int
	model         frame.Frame
	lowerBound    []float64
	upperBound    []float64
	iterations    int
	maxIterations int
	epsilon       float64
	goals         []goal
	opt           *nlopt.NLopt
	logger        golog.Logger
	jump          float64
	randSeed      *rand.Rand
	SolveWeights  SolverDistanceWeights
}

// CreateNloptIKSolver TODO
func CreateNloptIKSolver(mdl frame.Frame, logger golog.Logger, id int) *NloptIK {
	ik := &NloptIK{id: id, logger: logger}
	ik.randSeed = rand.New(rand.NewSource(1))
	ik.model = mdl
	// How close we want to get to the goal
	ik.epsilon = 0.001
	// The absolute smallest value able to be represented by a float64
	floatEpsilon := math.Nextafter(1, 2) - 1
	ik.maxIterations = 10000
	ik.iterations = 0
	ik.lowerBound, ik.upperBound = limitsToArrays(mdl.Dof())
	// How much to adjust joints to determine slope
	ik.jump = 0.00000001

	ik.SolveWeights = SolverDistanceWeights{XYZWeights{1.0, 1.0, 1.0}, XYZTHWeights{1.0, 1.0, 1.0, 1.0}}

	// May eventually need to be destroyed to prevent memory leaks
	// If we're in a situation where we're making lots of new nlopts rather than reusing this one
	opt, err := nlopt.NewNLopt(nlopt.LD_SLSQP, uint(len(ik.model.Dof())))
	if err != nil {
		panic(errors.Errorf("nlopt creation error: %w", err)) // TODO(biotinker): should return error or panic
	}
	ik.opt = opt

	// x is our joint positions
	// Gradient is, under the hood, a unsafe C structure that we are meant to mutate in place.
	nloptMinFunc := func(x, gradient []float64) float64 {

		ik.iterations++

		// TODO(pl): Might need to check if any of x is +/- Inf
		eePos, err := ik.model.Transform(frame.FloatsToInputs(x))
		if err != nil && eePos == nil {
			ik.logger.Errorf("error calculating eePos in nlopt %q", err)
			err = ik.opt.ForceStop()
			ik.logger.Errorf("forcestop error %q", err)
		}
		dx := make([]float64, 7)

		// Update dx with the delta to the desired position
		for _, nextGoal := range ik.getGoals() {
			dxDelta := spatial.PoseDelta(eePos, nextGoal.GoalTransform)

			dxIdx := nextGoal.EffectorID * len(dxDelta)
			for i, delta := range dxDelta {
				dx[dxIdx+i] = delta
			}
		}

		dist := WeightedSquaredNorm(dx, ik.SolveWeights)

		if len(gradient) > 0 {
			for i := range gradient {
				// Deep copy of our current joint positions
				xBak := append([]float64{}, x...)
				xBak[i] += ik.jump
				eePos, err := ik.model.Transform(frame.FloatsToInputs(xBak))
				if err != nil && eePos == nil {
					ik.logger.Errorf("error calculating eePos in nlopt %q", err)
					err = ik.opt.ForceStop()
					ik.logger.Errorf("forcestop error %q", err)
				}
				dx2 := make([]float64, 7)
				for _, nextGoal := range ik.getGoals() {
					dxDelta := spatial.PoseDelta(eePos, nextGoal.GoalTransform)
					dxIdx := nextGoal.EffectorID * len(dxDelta)
					for i, delta := range dxDelta {
						dx2[dxIdx+i] = delta
					}
				}
				dist2 := WeightedSquaredNorm(dx2, ik.SolveWeights)

				gradient[i] = (dist2 - dist) / (2 * ik.jump)
			}
		}
		return dist
	}

	err = multierr.Combine(
		opt.SetFtolAbs(floatEpsilon),
		opt.SetFtolRel(floatEpsilon),
		opt.SetLowerBounds(ik.lowerBound),
		opt.SetMinObjective(nloptMinFunc),
		opt.SetStopVal(ik.epsilon*ik.epsilon),
		opt.SetUpperBounds(ik.upperBound),
		opt.SetXtolAbs1(floatEpsilon),
		opt.SetXtolRel(floatEpsilon),
		opt.SetMaxEval(8001),
	)

	if err != nil {
		panic(err) // TODO(biotinker): return error?
	}

	return ik
}

// addGoal adds a nlopt IK goal
func (ik *NloptIK) addGoal(newGoal *pb.ArmPosition, effectorID int) {

	goalQuat := spatial.NewPoseFromArmPos(newGoal)
	ik.goals = append(ik.goals, goal{goalQuat, effectorID})
}

// clearGoals clears all goals for the Ik object
func (ik *NloptIK) clearGoals() {
	ik.goals = []goal{}
}

// GetGoals returns the list of all current goal positions
func (ik *NloptIK) getGoals() []goal {
	return ik.goals
}

// SetSolveWeights sets the slve weights
func (ik *NloptIK) SetSolveWeights(weights SolverDistanceWeights) {
	ik.SolveWeights = weights
}

// Solve runs the actual solver and returns a list of all
func (ik *NloptIK) Solve(ctx context.Context, newGoal *pb.ArmPosition, seedAngles []frame.Input) ([]frame.Input, error) {
	var err error

	// Allow ~160 degrees of swing at most
	allowableSwing := 2.8
	tries := 1
	ik.iterations = 0
	startingRadians := ik.GenerateRandomPositions()

	// Solver with ID 1 seeds off current angles
	if ik.id == 1 {
		if len(seedAngles) > len(ik.model.Dof()) {
			return nil, errors.New("passed in too many joint positions")
		}
		startingRadians = seedAngles

		// Set initial restrictions on joints for more intuitive movement
		err = ik.updateBounds(startingRadians, tries)
		if err != nil {
			return nil, err
		}
	} else {
		// Solvers whose ID is not 1 should skip ahead directly to trying random seeds
		tries = 30
	}
	ik.addGoal(newGoal, 0)
	defer ik.clearGoals()

	select {
	case <-ctx.Done():
		ik.logger.Info("solver halted before solving start; possibly solving twice in a row?")
		return nil, err
	default:
	}

	var solutions [][]frame.Input

	for ik.iterations < ik.maxIterations {
		retrySeed := false
		select {
		case <-ctx.Done():
			return nil, err
		default:
		}
		ik.iterations++
		anglesRaw, result, nloptErr := ik.opt.Optimize(frame.InputsToFloats(startingRadians))
		if nloptErr != nil {
			// This just *happens* sometimes due to weirdnesses in nonlinear randomized problems.
			// Ignore it, something else will find a solution
			err = multierr.Combine(err, nloptErr)
		}
		angles := frame.FloatsToInputs(anglesRaw)

		if result < ik.epsilon*ik.epsilon {

			// Check whether we have a large arm swing, and if so, reduce the swing amount and seed the solver off of
			// those joint angles with the swing removed.
			// NOTE: This will prevent *any* movement sufficiently far from the starting position. If attempting a
			// large movement, waypoints are required.
			swing, newAngles := checkExcessiveSwing(seedAngles, angles, allowableSwing)

			if swing {
				retrySeed = true
				startingRadians = newAngles
			} else {
				solution := angles
				// Return immediately if we have a "natural" solution, i.e. one where the halfway point is on the way
				// to the end point
				swing, newErr := calcSwingPct(seedAngles, solution, ik.model)
				if newErr != nil {
					// out-of-bounds angles. Shouldn't happen, but if it does, record the error and move on without
					// keeping the invalid solution
					err = multierr.Combine(err, newErr)
				} else if swing < 0.5 {
					return solution, err
				} else {
					solutions = append(solutions, solution)
				}
			}
		}
		tries++
		if tries < 30 {
			err = ik.updateBounds(seedAngles, tries)
			if err != nil {
				return nil, err
			}
		} else if !retrySeed {
			err = multierr.Combine(
				ik.opt.SetLowerBounds(ik.lowerBound),
				ik.opt.SetUpperBounds(ik.upperBound),
			)
			if err != nil {
				return nil, err
			}
			startingRadians = ik.GenerateRandomPositions()
		}
	}
	if len(solutions) > 0 {
		return bestSolution(seedAngles, solutions, ik.model)
	}
	return nil, multierr.Combine(errors.New("kinematics could not solve for position"), err)
}

// SetSeed sets the random seed of this solver
func (ik *NloptIK) SetSeed(seed int64) {
	ik.randSeed = rand.New(rand.NewSource(seed))
}

// GenerateRandomPositions generates a random set of positions within the limits of this solver.
func (ik *NloptIK) GenerateRandomPositions() []frame.Input {
	pos := make([]frame.Input, len(ik.model.Dof()))
	for i, l := range ik.lowerBound {
		jRange := math.Abs(ik.upperBound[i] - l)
		// Note that rand is unseeded and so will produce the same sequence of floats every time
		// However, since this will presumably happen at different positions to different joints, this shouldn't matter
		pos[i] = frame.Input{ik.randSeed.Float64()*jRange + l}
	}
	return pos
}

// Model returns the associated model
func (ik *NloptIK) Model() frame.Frame {
	return ik.model
}

// updateBounds will set the allowable maximum/minimum joint angles to disincentivise large swings before small swings
// have been tried.
func (ik *NloptIK) updateBounds(seed []frame.Input, tries int) error {
	rangeStep := 0.1
	newLower := make([]float64, len(ik.lowerBound))
	newUpper := make([]float64, len(ik.upperBound))

	for i, pos := range seed {
		newLower[i] = math.Max(ik.lowerBound[i], pos.Value-(rangeStep*float64(tries*(i+1))))
		newUpper[i] = math.Min(ik.upperBound[i], pos.Value+(rangeStep*float64(tries*(i+1))))

		// Allow full freedom of movement for the two most distal joints
		if i > len(seed)-2 {
			newLower[i] = ik.lowerBound[i]
			newUpper[i] = ik.upperBound[i]
		}
	}
	return multierr.Combine(
		ik.opt.SetLowerBounds(newLower),
		ik.opt.SetUpperBounds(newUpper),
	)
}
