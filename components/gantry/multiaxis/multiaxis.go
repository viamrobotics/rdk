// Package multiaxis implements a multi-axis gantry.
package multiaxis

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/gantry"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
)

var model = resource.DefaultModelFamily.WithModel("multiaxis")

// Config is used for converting multiAxis config attributes.
type Config struct {
	SubAxes []string `json:"subaxes_list"`
}

type multiAxis struct {
	resource.Named
	resource.AlwaysRebuild
	subAxes   []gantry.Gantry
	lengthsMm []float64
	logger    golog.Logger
	model     referenceframe.Model
	opMgr     operation.SingleOperationManager
	workers   sync.WaitGroup
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	var deps []string

	if len(conf.SubAxes) == 0 {
		return nil, utils.NewConfigValidationError(path, errors.New("need at least one axis"))
	}

	deps = append(deps, conf.SubAxes...)
	return deps, nil
}

func init() {
	resource.RegisterComponent(gantry.API, model, resource.Registration[gantry.Gantry, *Config]{
		Constructor: newMultiAxis,
	})
}

// NewMultiAxis creates a new-multi axis gantry.
func newMultiAxis(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger golog.Logger,
) (gantry.Gantry, error) {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}

	mAx := &multiAxis{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
	}

	for _, s := range newConf.SubAxes {
		subAx, err := gantry.FromDependencies(deps, s)
		if err != nil {
			return nil, errors.Wrapf(err, "no axes named [%s]", s)
		}
		mAx.subAxes = append(mAx.subAxes, subAx)
	}

	mAx.lengthsMm, err = mAx.Lengths(ctx, nil)
	if err != nil {
		return nil, err
	}

	return mAx, nil
}

// MoveToPosition moves along an axis using inputs in millimeters.
func (g *multiAxis) MoveToPosition(ctx context.Context, positions []float64, extra map[string]interface{}) error {
	ctx, done := g.opMgr.New(ctx)
	defer done()

	if len(positions) == 0 {
		return errors.Errorf("need position inputs for %v-axis gantry, have %v positions", len(g.subAxes), len(positions))
	}

	if len(positions) != len(g.lengthsMm) {
		return errors.Errorf(
			"number of input positions %v does not match total gantry axes count %v",
			len(positions), len(g.lengthsMm),
		)
	}

	idx := 0
	for _, subAx := range g.subAxes {
		subAxNum, err := subAx.Lengths(ctx, extra)
		if err != nil {
			return err
		}

		pos := positions[idx : idx+len(subAxNum)]
		idx += len(subAxNum)

		err = subAx.MoveToPosition(ctx, pos, extra)
		if err != nil && !errors.Is(err, context.Canceled) {
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
	ctx, done := g.opMgr.New(ctx)
	defer done()

	return g.MoveToPosition(ctx, referenceframe.InputsToFloats(goal), nil)
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
	for _, subAx := range g.subAxes {
		currG := subAx
		g.workers.Add(1)
		utils.ManagedGo(func() {
			if err := currG.Stop(ctx, extra); err != nil {
				g.logger.Errorw("failed to stop subaxis", "error", err)
			}
		}, g.workers.Done)
	}
	return nil
}

// Close calls stop.
func (g *multiAxis) Close(ctx context.Context) error {
	return g.Stop(ctx, nil)
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
	positions, err := g.Position(ctx, nil)
	if err != nil {
		return nil, err
	}

	return referenceframe.FloatsToInputs(positions), nil
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
