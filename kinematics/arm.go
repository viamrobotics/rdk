package kinematics

import (
	"context"
	"io/ioutil"
	"math"

	"github.com/go-errors/errors"

	goutils "go.viam.com/utils"

	"go.viam.com/core/arm"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/spatialmath"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
)

// Arm TODO
type Arm struct {
	real       arm.Arm
	Model      *Model
	ik         InverseKinematics
	effectorID int
}

// NewArmJSONFile TODO
func NewArmJSONFile(real arm.Arm, jsonFile string, cores int, logger golog.Logger) (*Arm, error) {
	jsonData, err := ioutil.ReadFile(jsonFile)
	if err != nil {
		return nil, err
	}
	return NewArm(real, jsonData, cores, logger)
}

// NewArm returns a new kinematics.Model from a correctly formatted JSON file
func NewArm(real arm.Arm, jsonData []byte, cores int, logger golog.Logger) (*Arm, error) {
	// We want to make (cores + 1) copies of our model
	// Our master copy, plus one for each of the IK engines to work with
	// We create them all now because deep copies of sufficiently complicated structs is a pain

	if cores < 1 {
		return nil, errors.New("need to have at least one CPU core")
	}
	models := make([]*Model, cores+1)
	for i := 0; i <= cores; i++ {
		model, err := ParseJSON(jsonData)
		if err != nil {
			return nil, err
		}
		models[i] = model
	}

	ik := CreateCombinedIKSolver(models, logger)
	return &Arm{real, models[0], ik, 0}, nil
}

// Close attempts to close the real arm.
func (k *Arm) Close() error {
	return goutils.TryClose(k.real) // TODO(erh): who owns this?
}

// GetForwardPosition returns the end effector's current Position
func (k *Arm) GetForwardPosition() *pb.ArmPosition {
	k.Model.ForwardPosition()

	pos6d := k.Model.Get6dPosition(k.effectorID)

	return pos6d
}

// SetForwardPosition sets a new goal position.
// Uses ZYX Euler rotation order.
// Takes degrees as input and converts to radians for kinematics use.
func (k *Arm) SetForwardPosition(pos *pb.ArmPosition) error {
	transform := spatialmath.NewDualQuaternionFromArmPos(pos)
	// See: https://en.wikipedia.org/wiki/Dual_quaternion#More_on_spatial_displacements

	k.ik.AddGoal(transform, k.effectorID)
	couldSolve := k.ik.Solve()
	k.Model.ForwardPosition()
	k.ik.ClearGoals()
	if couldSolve {
		return nil
	}
	return errors.Errorf("could not solve for position. Target: %v", pos)
}

// modelJointsPosition returns the arm's current joint angles in degrees
func (k *Arm) modelJointsPosition() []float64 {
	angles := k.Model.GetPosition()
	for i, angle := range angles {
		angles[i] = angle * 180 / math.Pi
	}
	return angles
}

// SetJointPositions sets new joint angles. Takes degrees, passes radians to Model
func (k *Arm) SetJointPositions(angles []float64) {
	radAngles := make([]float64, len(angles))
	for i, angle := range angles {
		radAngles[i] = utils.DegToRad(angle)
	}
	k.Model.SetPosition(radAngles)
}

// CurrentJointPositions returns the arm's current joint positions based on what has been set.
func (k *Arm) CurrentJointPositions(ctx context.Context) (*pb.JointPositions, error) {
	// If the real arm returns empty struct, nil then that means we should use the kinematics angles
	jp, err := k.real.CurrentJointPositions(ctx)

	if len(jp.Degrees) == 0 && err == nil {
		jp = &pb.JointPositions{Degrees: k.modelJointsPosition()}
	}
	return jp, err
}

// CurrentPosition returns the arm's current position based on what has been set.
func (k *Arm) CurrentPosition(ctx context.Context) (*pb.ArmPosition, error) {
	curPos, err := k.CurrentJointPositions(ctx)
	if err != nil {
		return &pb.ArmPosition{}, err
	}
	k.SetJointPositions(curPos.Degrees)
	pos := k.GetForwardPosition()

	return pos, nil

}

// JointMoveDelta is not yet implemented.
func (k *Arm) JointMoveDelta(ctx context.Context, joint int, amountDegs float64) error {
	return errors.New("not done yet")
}

// MoveToJointPositions instructs the arm to move to the given joint positions.
func (k *Arm) MoveToJointPositions(ctx context.Context, jp *pb.JointPositions) error {
	err := k.real.MoveToJointPositions(ctx, jp)
	if err == nil {
		k.SetJointPositions(jp.Degrees)
	}
	return err
}

// MoveToPosition instructs the arm to move to the current position.
func (k *Arm) MoveToPosition(ctx context.Context, pos *pb.ArmPosition) error {
	err := k.SetForwardPosition(pos)
	if err != nil {
		return err
	}

	joints := &pb.JointPositions{Degrees: k.modelJointsPosition()}

	return k.MoveToJointPositions(ctx, joints)
}
