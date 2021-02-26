package arm

import (
	"fmt"
	"math"

	"go.viam.com/robotcore/kinematics"
	"go.viam.com/robotcore/kinematics/kinmath"
)

type Kinematics struct {
	Model      *kinematics.Model
	ik         kinematics.InverseKinematics
	effectorID int
}

// Returns a new kinematics.Model from a correctly formatted XML file
// Note that ParseFile is currently very fragile
func NewRobot(jsonFile string) (*Kinematics, error) {
	m, err := kinematics.ParseJSONFile(jsonFile)
	if err != nil {
		return nil, err
	}
	// TODO: configurable IK method once more than one is supported
	ik := kinematics.CreateJacobianIKSolver(m)
	return &Kinematics{m, ik, 0}, nil
}

// Returns the end effector's current Position
func (k *Kinematics) GetForwardPosition() Position {
	k.Model.ForwardPosition()

	pos6d := k.Model.Get6dPosition(k.effectorID)

	pos := Position{}
	pos.X = pos6d[0]
	pos.Y = pos6d[1]
	pos.Z = pos6d[2]
	pos.Rx = pos6d[3]
	pos.Ry = pos6d[4]
	pos.Rz = pos6d[5]

	return pos
}

// Sets a new goal position
// Uses ZYX euler rotation order
func (k *Kinematics) SetForwardPosition(pos Position) error {
	transform := kinmath.NewTransformFromRotation(pos.Rx, pos.Ry, pos.Rz)
	transform.SetX(pos.X)
	transform.SetY(pos.Y)
	transform.SetZ(pos.Z)

	k.ik.AddGoal(transform, k.effectorID)
	couldSolve := k.ik.Solve()
	k.Model.ForwardPosition()
	k.ik.ClearGoals()
	if couldSolve {
		return nil
	}
	return fmt.Errorf("could not solve for position. Target: %v", pos)
}

// Returns the arm's current joint angles in degrees
func (k *Kinematics) GetJointPositions() []float64 {
	angles := k.Model.GetPosition()
	for i, angle := range angles {
		angles[i] = angle * 180 / math.Pi
	}
	return angles
}

// Sets new joint angles. Takes degrees, passes radians to Model
func (k *Kinematics) SetJointPositions(angles []float64) {
	for i, angle := range angles {
		angles[i] = angle * math.Pi / 180
	}
	k.Model.SetPosition(angles)
}
