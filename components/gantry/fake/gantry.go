// Package fake implements a fake gantry.
package fake

import (
	"context"
	_ "embed"
	"encoding/json"

	"github.com/pkg/errors"

	"go.viam.com/rdk/components/gantry"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// errAttrCfgPopulation is the returned error if the Config's fields are fully populated.
var errAttrCfgPopulation = errors.New("can only populate either model_path or length_mm - not both")

// Model is the name used to refer to the fake gantry model.
var Model = resource.DefaultModelFamily.WithModel("fake")

// Config is used for converting config attributes.
// Fake gantry is single-axis only; use model_path for a custom prismatic direction.
type Config struct {
	ModelFilePath string  `json:"model_path,omitempty"`
	LengthMm      float64 `json:"length_mm,omitempty"`
}

//go:embed test_gantry_model.json
var gantryModelJSON []byte

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, []string, error) {
	var err error
	switch {
	case conf.ModelFilePath != "" && conf.LengthMm != 0:
		err = errAttrCfgPopulation
	case conf.ModelFilePath != "":
		_, err = referenceframe.KinematicModelFromFile(conf.ModelFilePath, "")
	case conf.LengthMm != 0:
		err = conf.validateLengthMm()
	}
	return nil, nil, err
}

func (conf *Config) validateLengthMm() error {
	if conf.LengthMm <= 0 {
		return errors.Errorf("length_mm must be positive, got %v", conf.LengthMm)
	}
	return nil
}

func init() {
	resource.RegisterComponent(gantry.API, Model, resource.Registration[gantry.Gantry, *Config]{
		Constructor: NewGantry,
	})
}

// NewGantry returns a new fake gantry.
func NewGantry(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger) (gantry.Gantry, error) {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}

	model, err := buildModel(conf, newConf)
	if err != nil {
		return nil, err
	}
	if len(model.DoF()) != 1 {
		return nil, errors.Errorf("gantry model must have exactly one degree of freedom, got %d", len(model.DoF()))
	}

	return &Gantry{
		Named:          conf.ResourceName().AsNamed(),
		positionsMm:    []float64{model.DoF()[0].Max / 2},
		speedsMmPerSec: []float64{50},
		lengthsMm:      []float64{model.DoF()[0].Max},
		model:          model,
		logger:         logger,
	}, nil
}

func buildModel(cfg resource.Config, newConf *Config) (referenceframe.Model, error) {
	var (
		model referenceframe.Model
		err   error
	)
	modelPath := newConf.ModelFilePath

	switch {
	case modelPath != "" && newConf.LengthMm != 0:
		err = errAttrCfgPopulation
	case modelPath != "":
		model, err = referenceframe.KinematicModelFromFile(modelPath, cfg.Name)
	case newConf.LengthMm != 0:
		model, err = modelFromLength(cfg.Name, newConf.LengthMm)
	default:
		// if no model_path or length_mm are specified, use the embedded single-axis model
		model, err = referenceframe.UnmarshalModelJSON(gantryModelJSON, cfg.Name)
	}

	return model, err
}

// Gantry is a fake gantry that can simply read and set properties.
type Gantry struct {
	resource.Named
	resource.TriviallyCloseable
	positionsMm    []float64
	speedsMmPerSec []float64
	lengthsMm      []float64
	model          referenceframe.Model
	logger         logging.Logger
}

// Position returns the position in mm.
func (g *Gantry) Position(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
	return g.positionsMm, nil
}

// Lengths returns the lengths of the axes of the gantry in mm.
func (g *Gantry) Lengths(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
	return g.lengthsMm, nil
}

// Home runs the homing sequence of the gantry and returns true once completed.
func (g *Gantry) Home(ctx context.Context, extra map[string]interface{}) (bool, error) {
	g.logger.CInfo(ctx, "homing")
	return true, nil
}

// MoveToPosition sets the position in mm.
func (g *Gantry) MoveToPosition(ctx context.Context, positionsMm, speedsMmPerSec []float64, extra map[string]interface{}) error {
	for i, position := range positionsMm {
		if position < 0 || position > g.lengthsMm[i] {
			return errors.Errorf("position %v out of range [0, %v]", position, g.lengthsMm[i])
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

// CurrentInputs returns the current inputs of the fake gantry.
func (g *Gantry) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	res, err := g.Position(ctx, nil)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// GoToInputs moves the fake gantry to the given inputs.
func (g *Gantry) GoToInputs(ctx context.Context, inputSteps ...[]referenceframe.Input) error {
	for _, goal := range inputSteps {
		err := g.MoveToPosition(ctx, goal, g.speedsMmPerSec, nil)
		if err != nil {
			return err
		}
	}
	return nil
}

func modelFromLength(name string, lengthMm float64) (referenceframe.Model, error) {
	mcfg := &referenceframe.ModelConfigJSON{}
	if err := json.Unmarshal(gantryModelJSON, mcfg); err != nil {
		return nil, err
	}
	if len(mcfg.Joints) != 1 {
		return nil, errors.Errorf("embedded gantry model must have exactly one joint, got %d", len(mcfg.Joints))
	}
	mcfg.Joints[0].Max = lengthMm
	return mcfg.ParseConfig(name)
}
