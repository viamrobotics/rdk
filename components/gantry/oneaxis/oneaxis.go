// Package oneaxis implements a one-axis gantry.
package oneaxis

import (
	"context"
	"fmt"
	"sync"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.uber.org/multierr"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

type oneAxis struct {
	ctx context.Context
	generic.Unimplemented
	name string

	board board.Board
	motor motor.Motor

	positionLimits []float64
	gantryRange    float64

	lengthMm        float64
	mmPerRevolution float64
	rpm             float64

	model referenceframe.Model

	logger golog.Logger
	opMgr  operation.SingleOperationManager
	mu     sync.Mutex
}

// newOneAxis creates a new one axis gantry.
func newOneAxis(
	ctx context.Context,
	deps registry.Dependencies,
	cfg config.Component,
	logger golog.Logger,
) (*oneAxis, error) {
	conf, ok := cfg.ConvertedAttributes.(*AttrConfig)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(conf, cfg.ConvertedAttributes)
	}

	_motor, err := motor.FromDependencies(deps, conf.Motor)
	if err != nil {
		return nil, err
	}
	features, err := _motor.Properties(ctx, nil)
	if err != nil {
		return nil, err
	}
	ok = features[motor.PositionReporting]
	if !ok {
		return nil, motor.NewFeatureUnsupportedError(motor.PositionReporting, conf.Motor)
	}

	board, err := board.FromDependencies(deps, conf.Board)
	if err != nil {
		return nil, err
	}

	oAx := &oneAxis{
		ctx:             ctx,
		name:            cfg.Name,
		motor:           _motor,
		logger:          logger,
		lengthMm:        conf.LengthMm,
		mmPerRevolution: conf.MmPerRevolution,
		rpm:             conf.GantryRPM,
		board:           board,
	}

	err = oAx.createModel(conf.Axis)
	if err != nil {
		return oAx, err
	}

	return oAx, nil
}

func (g *oneAxis) createModel(axis r3.Vector) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	var errs error
	m := referenceframe.NewSimpleModel("")

	f, err := referenceframe.NewStaticFrame(g.name, spatialmath.NewZeroPose())
	errs = multierr.Combine(errs, err)
	m.OrdTransforms = append(m.OrdTransforms, f)

	f, err = referenceframe.NewTranslationalFrame(g.name, axis, referenceframe.Limit{Min: 0, Max: g.lengthMm})
	errs = multierr.Combine(errs, err)

	if errs != nil {
		return errs
	}

	m.OrdTransforms = append(m.OrdTransforms, f)
	g.model = m

	return errs
}

func linearToRotational(position, positionA, lengthMm, gantryRange float64) float64 {
	x := position / lengthMm
	x = positionA + (x * gantryRange)
	return x
}

// Position returns the position in millimeters.
func (g *oneAxis) Position(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	pos, err := g.motor.Position(ctx, extra)
	if err != nil {
		return []float64{}, err
	}

	// this will never divide by zero since we cannot
	// create a gantry with a zero range
	x := g.lengthMm * ((pos - g.positionLimits[0]) / g.gantryRange)
	return []float64{x}, nil
}

// Lengths returns the physical lengths of an axis of a Gantry.
func (g *oneAxis) Lengths(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
	return []float64{g.lengthMm}, nil
}

// MoveToPosition moves along an axis using inputs in millimeters.
func (g *oneAxis) MoveToPosition(
	ctx context.Context,
	positions []float64,
	worldState *referenceframe.WorldState,
	extra map[string]interface{},
) error {
	ctx, done := g.opMgr.New(ctx)
	defer done()
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(positions) != 1 {
		return fmt.Errorf("oneAxis gantry MoveToPosition needs 1 position, got: %v", len(positions))
	}

	if positions[0] < 0 || positions[0] > g.lengthMm {
		return fmt.Errorf("oneAxis gantry position out of range, got %.02f max is %.02f", positions[0], g.lengthMm)
	}

	x := linearToRotational(0, positions[0], g.lengthMm, g.gantryRange)

	err := g.motor.GoTo(ctx, g.rpm, x, extra)
	if err != nil {
		return err
	}
	return nil
}

// Stop stops the motor of the gantry.
func (g *oneAxis) Stop(ctx context.Context, extra map[string]interface{}) error {
	g.opMgr.CancelRunning(ctx)
	return g.motor.Stop(ctx, extra)
}

// IsMoving returns whether the gantry is moving.
func (g *oneAxis) IsMoving(ctx context.Context) (bool, error) {
	return g.opMgr.OpRunning(), nil
}

// ModelFrame returns the frame model of the Gantry.
func (g *oneAxis) ModelFrame() referenceframe.Model {
	return g.model
}

// CurrentInputs returns the current inputs of the Gantry frame.
func (g *oneAxis) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	res, err := g.Position(ctx, nil)
	if err != nil {
		return nil, err
	}
	return referenceframe.FloatsToInputs(res), nil
}

// GoToInputs moves the gantry to a goal position in the Gantry frame.
func (g *oneAxis) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	return g.MoveToPosition(ctx, referenceframe.InputsToFloats(goal), &referenceframe.WorldState{}, nil)
}
