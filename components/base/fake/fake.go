// Package fake implements a fake base.
package fake

import (
	"bytes"
	"context"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/base"
	limo "go.viam.com/rdk/components/base/agilex"
	"go.viam.com/rdk/components/base/boat"
	"go.viam.com/rdk/components/base/wheeled"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/spatialmath"
)

// ModelName is the name of the fake model of a base component.
var ModelName = resource.NewDefaultModel("fake")

// Config is used for converting config attributes.
type Config struct {
	BaseModel string `json:"base-model,omitempty"`
}

func init() {
	registry.RegisterComponent(
		base.Subtype,
		ModelName,
		registry.Component{
			Constructor: func(
				ctx context.Context,
				_ registry.Dependencies,
				config config.Component,
				logger golog.Logger,
			) (interface{}, error) {
				return &Base{Name: config.Name}, nil
			},
		},
	)
}

var _ = base.LocalBase(&Base{})

// Base is a fake base that returns what it was provided in each method.
type Base struct {
	generic.Echo
	Name              string
	CloseCount        int
	logger            golog.Logger
	modelName         string
	collisionGeometry spatialmath.Geometry
}

// NewBase instantiates a new base of the fake model type.
func NewBase(ctx context.Context, cfg config.Component, logger golog.Logger) (base.LocalBase, error) {
	// TODO(rb): This should not ultimately be using the wheeled base package to do this
	geometry, err := base.CollisionGeometry(cfg)
	if err != nil {
		return nil, err
	}

	return &Base{
		Name:              cfg.Name,
		logger:            logger,
		modelName:         cfg.ConvertedAttributes.(*Config).BaseModel,
		collisionGeometry: geometry,
	}, nil
}

// MoveStraight does nothing.
func (b *Base) MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64, extra map[string]interface{}) error {
	return nil
}

// Spin does nothing.
func (b *Base) Spin(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error {
	return nil
}

// SetPower does nothing.
func (b *Base) SetPower(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
	return nil
}

// SetVelocity does nothing.
func (b *Base) SetVelocity(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
	return nil
}

// Width returns some arbitrary width.
func (b *Base) Width(ctx context.Context) (int, error) {
	return 600, nil
}

// Stop does nothing.
func (b *Base) Stop(ctx context.Context, extra map[string]interface{}) error {
	return nil
}

// IsMoving always returns false.
func (b *Base) IsMoving(ctx context.Context) (bool, error) {
	return false, nil
}

// Close does nothing.
func (b *Base) Close() {
	b.CloseCount++
}

type kinematicBase struct {
	*Base
	model  referenceframe.Model
	slam   slam.Service
	inputs []referenceframe.Input
}

// WrapWithKinematics creates a KinematicBase from the.
func (b *Base) WrapWithKinematics(ctx context.Context, slamSvc slam.Service) (base.KinematicBase, error) {
	// gets the extents of the SLAM map
	data, err := slam.GetPointCloudMapFull(ctx, slamSvc)
	if err != nil {
		return nil, err
	}
	dims, err := pointcloud.GetPCDMetaData(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	limits := []referenceframe.Limit{{Min: dims.MinX, Max: dims.MaxX}, {Min: dims.MinZ, Max: dims.MaxZ}}
	model, err := buildModel(b.modelName, b.Name, b.collisionGeometry, limits)
	if err != nil {
		return nil, errors.Wrap(err, "fake base cannot be created")
	}
	return &kinematicBase{
		Base:   b,
		model:  model,
		slam:   slamSvc,
		inputs: make([]referenceframe.Input, len(model.DoF())),
	}, nil
}

func (kb *kinematicBase) ModelFrame() referenceframe.Model {
	return kb.model
}

func (kb *kinematicBase) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	return kb.inputs, nil
}

func (kb *kinematicBase) GoToInputs(ctx context.Context, inputs []referenceframe.Input) error {
	_, err := kb.model.Transform(inputs)
	kb.inputs = inputs
	return err
}

func buildModel(model, name string, collisionGeometry spatialmath.Geometry, limits []referenceframe.Limit) (referenceframe.Model, error) {
	switch resource.ModelName(model) {
	case wheeled.ModelName.Name:
		return wheeled.Model(name, collisionGeometry, limits)
	case boat.ModelName.Name, limo.ModelName.Name:
		return nil, errors.Errorf("base-model %s does not satisfy KinematicBase", model)
	default:
		return nil, errors.Errorf("unsupported base-model: %s", model)
	}
}
