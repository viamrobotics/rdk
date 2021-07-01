package kinematics

import (
	"context"
	"io/ioutil"
	"math"

	"github.com/go-errors/errors"

	goutils "go.viam.com/utils"

	"go.viam.com/core/arm"
	pb "go.viam.com/core/proto/api/v1"

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

// modelJointsPosition returns the arm's current joint angles in degrees
func (k *Arm) modelJointsPosition() []float64 {
	angles := k.Model.GetPosition()
	for i, angle := range angles {
		angles[i] = angle * 180 / math.Pi
	}
	return angles
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
	pos := ComputePosition(k.Model, curPos)

	return pos, nil

}

// JointMoveDelta is not yet implemented.
func (k *Arm) JointMoveDelta(ctx context.Context, joint int, amountDegs float64) error {
	return errors.New("not done yet")
}

// MoveToJointPositions instructs the arm to move to the given joint positions.
func (k *Arm) MoveToJointPositions(ctx context.Context, jp *pb.JointPositions) error {
	return k.real.MoveToJointPositions(ctx, jp)
}

// MoveToPosition instructs the arm to move to the current position.
func (k *Arm) MoveToPosition(ctx context.Context, pos *pb.ArmPosition) error {
	seedJoints, err := k.CurrentJointPositions(ctx)
	if err != nil {
		return err
	}
	couldSolve, joints := k.ik.Solve(pos, seedJoints)
	if couldSolve {
		return k.MoveToJointPositions(ctx, joints)
	}
	return errors.Errorf("could not solve for position. Target: %v", pos)
}
