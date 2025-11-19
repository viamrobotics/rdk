// Package fake implements a fake gantry.
package fake

import (
	"context"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/components/gantry"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils"
)

func init() {
	resource.RegisterComponent(
		gantry.API,
		resource.DefaultModelFamily.WithModel("fake"),
		resource.Registration[gantry.Gantry, resource.NoNativeConfig]{
			Constructor: func(
				ctx context.Context,
				_ resource.Dependencies,
				conf resource.Config,
				logger logging.Logger,
			) (gantry.Gantry, error) {
				return NewGantry(conf.ResourceName(), logger), nil
			},
		})
}

// NewGantry returns a new fake gantry.
func NewGantry(name resource.Name, logger logging.Logger) gantry.Gantry {
	m := referenceframe.NewSimpleModel("test_gantry")
	pose := spatialmath.NewZeroPose()
	baseRailGeom, err := spatialmath.NewBox(pose, r3.Vector{500, 100, 100}, "base_rail")
	if err != nil {
		logger.CErrorf(context.Background(), "could not create base rail geometry: %v", err)
	}
	f, err := referenceframe.NewStaticFrameWithGeometry("base_rail", pose, baseRailGeom)
	if err != nil {
		logger.CErrorf(context.Background(), "could not create static frame: %v", err)
	}
	m.SetOrdTransforms(append(m.OrdTransforms(), f))

	carriageGeom, err := spatialmath.NewBox(pose, r3.Vector{150, 120, 10}, "carriage")
	if err != nil {
		logger.CErrorf(context.Background(), "could not create carriage geometry: %v", err)
	}
	f, err = referenceframe.NewTranslationalFrameWithGeometry(
		"carriage", r3.Vector{1, 0, 0}, referenceframe.Limit{Min: 0, Max: 500}, carriageGeom)
	if err != nil {
		logger.CErrorf(context.Background(), "could not create translational frame: %v", err)
	}
	m.SetOrdTransforms(append(m.OrdTransforms(), f))

	return &Gantry{
		testutils.NewUnimplementedResource(name),
		resource.TriviallyReconfigurable{},
		resource.TriviallyCloseable{},
		[]float64{1.2},
		[]float64{120},
		[]float64{5},
		2,
		m,
		logger,
	}
}

// Gantry is a fake gantry that can simply read and set properties.
type Gantry struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	positionsMm    []float64
	speedsMmPerSec []float64
	lengths        []float64
	lengthMeters   float64
	model          referenceframe.Model
	logger         logging.Logger
}

// Position returns the position in meters.
func (g *Gantry) Position(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
	return g.positionsMm, nil
}

// Lengths returns the position in meters.
func (g *Gantry) Lengths(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
	return g.lengths, nil
}

// Home runs the homing sequence of the gantry and returns true once completed.
func (g *Gantry) Home(ctx context.Context, extra map[string]interface{}) (bool, error) {
	g.logger.CInfo(ctx, "homing")
	return true, nil
}

// MoveToPosition is in meters.
func (g *Gantry) MoveToPosition(ctx context.Context, positionsMm, speedsMmPerSec []float64, extra map[string]interface{}) error {
	g.positionsMm = positionsMm
	g.speedsMmPerSec = speedsMmPerSec
	return nil
}

// Stop doesn't do anything for a fake gantry.
func (g *Gantry) Stop(ctx context.Context, extra map[string]interface{}) error {
	return nil
}

// IsMoving is always false for a fake gantry.
func (g *Gantry) IsMoving(ctx context.Context) (bool, error) {
	return false, nil
}

// Geometries returns the geometries of the gantry.
func (g *Gantry) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	inputs, err := g.CurrentInputs(ctx)
	if err != nil {
		return nil, err
	}
	gif, err := g.model.Geometries(inputs)
	if err != nil {
		return nil, err
	}
	return gif.Geometries(), nil
}

// Kinematics returns the kinematic model associated with the gantry.
func (g *Gantry) Kinematics(ctx context.Context) (referenceframe.Model, error) {
	return g.model, nil
}

// CurrentInputs returns positions in the Gantry frame model..
func (g *Gantry) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	res, err := g.Position(ctx, nil)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// GoToInputs moves using the Gantry frames..
func (g *Gantry) GoToInputs(ctx context.Context, inputSteps ...[]referenceframe.Input) error {
	for _, goal := range inputSteps {
		err := g.MoveToPosition(ctx, goal, g.speedsMmPerSec, nil)
		if err != nil {
			return err
		}
	}
	return nil
}
