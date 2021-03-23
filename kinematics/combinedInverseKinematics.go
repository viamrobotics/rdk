package kinematics

import (
	//~ "fmt"
	//~ "github.com/edaniels/golog"
	"github.com/edaniels/golog"
	"go.viam.com/robotcore/kinematics/kinmath"
)

type CombinedIK struct {
	solvers []InverseKinematics
	Mdl     *Model
	ID      int
}

type ReturnTest struct {
	ID      int
	Success bool
}

// Creates a combined parallel IK solver with the number of models given
// Must pass at least two models. Two will produce one jacobian IK solver, and all additional
// models will create nlopt solvers with different random seeds
func CreateCombinedIKSolver(models []*Model, logger golog.Logger) *CombinedIK {
	ik := &CombinedIK{}
	if len(models) < 2 {
		// Anything calling this should check core counts
		return nil
	}
	ik.Mdl = models[0]
	models[1].SetSeed(1)
	ik.solvers = append(ik.solvers, CreateJacobianIKSolver(models[1]))
	for i := 2; i < len(models); i++ {
		models[i].SetSeed(int64(i))
		ik.solvers = append(ik.solvers, CreateNloptIKSolver(models[i], logger))
	}
	for i, solver := range ik.solvers {
		solver.SetID(i)
	}
	return ik
}

func (ik *CombinedIK) AddGoal(trans *kinmath.Transform, effectorID int) {
	for _, solver := range ik.solvers {
		solver.AddGoal(trans, effectorID)
	}
}

func (ik *CombinedIK) SetID(id int) {
	ik.ID = id
}

func (ik *CombinedIK) GetID() int {
	return ik.ID
}

func (ik *CombinedIK) GetMdl() *Model {
	return ik.Mdl
}

func (ik *CombinedIK) ClearGoals() {
	for _, solver := range ik.solvers {
		solver.ClearGoals()
	}
}

func (ik *CombinedIK) Halt() {
	for _, solver := range ik.solvers {
		solver.Halt()
	}
}

func (ik *CombinedIK) GetGoals() []Goal {
	return ik.solvers[0].GetGoals()
}

func (ik *CombinedIK) GetSolvers() []InverseKinematics {
	return ik.solvers
}

func runSolver(solver InverseKinematics, c chan ReturnTest) {
	solved := solver.Solve()
	c <- ReturnTest{solver.GetID(), solved}
}

func (ik *CombinedIK) Solve() bool {
	pos := ik.Mdl.GetPosition()
	c := make(chan ReturnTest)

	for _, solver := range ik.solvers {
		solver.GetMdl().SetPosition(pos)
		solver.GetMdl().ForwardPosition()
		go runSolver(solver, c)
	}

	returned := 0
	myRT := ReturnTest{-1, false}

	//~ // Wait until either 1) we have a success or 2) all solvers have returned false
	for !myRT.Success && returned < len(ik.solvers) {
		myRT = <-c
		returned++
		if myRT.Success {
			ik.Halt()
			ik.Mdl.SetPosition(ik.solvers[myRT.ID].GetMdl().GetPosition())
			ik.Mdl.ForwardPosition()
		}
	}
	return myRT.Success
	//~ return ik.solvers[1].Solve()
}
