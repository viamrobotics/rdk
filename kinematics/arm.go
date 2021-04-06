package kinematics

import (
	"context"
	"errors"
	"fmt"
	"math"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/kinematics/kinmath"
	pb "go.viam.com/robotcore/proto/api/v1"

	"github.com/edaniels/golog"
)

type Arm struct {
	real       api.Arm
	Model      *Model
	ik         InverseKinematics
	effectorID int
}

// Returns a new kinematics.Model from a correctly formatted JSON file
func NewArm(real api.Arm, jsonFile string, cores int, logger golog.Logger) (*Arm, error) {
	// We want to make (cores + 1) copies of our model
	// Our master copy, plus one for each of the IK engines to work with
	// We create them all now because deep copies of sufficiently complicated structs is a pain

	if cores < 1 {
		return nil, errors.New("need to have at least one CPU core")
	}
	models := make([]*Model, cores+1)
	for i := 0; i <= cores; i++ {
		model, err := ParseJSONFile(jsonFile, logger)
		if err != nil {
			return nil, err
		}
		models[i] = model
	}

	ik := CreateCombinedIKSolver(models, logger)
	return &Arm{real, models[0], ik, 0}, nil
}

func (k *Arm) Close(ctx context.Context) {
	k.real.Close(ctx) // TODO(erh): who owns this?
}

// Returns the end effector's current Position
func (k *Arm) GetForwardPosition() *pb.ArmPosition {
	k.Model.ForwardPosition()

	pos6d := k.Model.Get6dPosition(k.effectorID)

	pos := &pb.ArmPosition{}
	pos.X = pos6d[0]
	pos.Y = pos6d[1]
	pos.Z = pos6d[2]
	pos.RX = pos6d[3]
	pos.RY = pos6d[4]
	pos.RZ = pos6d[5]

	return pos
}

// Sets a new goal position
// Uses ZYX euler rotation order
// Takes DEGREES and converts to radians
func (k *Arm) SetForwardPosition(pos *pb.ArmPosition) error {
	pos.RX *= math.Pi / 180
	pos.RY *= math.Pi / 180
	pos.RZ *= math.Pi / 180
	transform := kinmath.NewQuatTransFromRotation(pos.RX, pos.RY, pos.RZ)
	transform.SetX(pos.X / 2)
	transform.SetY(pos.Y / 2)
	transform.SetZ(pos.Z / 2)

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
func (k *Arm) modelJointsPosition() []float64 {
	angles := k.Model.GetPosition()
	for i, angle := range angles {
		angles[i] = angle * 180 / math.Pi
	}
	return angles
}

// Sets new joint angles. Takes degrees, passes radians to Model
func (k *Arm) SetJointPositions(angles []float64) {
	radAngles := make([]float64, len(angles))
	for i, angle := range angles {
		radAngles[i] = angle * math.Pi / 180
	}
	k.Model.SetPosition(radAngles)
}

func (k *Arm) CurrentJointPositions(ctx context.Context) (*pb.JointPositions, error) {
	// If the real arm returns empty struct, nil then that means we should use the kinematics angles
	jp, err := k.real.CurrentJointPositions(ctx)

	if len(jp.Degrees) == 0 && err == nil {
		jp = &pb.JointPositions{Degrees: k.modelJointsPosition()}
	}
	return jp, err
}

func (k *Arm) CurrentPosition(ctx context.Context) (*pb.ArmPosition, error) {
	curPos, err := k.CurrentJointPositions(ctx)
	if err != nil {
		return &pb.ArmPosition{}, err
	}
	k.SetJointPositions(curPos.Degrees)
	pos := k.GetForwardPosition()

	pos.X /= 1000
	pos.Y /= 1000
	pos.Z /= 1000
	return pos, nil

}

func (k *Arm) JointMoveDelta(ctx context.Context, joint int, amount float64) error {
	return fmt.Errorf("not done yet")
}

func (k *Arm) MoveToJointPositions(ctx context.Context, jp *pb.JointPositions) error {
	err := k.real.MoveToJointPositions(ctx, jp)
	if err == nil {
		k.SetJointPositions(jp.Degrees)
	}
	return err
}

func (k *Arm) MoveToPosition(ctx context.Context, pos *pb.ArmPosition) error {
	pos.X *= 1000
	pos.Y *= 1000
	pos.Z *= 1000

	err := k.SetForwardPosition(pos)
	if err != nil {
		return err
	}

	joints := &pb.JointPositions{Degrees: k.modelJointsPosition()}

	return k.real.MoveToJointPositions(ctx, joints)
}
