// Package multiaxis implements a multi-axis gantry.
package multiaxis

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/gantry"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	rdkutils "go.viam.com/rdk/utils"
)

var model = resource.DefaultModelFamily.WithModel("multi-axis")

// Config is used for converting multiAxis config attributes.
type Config struct {
	SubAxes            []string `json:"subaxes_list"`
	MoveSimultaneously *bool    `json:"move_simultaneously,omitempty"`
}

type multiAxis struct {
	resource.Named
	resource.AlwaysRebuild
	subAxes            []gantry.Gantry
	lengthsMm          []float64
	logger             logging.Logger
	moveSimultaneously bool
	model              referenceframe.Model
	opMgr              *operation.SingleOperationManager
	workers            sync.WaitGroup
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	var deps []string

	if len(conf.SubAxes) == 0 {
		return nil, resource.NewConfigValidationError(path, errors.New("need at least one axis"))
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
	logger logging.Logger,
) (gantry.Gantry, error) {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}

	mAx := &multiAxis{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
		opMgr:  operation.NewSingleOperationManager(),
	}

	for _, s := range newConf.SubAxes {
		subAx, err := gantry.FromDependencies(deps, s)
		if err != nil {
			return nil, errors.Wrapf(err, "no axes named [%s]", s)
		}
		mAx.subAxes = append(mAx.subAxes, subAx)
	}

	mAx.moveSimultaneously = false
	if newConf.MoveSimultaneously != nil {
		mAx.moveSimultaneously = *newConf.MoveSimultaneously
	}

	mAx.lengthsMm, err = mAx.Lengths(ctx, nil)
	if err != nil {
		return nil, err
	}

	return mAx, nil
}

// Home runs the homing sequence of the gantry and returns true once completed.
func (g *multiAxis) Home(ctx context.Context, extra map[string]interface{}) (bool, error) {
	for _, subAx := range g.subAxes {
		homed, err := subAx.Home(ctx, nil)
		if err != nil {
			return false, err
		}
		if !homed {
			return false, nil
		}
	}
	return true, nil
}

// MoveToPosition moves along an axis using inputs in millimeters.
func (g *multiAxis) MoveToPosition(ctx context.Context, positions, speeds []float64, extra map[string]interface{}) error {
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

	fs := []rdkutils.SimpleFunc{}
	idx := 0
	for _, subAx := range g.subAxes {
		subAxNum, err := subAx.Lengths(ctx, extra)
		if err != nil {
			return err
		}

		pos := positions[idx : idx+len(subAxNum)]
		var speed []float64
		// if speeds is an empty list, speed will be set to the default in the subAx MoveToPosition call
		if len(speeds) == 0 {
			speed = []float64{}
		} else {
			speed = speeds[idx : idx+len(subAxNum)]
		}
		idx += len(subAxNum)

		if g.moveSimultaneously {
			singleGantry := subAx
			fs = append(fs, func(ctx context.Context) error { return singleGantry.MoveToPosition(ctx, pos, speed, nil) })
		} else {
			err = subAx.MoveToPosition(ctx, pos, speed, extra)
			if err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
		}
	}
	if g.moveSimultaneously {
		if _, err := rdkutils.RunInParallel(ctx, fs); err != nil {
			return multierr.Combine(err, g.Stop(ctx, nil))
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

	// MoveToPosition will use the default gantry speed when an empty float is passed in
	speeds := []float64{}
	return g.MoveToPosition(ctx, referenceframe.InputsToFloats(goal), speeds, nil)
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
