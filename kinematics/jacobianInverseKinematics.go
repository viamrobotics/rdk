package kinematics

import (
	"fmt"
	//~ "math"
	"github.com/go-gl/mathgl/mgl64"
	"gonum.org/v1/gonum/mat"
	"go.viam.com/robotcore/kinematics/kinmath"
)

type JacobianIK struct {
	Mdl        *Model
	epsilon    float64
	iterations int
	svd        bool
	Goals      []Goal
	ID         int
	halt       bool
}

func CreateJacobianIKSolver(mdl *Model) *JacobianIK {
	ik := JacobianIK{}
	ik.Mdl = mdl
	ik.epsilon = 0.0001
	ik.iterations = 3000
	ik.svd = true
	return &ik
}

func (ik *JacobianIK) AddGoal(trans *kinmath.QuatTrans, effectorID int) {
	newtrans := &kinmath.QuatTrans{}
	*newtrans = *trans
	ik.Goals = append(ik.Goals, Goal{newtrans, effectorID})
}

func (ik *JacobianIK) SetID(id int) {
	ik.ID = id
}

func (ik *JacobianIK) GetID() int {
	return ik.ID
}

func (ik *JacobianIK) GetMdl() *Model {
	return ik.Mdl
}

func (ik *JacobianIK) ClearGoals() {
	ik.Goals = []Goal{}
}

func (ik *JacobianIK) GetGoals() []Goal {
	return ik.Goals
}

func (ik *JacobianIK) Halt() {
	ik.halt = true
}

func (ik *JacobianIK) Solve() bool {
	ik.halt = false
	// q is the position over which we will iterate
	q := ik.Mdl.GetPosition()

	// qNorm will be the normalized version of q, used to actually calculate kinematics etc
	qNorm := ik.Mdl.GetPosition()
	iteration := 0

	// Variables used to mutate the original position to help avoid getting stuck
	origJointPos := ik.Mdl.GetPosition()
	// Which joint to mutate
	jointMut := 0
	// How much to mutate it by
	jointAmt := 0.05
	var dxDelta []float64

	for iteration < ik.iterations && !ik.halt {
		for iteration < ik.iterations && !ik.halt {
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
				ik.Mdl.SetPosition(qNorm)
				qNorm = ik.Mdl.ZeroInlineRotation(qNorm)
				if ik.Mdl.IsValid(qNorm) {
					return true
				}
			}

			ik.Mdl.CalculateJacobian()
			
			ik.Mdl.CalculateJacobianInverse(0, ik.svd)
			invJ := ik.Mdl.GetJacobianInverse()
			
			dq := invJ.MulNx1(nil, mgl64.NewVecNFromData(dx)).Raw()
			
			q = ik.Mdl.Step(q, dq)
			
			q = ik.Mdl.Normalize(q)

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

func printMat(m *mgl64.MatMxN, name string){
	j2 := mat.NewDense(m.NumRows(),m.NumCols(), m.Transpose(nil).Raw())
	fc := mat.Formatted(j2, mat.Prefix("      "), mat.Squeeze())
	fmt.Printf("%s = %v", name, fc)
	fmt.Println("")
}
