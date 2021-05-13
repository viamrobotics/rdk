package kinematics

import (
	"go.viam.com/core/kinematics/kinmath"
	"go.viam.com/core/rlog"

	"github.com/edaniels/golog"
	"github.com/go-gl/mathgl/mgl64"
	"gonum.org/v1/gonum/mat"
)

type JacobianIK struct {
	Mdl           *Model
	epsilon       float64
	iterations    int
	svd           bool
	Goals         []Goal
	ID            int
	requestHaltCh chan struct{}
	haltedCh      chan struct{}
}

func CreateJacobianIKSolver(mdl *Model) *JacobianIK {
	var ik JacobianIK
	ik.resetHalting()
	ik.Mdl = mdl
	ik.epsilon = 0.01
	ik.iterations = 3000
	ik.svd = true
	return &ik
}

func (ik *JacobianIK) AddGoal(trans *kinmath.QuatTrans, effectorID int) {
	newtrans := &kinmath.QuatTrans{}
	*newtrans = *trans
	ik.Goals = append(ik.Goals, Goal{newtrans, effectorID})
	ik.resetHalting()
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
	ik.resetHalting()
}

func (ik *JacobianIK) GetGoals() []Goal {
	return ik.Goals
}

func (ik *JacobianIK) resetHalting() {
	ik.requestHaltCh = make(chan struct{})
	ik.haltedCh = make(chan struct{})
}

func (ik *JacobianIK) Halt() {
	close(ik.requestHaltCh)
	<-ik.haltedCh
	ik.resetHalting()
}

func (ik *JacobianIK) Solve() bool {
	select {
	case <-ik.haltedCh:
		rlog.Logger.Info("solver halted before solving start; possibly solving twice in a row?")
		return false
	default:
	}
	defer close(ik.haltedCh)
	// q is the position over which we will iterate
	q := ik.Mdl.GetPosition()

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
			select {
			case <-ik.requestHaltCh:
				return false
			default:
			}
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
			if WeightedSquaredNorm(dx, ik.Mdl.DistCfg) < ik.epsilon*ik.epsilon {
				ik.Mdl.SetPosition(q)
				q = ik.Mdl.ZeroInlineRotation(q)
				if ik.Mdl.IsValid(q) {
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
			ik.Mdl.SetPosition(ik.Mdl.RandomJointPositions())
		}
	}
	ik.Mdl.SetPosition(origJointPos)
	return false
}

func PrintMat(m *mgl64.MatMxN, name string, logger golog.Logger) {
	j2 := mat.NewDense(m.NumRows(), m.NumCols(), m.Transpose(nil).Raw())
	fc := mat.Formatted(j2, mat.Prefix("      "), mat.Squeeze())
	logger.Debugf("%s = %v", name, fc)
}
