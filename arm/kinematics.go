package arm

import (
	"errors"
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

// Returns a new kinematics.Model from a correctly formatted JSON file
func NewRobot(jsonFile string, cores int) (*Kinematics, error) {
	// We want to make (cores + 1) copies of our model
	// Our master copy, plus one for each of the IK engines to work with
	// We create them all now because deep copies of sufficiently complicated structs is a pain

	if cores < 1 {
		return nil, errors.New("need to have at least one CPU core")
	}
	models := make([]*kinematics.Model, cores+1)
	for i := 0; i <= cores; i++ {
		model, err := kinematics.ParseJSONFile(jsonFile)
		if err != nil {
			return nil, err
		}
		models[i] = model
	}

	ik := kinematics.CreateCombinedIKSolver(models)
	return &Kinematics{models[0], ik, 0}, nil
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
	radAngles := make([]float64, len(angles))
	for i, angle := range angles {
		radAngles[i] = angle * math.Pi / 180
	}
	k.Model.SetPosition(radAngles)
}
