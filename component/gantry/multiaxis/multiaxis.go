// Package multiaxis implements a multi-axis gantry.
package multiaxis

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/gantry"
	"go.viam.com/rdk/config"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
	rdkutils "go.viam.com/rdk/utils"
)

const modelname = "multiaxis"

// AttrConfig is used for converting multiAxis config attributes.
type AttrConfig struct {
	SubAxes []string `json:"subaxes_list"`
}

type multiAxis struct {
	name      string
	subAxes   []gantry.Gantry
	lengthsMm []float64
	logger    golog.Logger
	model     referenceframe.Model
}

// Validate ensures all parts of the config are valid.
func (config *AttrConfig) Validate(path string) error {
	if len(config.SubAxes) == 0 {
		return utils.NewConfigValidationError(path, errors.New("need at least one axis"))
	}

	return nil
}

func init() {
	registry.RegisterComponent(gantry.Subtype, modelname, registry.Component{
		Constructor: func(ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return newMultiAxis(ctx, r, config, logger)
		},
	})

	config.RegisterComponentAttributeMapConverter(config.ComponentTypeGantry, modelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&AttrConfig{})
}

// NewMultiAxis creates a new-multi axis gantry.
func newMultiAxis(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (gantry.Gantry, error) {
	conf, ok := config.ConvertedAttributes.(*AttrConfig)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(conf, config.ConvertedAttributes)
	}

	mAx := &multiAxis{
		name:   config.Name,
		logger: logger,
	}

	for _, s := range conf.SubAxes {
		oneAx, err := r.ResourceByName(gantry.Named(s))
		if err != nil {
			return nil, errors.Wrapf(err, "no axes named [%s]", s)
		}
		subAx, ok := oneAx.(gantry.Gantry)
		if !ok {
			return nil, errors.Errorf("gantry named [%s] is not a gantry, is a %T", s, oneAx)
		}
		mAx.subAxes = append(mAx.subAxes, subAx)
	}

	var err error
	mAx.lengthsMm, err = mAx.GetLengths(ctx)
	if err != nil {
		return nil, err
	}

	return mAx, nil
}

// MoveToPosition moves along an axis using inputs in millimeters.
func (g *multiAxis) MoveToPosition(ctx context.Context, positions []float64, worldState *commonpb.WorldState) error {
	if len(positions) == 0 {
		return errors.Errorf("need position inputs for %v-axis gantry, have %v positions", len(g.subAxes), len(positions))
	}

	idx := 0
	for _, subAx := range g.subAxes {
		subAxNum, err := subAx.GetLengths(ctx)
		if err != nil {
			return err
		}

		err = subAx.MoveToPosition(ctx, positions[idx:idx+len(subAxNum)-1], worldState)
		if err != nil {
			return err
		}
		idx += len(subAxNum) - 1
	}
	return nil
}

// GoToInputs moves the gantry to a goal position in the Gantry frame.
func (g *multiAxis) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	if len(g.subAxes) == 0 {
		return errors.New("no subaxes found for inputs")
	}

	idx := 0
	for _, subAx := range g.subAxes {
		subAxNum, err := subAx.GetLengths(ctx)
		if err != nil {
			return err
		}

		err = subAx.MoveToPosition(ctx, referenceframe.InputsToFloats(goal[idx:idx+len(subAxNum)-1]), &commonpb.WorldState{})
		if err != nil {
			return err
		}
		idx += len(subAxNum) - 1
	}
	return nil
}

// GetPosition returns the position in millimeters.
func (g *multiAxis) GetPosition(ctx context.Context) ([]float64, error) {
	positions := []float64{}
	for _, subAx := range g.subAxes {
		pos, err := subAx.GetPosition(ctx)
		if err != nil {
			return nil, err
		}
		positions = append(positions, pos...)
	}
	return positions, nil
}

// GetLengths returns the physical lengths of all axes of a multi-axis Gantry.
func (g *multiAxis) GetLengths(ctx context.Context) ([]float64, error) {
	lengths := []float64{}
	for _, subAx := range g.subAxes {
		lng, err := subAx.GetLengths(ctx)
		if err != nil {
			return nil, err
		}
		lengths = append(lengths, lng...)
	}
	return lengths, nil
}

// CurrentInputs returns the current inputs of the Gantry frame.
func (g *multiAxis) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	if len(g.subAxes) == 0 {
		return nil, errors.New("no subaxes found for inputs")
	}
	inputs := []float64{}
	for _, subAx := range g.subAxes {
		in, err := subAx.GetPosition(ctx)
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, in...)
	}
	return referenceframe.FloatsToInputs(inputs), nil
}

//  ModelFrame returns the frame model of the Gantry.
func (g *multiAxis) ModelFrame() referenceframe.Model {
	if g.model == nil {
		model := referenceframe.NewSimpleModel()
		for _, subAx := range g.subAxes {
			model.OrdTransforms = append(model.OrdTransforms, subAx.ModelFrame())
		}
		g.model = model
	}
	return g.model
}
