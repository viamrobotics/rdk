// Package trossen implements a trossen gripper.
package trossen

import (
	"context"
	"errors"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/components/arm"
	trossenarm "go.viam.com/rdk/components/arm/trossen"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	rdkutils "go.viam.com/rdk/utils"
)

// This is an implementation of the Gripper API for Trossen arms. The gripper
// requires a single Trossen arm component in its dependencies upon configuration,
// as all of the gripper functions defer to DoCommand on the arm implementation. See
// components/arm/trossen/trossen.go for more about the arm implementations
// and supported arms

var (
	modelNameWX250s = resource.NewDefaultModel("trossen-wx250s")
	modelNameVX300s = resource.NewDefaultModel("trossen-vx300s")
)

// AttrConfig is the config for a trossen gripper. WARNING: These
// attributes no longer do anything and should be removed in a
// later commit - GV
type AttrConfig struct {
	SerialPath string `json:"serial_path,omitempty"`
	BaudRate   int    `json:"serial_baud_rate,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (config *AttrConfig) Validate(path string) error {
	return nil
}

func init() {
	registry.RegisterComponent(gripper.Subtype, modelNameWX250s, registry.Component{
		Constructor: func(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			attr, ok := config.ConvertedAttributes.(*AttrConfig)
			if !ok {
				return nil, rdkutils.NewUnexpectedTypeError(attr, config.ConvertedAttributes)
			}
			return newGripper(attr, logger, deps)
		},
	})

	config.RegisterComponentAttributeMapConverter(gripper.Subtype, modelNameWX250s,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &AttrConfig{})

	registry.RegisterComponent(gripper.Subtype, modelNameVX300s, registry.Component{
		Constructor: func(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			attr, ok := config.ConvertedAttributes.(*AttrConfig)
			if !ok {
				return nil, rdkutils.NewUnexpectedTypeError(attr, config.ConvertedAttributes)
			}
			return newGripper(attr, logger, deps)
		},
	})

	config.RegisterComponentAttributeMapConverter(gripper.Subtype, modelNameVX300s,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &AttrConfig{})
}

// Gripper represents an instance of a Trossen gripper.
type Gripper struct {
	trossenArm arm.LocalArm
	generic.Unimplemented
}

// newGripper TODO.
func newGripper(attributes *AttrConfig, logger golog.Logger, deps registry.Dependencies) (gripper.LocalGripper, error) {
	var _arm arm.LocalArm
	// TODO: an arm name should be specified for the gripper as a configuration
	// attribute in a future commit. This is a breaking change that needs to be
	// scoped - GV
	for _, d := range deps {
		a, ok := d.(arm.LocalArm)
		if ok {
			_arm = a
		} else if _arm != nil {
			return nil, errors.New(
				"multiple arms found in dependencies, trossen gripper needs one specific trossen arm in dependencies",
			)
		}
	}
	if _arm == nil {
		return nil, errors.New("need a trossen arm in depends_on")
	}
	newGripper := Gripper{trossenArm: _arm}
	return &newGripper, nil
}

// Open opens the gripper by defering to the arm.
func (g *Gripper) Open(ctx context.Context, extra map[string]interface{}) error {
	_, err := g.trossenArm.DoCommand(ctx,
		map[string]interface{}{"command": trossenarm.TrossenGripperOpen},
	)
	return err
}

// Grab closes the gripper by defering to the arm.
func (g *Gripper) Grab(ctx context.Context, extra map[string]interface{}) (bool, error) {
	cmdResp, err := g.trossenArm.DoCommand(ctx,
		map[string]interface{}{"command": trossenarm.TrossenGripperClose},
	)
	if err != nil {
		return false, err
	}
	return cmdResp["grabbed"].(bool), nil
}

// Stop is unimplemented for Gripper.
func (g *Gripper) Stop(ctx context.Context, extra map[string]interface{}) error {
	return g.trossenArm.Stop(ctx, extra)
}

// IsMoving returns whether the gripper is moving.
func (g *Gripper) IsMoving(ctx context.Context) (bool, error) {
	return g.trossenArm.IsMoving(ctx)
}

// ModelFrame is unimplemented for Gripper.
func (g *Gripper) ModelFrame() referenceframe.Model {
	return nil
}
