// Package fake implements a fake arm.
package fake

import (
	"context"

	// used to import model referenceframe.
	_ "embed"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/config"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/arm/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

//go:embed static_arm_model.json
var armModelJSON []byte

func init() {
	registry.RegisterComponent(arm.Subtype, "fake", registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			if config.Attributes.Bool("fail_new", false) {
				return nil, errors.New("whoops")
			}
			return NewArm(config)
		},
	})
}

// NewArm returns a new fake arm.
func NewArm(cfg config.Component) (arm.Arm, error) {
	name := cfg.Name
	model, err := referenceframe.UnmarshalModelJSON(armModelJSON, "")
	if err != nil {
		return nil, err
	}
	return &Arm{
		Name:     name,
		position: &commonpb.Pose{},
		joints:   &pb.JointPositions{Degrees: []float64{0, 0, 0, 0, 0, 0}},
		model:    model,
	}, nil
}

// Arm is a fake arm that can simply read and set properties.
type Arm struct {
	generic.Echo
	Name       string
	position   *commonpb.Pose
	joints     *pb.JointPositions
	CloseCount int
	model      referenceframe.Model
}

// ModelFrame returns the dynamic frame of the model.
func (a *Arm) ModelFrame() referenceframe.Model {
	return a.model
}

// GetEndPosition returns the set position.
func (a *Arm) GetEndPosition(ctx context.Context) (*commonpb.Pose, error) {
	return a.position, nil
}

// MoveToPosition sets the position.
func (a *Arm) MoveToPosition(ctx context.Context, c *commonpb.Pose, worldState *commonpb.WorldState) error {
	a.position = c
	return nil
}

// MoveToJointPositions sets the joints.
func (a *Arm) MoveToJointPositions(ctx context.Context, joints *pb.JointPositions) error {
	a.joints = joints
	return nil
}

// GetJointPositions returns the set joints.
func (a *Arm) GetJointPositions(ctx context.Context) (*pb.JointPositions, error) {
	return a.joints, nil
}

// Stop doesn't do anything for a fake arm.
func (a *Arm) Stop(ctx context.Context) error {
	return nil
}

// CurrentInputs TODO.
func (a *Arm) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	res, err := a.GetJointPositions(ctx)
	if err != nil {
		return nil, err
	}
	return referenceframe.JointPosToInputs(res), nil
}

// GoToInputs TODO.
func (a *Arm) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	return a.MoveToJointPositions(ctx, referenceframe.InputsToJointPos(goal))
}

// Close does nothing.
func (a *Arm) Close() {
	a.CloseCount++
}
