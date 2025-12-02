// Package fake implements a fake gantry.
package fake

import (
	"context"
	_ "embed"
	"fmt"

	"go.viam.com/rdk/components/gantry"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils"
)

//go:embed test_gantry_model.json
var gantryModelJSON []byte

// Config is used for converting config attributes.
type Config struct {
	ModelFilePath string `json:"model-path,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, []string, error) {
	var err error
	if conf.ModelFilePath != "" {
		_, err = referenceframe.KinematicModelFromFile(conf.ModelFilePath, "")
	}
	return nil, nil, err
}

func makeGantryModel(cfg resource.Config, newConf *Config) (referenceframe.Model, error) {
	var (
		model referenceframe.Model
		err   error
	)
	modelPath := newConf.ModelFilePath

	switch {
	case modelPath != "":
		model, err = referenceframe.KinematicModelFromFile(modelPath, cfg.Name)
	default:
		// if no arm model is specified, we return a fake arm with 1 dof and 0 spatial transformation
		model, err = referenceframe.UnmarshalModelJSON(gantryModelJSON, cfg.Name)
	}
	return model, err
}

func init() {
	resource.RegisterComponent(
		gantry.API,
		resource.DefaultModelFamily.WithModel("fake"),
		resource.Registration[gantry.Gantry, *Config]{
			Constructor: func(
				ctx context.Context,
				_ resource.Dependencies,
				conf resource.Config,
				logger logging.Logger,
			) (gantry.Gantry, error) {
				return NewGantry(conf, logger)
			},
		})
}

// NewGantry returns a new fake gantry.
func NewGantry(conf resource.Config, logger logging.Logger) (gantry.Gantry, error) {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}

	m, err := makeGantryModel(conf, newConf)
	if err != nil {
		return nil, err
	}

	return &Gantry{
		testutils.NewUnimplementedResource(conf.ResourceName()),
		resource.TriviallyReconfigurable{},
		resource.TriviallyCloseable{},
		[]float64{120},
		[]float64{50},
		[]float64{350},
		m,
		logger,
	}, nil
}

// Gantry is a fake gantry that can simply read and set properties.
type Gantry struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	positionsMm    []float64
	speedsMmPerSec []float64
	lengthsMm      []float64
	model          referenceframe.Model
	logger         logging.Logger
}

// Position returns the position in meters.
func (g *Gantry) Position(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
	return g.positionsMm, nil
}

// Lengths returns the position in meters.
func (g *Gantry) Lengths(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
	return g.lengthsMm, nil
}

// Home runs the homing sequence of the gantry and returns true once completed.
func (g *Gantry) Home(ctx context.Context, extra map[string]interface{}) (bool, error) {
	g.logger.CInfo(ctx, "homing")
	return true, nil
}

// MoveToPosition is in meters.
func (g *Gantry) MoveToPosition(ctx context.Context, positionsMm, speedsMmPerSec []float64, extra map[string]interface{}) error {
	for i, position := range positionsMm {
		if position < 0 || position > g.lengthsMm[i] {
			return fmt.Errorf("position %v out of range [0, %v]", position, g.lengthsMm[i])
		}
	}

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
