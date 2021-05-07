package kinematics

import (
	"fmt"
	"math"

	"github.com/edaniels/golog"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/go-nlopt/nlopt"
	"go.uber.org/multierr"

	"go.viam.com/robotcore/kinematics/kinmath"
)

type NloptIK struct {
	Mdl           *Model
	lowerBound    []float64
	upperBound    []float64
	iterations    int
	maxIterations int
	epsilon       float64
	Goals         []Goal
	opt           *nlopt.NLopt
	ID            int
	requestHaltCh chan struct{}
	haltedCh      chan struct{}
	logger        golog.Logger
}

func CreateNloptIKSolver(mdl *Model, logger golog.Logger) *NloptIK {
	ik := &NloptIK{logger: logger}
	ik.resetHalting()
	ik.Mdl = mdl
	// How close we want to get to the goal
	ik.epsilon = 0.01
	// The absolute smallest value able to be represented by a float64
	floatEpsilon := math.Nextafter(1, 2) - 1
	ik.maxIterations = 50000
	ik.iterations = 0
	ik.lowerBound = mdl.GetMinimum()
	ik.upperBound = mdl.GetMaximum()

	// May eventually need to be destroyed to prevent memory leaks
	// If we're in a situation where we're making lots of new nlopts rather than reusing this one
	opt, err := nlopt.NewNLopt(nlopt.LD_SLSQP, uint(mdl.GetDofPosition()))
	if err != nil {
		panic(fmt.Errorf("nlopt creation error: %w", err)) // TODO(biotinker): should return error or panic
	}
	ik.opt = opt

	// x is our joint positions
	// Gradient is, under the hood, a unsafe C structure that we are meant to mutate in place.
	nloptMinFunc := func(x, gradient []float64) float64 {

		ik.iterations++

		// TODO(pl): Might need to check if any of x is +/- Inf
		ik.Mdl.SetPosition(x)
		ik.Mdl.ForwardPosition()
		dx := make([]float64, ik.Mdl.GetOperationalDof()*6)

		// Update dx with the delta to the desired position
		for _, goal := range ik.GetGoals() {
			dxDelta := ik.Mdl.GetOperationalPosition(goal.EffectorID).ToDelta(goal.GoalTransform)
			dxIdx := goal.EffectorID * len(dxDelta)
			for i, delta := range dxDelta {
				dx[dxIdx+i] = delta
			}
		}

		if len(gradient) > 0 {
			// mgl64 functions have both a return and an in-place modification, annoyingly, which is why this is split
			// out into a bunch of variables instead of being nicely composed.
			ik.Mdl.CalculateJacobian()
			j := ik.Mdl.GetJacobian()
			grad2 := mgl64.NewVecN(len(dx))
			j2 := j.Transpose(mgl64.NewMatrix(j.NumCols(), j.NumRows()))
			j2 = j2.Mul(j2, -2)

			// Linter thinks this is ineffectual because it doesn't know about CGo doing magic with pointers
			gradient2 := j2.MulNx1(grad2, mgl64.NewVecNFromData(dx)).Raw()
			for i, v := range gradient2 {
				gradient[i] = v
				// Do some rounding on large (>2^15) numbers because of floating point inprecision
				// Shouldn't matter since these values should converge to zero
				// If you get weird results like calculations terminating early or gradient acting like it isn't updating
				// Then this might be your culprit
				if math.Abs(v) > 1<<15 {
					gradient[i] = math.Round(v)
				}
			}
		}

		// We need to use gradient to make the linter happy
		if len(gradient) > 0 {
			return WeightedSquaredNorm(dx, ik.Mdl.DistCfg)
		}
		return WeightedSquaredNorm(dx, ik.Mdl.DistCfg)
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

func (ik *NloptIK) AddGoal(trans *kinmath.QuatTrans, effectorID int) {
	newtrans := &kinmath.QuatTrans{}
	*newtrans = *trans
	ik.Goals = append(ik.Goals, Goal{newtrans, effectorID})
	ik.resetHalting()
}

func (ik *NloptIK) SetID(id int) {
	ik.ID = id
}

func (ik *NloptIK) GetID() int {
	return ik.ID
}

func (ik *NloptIK) GetMdl() *Model {
	return ik.Mdl
}

func (ik *NloptIK) ClearGoals() {
	ik.Goals = []Goal{}
	ik.resetHalting()
}

func (ik *NloptIK) GetGoals() []Goal {
	return ik.Goals
}

func (ik *NloptIK) resetHalting() {
	ik.requestHaltCh = make(chan struct{})
	ik.haltedCh = make(chan struct{})
}

func (ik *NloptIK) Halt() {
	close(ik.requestHaltCh)
	err := ik.opt.ForceStop()
	if err != nil {
		ik.logger.Info("nlopt halt error: ", err)
	}
	<-ik.haltedCh
	ik.resetHalting()
}

func (ik *NloptIK) Solve() bool {
	select {
	case <-ik.haltedCh:
		ik.logger.Info("solver halted before solving start; possibly solving twice in a row?")
		return false
	default:
	}
	defer close(ik.haltedCh)
	ik.iterations = 0
	origJointPos := ik.Mdl.GetPosition()
	for ik.iterations < ik.maxIterations {
		select {
		case <-ik.requestHaltCh:
			return false
		default:
		}
		ik.iterations++
		angles, result, err := ik.opt.Optimize(ik.Mdl.GetPosition())
		if err != nil {
			// This just *happens* sometimes due to weirdnesses in nonlinear randomized problems.
			// Ignore it, something else will find a solution
			if ik.opt.LastStatus() != "FAILURE" && ik.opt.LastStatus() != "FORCED_STOP" {
				ik.logger.Info("nlopt optimization error: ", err)
			}
		}

		if result < ik.epsilon*ik.epsilon {
			angles = ik.Mdl.ZeroInlineRotation(angles)
			ik.Mdl.SetPosition(angles)
			return true
		}
		ik.Mdl.SetPosition(ik.Mdl.RandomJointPositions())
	}
	ik.Mdl.SetPosition(origJointPos)
	return false
}
