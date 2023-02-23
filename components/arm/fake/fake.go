// Package fake implements a fake arm.
package fake

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	pb "go.viam.com/api/component/arm/v1"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/arm/eva"
	ur "go.viam.com/rdk/components/arm/universalrobots"
	"go.viam.com/rdk/components/arm/xarm"
	"go.viam.com/rdk/components/arm/yahboom"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// errAttrCfgPopulation is the returned error if the AttrConfig's fields are fully populated.
var errAttrCfgPopulation = errors.New("can only populate either ArmModel or ModelPath - not both")

// ModelName is the string used to refer to the fake arm model.
var ModelName = resource.NewDefaultModel("fake")

// AttrConfig is used for converting config attributes.
type AttrConfig struct {
	ArmModel      string `json:"arm-model,omitempty"`
	ModelFilePath string `json:"model-path,omitempty"`
}

func modelFromName(model, name string) (referenceframe.Model, error) {
	switch resource.ModelName(model) {
	case xarm.ModelName6DOF.Name:
		return xarm.Model(name, 6)
	case xarm.ModelName7DOF.Name:
		return xarm.Model(name, 7)
	case ur.ModelName.Name:
		return ur.Model(name)
	case yahboom.ModelName.Name:
		return yahboom.Model(name)
	case eva.ModelName.Name:
		return eva.Model(name)
	default:
		return nil, errors.Errorf("fake arm cannot be created, unsupported arm_model: %s", model)
	}
}

// Validate ensures all parts of the config are valid.
func (config *AttrConfig) Validate(path string) error {
	var err error
	switch {
	case config.ArmModel != "" && config.ModelFilePath != "":
		err = errAttrCfgPopulation
	case config.ArmModel != "" && config.ModelFilePath == "":
		_, err = modelFromName(config.ArmModel, "")
	case config.ArmModel == "" && config.ModelFilePath != "":
		_, err = referenceframe.ModelFromPath(config.ModelFilePath, "")
	}
	return err
}

func init() {
	registry.RegisterComponent(arm.Subtype, ModelName, registry.Component{
		Constructor: func(ctx context.Context, _ registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			return NewArm(config, logger)
		},
	})

	config.RegisterComponentAttributeMapConverter(
		arm.Subtype,
		ModelName,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&AttrConfig{},
	)
}

// NewArm returns a new fake arm.
func NewArm(cfg config.Component, logger golog.Logger) (arm.LocalArm, error) {
	var (
		model referenceframe.Model
		err   error
	)

	modelPath := cfg.ConvertedAttributes.(*AttrConfig).ModelFilePath
	armModel := cfg.ConvertedAttributes.(*AttrConfig).ArmModel
	switch {
	case armModel != "" && modelPath != "":
		err = errAttrCfgPopulation
	case armModel != "":
		model, err = modelFromName(cfg.ConvertedAttributes.(*AttrConfig).ArmModel, cfg.Name)
	case modelPath != "":
		model, err = referenceframe.ModelFromPath(modelPath, cfg.Name)
	default:
		// if no arm model is specified, we return an empty arm with 0 dof and 0 spatial transformation
		model = referenceframe.NewSimpleModel(cfg.Name)
	}
	if err != nil {
		return nil, err
	}

	return &Arm{
		Name:   cfg.Name,
		joints: &pb.JointPositions{Values: make([]float64, len(model.DoF()))},
		model:  model,
		logger: logger,
	}, nil
}

// Arm is a fake arm that can simply read and set properties.
type Arm struct {
	generic.Echo
	Name       string
	joints     *pb.JointPositions
	CloseCount int
	logger     golog.Logger
	model      referenceframe.Model
}

// ModelFrame returns the dynamic frame of the model.
func (a *Arm) ModelFrame() referenceframe.Model {
	return a.model
}

// EndPosition returns the set position.
func (a *Arm) EndPosition(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
	joints, err := a.JointPositions(ctx, extra)
	if err != nil {
		return nil, err
	}
	return motionplan.ComputeOOBPosition(a.model, joints)
}

// MoveToPosition sets the position.
func (a *Arm) MoveToPosition(
	ctx context.Context,
	pos spatialmath.Pose,
	worldState *referenceframe.WorldState,
	extra map[string]interface{},
) error {
	joints, err := a.JointPositions(ctx, extra)
	if err != nil {
		return err
	}
	solution, err := motionplan.PlanFrameMotion(ctx, a.logger, pos, a.model, a.model.InputFromProtobuf(joints), nil)
	if err != nil {
		return err
	}
	return arm.GoToWaypoints(ctx, a, solution)
}

// MoveToJointPositions sets the joints.
func (a *Arm) MoveToJointPositions(ctx context.Context, joints *pb.JointPositions, extra map[string]interface{}) error {
	if err := arm.CheckDesiredJointPositions(ctx, a, joints.Values); err != nil {
		return err
	}
	inputs := a.model.InputFromProtobuf(joints)
	pos, err := a.model.Transform(inputs)
	if err != nil {
		return err
	}
	_ = pos
	copy(a.joints.Values, joints.Values)
	return nil
}

// JointPositions returns joints.
func (a *Arm) JointPositions(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
	retJoint := &pb.JointPositions{Values: a.joints.Values}
	return retJoint, nil
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
	positionDegs := a.model.ProtobufFromInput(goal)
	if err := arm.CheckDesiredJointPositions(ctx, a, positionDegs.Values); err != nil {
		return err
	}
	return a.MoveToJointPositions(ctx, positionDegs, nil)
}

// Close does nothing.
func (a *Arm) Close() {
	a.CloseCount++
}
