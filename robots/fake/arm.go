package fake

import (
	"context"

	"github.com/go-errors/errors"

	"go.viam.com/core/arm"
	"go.viam.com/core/config"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"

	"github.com/edaniels/golog"
)

func init() {
	registry.RegisterArm("fake", func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (arm.Arm, error) {
		return NewArm(config.Name), nil
	})
}

// NewArm returns a new fake arm.
func NewArm(name string) *Arm {
	return &Arm{
		Name:     name,
		position: &pb.ArmPosition{},
		joints:   &pb.JointPositions{Degrees: []float64{0, 0, 0, 0, 0, 0}},
	}
}

// Arm is a fake arm that can simply read and set properties.
type Arm struct {
	Name       string
	position   *pb.ArmPosition
	joints     *pb.JointPositions
	CloseCount int
}

// CurrentPosition returns the set position.
func (a *Arm) CurrentPosition(ctx context.Context) (*pb.ArmPosition, error) {
	return a.position, nil
}

// MoveToPosition sets the position.
func (a *Arm) MoveToPosition(ctx context.Context, c *pb.ArmPosition) error {
	a.position = c
	return nil
}

// MoveToJointPositions sets the joints.
func (a *Arm) MoveToJointPositions(ctx context.Context, joints *pb.JointPositions) error {
	a.joints = joints
	return nil
}

// CurrentJointPositions returns the set joints.
func (a *Arm) CurrentJointPositions(ctx context.Context) (*pb.JointPositions, error) {
	return a.joints, nil
}

// JointMoveDelta returns an error.
func (a *Arm) JointMoveDelta(ctx context.Context, joint int, amountDegs float64) error {
	return errors.New("arm JointMoveDelta does nothing")
}

// Close does nothing.
func (a *Arm) Close() error {
	a.CloseCount++
	return nil
}
