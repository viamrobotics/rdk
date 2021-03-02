package kinematics

import (
	//~ "fmt"
	"math"

	"github.com/edaniels/golog"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/go-nlopt/nlopt"
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
}

func errCheck(err error) {
	if err != nil {
		golog.Global.Error("nlopt init error: ", err)
	}
}

func CreateNloptIKSolver(mdl *Model) *NloptIK {
	ik := &NloptIK{}
	ik.Mdl = mdl
	ik.epsilon = 0.0001
	floatEpsilon := math.Nextafter(1, 2) - 1
	ik.maxIterations = 1
	ik.iterations = 0
	ik.lowerBound = mdl.GetMinimum()
	ik.upperBound = mdl.GetMaximum()

	// May eventually need to be destroyed to prevent memory leaks
	// If we're in a situation where we're making lots of new nlopts rather than reusing this one
	opt, err := nlopt.NewNLopt(nlopt.LD_SLSQP, uint(mdl.GetDofPosition()))
	if err != nil {
		golog.Global.Error("nlopt creation error: ", err)
		return &NloptIK{}
	}
	ik.opt = opt

	// x is our joint positions
	// Gradient is, under the hood, a unsafe C structure that we are meant to mutate in place.
	nloptMinFunc := func(x, gradient []float64) float64 {

		ik.iterations++

		// TODO: Might need to check if any of x is +/- Inf
		ik.Mdl.SetPosition(x)
		ik.Mdl.ForwardPosition()
		dx := make([]float64, ik.Mdl.GetOperationalDof()*6)

		// Update dx with the delta to the desired position
		for _, goal := range ik.GetGoals() {
			dxDelta := ik.Mdl.GetOperationalPosition(goal.EffectorID).ToDelta(goal.GoalTransform)
			dxIdx := goal.EffectorID * 6
			for i, delta := range dxDelta {
				dx[dxIdx+i] = delta
			}
		}

		if len(gradient) > 0 {
			//~ // mgl64 functions have both a return and an in-place modification, annoyingly, which is why this is split
			//~ // out into a bunch of variables instead of being nicely composed.
			ik.Mdl.CalculateJacobian()
			j := ik.Mdl.GetJacobian()
			grad2 := mgl64.NewVecN(len(dx))
			j2 := j.Transpose(mgl64.NewMatrix(j.NumRowCols()))
			j2 = j2.Mul(j2, -2)

			//~ // Linter thinks this is ineffectual because it doesn't know about CGo doing magic with pointers
			gradient2 := j2.MulNx1(grad2, mgl64.NewVecNFromData(dx)).Raw()
			for i, v := range gradient2 {
				gradient[i] = v
				// Do some rounding on large (>2^16) numbers because of floating point inprecision
				// Shouldn't matter since these values should converge to zero
				// If you get weird results like calculations terminating early or gradient acting like it isn't updating
				// Then this might be your culprit
				if math.Abs(v) > 65535 {
					gradient[i] = math.Round(v)
				}
			}
		}
		// We need to use gradient to make the linter happy
		if len(gradient) > 0 {
			return SquaredNorm(dx)
		}
		return SquaredNorm(dx)
	}
	//~ nloptMinFunc := func(x, gradient []float64) float64 {

	errCheck(opt.SetFtolAbs(floatEpsilon))
	errCheck(opt.SetFtolRel(floatEpsilon))
	errCheck(opt.SetLowerBounds(ik.lowerBound))
	errCheck(opt.SetMinObjective(nloptMinFunc))
	errCheck(opt.SetStopVal(ik.epsilon * ik.epsilon))
	errCheck(opt.SetUpperBounds(ik.upperBound))
	errCheck(opt.SetXtolAbs1(floatEpsilon))
	errCheck(opt.SetXtolRel(floatEpsilon))
	errCheck(opt.SetMaxEval(8001))

	return ik
}

func (ik *NloptIK) AddGoal(trans *kinmath.Transform, effectorID int) {
	newtrans := &kinmath.Transform{}
	*newtrans = *trans
	ik.Goals = append(ik.Goals, Goal{newtrans, effectorID})
}

func (ik *NloptIK) ClearGoals() {
	ik.Goals = []Goal{}
}

func (ik *NloptIK) GetGoals() []Goal {
	return ik.Goals
}

func (ik *NloptIK) Solve() bool {
	origJointPos := ik.Mdl.GetPosition()
	for ik.iterations < ik.maxIterations {
		angles, result, err := ik.opt.Optimize(ik.Mdl.GetPosition())
		if err != nil {
			golog.Global.Error("nlopt optimization error: ", err)
		}

		if result < ik.epsilon*ik.epsilon {
			ik.Mdl.SetPosition(angles)
			return true
		}

		ik.Mdl.SetPosition(ik.Mdl.RandomJointPositions())
	}
	ik.Mdl.SetPosition(origJointPos)
	return false
}
