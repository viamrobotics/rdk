package multiAxis

import (
	"context"
	"fmt"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/gantry"
	"go.viam.com/rdk/component/gantry/oneAxis"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
	rdkutils "go.viam.com/rdk/utils"
)

type MultiAxisConfig struct {
	LimitBoard string   `json:"limitBoard"` // used to read limit switch pins and control motor with gpio pins
	SubAxes    []string `json:"subaxes_list"`
}

type multiAxis struct {
	name       string
	oA         []gantry.Gantry
	lengths_mm []float64
	logger     golog.Logger
}

func (config *MultiAxisConfig) Validate(path string) error {
	if config.LimitBoard == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "limitBoard")
	}

	if len(config.SubAxes) == 0 {
		return utils.NewConfigValidationError(path, errors.New("need at least one axis"))
	}

	return nil
}

func init() {
	registry.RegisterComponent(gantry.Subtype, "multiAxis", registry.Component{
		Constructor: func(ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			multaxisGantryConfig, ok := config.ConvertedAttributes.(*MultiAxisConfig)
			if !ok {
				return nil, rdkutils.NewUnexpectedTypeError(multaxisGantryConfig, config.ConvertedAttributes)
			}
			return NewMultiAxis(ctx, r, config, logger)
		},
	})
}

// NewMultiAxis creates a new-multi axis gantry.
func NewMultiAxis(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (gantry.Gantry, error) {
	g := &multiAxis{
		name:   config.Name,
		logger: logger,
	}

	var err error
	g.oA[0], err = oneAxis.NewOneAxis(ctx, r, config, logger)
	if err != nil {
		return nil, errors.New("error instantiating ne single axis")
	}

	return g, nil
}

// TODO incorporate frames into movement function above.
func (g *multiAxis) ModelFrame() referenceframe.Model {
	m := referenceframe.NewSimpleModel()
	for idx := range g.oA {
		l := 1.0
		f, err := referenceframe.NewTranslationalFrame(
			g.name,
			[]bool{true},
			[]referenceframe.Limit{{0, l}},
		)
		if err != nil {
			panic(fmt.Errorf("error creating frame %v, should be impossible %w", idx, err))
		}
		m.OrdTransforms = append(m.OrdTransforms, f)
	}

	return m
}

func (g *multiAxis) MoveToPosition(ctx context.Context, positions []float64) error {
	for _, singleaxis := range g.oA {
		singleaxis.MoveToPosition(ctx, positions)
	}
	return nil
}

// Will be used in motor movement function above.
func (g *multiAxis) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	g.oA[0].MoveToPosition(ctx, referenceframe.InputsToFloats(goal))
	return nil
}

func (g *multiAxis) CurrentPosition(ctx context.Context) ([]float64, error) {
	posOut := []float64{}
	for idx, singleax := range g.oA {
		pos, err := singleax.CurrentPosition(ctx)
		if err != nil {
			return nil, err
		}
		posOut = append(posOut, pos[idx])
	}
	return posOut, nil
}

func (g *multiAxis) Lengths(ctx context.Context) ([]float64, error) {
	lengthsOut := []float64{}
	for _, singleax := range g.oA {
		length, err := singleax.Lengths(ctx)
		if err != nil {
			return nil, err
		}
		lengthsOut = append(lengthsOut, length[0])
	}
	return lengthsOut, nil
}

func (g *multiAxis) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	resOut := []float64{}
	for idx, singleax := range g.oA {
		res, err := singleax.CurrentPosition(ctx)
		if err != nil {
			return nil, err
		}
		resOut = append(resOut, res[idx]) // test if this returs the right thing
	}

	return referenceframe.FloatsToInputs(resOut), nil
}
