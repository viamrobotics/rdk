package arm

import (
	"fmt"
	"math"

	"github.com/viamrobotics/robotcore/kinematics/go-kin/kinmath"
	"github.com/viamrobotics/robotcore/kinematics/go-kin/mdl"
)

type Kinematics struct {
	ik         mdl.InverseKinematics
	effectorID int
}

// Returns a new mdl.Model from a correctly formatted XML file
// Note that ParseFile is currently very fragile
func NewRobot(xmlFile string) (*Kinematics, error) {
	m, err := mdl.ParseFile(xmlFile)
	if err != nil {
		return nil, err
	}
	// TODO: configurable IK method once more than one is supported
	ik := mdl.CreateJacobianIKSolver(m)
	return &Kinematics{ik, 0}, nil
}

// Returns the end effector's current Position
func (k *Kinematics) GetForwardPosition() Position {
	k.ik.ForwardPosition()

	// Angles will be in radians
	pos6d := k.ik.Get6dPosition(k.effectorID)

	pos := Position{}
	pos.X = pos6d[0]
	pos.Y = pos6d[1]
	pos.Z = pos6d[2]
	pos.Rx = pos6d[3] * 180 / math.Pi
	pos.Ry = pos6d[4] * 180 / math.Pi
	pos.Rz = pos6d[5] * 180 / math.Pi

	return pos
}

// Sets a new goal position
// Uses ZYX euler rotation order
func (k *Kinematics) SetForwardPosition(pos Position) error {
	transform := kinmath.NewTransformFromRotation(pos.Rx, pos.Ry, pos.Rz)
	transform.SetX(pos.X)
	transform.SetY(pos.Y)
	transform.SetZ(pos.Z)

	k.ik.AddGoal(transform, effectorID)
	couldSolve := k.ik.Solve()
	k.ik.ForwardPosition()
	if couldSolve {
		return nil
	} else {
		return fmt.Errorf("Could not solve for position")
	}
}

// Returns the arm's current joint angles in degrees
func (k *Kinematics) GetJointPositions() []float64 {
	angles := k.ik.Mdl.GetPosition()
	for i, angle := range angles {
		angles[i] = angle * 180 / math.Pi
	}
	return angles
}

// Sets new joint positions
func (k *Kinematics) SetJointPositions(pos []float64) {
	k.ik.Mdl.SetPosition(pos)
}
