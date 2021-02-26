package kinematics

import (
	"math"

	"github.com/go-nlopt/nlopt"
	"go.viam.com/robotcore/kinematics/kinmath"
)

type NloptIK struct {
	Mdl *Model
	//~ lowerBound []float64
	//~ upperBound []float64
	iterations int
	epsilon    float64
	Goals      []Goal
}

func CreateIKSolver(mdl *Model) *NloptIK {
	ik := NloptIK{}
	ik.Mdl = mdl
	ik.epsilon = 0.001
	ik.iterations = 1000
	return &ik
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
	opt, err := nlopt.NewNLopt(nlopt.LD_MMA, 2)
	if err != nil {
		return false
	}
	defer opt.Destroy()

	err = opt.SetLowerBounds([]float64{math.Inf(-1), 0.})
	if err != nil {
		return false
	}
	return false
}
