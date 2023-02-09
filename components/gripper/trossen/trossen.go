// Package trossen implements a trossen gripper.
package trossen

import (
	"context"
	"fmt"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils"

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
// as all of the gripper functions defer to commands on the arm implementation. See
// components/arm/trossen/trossen.go for more about the arm implementations
// and supported arms

var (
	modelNameWX250s = resource.NewDefaultModel("trossen-wx250s")
	modelNameVX300s = resource.NewDefaultModel("trossen-vx300s")
)

// AttrConfig is the config for a trossen gripper.
type AttrConfig struct {
	Arm string `json:"arm"`
}

// Validate ensures all parts of the config are valid.
func (config *AttrConfig) Validate(path string) ([]string, error) {
	if config.Arm == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "arm")
	}
	return []string{config.Arm}, nil
}

func init() {
	registry.RegisterComponent(gripper.Subtype, modelNameWX250s, registry.Component{
		Constructor: func(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			attr, ok := config.ConvertedAttributes.(*AttrConfig)
			if !ok {
				return nil, rdkutils.NewUnexpectedTypeError(attr, config.ConvertedAttributes)
			}
			return newGripper(config.Name, logger, deps, attr)
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
			return newGripper(config.Name, logger, deps, attr)
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
	name       string
	trossenArm *trossenarm.Arm
	logger     golog.Logger
	generic.Unimplemented
}

// newGripper returns an instance of a trossen Gripper.
func newGripper(name string, logger golog.Logger, deps registry.Dependencies, attr *AttrConfig) (gripper.LocalGripper, error) {
	var _arm *trossenarm.Arm
	a, err := arm.FromDependencies(deps, attr.Arm)
	if err != nil {
		return nil, err
	}
	_arm, ok := rdkutils.UnwrapProxy(a).(*trossenarm.Arm)
	if !ok {
		return nil, errors.Errorf(
			"arm specified for trossen gripper %s is not a trossen arm", name)
	}
	newGripper := Gripper{trossenArm: _arm, logger: logger, name: name}
	return &newGripper, nil
}

// Open opens the gripper by defering to the arm.
func (g *Gripper) Open(ctx context.Context, extra map[string]interface{}) error {
	if err := g.trossenArm.OpenGripper(ctx); err != nil {
		return errors.Wrap(err, fmt.Sprintf("open failed for trossen gripper %s", g.name))
	}
	return nil
}

// Grab closes the gripper by defering to the arm.
func (g *Gripper) Grab(ctx context.Context, extra map[string]interface{}) (bool, error) {
	hasGrabbed, err := g.trossenArm.Grab(ctx)
	if err != nil {
		return false, errors.Wrap(err, fmt.Sprintf("open failed for trossen gripper %s", g.name))
	}
	return hasGrabbed, nil
}

// Stop is stops the gripper servo on the trossen arm.
func (g *Gripper) Stop(ctx context.Context, extra map[string]interface{}) error {
	return g.trossenArm.StopGripper(ctx)
}

// IsMoving returns whether the gripper is moving.
func (g *Gripper) IsMoving(ctx context.Context) (bool, error) {
	return g.trossenArm.GripperIsMoving(ctx)
}

// ModelFrame is unimplemented for Gripper.
func (g *Gripper) ModelFrame() referenceframe.Model {
	return nil
}
