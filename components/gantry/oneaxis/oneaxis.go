// Package oneaxis implements a oneaxis gantry.
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

var model = resource.DefaultModelFamily.WithModel("oneaxis")

// Config is used for converting oneAxis config attributes.
type Config struct {
	Board           string    `json:"board,omitempty"` // used to read limit switch pins and control motor with gpio pins
	Motor           string    `json:"motor"`
	LimitSwitchPins []string  `json:"limit_pins,omitempty"`
	LimitPinEnabled *bool     `json:"limit_pin_enabled_high,omitempty"`
	LengthMm        float64   `json:"length_mm"`
	MmPerRevolution float64   `json:"mm_per_rev,omitempty"`
	GantryRPM       float64   `json:"gantry_rpm,omitempty"`
	Axis            r3.Vector `json:"axis"`
}

// Validate ensures all parts of the config are valid.
func (cfg *Config) Validate(path string) ([]string, error) {
	var deps []string

	if len(cfg.Motor) == 0 {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "motor")
	}
	deps = append(deps, cfg.Motor)

	if cfg.LengthMm <= 0 {
		err := utils.NewConfigValidationFieldRequiredError(path, "length_mm")
		return nil, errors.Wrap(err, "length must be non-zero and positive")
	}

	if len(cfg.Board) == 0 && len(cfg.LimitSwitchPins) > 0 {
		return nil, errors.New("gantries with limit_pins require a board to sense limit hits")
	}
	deps = append(deps, cfg.Board)

	if len(cfg.LimitSwitchPins) == 1 && cfg.MmPerRevolution == 0 {
		return nil, errors.New("the oneaxis gantry has one limit switch axis, needs pulley radius to set position limits")
	}

	if len(cfg.LimitSwitchPins) > 0 && cfg.LimitPinEnabled == nil {
		return nil, errors.New("limit pin enabled must be set to true or false")
	}

	if cfg.Axis.X == 1 && cfg.Axis.Y == 1 ||
		cfg.Axis.X == 1 && cfg.Axis.Z == 1 || cfg.Axis.Y == 1 && cfg.Axis.Z == 1 {
		return nil, errors.New("only one translational axis of movement allowed for single axis gantry")
	}

	return deps, nil
}

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
	positionRange   float64

	lengthMm        float64
	mmPerRevolution float64
	rpm             float64

	model referenceframe.Model
	axis  r3.Vector

	logger golog.Logger
	opMgr  operation.SingleOperationManager
}

// newOneAxis creates a new one axis gantry.
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

	np := len(oAx.limitSwitchPins)
	switch np {
	case 1, 2:

		board, err := board.FromDependencies(deps, newConf.Board)
		if err != nil {
			return nil, err
		}
		oAx.board = board

		PinEnable := *newConf.LimitPinEnabled
		oAx.limitHigh = PinEnable

		if oAx.mmPerRevolution <= 0 && np == 1 {
			return nil, errors.New("gantry with one limit switch per axis needs a mm_per_length ratio defined")
		}
	case 0:
		// do nothing, validation takes care of all checks already
	default:
		np := len(oAx.limitSwitchPins)
		return nil, errors.Errorf("invalid gantry type: need 1, 2 or 0 pins per axis, have %v pins", np)
	}

	if err := oAx.home(ctx, np); err != nil {
		return nil, err
	}

	return oAx, nil
}

func (g *oneAxis) home(ctx context.Context, np int) error {
	ctx, done := g.opMgr.New(ctx)
	defer done()
	// Mapping one limit switch motor0->limsw0, motor1 ->limsw1, motor 2 -> limsw2
	// Mapping two limit switch motor0->limSw0,limSw1; motor1->limSw2,limSw3; motor2->limSw4,limSw5
	switch np {
	// An axis with one limit switch will go till it hits the limit switch, encode that position as the
	// zero position of the oneaxis, and adds a second position limit based on the steps per length.
	case 1:
		if err := g.homeOneLimSwitch(ctx); err != nil {
			return err
		}
	// An axis with two limit switches will go till it hits the first limit switch, encode that position as the
	// zero position of the oneaxis, then go till it hits the second limit switch, then encode that position as the
	// at-length position of the oneaxis.
	case 2:
		if err := g.homeTwoLimSwitch(ctx); err != nil {
			return err
		}
	// An axis with an encoder will encode the
	case 0:
		if err := g.homeEncoder(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (g *oneAxis) homeTwoLimSwitch(ctx context.Context) error {
	positionA, err := g.testLimit(ctx, true)
	if err != nil {
		return err
	}

	positionB, err := g.testLimit(ctx, false)
	if err != nil {
		return err
	}

	g.logger.Debugf("positionA: %0.2f positionB: %0.2f", positionA, positionB)

	g.positionLimits = []float64{positionA, positionB}
	g.positionRange = positionB - positionA

	// Go backwards so limit stops are not hit.
	x := g.gantryToMotorPosition(0.8 * g.lengthMm)
	if err = g.motor.GoTo(ctx, g.rpm, x, nil); err != nil {
		return err
	}
	return nil
}

func (g *oneAxis) homeOneLimSwitch(ctx context.Context) error {
	// One pin always and only should go backwards.
	positionA, err := g.testLimit(ctx, true)
	if err != nil {
		return err
	}

	revPerLength := g.lengthMm / g.mmPerRevolution
	positionB := positionA + revPerLength

	g.positionLimits = []float64{positionA, positionB}

	// Go backwards so limit stops are not hit.
	x := g.gantryToMotorPosition(0.2 * g.lengthMm)
	if err = g.motor.GoTo(ctx, g.rpm, x, nil); err != nil {
		return err
	}

	return nil
}

// home encoder assumes that you have places one of the stepper motors where you
// want your zero position to be, you need to know which way is "forward"
// on your motor.
func (g *oneAxis) homeEncoder(ctx context.Context) error {
	revPerLength := g.lengthMm / g.mmPerRevolution

	positionA, err := g.motor.Position(ctx, nil)
	if err != nil {
		return err
	}

	positionB := positionA + revPerLength

	g.positionLimits = []float64{positionA, positionB}
	return nil
}

func (g *oneAxis) gantryToMotorPosition(positions float64) float64 {
	x := positions / g.lengthMm
	x = g.positionLimits[0] + (x * g.positionRange)

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

// this function may need to be run in the background upon initialisation of the ganty,
// also may need to use a digital intterupt pin instead of a gpio pin.
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

	x := g.lengthMm * ((pos - g.positionLimits[0]) / g.positionRange)

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

	x := g.gantryToMotorPosition(positions[0])
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
