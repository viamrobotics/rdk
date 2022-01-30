// Package multiaxis implements a multi-axis gantry.
package multiaxis

import (
	"context"
	"fmt"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/gantry"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
	rdkutils "go.viam.com/rdk/utils"
)

// Config is used for converting multiAxis config attributes.
type Config struct {
	SubAxes []string `json:"subaxes_list"`
}

type multiAxis struct {
	name      string
	subAxes   []gantry.Gantry
	lengthsMm []float64
	logger    golog.Logger
}

// Validate ensures all parts of the config are valid.
func (config *Config) Validate(path string) error {
	if len(config.SubAxes) == 0 {
		return utils.NewConfigValidationError(path, errors.New("need at least one axis"))
	}

	return nil
}

func init() {
	registry.RegisterComponent(gantry.Subtype, "multiaxis", registry.Component{
		Constructor: func(ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			multaxisGantryConfig, ok := config.ConvertedAttributes.(*Config)
			if !ok {
				return nil, rdkutils.NewUnexpectedTypeError(multaxisGantryConfig, config.ConvertedAttributes)
			}
			return newMultiAxis(ctx, r, config, logger)
		},
	})
}

// NewMultiAxis creates a new-multi axis gantry.
func newMultiAxis(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (gantry.Gantry, error) {
	g := &multiAxis{
		name:   config.Name,
		logger: logger,
	}

	subAxes := config.Attributes.StringSlice("subaxes_list")
	if len(subAxes) == 0 {
		return nil, errors.New("no subaxes found for multiaxis gantry")
	}

	for _, s := range subAxes {
		oneAx, ok := r.ResourceByName(gantry.Named(s))
		if !ok {
			return nil, errors.Errorf("no axes named [%s]", s)
		}
		subAxis, ok := oneAx.(gantry.Gantry)
		if !ok {
			return nil, errors.Errorf("gantry named [%s] is not a gantry, is a %T", s, oneAx)
		}
		g.subAxes = append(g.subAxes, subAxis)
	}

	lengthsMm, err := g.GetLengths(ctx)
	if err != nil {
		return nil, err
	}
	g.lengthsMm = lengthsMm

	return g, nil
}

// MoveToPosition moves along an axis using inputs in millimeters.
func (g *multiAxis) MoveToPosition(ctx context.Context, positions []float64) error {
	if len(positions) == 0 {
		return errors.Errorf("need position inputs for %v-axis gantry, have %v positions", len(g.subAxes), len(positions))
	}

	for _, subAx := range g.subAxes {
		err := subAx.MoveToPosition(ctx, positions)
		if err != nil {
			return err
		}
	}
	return nil
}

// GoToInputs moves the gantry to a goal position in the Gantry frame.
func (g *multiAxis) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	if len(g.subAxes) == 0 {
		return errors.New("no subaxes found for inputs")
	}

	for _, subAx := range g.subAxes {
		err := subAx.MoveToPosition(ctx, referenceframe.InputsToFloats((goal)))
		if err != nil {
			return err
		}
	}
	return nil
}

// GetPosition returns the position in millimeters.
func (g *multiAxis) GetPosition(ctx context.Context) ([]float64, error) {
	posOut := []float64{}
	for idx, subAx := range g.subAxes {
		pos, err := subAx.GetPosition(ctx)
		if err != nil {
			return nil, err
		}
		posOut = append(posOut, pos[idx])
	}
	return posOut, nil
}

// GetLengths returns the physical lengths of all axes of a multi-axis Gantry.
func (g *multiAxis) GetLengths(ctx context.Context) ([]float64, error) {
	lengthsOut := []float64{}
	for _, subAx := range g.subAxes {
		length, err := subAx.GetLengths(ctx)
		if err != nil {
			return nil, err
		}
		lengthsOut = append(lengthsOut, length[0])
	}
	return lengthsOut, nil
}

// CurrentInputs returns the current inputs of the Gantry frame.
func (g *multiAxis) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	if len(g.subAxes) == 0 {
		return nil, errors.New("no subaxes found for inputs")
	}
	resOut := []float64{}
	for idx, subAx := range g.subAxes {
		res, err := subAx.GetPosition(ctx)
		if err != nil {
			return nil, err
		}
		resOut = append(resOut, res[idx]) // test if this returs the right thing
	}

	return referenceframe.FloatsToInputs(resOut), nil
}

//  ModelFrame returns the frame model of the Gantry.
// TO DO test model frame with #471.
func (g *multiAxis) ModelFrame() referenceframe.Model {
	m := referenceframe.NewSimpleModel()
	for idx := range g.subAxes {
		f, err := referenceframe.NewTranslationalFrame(
			g.name,
			[]bool{true}, // TODO convert to r3.Vector once #471 is merged.
			[]referenceframe.Limit{{0, g.lengthsMm[idx]}},
		)
		if err != nil {
			panic(fmt.Errorf("error creating frame %v, should be impossible %w", idx, err))
		}
		m.OrdTransforms = append(m.OrdTransforms, f)
	}

	return m
}
