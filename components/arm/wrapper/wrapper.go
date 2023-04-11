// Package wrapper is a package that defines an implementation that wraps a partially implemented arm
package wrapper

import (
	"context"

	"github.com/edaniels/golog"
	pb "go.viam.com/api/component/arm/v1"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// AttrConfig is used for converting config attributes.
type AttrConfig struct {
	ModelFilePath string `json:"model-path"`
	ArmName       string `json:"arm-name"`
}

var model = resource.NewDefaultModel("wrapper_arm")

// Validate ensures all parts of the config are valid.
func (cfg *AttrConfig) Validate(path string) ([]string, error) {
	var deps []string
	if cfg.ArmName == "" {
		return nil, goutils.NewConfigValidationFieldRequiredError(path, "arm-name")
	}
	if _, err := referenceframe.ModelFromPath(cfg.ModelFilePath, ""); err != nil {
		return nil, err
	}
	deps = append(deps, cfg.ArmName)
	return deps, nil
}

func init() {
	registry.RegisterComponent(arm.Subtype, model, registry.Component{
		Constructor: func(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			return NewWrapperArm(config, deps, logger)
		},
	})

	config.RegisterComponentAttributeMapConverter(arm.Subtype, model,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&AttrConfig{},
	)
}

// Arm wraps a partial implementation of another arm.
type Arm struct {
	generic.Unimplemented
	Name   string
	model  referenceframe.Model
	actual arm.Arm
	logger golog.Logger
	opMgr  operation.SingleOperationManager
}

// NewWrapperArm returns a wrapper component for another arm.
func NewWrapperArm(cfg config.Component, deps registry.Dependencies, logger golog.Logger) (arm.LocalArm, error) {
	attrs, ok := cfg.ConvertedAttributes.(*AttrConfig)
	if !ok {
		return nil, utils.NewUnexpectedTypeError(attrs, cfg.ConvertedAttributes)
	}

	modelPath := attrs.ModelFilePath
	model, err := referenceframe.ModelFromPath(modelPath, cfg.Name)
	if err != nil {
		return nil, err
	}

	wrappedArm, err := arm.FromDependencies(deps, attrs.ArmName)
	if err != nil {
		return nil, err
	}
	return &Arm{
		Name:   cfg.Name,
		model:  model,
		actual: wrappedArm,
		logger: logger,
	}, nil
}

// UpdateAction helps hinting the reconfiguration process on what strategy to use given a modified config.
// See config.UpdateActionType for more information.
func (wrapper *Arm) UpdateAction(c *config.Component) config.UpdateActionType {
	if _, ok := c.ConvertedAttributes.(*AttrConfig); !ok {
		return config.Rebuild
	}

	modelFilePath := c.ConvertedAttributes.(*AttrConfig).ModelFilePath
	armName := c.ConvertedAttributes.(*AttrConfig).ArmName
	if modelFilePath != "" && armName == "" {
		// there is case where ok == true but newCfg.ModelFilePath == ""
		// because newCfg.ArmName is required as well.
		if model, err := referenceframe.ModelFromPath(modelFilePath, ""); err != nil {
			// unlikely to hit debug as we check for errors in Validate()
			wrapper.logger.Debugw("invalid model file path:", "error", err.Error())
		} else {
			wrapper.model = model
		}
		return config.None
	}

	return config.Reconfigure
}

// ModelFrame returns the dynamic frame of the model.
func (wrapper *Arm) ModelFrame() referenceframe.Model {
	return wrapper.model
}

// EndPosition returns the set position.
func (wrapper *Arm) EndPosition(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
	joints, err := wrapper.JointPositions(ctx, extra)
	if err != nil {
		return nil, err
	}
	return motionplan.ComputeOOBPosition(wrapper.model, joints)
}

// MoveToPosition sets the position.
func (wrapper *Arm) MoveToPosition(ctx context.Context, pos spatialmath.Pose, extra map[string]interface{}) error {
	ctx, done := wrapper.opMgr.New(ctx)
	defer done()
	return arm.Move(ctx, wrapper.logger, wrapper, pos)
}

// MoveToJointPositions sets the joints.
func (wrapper *Arm) MoveToJointPositions(ctx context.Context, joints *pb.JointPositions, extra map[string]interface{}) error {
	// check that joint positions are not out of bounds
	if err := arm.CheckDesiredJointPositions(ctx, wrapper, joints.Values); err != nil {
		return err
	}
	ctx, done := wrapper.opMgr.New(ctx)
	defer done()

	return wrapper.actual.MoveToJointPositions(ctx, joints, extra)
}

// JointPositions returns the set joints.
func (wrapper *Arm) JointPositions(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
	joints, err := wrapper.actual.JointPositions(ctx, extra)
	if err != nil {
		return nil, err
	}
	return joints, nil
}

// Stop stops the actual arm.
func (wrapper *Arm) Stop(ctx context.Context, extra map[string]interface{}) error {
	ctx, done := wrapper.opMgr.New(ctx)
	defer done()

	return wrapper.actual.Stop(ctx, extra)
}

// IsMoving returns whether the arm is moving.
func (wrapper *Arm) IsMoving(ctx context.Context) (bool, error) {
	return wrapper.opMgr.OpRunning(), nil
}

// CurrentInputs returns the current inputs of the arm.
func (wrapper *Arm) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	res, err := wrapper.actual.JointPositions(ctx, nil)
	if err != nil {
		return nil, err
	}
	return wrapper.model.InputFromProtobuf(res), nil
}

// GoToInputs moves the arm to the specified goal inputs.
func (wrapper *Arm) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	// check that joint positions are not out of bounds
	positionDegs := wrapper.model.ProtobufFromInput(goal)
	if err := arm.CheckDesiredJointPositions(ctx, wrapper, positionDegs.Values); err != nil {
		return err
	}
	return wrapper.MoveToJointPositions(ctx, positionDegs, nil)
}
