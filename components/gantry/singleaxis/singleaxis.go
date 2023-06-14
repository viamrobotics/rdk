// Package singleaxis implements a single-axis gantry.
package singleaxis

import (
	"context"
	"fmt"
	"sync"
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

var (
	model = resource.DefaultModelFamily.WithModel("single-axis")
	wg    sync.WaitGroup
)

// limitErrorMargin is added or subtracted from the location of the limit switch to ensure the switch is not passed.
const limitErrorMargin = 0.25

// Config is used for converting singleAxis config attributes.
type Config struct {
	Board           string   `json:"board,omitempty"` // used to read limit switch pins and control motor with gpio pins
	Motor           string   `json:"motor"`
	LimitSwitchPins []string `json:"limit_pins,omitempty"`
	LimitPinEnabled *bool    `json:"limit_pin_enabled_high,omitempty"`
	LengthMm        float64  `json:"length_mm"`
	MmPerRevolution float64  `json:"mm_per_rev,omitempty"`
	GantryRPM       float64  `json:"gantry_rpm,omitempty"`
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
		return nil, errors.New("the single-axis gantry has one limit switch axis, needs pulley radius to set position limits")
	}

	if len(cfg.LimitSwitchPins) > 0 && cfg.LimitPinEnabled == nil {
		return nil, errors.New("limit pin enabled must be set to true or false")
	}

	return deps, nil
}

func init() {
	resource.RegisterComponent(gantry.API, model, resource.Registration[gantry.Gantry, *Config]{
		Constructor: newSingleAxis,
	})
}

type singleAxis struct {
	resource.Named

	board board.Board
	motor motor.Motor
	mu    sync.Mutex

	limitSwitchPins []string
	limitHigh       bool
	positionLimits  []float64
	positionRange   float64

	lengthMm        float64
	mmPerRevolution float64
	rpm             float64

	model referenceframe.Model
	frame r3.Vector

	logger golog.Logger
	opMgr  operation.SingleOperationManager
}

// newSingleAxis creates a new single axis gantry.
func newSingleAxis(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger golog.Logger) (gantry.Gantry, error) {
	sAx := &singleAxis{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
	}

	if err := sAx.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return sAx, nil
}

func (g *singleAxis) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	g.logger.Error(time.Now())
	g.mu.Lock()
	defer g.mu.Unlock()
	needsToReHome := false
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	// Changing these attributes does not rerun homing
	g.lengthMm = newConf.LengthMm
	g.mmPerRevolution = newConf.MmPerRevolution
	if g.mmPerRevolution <= 0 && len(newConf.LimitSwitchPins) == 1 {
		return errors.New("gantry with one limit switch per axis needs a mm_per_length ratio defined")
	}
	g.frame = conf.Frame.Translation
	g.rpm = newConf.GantryRPM
	if g.rpm == 0 {
		g.rpm = 100
	}

	// Rerun homing if the board has changed
	if g.board == nil || g.board.Name().ShortName() != newConf.Board {
		board, err := board.FromDependencies(deps, newConf.Board)
		if err != nil {
			return err
		}
		g.board = board
		needsToReHome = true
	}

	// Rerun homing if the motor changes
	if g.motor == nil || g.motor.Name().ShortName() != newConf.Motor {
		needsToReHome = true
		motorDep, err := motor.FromDependencies(deps, newConf.Motor)
		if err != nil {
			return err
		}
		features, err := motorDep.Properties(ctx, nil)
		if err != nil {
			return err
		}
		ok := features[motor.PositionReporting]
		if !ok {
			return motor.NewFeatureUnsupportedError(motor.PositionReporting, newConf.Motor)
		}
		g.motor = motorDep
	}

	// Rerun homing if anything with the limit switch pins changes
	if (len(g.limitSwitchPins) != len(newConf.LimitSwitchPins)) || (g.limitHigh != *newConf.LimitPinEnabled) {
		g.limitHigh = *newConf.LimitPinEnabled
		needsToReHome = true
		g.limitSwitchPins = newConf.LimitSwitchPins
	} else {
		for i, pin := range newConf.LimitSwitchPins {
			if pin != g.limitSwitchPins[i] {
				g.limitSwitchPins[i] = pin
				needsToReHome = true
			}
		}
	}
	if len(newConf.LimitSwitchPins) > 2 {
		return errors.Errorf("invalid gantry type: need 1, 2 or 0 pins per axis, have %v pins", len(newConf.LimitSwitchPins))
	}

	if needsToReHome {
		wg.Add(1)
		go func() {
			// Decrement the counter when the go routine completes
			defer wg.Done()
			if err = g.home(ctx, len(newConf.LimitSwitchPins)); err != nil {
				g.logger.Error(err)
			}
		}()
		wg.Wait()
	}

	g.logger.Error(time.Now())
	return nil
}

func (g *singleAxis) home(ctx context.Context, np int) error {
	ctx, done := g.opMgr.New(ctx)
	defer done()

	switch np {
	// An axis with an encoder will encode the zero position, and add the second position limit
	// based on the steps per length
	case 0:
		if err := g.homeEncoder(ctx); err != nil {
			return err
		}
	// An axis with one limit switch will go till it hits the limit switch, encode that position as the
	// zero position of the singleAxis, and adds a second position limit based on the steps per length.
	// An axis with two limit switches will go till it hits the first limit switch, encode that position as the
	// zero position of the singleAxis, then go till it hits the second limit switch, then encode that position as the
	// at-length position of the singleAxis.
	case 1, 2:
		if err := g.homeLimSwitch(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (g *singleAxis) homeLimSwitch(ctx context.Context) error {
	var positionA, positionB, start float64
	positionA, err := g.testLimit(ctx, true)
	if err != nil {
		return err
	}

	if len(g.limitSwitchPins) > 1 {
		// Multiple limit switches, get positionB from testLimit
		positionB, err = g.testLimit(ctx, false)
		if err != nil {
			return err
		}
		start = 0.8
	} else {
		// Only one limit switch, calculate positionB
		revPerLength := g.lengthMm / g.mmPerRevolution
		positionB = positionA + revPerLength
		start = 0.2
	}

	g.positionLimits = []float64{positionA, positionB}
	g.positionRange = positionB - positionA
	if g.positionRange == 0 {
		g.logger.Error("positionRange is 0 or not a valid number")
	}
	g.logger.Debugf("positionA: %0.2f positionB: %0.2f range: %0.2f", g.positionRange)

	// Go to start position so limit stops are not hit.
	if err = g.goToStart(ctx, start); err != nil {
		return err
	}

	return nil
}

// home encoder assumes that you have places one of the stepper motors where you
// want your zero position to be, you need to know which way is "forward"
// on your motor.
func (g *singleAxis) homeEncoder(ctx context.Context) error {
	revPerLength := g.lengthMm / g.mmPerRevolution

	positionA, err := g.motor.Position(ctx, nil)
	if err != nil {
		return err
	}

	positionB := positionA + revPerLength

	g.positionLimits = []float64{positionA, positionB}
	return nil
}

func (g *singleAxis) goToStart(ctx context.Context, percent float64) error {
	x := g.gantryToMotorPosition(percent * g.lengthMm)
	if err := g.motor.GoTo(ctx, g.rpm, x, nil); err != nil {
		return err
	}
	return nil
}

func (g *singleAxis) gantryToMotorPosition(positions float64) float64 {
	x := positions / g.lengthMm
	x = g.positionLimits[0] + (x * g.positionRange)
	return x
}

func (g *singleAxis) testLimit(ctx context.Context, zero bool) (float64, error) {
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
	// Short pause after stopping to increase the precision of the position of each limit switch
	position, err := g.motor.Position(ctx, nil)
	time.Sleep(250 * time.Millisecond)
	return position, err
}

// this function may need to be run in the background upon initialisation of the ganty,
// also may need to use a digital intterupt pin instead of a gpio pin.
func (g *singleAxis) limitHit(ctx context.Context, zero bool) (bool, error) {
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
func (g *singleAxis) Position(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
	pos, err := g.motor.Position(ctx, extra)
	if err != nil {
		return []float64{}, err
	}

	x := g.lengthMm * ((pos - g.positionLimits[0]) / g.positionRange)

	return []float64{x}, nil
}

// Lengths returns the physical lengths of an axis of a Gantry.
func (g *singleAxis) Lengths(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return []float64{g.lengthMm}, nil
}

// MoveToPosition moves along an axis using inputs in millimeters.
func (g *singleAxis) MoveToPosition(ctx context.Context, positions []float64, extra map[string]interface{}) error {
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
		// Stops if position x is past the 0 limit switch
		if x <= (g.positionLimits[0] + limitErrorMargin) {
			g.logger.Debugf("limit: %.2f", g.positionLimits[0]+limitErrorMargin)
			g.logger.Debugf("position x: %.2f", x)
			g.logger.Error("Cannot move past limit switch!")
			return g.motor.Stop(ctx, extra)
		}

		// Stops if position x is past the at-length limit switch
		if x >= (g.positionLimits[1] - limitErrorMargin) {
			g.logger.Debugf("limit: %.2f", g.positionLimits[1]-limitErrorMargin)
			g.logger.Debugf("position x: %.2f", x)
			g.logger.Error("Cannot move past limit switch!")
			return g.motor.Stop(ctx, extra)
		}
	}

	g.logger.Debugf("going to %.2f at speed %.2f", x, g.rpm)
	if err := g.motor.GoTo(ctx, g.rpm, x, extra); err != nil {
		return err
	}
	return nil
}

// Stop stops the motor of the gantry.
func (g *singleAxis) Stop(ctx context.Context, extra map[string]interface{}) error {
	ctx, done := g.opMgr.New(ctx)
	defer done()
	return g.motor.Stop(ctx, extra)
}

// Close calls stop.
func (g *singleAxis) Close(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.Stop(ctx, nil)
}

// IsMoving returns whether the gantry is moving.
func (g *singleAxis) IsMoving(ctx context.Context) (bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.opMgr.OpRunning(), nil
}

// ModelFrame returns the frame model of the Gantry.
func (g *singleAxis) ModelFrame() referenceframe.Model {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.model == nil {
		var errs error
		m := referenceframe.NewSimpleModel("")

		f, err := referenceframe.NewStaticFrame(g.Name().ShortName(), spatial.NewZeroPose())
		errs = multierr.Combine(errs, err)
		m.OrdTransforms = append(m.OrdTransforms, f)

		f, err = referenceframe.NewTranslationalFrame(g.Name().ShortName(), g.frame, referenceframe.Limit{Min: 0, Max: g.lengthMm})
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
func (g *singleAxis) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	res, err := g.Position(ctx, nil)
	if err != nil {
		return nil, err
	}
	return referenceframe.FloatsToInputs(res), nil
}

// GoToInputs moves the gantry to a goal position in the Gantry frame.
func (g *singleAxis) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.MoveToPosition(ctx, referenceframe.InputsToFloats(goal), nil)
}
