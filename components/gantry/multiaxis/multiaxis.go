// Package multiaxis implements a multi-axis gantry.
package multiaxis

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/gantry"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	rdkutils "go.viam.com/rdk/utils"
)

var modelname = resource.NewDefaultModel("multiaxis")

// AttrConfig is used for converting multiAxis config attributes.
type AttrConfig struct {
	SubAxes []string `json:"subaxes_list"`
}

type multiAxis struct {
	generic.Unimplemented
	name      string
	subAxes   []gantry.Gantry
	lengthsMm []float64
	logger    golog.Logger
	model     referenceframe.Model
	opMgr     operation.SingleOperationManager
}

// Validate ensures all parts of the config are valid.
func (config *AttrConfig) Validate(path string) ([]string, error) {
	var deps []string

	if len(config.SubAxes) == 0 {
		return nil, utils.NewConfigValidationError(path, errors.New("need at least one axis"))
	}

	deps = append(deps, config.SubAxes...)
	return deps, nil
}

func init() {
	registry.RegisterComponent(gantry.Subtype, modelname, registry.Component{
		Constructor: func(ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return newMultiAxis(ctx, deps, config, logger)
		},
	})

	config.RegisterComponentAttributeMapConverter(gantry.Subtype, modelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&AttrConfig{})
}

// NewMultiAxis creates a new-multi axis gantry.
func newMultiAxis(
	ctx context.Context,
	deps registry.Dependencies,
	config config.Component,
	logger golog.Logger,
) (gantry.LocalGantry, error) {
	conf, ok := config.ConvertedAttributes.(*AttrConfig)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(conf, config.ConvertedAttributes)
	}

	mAx := &multiAxis{
		name:   config.Name,
		logger: logger,
	}

	for _, s := range conf.SubAxes {
		subAx, err := gantry.FromDependencies(deps, s)
		if err != nil {
			return nil, errors.Wrapf(err, "no axes named [%s]", s)
		}
		mAx.subAxes = append(mAx.subAxes, subAx)
	}

	var err error
	mAx.lengthsMm, err = mAx.Lengths(ctx, nil)
	if err != nil {
		return nil, err
	}

	return mAx, nil
}

// MoveToPosition moves along an axis using inputs in millimeters.
func (g *multiAxis) MoveToPosition(
	ctx context.Context,
	positions []float64,
	worldState *referenceframe.WorldState,
	extra map[string]interface{},
) error {
	ctx, done := g.opMgr.New(ctx)
	defer done()

	if len(positions) == 0 {
		return errors.Errorf("need position inputs for %v-axis gantry, have %v positions", len(g.subAxes), len(positions))
	}

	idx := 0
	for _, subAx := range g.subAxes {
		subAxNum, err := subAx.Lengths(ctx, extra)
		if err != nil {
			return err
		}

		err = subAx.MoveToPosition(ctx, positions[idx:idx+len(subAxNum)-1], worldState, extra)
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
	ctx, done := g.opMgr.New(ctx)
	defer done()

	idx := 0
	for _, subAx := range g.subAxes {
		subAxNum, err := subAx.Lengths(ctx, nil)
		if err != nil {
			return err
		}

		err = subAx.MoveToPosition(ctx, referenceframe.InputsToFloats(goal[idx:idx+len(subAxNum)-1]), &referenceframe.WorldState{}, nil)
		if err != nil {
			return err
		}
		idx += len(subAxNum) - 1
	}
	return nil
}

// Position returns the position in millimeters.
func (g *multiAxis) Position(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
	positions := []float64{}
	for _, subAx := range g.subAxes {
		pos, err := subAx.Position(ctx, extra)
		if err != nil {
			return nil, err
		}
		positions = append(positions, pos...)
	}
	return positions, nil
}

// Lengths returns the physical lengths of all axes of a multi-axis Gantry.
func (g *multiAxis) Lengths(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
	lengths := []float64{}
	for _, subAx := range g.subAxes {
		lng, err := subAx.Lengths(ctx, extra)
		if err != nil {
			return nil, err
		}
		lengths = append(lengths, lng...)
	}
	return lengths, nil
}

// Stop stops the subaxes of the gantry simultaneously.
func (g *multiAxis) Stop(ctx context.Context, extra map[string]interface{}) error {
	ctx, done := g.opMgr.New(ctx)
	defer done()
	wg := sync.WaitGroup{}
	for _, subAx := range g.subAxes {
		currG := subAx
		wg.Add(1)
		utils.ManagedGo(func() {
			if err := currG.Stop(ctx, extra); err != nil {
				g.logger.Errorw("failed to stop subaxis", "error", err)
			}
		}, wg.Done)
	}
	return nil
}

// IsMoving returns whether the gantry is moving.
func (g *multiAxis) IsMoving(ctx context.Context) (bool, error) {
	return g.opMgr.OpRunning(), nil
}

// CurrentInputs returns the current inputs of the Gantry frame.
func (g *multiAxis) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	if len(g.subAxes) == 0 {
		return nil, errors.New("no subaxes found for inputs")
	}
	inputs := []float64{}
	for _, subAx := range g.subAxes {
		in, err := subAx.Position(ctx, nil)
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, in...)
	}
	return referenceframe.FloatsToInputs(inputs), nil
}

// ModelFrame returns the frame model of the Gantry.
func (g *multiAxis) ModelFrame() referenceframe.Model {
	if g.model == nil {
		model := referenceframe.NewSimpleModel("")
		for _, subAx := range g.subAxes {
			model.OrdTransforms = append(model.OrdTransforms, subAx.ModelFrame())
		}
		g.model = model
	}
	return g.model
}
