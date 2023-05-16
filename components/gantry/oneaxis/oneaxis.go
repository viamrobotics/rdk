// Package oneaxis implements a one-axis gantry.
package oneaxis

import (
	"context"
	"fmt"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	utils "go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/gantry"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	spatial "go.viam.com/rdk/spatialmath"
)

func init() {
	resource.RegisterComponent(gantry.API, model, resource.Registration[gantry.Gantry, *Config]{
		Constructor: newOneAxis,
	})
}

type oneAxis struct {
	resource.Named
	resource.AlwaysRebuild

	board board.Board
	motor motor.Motor

	limitSwitchPins []string
	limitHigh       bool
	positionLimits  []float64
	posRange        float64

	lengthMm        float64
	mmPerRevolution float64
	rpm             float64

	model referenceframe.Model
	axis  r3.Vector

	logger golog.Logger
	opMgr  operation.SingleOperationManager
}

// NewOneAxis creates a new one axis gantry.
func newOneAxis(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger golog.Logger) (gantry.Gantry, error) {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}

	motorDep, err := motor.FromDependencies(deps, newConf.Motor)
	if err != nil {
		return nil, err
	}
	features, err := motorDep.Properties(ctx, nil)
	if err != nil {
		return nil, err
	}
	ok := features[motor.PositionReporting]
	if !ok {
		return nil, motor.NewFeatureUnsupportedError(motor.PositionReporting, newConf.Motor)
	}

	oAx := &oneAxis{
		Named:           conf.ResourceName().AsNamed(),
		motor:           motorDep,
		logger:          logger,
		limitSwitchPins: newConf.LimitSwitchPins,
		lengthMm:        newConf.LengthMm,
		mmPerRevolution: newConf.MmPerRevolution,
		rpm:             newConf.GantryRPM,
		axis:            newConf.Axis,
	}

	if oAx.rpm == 0 {
		oAx.rpm = 100
	}

	switch len(oAx.limitSwitchPins) {
	case 1:
		if oAx.mmPerRevolution <= 0 {
			return nil, errors.New("gantry with one limit switch per axis needs a mm_per_length ratio defined")
		}

		board, err := board.FromDependencies(deps, newConf.Board)
		if err != nil {
			return nil, err
		}
		oAx.board = board

		PinEnable := *newConf.LimitPinEnabled
		oAx.limitHigh = PinEnable

	case 2:
		board, err := board.FromDependencies(deps, newConf.Board)
		if err != nil {
			return nil, err
		}
		oAx.board = board

		PinEnable := *newConf.LimitPinEnabled
		oAx.limitHigh = PinEnable

	case 0:
		// do nothing
	default:
		np := len(oAx.limitSwitchPins)
		return nil, errors.Errorf("invalid gantry type: need 1, 2 or 0 pins per axis, have %v pins", np)
	}

	if err := oAx.home(ctx); err != nil {
		return nil, err
	}

	oAx.posRange = oAx.positionLimits[1] - oAx.positionLimits[0]

	return oAx, nil
}

func (g *oneAxis) rotationalToLinear(positions float64) float64 {
	x := positions / g.lengthMm
	x = g.positionLimits[0] + (x * g.posRange)

	return x
}

func (g *oneAxis) testLimit(ctx context.Context, zero bool) (float64, error) {
	defer utils.UncheckedErrorFunc(func() error {
		return g.motor.Stop(ctx, nil)
	})

	d := -1.0
	if !zero {
		d *= -1
	}

	err := g.motor.GoFor(ctx, d*g.rpm, 0, nil)
	if err != nil {
		return 0, err
	}

	start := time.Now()
	for {
		hit, err := g.limitHit(ctx, zero)
		if err != nil {
			return 0, err
		}
		if hit {
			err = g.motor.Stop(ctx, nil)
			if err != nil {
				return 0, err
			}
			break
		}

		elapsed := start.Sub(start)
		if elapsed > (time.Second * 15) {
			return 0, errors.New("gantry timed out testing limit")
		}

		if !utils.SelectContextOrWait(ctx, time.Millisecond*10) {
			return 0, ctx.Err()
		}
	}

	return g.motor.Position(ctx, nil)
}

func (g *oneAxis) limitHit(ctx context.Context, zero bool) (bool, error) {
	offset := 0
	if !zero {
		offset = 1
	}
	pin, err := g.board.GPIOPinByName(g.limitSwitchPins[offset])
	if err != nil {
		return false, err
	}
	high, err := pin.Get(ctx, nil)

	return high == g.limitHigh, err
}

// Position returns the position in millimeters.
func (g *oneAxis) Position(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
	pos, err := g.motor.Position(ctx, extra)
	if err != nil {
		return []float64{}, err
	}

	x := g.lengthMm * ((pos - g.positionLimits[0]) / g.posRange)

	return []float64{x}, nil
}

// Lengths returns the physical lengths of an axis of a Gantry.
func (g *oneAxis) Lengths(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
	return []float64{g.lengthMm}, nil
}

// MoveToPosition moves along an axis using inputs in millimeters.
func (g *oneAxis) MoveToPosition(ctx context.Context, positions []float64, extra map[string]interface{}) error {
	ctx, done := g.opMgr.New(ctx)
	defer done()

	if len(positions) != 1 {
		return fmt.Errorf("MoveToPosition needs 1 position to move, got: %v", len(positions))
	}

	if positions[0] < 0 || positions[0] > g.lengthMm {
		return fmt.Errorf("out of range (%.2f) min: 0 max: %.2f", positions[0], g.lengthMm)
	}

	x := g.rotationalToLinear(positions[0])
	// Limit switch errors that stop the motors.
	// Currently needs to be moved by underlying gantry motor.
	if len(g.limitSwitchPins) > 0 {
		hit, err := g.limitHit(ctx, true)
		if err != nil {
			return err
		}

		// Hits backwards limit switch, goes in forwards direction for two revolutions
		if hit {
			if x < g.positionLimits[0] {
				dir := float64(1)
				return g.motor.GoFor(ctx, dir*g.rpm, 2, extra)
			}
			return g.motor.Stop(ctx, extra)
		}

		// Hits forward limit switch, goes in backwards direction for two revolutions
		hit, err = g.limitHit(ctx, false)
		if err != nil {
			return err
		}
		if hit {
			if x > g.positionLimits[1] {
				dir := float64(-1)
				return g.motor.GoFor(ctx, dir*g.rpm, 2, extra)
			}
			return g.motor.Stop(ctx, extra)
		}

		if err = g.motor.GoTo(ctx, g.rpm, x, extra); err != nil {
			return err
		}
	}

	g.logger.Debugf("going to %.2f at speed %.2f", x, g.rpm)
	if err := g.motor.GoTo(ctx, g.rpm, x, extra); err != nil {
		return err
	}
	return nil
}

// Stop stops the motor of the gantry.
func (g *oneAxis) Stop(ctx context.Context, extra map[string]interface{}) error {
	ctx, done := g.opMgr.New(ctx)
	defer done()
	return g.motor.Stop(ctx, extra)
}

// Close calls stop.
func (g *oneAxis) Close(ctx context.Context) error {
	return g.Stop(ctx, nil)
}

// IsMoving returns whether the gantry is moving.
func (g *oneAxis) IsMoving(ctx context.Context) (bool, error) {
	return g.opMgr.OpRunning(), nil
}

// ModelFrame returns the frame model of the Gantry.
func (g *oneAxis) ModelFrame() referenceframe.Model {
	if g.model == nil {
		var errs error
		m := referenceframe.NewSimpleModel("")

		f, err := referenceframe.NewStaticFrame(g.Name().ShortName(), spatial.NewZeroPose())
		errs = multierr.Combine(errs, err)
		m.OrdTransforms = append(m.OrdTransforms, f)

		f, err = referenceframe.NewTranslationalFrame(g.Name().ShortName(), g.axis, referenceframe.Limit{Min: 0, Max: g.lengthMm})
		errs = multierr.Combine(errs, err)

		if errs != nil {
			g.logger.Error(errs)
			return nil
		}

		m.OrdTransforms = append(m.OrdTransforms, f)
		g.model = m
	}
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
	return g.MoveToPosition(ctx, referenceframe.InputsToFloats(goal), nil)
}
