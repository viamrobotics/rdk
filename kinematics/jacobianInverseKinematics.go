package kinematics

import (
	//~ "fmt"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/viamrobotics/robotcore/kinematics/kinmath"
)

type JacobianIK struct {
	Mdl        *Model
	epsilon    float64
	iterations int
	svd        bool
	Goals      []Goal
}

func CreateJacobianIKSolver(mdl *Model) *JacobianIK {
	ik := JacobianIK{}
	ik.Mdl = mdl
	ik.epsilon = 0.0001
	ik.iterations = 3000
	ik.svd = true
	return &ik
}

func (ik *JacobianIK) AddGoal(trans *kinmath.Transform, effectorID int) {
	newtrans := &kinmath.Transform{}
	*newtrans = *trans
	ik.Goals = append(ik.Goals, Goal{newtrans, effectorID})
}

func (ik *JacobianIK) ClearGoals() {
	ik.Goals = []Goal{}
}

func (ik *JacobianIK) GetGoals() []Goal {
	return ik.Goals
}

func (ik *JacobianIK) Solve() bool {
	// q is the position over which we will iterate
	q := ik.Mdl.GetPosition()

	// qNorm will be the normalized version of q, used to actually calculate kinematics etc
	//~ qNorm := ik.Mdl.GetPosition()
	iteration := 0

	// Variables used to mutate the original position to help avoid getting stuck
	origJointPos := ik.Mdl.GetPosition()
	// Which joint to mutate
	jointMut := 0
	// How much to mutate it by
	jointAmt := 0.05
	var dxDelta []float64

	for iteration < ik.iterations {
		for iteration < ik.iterations {
			iteration++
			ik.Mdl.ForwardPosition()
			dx := make([]float64, ik.Mdl.GetOperationalDof()*6)

			// Update dx with the delta to the desired position
			for _, goal := range ik.GetGoals() {
				dxDelta = ik.Mdl.GetOperationalPosition(goal.EffectorID).ToDelta(goal.GoalTransform)
				dxIdx := goal.EffectorID * 6
				for i, delta := range dxDelta {
					dx[dxIdx+i] = delta
				}
			}

			// Check if q is valid for our desired position
			if SquaredNorm(dx) < ik.epsilon*ik.epsilon {

				ik.Mdl.SetPosition(q)

				if ik.Mdl.IsValid(q) {
					return true
				}
			}

			ik.Mdl.CalculateJacobian()

			ik.Mdl.CalculateJacobianInverse(0, ik.svd)

			dq := ik.Mdl.GetJacobianInverse().MulNx1(nil, mgl64.NewVecNFromData(dx)).Raw()

			newPos := ik.Mdl.Step(q, dq)
			q = newPos
			//~ qNorm = ik.Mdl.Normalize(q)

			ik.Mdl.SetPosition(q)

			if iteration%150 == 0 {
				break
			}
		}
		if jointMut < len(origJointPos) {
			//~ fmt.Println("mutating!")
			var mutJointPos []float64
			mutJointPos = append(mutJointPos, origJointPos...)
			mutJointPos[jointMut] += jointAmt
			ik.Mdl.SetPosition(mutJointPos)

			// Test +/- jointAmt
			jointAmt *= -1
			if jointAmt > 0 {
				jointMut++
			}
		} else {
			//~ fmt.Println("setting random!")
			ik.Mdl.SetPosition(ik.Mdl.RandomJointPositions())
		}
	}
	ik.Mdl.SetPosition(origJointPos)
	return false
}
