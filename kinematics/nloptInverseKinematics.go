package kinematics

import (
	"context"
	"math"
	"math/rand"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"
	"github.com/go-nlopt/nlopt"
	"go.uber.org/multierr"

	"go.viam.com/core/arm"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/spatialmath"
)

// NloptIK TODO
type NloptIK struct {
	model         *Model
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
}

// CreateNloptIKSolver TODO
func CreateNloptIKSolver(mdl *Model, logger golog.Logger) *NloptIK {
	ik := &NloptIK{logger: logger}
	ik.randSeed = rand.New(rand.NewSource(1))
	ik.model = mdl
	// How close we want to get to the goal
	ik.epsilon = 0.01
	// The absolute smallest value able to be represented by a float64
	floatEpsilon := math.Nextafter(1, 2) - 1
	ik.maxIterations = 10000
	ik.iterations = 0
	ik.lowerBound = mdl.MinimumJointLimits()
	ik.upperBound = mdl.MaximumJointLimits()
	// How much to adjust joints to determine slope
	ik.jump = 0.00000001

	// May eventually need to be destroyed to prevent memory leaks
	// If we're in a situation where we're making lots of new nlopts rather than reusing this one
	opt, err := nlopt.NewNLopt(nlopt.LD_SLSQP, uint(ik.model.Dof()))
	if err != nil {
		panic(errors.Errorf("nlopt creation error: %w", err)) // TODO(biotinker): should return error or panic
	}
	ik.opt = opt

	// x is our joint positions
	// Gradient is, under the hood, a unsafe C structure that we are meant to mutate in place.
	nloptMinFunc := func(x, gradient []float64) float64 {

		ik.iterations++

		// TODO(pl): Might need to check if any of x is +/- Inf
		eePos := JointRadToQuat(ik.model, x)
		dx := make([]float64, ik.model.OperationalDof()*7)

		// Update dx with the delta to the desired position
		for _, nextGoal := range ik.getGoals() {
			dxDelta := eePos.ToDelta(nextGoal.GoalTransform)

			dxIdx := nextGoal.EffectorID * len(dxDelta)
			for i, delta := range dxDelta {
				dx[dxIdx+i] = delta
			}
		}

		dist := WeightedSquaredNorm(dx, ik.model.SolveWeights)

		if len(gradient) > 0 {
			maxGrad := 0.0

			for i := range gradient {
				// Deep copy of our current joint positions
				xBak := append([]float64{}, x...)
				xBak[i] += ik.jump
				eePos := JointRadToQuat(ik.model, xBak)
				dx2 := make([]float64, ik.model.OperationalDof()*7)
				for _, nextGoal := range ik.getGoals() {
					dxDelta := eePos.ToDelta(nextGoal.GoalTransform)
					dxIdx := nextGoal.EffectorID * len(dxDelta)
					for i, delta := range dxDelta {
						dx2[dxIdx+i] = delta
					}
				}
				dist2 := WeightedSquaredNorm(dx2, ik.model.SolveWeights)

				gradient[i] = (dist2 - dist) / (20000 * ik.jump)
				if math.Abs(gradient[i]) > maxGrad {
					maxGrad = math.Abs(gradient[i])
				}
			}
			// Scale gradient so that largest value is not > 2pi
			if maxGrad > 2*math.Pi {
				for i, v := range gradient {
					gradient[i] = v / (maxGrad / (2 * math.Pi))
				}
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

	goalQuat := spatialmath.NewDualQuaternionFromArmPos(newGoal)
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

// Solve attempts to solve for all goals
func (ik *NloptIK) Solve(ctx context.Context, newGoal *pb.ArmPosition, seedAngles *pb.JointPositions) (*pb.JointPositions, error) {
	var err error

	// Allow ~160 degrees of swing at most
	allowableSwing := 2.8

	if len(seedAngles.Degrees) > len(ik.lowerBound) {
		return &pb.JointPositions{}, errors.New("passed in too many joint positions")
	}
	ik.addGoal(newGoal, 0)
	defer ik.clearGoals()

	select {
	case <-ctx.Done():
		ik.logger.Info("solver halted before solving start; possibly solving twice in a row?")
		return &pb.JointPositions{}, err
	default:
	}
	ik.iterations = 0
	startingRadians := arm.JointPositionsToRadians(seedAngles)

	// Set initial restrictions on joints for more intuitive movement
	tries := 1
	err = ik.updateBounds(startingRadians, tries)
	if err != nil {
		return &pb.JointPositions{}, err
	}

	for ik.iterations < ik.maxIterations {
		retrySeed := false
		select {
		case <-ctx.Done():
			return &pb.JointPositions{}, err
		default:
		}
		ik.iterations++
		angles, result, nloptErr := ik.opt.Optimize(startingRadians)
		if nloptErr != nil {
			// This just *happens* sometimes due to weirdnesses in nonlinear randomized problems.
			// Ignore it, something else will find a solution
			err = multierr.Combine(err, nloptErr)
		}

		if result < ik.epsilon*ik.epsilon && ik.model.AreJointPositionsValid(angles) {

			// Check whether we have a large arm swing, and if so, reduce the swing amount and seed the solver off of
			// those joint angles with the swing removed.
			// NOTE: This will prevent *any* movement sufficiently far from the starting position. If attempting a
			// large movement, waypoints are required.
			swing, newAngles := checkExcessiveSwing(arm.JointPositionsToRadians(seedAngles), angles, allowableSwing)

			if swing {
				retrySeed = true
				startingRadians = newAngles
			} else {
				angles = ZeroInlineRotation(ik.model, angles)
				return arm.JointPositionsFromRadians(angles), err
			}
		}
		tries++
		if tries < 30 {
			err = ik.updateBounds(arm.JointPositionsToRadians(seedAngles), tries)
			if err != nil {
				return &pb.JointPositions{}, err
			}
		} else if !retrySeed {
			err = multierr.Combine(
				ik.opt.SetLowerBounds(ik.lowerBound),
				ik.opt.SetUpperBounds(ik.upperBound),
			)
			if err != nil {
				return &pb.JointPositions{}, err
			}
			startingRadians = ik.model.GenerateRandomJointPositions(ik.randSeed)
		}
	}
	return &pb.JointPositions{}, multierr.Combine(errors.New("kinematics could not solve for position"), err)
}

// SetSeed sets the random seed of this solver
func (ik *NloptIK) SetSeed(seed int64) {
	ik.randSeed = rand.New(rand.NewSource(seed))
}

// Mdl returns the model associated with this IK.
func (ik *NloptIK) Mdl() *Model {
	return ik.model
}

// updateBounds will set the allowable maximum/minimum joint angles to disincentivise large swings before small swings
// have been tried.
func (ik *NloptIK) updateBounds(seed []float64, tries int) error {
	rangeStep := 0.1
	newLower := make([]float64, len(ik.lowerBound))
	newUpper := make([]float64, len(ik.upperBound))

	for i, pos := range seed {
		newLower[i] = math.Max(ik.lowerBound[i], pos-(rangeStep*float64(tries*(i+1))))
		newUpper[i] = math.Min(ik.upperBound[i], pos+(rangeStep*float64(tries*(i+1))))

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

// checkExcessiveSwing takes two lists of radians and ensures there are no huge swings from one to the other.
func checkExcessiveSwing(orig, solution []float64, allowableSwing float64) (bool, []float64) {
	swing := false
	newSolution := make([]float64, len(solution))
	for i, angle := range solution {
		newSolution[i] = angle
		//Allow swings in the 3 most distal joints
		if i < len(solution)-3 {
			// Check if swing greater than ~160 degrees.
			for newSolution[i]-orig[i] > allowableSwing {
				newSolution[i] -= math.Pi
				swing = true
			}
			for newSolution[i]-orig[i] < -allowableSwing {
				newSolution[i] += math.Pi
				swing = true
			}
		}
	}

	return swing, newSolution
}
