package kinematics

import (
	"math"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"
	"github.com/go-nlopt/nlopt"
	"go.uber.org/multierr"

	"go.viam.com/core/kinematics/kinmath"
)

// NloptIK TODO
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

// CreateNloptIKSolver TODO
func CreateNloptIKSolver(mdl *Model, logger golog.Logger) *NloptIK {
	ik := &NloptIK{logger: logger}
	ik.resetHalting()
	ik.Mdl = mdl
	// How close we want to get to the goal
	ik.epsilon = 0.0001
	// The absolute smallest value able to be represented by a float64
	floatEpsilon := math.Nextafter(1, 2) - 1
	ik.maxIterations = 5
	ik.iterations = 0
	ik.lowerBound = mdl.GetMinimum()
	ik.upperBound = mdl.GetMaximum()

	// May eventually need to be destroyed to prevent memory leaks
	// If we're in a situation where we're making lots of new nlopts rather than reusing this one
	opt, err := nlopt.NewNLopt(nlopt.LD_SLSQP, uint(mdl.GetDofPosition()))
	if err != nil {
		panic(errors.Errorf("nlopt creation error: %w", err)) // TODO(biotinker): should return error or panic
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
		
		dist := WeightedSquaredNorm(dx, ik.Mdl.DistCfg)

		if len(gradient) > 0 {
			
			for i, _ := range gradient {
				// Deep copy of our current joint positions
				xBak := append([]float64{}, x...)
				xBak[i] += 0.00000001
				ik.Mdl.SetPosition(xBak)
				ik.Mdl.ForwardPosition()
				dx2 := make([]float64, ik.Mdl.GetOperationalDof()*6)
				for _, goal := range ik.GetGoals() {
					dxDelta := ik.Mdl.GetOperationalPosition(goal.EffectorID).ToDelta(goal.GoalTransform)
					dxIdx := goal.EffectorID * len(dxDelta)
					for i, delta := range dxDelta {
						dx2[dxIdx+i] = delta
					}
				}
				dist2 := WeightedSquaredNorm(dx2, ik.Mdl.DistCfg)
				
				gradient[i] = dist2 - dist
			}
		}

		// We need to use gradient to make the linter happy
		//~ if len(gradient) > 0 {
			//~ return dist
		//~ }
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

// AddGoal TODO
func (ik *NloptIK) AddGoal(trans *kinmath.QuatTrans, effectorID int) {
	newtrans := &kinmath.QuatTrans{}
	*newtrans = *trans
	ik.Goals = append(ik.Goals, Goal{newtrans, effectorID})
	ik.resetHalting()
}

// SetID TODO
func (ik *NloptIK) SetID(id int) {
	ik.ID = id
}

// GetID TODO
func (ik *NloptIK) GetID() int {
	return ik.ID
}

// GetMdl TODO
func (ik *NloptIK) GetMdl() *Model {
	return ik.Mdl
}

// ClearGoals TODO
func (ik *NloptIK) ClearGoals() {
	ik.Goals = []Goal{}
	ik.resetHalting()
}

// GetGoals TODO
func (ik *NloptIK) GetGoals() []Goal {
	return ik.Goals
}

func (ik *NloptIK) resetHalting() {
	ik.requestHaltCh = make(chan struct{})
	ik.haltedCh = make(chan struct{})
}

// Halt TODO
func (ik *NloptIK) Halt() {
	close(ik.requestHaltCh)
	err := ik.opt.ForceStop()
	if err != nil {
		ik.logger.Info("nlopt halt error: ", err)
	}
	<-ik.haltedCh
	ik.resetHalting()
}

// Solve TODO
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
			ik.Mdl.ForwardPosition()
			return true
		}
		ik.Mdl.SetPosition(ik.Mdl.RandomJointPositions())
	}
	ik.Mdl.SetPosition(origJointPos)
	return false
}
