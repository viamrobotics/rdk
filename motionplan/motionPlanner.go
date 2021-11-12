package motionplan

import (
	"context"
	"errors"
	"math"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"

	"go.viam.com/core/kinematics"
	pb "go.viam.com/core/proto/api/v1"
	frame "go.viam.com/core/referenceframe"
	spatial "go.viam.com/core/spatialmath"
	"go.viam.com/core/utils"
)

// MotionPlanner defines a struct able to plan motion
type MotionPlanner interface {
	// Plan will take a context, a goal position, and an input start state and return a series of state waypoints which
	// should be visited in order to arrive at the goal while satisfying all constraints
	Plan(context.Context, *pb.Pose, []frame.Input) ([][]frame.Input, error)
	AddConstraint(string, Constraint)
	RemoveConstraint(string)
	Constraints() []string
	CheckConstraints(*ConstraintInput) (bool, float64)
	Resolution() float64 // how narrowly to check for constraints
	Frame() frame.Frame
}

// NewLinearMotionPlanner returns a linearMotionPlanner. This does a linear IK interpolation from start to goal.
// Assuming a direct motion is possible, it should find a valid path. It cannot navigate around obstacles.
// Probably cBiRRT should be used instead- it should give nearly as good results
func NewLinearMotionPlanner(frame frame.Frame, logger golog.Logger, nCPU int) (MotionPlanner, error) {
	ik, err := kinematics.CreateCombinedIKSolver(frame, logger, nCPU)
	if err != nil {
		return nil, err
	}
	mp := &linearMotionPlanner{solver: ik, frame: frame, idealMovementScore: 0.3, stepSize: 2., logger: logger}
	mp.visited = map[r3.Vector]bool{}
	mp.AddConstraint("interpolationConstraint", NewInterpolatingConstraint())
	mp.AddConstraint("jointSwingScorer", NewJointScorer())
	return mp, nil
}

// A straightforward motion planner that will path a straight line from start to end
type linearMotionPlanner struct {
	constraintHandler
	solver             kinematics.InverseKinematics
	frame              frame.Frame
	logger             golog.Logger
	idealMovementScore float64
	stepSize           float64
	visited            map[r3.Vector]bool
}

func (mp *linearMotionPlanner) Frame() frame.Frame {
	return mp.frame
}

func (mp *linearMotionPlanner) Resolution() float64 {
	return mp.stepSize
}

func (mp *linearMotionPlanner) Plan(ctx context.Context, goal *pb.Pose, seed []frame.Input) ([][]frame.Input, error) {
	var inputSteps [][]frame.Input

	seedPos, err := mp.frame.Transform(seed)
	if err != nil {
		return nil, err
	}
	goalPos := spatial.NewPoseFromProtobuf(fixOvIncrement(goal, spatial.PoseToProtobuf(seedPos)))

	// First, we break down the spatial distance and rotational distance from seed to goal, and determine the number
	// of steps needed to get from one to the other
	nSteps := getSteps(seedPos, goalPos, mp.stepSize)

	mp.logger.Debug("starting plan")

	// Create the required steps. nSteps is guaranteed to be at least 1.
	for i := 1; i <= nSteps; i++ {
		select {
		case <-ctx.Done():
			break
		default:
		}

		intPos := spatial.Interpolate(seedPos, goalPos, float64(i)/float64(nSteps))

		var step []frame.Input

		solutions, err := getSolutions(ctx, -1, mp.idealMovementScore, mp.solver, spatial.PoseToProtobuf(intPos), seed, mp)
		if err != nil {
			return nil, err
		}

		minScore := math.Inf(1)
		for score, sol := range solutions {
			if score < minScore {
				step = sol
			}
		}

		seed = step
		// Append deep copy of result to inputSteps
		inputSteps = append(inputSteps, append([]frame.Input{}, step...))
	}

	return inputSteps, nil
}

// getSteps will determine the number of steps which should be used to get from the seed to the goal.
// The returned value is guaranteed to be at least 1.
// stepSize represents both the max mm movement per step, and max R4AA degrees per step
func getSteps(seedPos, goalPos spatial.Pose, stepSize float64) int {

	mmDist := seedPos.Point().Distance(goalPos.Point())
	rDist := spatial.OrientationBetween(seedPos.Orientation(), goalPos.Orientation()).AxisAngles()

	nSteps := math.Max(math.Abs(mmDist/stepSize), math.Abs(utils.RadToDeg(rDist.Theta)/stepSize))
	return int(nSteps) + 1
}

// fixOvIncrement will detect whether the given goal position is a precise orientation increment of the current
// position, in which case it will detect whether we are leaving a pole. If we are an OV increment and leaving a pole,
// then Theta will be adjusted to give an expected smooth movement. The adjusted goal will be returned. Otherwise the
// original goal is returned.
// Rationale: if clicking the increment buttons in the interface, the user likely wants the most intuitive motion
// posible. If setting values manually, the user likely wants exactly what they requested.
func fixOvIncrement(pos, seed *pb.Pose) *pb.Pose {
	epsilon := 0.0001
	// Nothing to do for spatial translations or theta increments
	if pos.X != seed.X || pos.Y != seed.Y || pos.Z != seed.Z || pos.Theta != seed.Theta {
		return pos
	}
	// Check if seed is pointing directly at pole
	if 1-math.Abs(seed.OZ) > epsilon || pos.OZ != seed.OZ {
		return pos
	}

	// we only care about negative xInc
	xInc := pos.OX - seed.OX
	yInc := math.Abs(pos.OY - seed.OY)
	adj := 0.0
	if pos.OX == seed.OX {
		// no OX movement
		if yInc != 0.1 && yInc != 0.01 {
			// nonstandard increment
			return pos
		}
		// If wanting to point towards +Y and OZ<0, add 90 to theta, otherwise subtract 90
		if pos.OY-seed.OY > 0 {
			adj = 90
		} else {
			adj = -90
		}
	} else {
		if (xInc != -0.1 && xInc != -0.01) || pos.OY != seed.OY {
			return pos
		}
		// If wanting to point towards -X, increment by 180. Values over 180 or under -180 will be automatically wrapped
		adj = 180
	}
	if pos.OZ > 0 {
		adj *= -1
	}

	return &pb.Pose{
		X:     pos.X,
		Y:     pos.Y,
		Z:     pos.Z,
		Theta: pos.Theta + adj,
		OX:    pos.OX,
		OY:    pos.OY,
		OZ:    pos.OZ,
	}
}

// getSolutions will initiate an IK solver for the given position and seed, collect solutions, and score them by constraints.
// If maxSolutions is positive, once that many solutions have been collected, the solver will terminate and return that many solutions.
// If minScore is positive, if a solution scoring below that amount is found, the solver will terminate and return that one solution.
func getSolutions(ctx context.Context, maxSolutions int, minScore float64, solver kinematics.InverseKinematics, goal *pb.Pose, seed []frame.Input, mp MotionPlanner) (map[float64][]frame.Input, error) {

	seedPos, err := mp.Frame().Transform(seed)
	if err != nil {
		return nil, err
	}
	goalPos := spatial.NewPoseFromProtobuf(fixOvIncrement(goal, spatial.PoseToProtobuf(seedPos)))

	solutionGen := make(chan []frame.Input)
	ikErr := make(chan error)
	ctxWithCancel, cancel := context.WithCancel(ctx)

	// Spawn the IK solver to generate solutions until done
	go func() {
		defer close(ikErr)
		ikErr <- solver.Solve(ctxWithCancel, solutionGen, goalPos, seed)
	}()

	solutions := map[float64][]frame.Input{}

	// Solve the IK solver. Loop labels are required because `break` etc in a `select` will break only the `select`.
IK:
	for {
		select {
		case <-ctx.Done():
			return nil, errors.New("context Done signal")
		case step := <-solutionGen:
			cPass, cScore := mp.CheckConstraints(&ConstraintInput{
				seedPos,
				goalPos,
				seed,
				step,
				mp.Frame()})

			if cPass {
				if cScore < minScore && minScore >= 0 {
					solutions = map[float64][]frame.Input{}
					solutions[cScore] = step
					// good solution, stopping early
					break IK
				}

				solutions[cScore] = step
				if len(solutions) >= maxSolutions && maxSolutions > 0 {
					// sufficient solutions found, stopping early
					break IK
				}
			}
			// Skip the return check below until we have nothing left to read from solutionGen
			continue IK
		default:
		}

		select {
		case err = <-ikErr:
			// If we have a return from the IK solver, there are no more solutions, so we finish processing above
			// until we've drained the channel
			break IK
		default:
		}
	}
	cancel()
	if len(solutions) == 0 {
		return nil, errors.New("unable to solve for position")
	}

	return solutions, nil
}

func inputDist(from, to []frame.Input) float64 {
	dist := 0.
	for i, f := range from {
		dist += math.Abs(to[i].Value - f.Value)
	}
	return dist
}
