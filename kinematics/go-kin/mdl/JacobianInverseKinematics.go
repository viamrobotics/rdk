package mdl

import (
	"github.com/go-gl/mathgl/mgl64"
	"fmt"
	"github.com/viamrobotics/kinematics/go-kin/kinmath"
)


type JacobianIK struct{
	Mdl        *Model
	epsilon    float64
	iterations int
	svd        bool
	Goals      []Goal
}

func CreateJacobianIKSolver(mdl *Model) *JacobianIK{
	ik := JacobianIK{}
	ik.Mdl = mdl
	ik.epsilon = 0.001
	ik.iterations = 3000
	ik.svd = true
	return &ik
}

func (ik *JacobianIK) AddGoal(trans *kinmath.Transform, effectorID int){
	newtrans := &kinmath.Transform{}
	*newtrans = *trans
	ik.Goals = append(ik.Goals, Goal{newtrans, effectorID})
}

func (ik *JacobianIK) ClearGoals(){
	ik.Goals = []Goal{}
}

func (ik *JacobianIK) GetGoals() []Goal{
	return ik.Goals
}

func (ik *JacobianIK) Solve() bool{
	q := ik.Mdl.GetPosition()
	dq := SetZero(ik.Mdl.GetDofPosition())
	dx := SetZero(ik.Mdl.GetOperationalDof() * 6)
	//~ rand := SetZero(ik.Mdl.GetDof())
	iteration := 0
	
	for iteration < ik.iterations{
		for iteration < ik.iterations{
			iteration++
			ik.Mdl.ForwardPosition()
			dx = SetZero(ik.Mdl.GetOperationalDof() * 6)
			
			// Update dx with the delta to the desired position
			for _, goal := range(ik.GetGoals()){
				dxDelta := ik.Mdl.GetOperationalPosition(goal.EffectorID).ToDelta(goal.GoalTransform)
				dxIdx := goal.EffectorID * 6
				for i, delta := range(dxDelta){
					dx[dxIdx + i] = delta
				}
			}
			
			// Check if q is valid for our desired position
			if SquaredNorm(dx) < ik.epsilon * ik.epsilon{
				ik.Mdl.Normalize(q)
				ik.Mdl.SetPosition(q)
				
				if ik.Mdl.IsValid(q){
					fmt.Println(iteration)
					return true
				}
			}
			
			ik.Mdl.CalculateJacobian()
			
			ik.Mdl.CalculateJacobianInverse(0, ik.svd)

			dq = ik.Mdl.GetJacobianInverse().MulNx1(nil, mgl64.NewVecNFromData(dx)).Raw()

			newPos := ik.Mdl.Step(q, dq)
			ik.Mdl.SetPosition(newPos)
			q = newPos
			
			if iteration % 300 == 0{
				break
			}
		}
		// TODO
		//~ ik.Mdl.SetPosition(ik.Mdl.GetRandomJointPositions())
	}
	fmt.Println("solved")
	return false
}
