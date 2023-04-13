// Package fake implements a fake base.
package fake

import (
	"bytes"
	"context"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/base/wheeled"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/slam"
)

func init() {
	registry.RegisterComponent(
		base.Subtype,
		resource.NewDefaultModel("fake"),
		registry.Component{
			Constructor: func(
				ctx context.Context,
				_ registry.Dependencies,
				config config.Component,
				logger golog.Logger,
			) (interface{}, error) {
				return NewBase(ctx, config, logger)
			},
		},
	)
}

const defaultWidth = 600

var _ = base.LocalBase(&Base{})

// Base is a fake base that returns what it was provided in each method.
type Base struct {
	generic.Echo
	Name       string
	CloseCount int
	logger     golog.Logger
	geometry   *referenceframe.LinkConfig
}

// NewBase instantiates a new base of the fake model type.
func NewBase(ctx context.Context, cfg config.Component, logger golog.Logger) (base.LocalBase, error) {
	return &Base{
		Name:     cfg.Name,
		logger:   logger,
		geometry: cfg.Frame,
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
	return defaultWidth, nil
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

// WrapWithKinematics creates a KinematicBase from the fake Base so that it satisfies the ModelFramer and InputEnabled interfaces.
func (b *Base) WrapWithKinematics(ctx context.Context, slamSvc slam.Service) (base.KinematicBase, error) {
	geometry, err := base.CollisionGeometry(b.geometry)
	if err != nil {
		return nil, err
	}

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
	model, err := wheeled.Model(b.Name, geometry, limits)
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
