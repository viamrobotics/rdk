// Package fake implements a fake arm.
package fake

import (
	"context"
	// used to import model referenceframe.
	_ "embed"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/arm/v1"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
)

//go:embed static_arm_model.json
var armModelJSON []byte

const modelname = "fake"

// AttrConfig is the config for a fake arm.
type AttrConfig struct {
	FailNew bool `json:"fail_new"`
}

// Validate ensures all parts of the config are valid.
func (config *AttrConfig) Validate(path string) error {
	if config.FailNew {
		return errors.New("whoops")
	}
	return nil
}

func init() {
	registry.RegisterComponent(arm.Subtype, modelname, registry.Component{
		Constructor: func(ctx context.Context, _ registry.Dependencies, cfg config.Component, logger golog.Logger) (interface{}, error) {
			return NewArm(cfg)
		},
	})

	config.RegisterComponentAttributeMapConverter(arm.SubtypeName, modelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &AttrConfig{})
}

// NewArm returns a new fake arm.
func NewArm(cfg config.Component) (arm.LocalArm, error) {
	name := cfg.Name
	model, err := referenceframe.UnmarshalModelJSON(armModelJSON, "")
	if err != nil {
		return nil, err
	}
	return &Arm{
		Name:     name,
		position: &commonpb.Pose{},
		joints:   &pb.JointPositions{Values: []float64{0, 0, 0, 0, 0, 0}},
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

// EndPosition returns the set position.
func (a *Arm) EndPosition(ctx context.Context, extra map[string]interface{}) (*commonpb.Pose, error) {
	return a.position, nil
}

// MoveToPosition sets the position.
func (a *Arm) MoveToPosition(ctx context.Context, c *commonpb.Pose, worldState *commonpb.WorldState, extra map[string]interface{}) error {
	a.position = c
	return nil
}

// MoveToJointPositions sets the joints.
func (a *Arm) MoveToJointPositions(ctx context.Context, joints *pb.JointPositions, extra map[string]interface{}) error {
	a.joints = joints
	return nil
}

// JointPositions returns the set joints.
func (a *Arm) JointPositions(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
	return a.joints, nil
}

// Stop doesn't do anything for a fake arm.
func (a *Arm) Stop(ctx context.Context, extra map[string]interface{}) error {
	return nil
}

// IsMoving is always false for a fake arm.
func (a *Arm) IsMoving(ctx context.Context) (bool, error) {
	return false, nil
}

// CurrentInputs TODO.
func (a *Arm) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	res, err := a.JointPositions(ctx, nil)
	if err != nil {
		return nil, err
	}
	return a.model.InputFromProtobuf(res), nil
}

// GoToInputs TODO.
func (a *Arm) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	return a.MoveToJointPositions(ctx, a.model.ProtobufFromInput(goal), nil)
}

// Close does nothing.
func (a *Arm) Close() {
	a.CloseCount++
}

// UpdateAction helps hinting the reconfiguration process on what strategy to use given a modified config.
// See config.UpdateActionType for more information.
func (a *Arm) UpdateAction(cfg *config.Component) config.UpdateActionType {
	return config.Reconfigure
}
