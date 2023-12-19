// Package singleaxis implements a single-axis gantry.
package singleaxis

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	utils "go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/gantry"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	spatial "go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

var (
	model = resource.DefaultModelFamily.WithModel("single-axis")
	// homingTimeout (nanoseconds) is calculated using the gantry's rpm, mmPerRevolution, and lengthMm.
	homingTimeout = time.Duration(15e9)
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
	MmPerRevolution float64  `json:"mm_per_rev"`
	GantryMmPerSec  float64  `json:"gantry_mm_per_sec,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (cfg *Config) Validate(path string) ([]string, error) {
	var deps []string

	if len(cfg.Motor) == 0 {
		return nil, resource.NewConfigValidationFieldRequiredError(path, "motor")
	}
	deps = append(deps, cfg.Motor)

	if cfg.LengthMm <= 0 {
		err := resource.NewConfigValidationFieldRequiredError(path, "length_mm")
		return nil, errors.Wrap(err, "length must be non-zero and positive")
	}

	if cfg.MmPerRevolution <= 0 {
		err := resource.NewConfigValidationFieldRequiredError(path, "mm_per_rev")
		return nil, errors.Wrap(err, "mm_per_rev must be non-zero and positive")
	}

	if cfg.Board == "" && len(cfg.LimitSwitchPins) > 0 {
		return nil, errors.New("gantries with limit_pins require a board to sense limit hits")
	}

	if cfg.Board != "" {
		deps = append(deps, cfg.Board)
	}

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

	cancelFunc              func()
	logger                  logging.Logger
	opMgr                   *operation.SingleOperationManager
	activeBackgroundWorkers sync.WaitGroup
}

// newSingleAxis creates a new single axis gantry.
func newSingleAxis(
	ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger,
) (gantry.Gantry, error) {
	sAx := &singleAxis{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
		opMgr:  operation.NewSingleOperationManager(),
	}

	if err := sAx.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return sAx, nil
}

func (g *singleAxis) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	if g.motor != nil {
		if err := g.motor.Stop(ctx, nil); err != nil {
			return err
		}
	}

	if g.cancelFunc != nil {
		g.cancelFunc()
		g.activeBackgroundWorkers.Wait()
	}

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

	// Add a default frame, then overwrite with the config frame if that is supplied
	g.frame = r3.Vector{X: 1.0, Y: 0, Z: 0}
	if conf.Frame != nil {
		g.frame = conf.Frame.Translation
	}

	rpm := g.gantryToMotorSpeeds(newConf.GantryMmPerSec)
	g.rpm = rpm
	if g.rpm == 0 {
		g.logger.CWarn(ctx, "gantry_mm_per_sec not provided, defaulting to 100 motor rpm")
		g.rpm = 100
	}

	// Rerun homing if the board has changed
	if newConf.Board != "" {
		if g.board == nil || g.board.Name().ShortName() != newConf.Board {
			board, err := board.FromDependencies(deps, newConf.Board)
			if err != nil {
				return err
			}
			g.board = board
			needsToReHome = true
		}
	}

	// Rerun homing if the motor changes
	if g.motor == nil || g.motor.Name().ShortName() != newConf.Motor {
		needsToReHome = true
		motorDep, err := motor.FromDependencies(deps, newConf.Motor)
		if err != nil {
			return err
		}
		properties, err := motorDep.Properties(ctx, nil)
		if err != nil {
			return err
		}
		ok := properties.PositionReporting
		if !ok {
			return motor.NewPropertyUnsupportedError(properties, newConf.Motor)
		}
		g.motor = motorDep
	}

	// Rerun homing if anything with the limit switch pins changes
	if newConf.LimitPinEnabled != nil && len(newConf.LimitSwitchPins) != 0 {
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
	}
	if len(newConf.LimitSwitchPins) > 2 {
		return errors.Errorf("invalid gantry type: need 1, 2 or 0 pins per axis, have %v pins", len(newConf.LimitSwitchPins))
	}

	if needsToReHome {
		g.logger.CInfof(ctx, "single-axis gantry '%v' needs to re-home", g.Named.Name().ShortName())
		g.positionRange = 0
		g.positionLimits = []float64{0, 0}
	}
	ctx, cancelFunc := context.WithCancel(context.Background())
	g.cancelFunc = cancelFunc
	g.checkHit(ctx)

	return nil
}

// Home runs the homing sequence of the gantry, starts checkHit in the background, and returns true once completed.
func (g *singleAxis) Home(ctx context.Context, extra map[string]interface{}) (bool, error) {
	if g.cancelFunc != nil {
		g.cancelFunc()
		g.activeBackgroundWorkers.Wait()
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	homed, err := g.doHome(ctx)
	if err != nil {
		return homed, err
	}
	ctx, cancelFunc := context.WithCancel(context.Background())
	g.cancelFunc = cancelFunc
	g.checkHit(ctx)
	return true, nil
}

func (g *singleAxis) checkHit(ctx context.Context) {
	g.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer utils.UncheckedErrorFunc(func() error {
			g.mu.Lock()
			defer g.mu.Unlock()
			return g.motor.Stop(ctx, nil)
		})
		defer g.activeBackgroundWorkers.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			for i := 0; i < len(g.limitSwitchPins); i++ {
				hit, err := g.limitHit(ctx, i)
				if err != nil {
					g.logger.CError(ctx, err)
				}

				if hit {
					child, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
					g.mu.Lock()
					if err := g.motor.Stop(ctx, nil); err != nil {
						g.logger.CError(ctx, err)
					}
					g.mu.Unlock()
					<-child.Done()
					cancel()
					g.mu.Lock()
					if err := g.moveAway(ctx, i); err != nil {
						g.logger.CError(ctx, err)
					}
					g.mu.Unlock()
				}
			}
		}
	})
}

// Once a limit switch is hit in any move call (from the motor or the gantry component),
// this function stops the motor, and reverses the direction of movement until the limit
// switch is no longer activated.
func (g *singleAxis) moveAway(ctx context.Context, pin int) error {
	dir := 1.0
	if pin != 0 {
		dir = -1.0
	}
	if err := g.motor.GoFor(ctx, dir*g.rpm, 0, nil); err != nil {
		return err
	}
	defer utils.UncheckedErrorFunc(func() error {
		return g.motor.Stop(ctx, nil)
	})
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		hit, err := g.limitHit(ctx, pin)
		if err != nil {
			return err
		}
		if !hit {
			if err := g.motor.Stop(ctx, nil); err != nil {
				return err
			}
			return nil
		}
	}
}

// doHome is a helper function that runs the actual homing sequence.
func (g *singleAxis) doHome(ctx context.Context) (bool, error) {
	np := len(g.limitSwitchPins)
	ctx, done := g.opMgr.New(ctx)
	defer done()

	switch np {
	// An axis with an encoder will encode the zero position, and add the second position limit
	// based on the steps per length
	case 0:
		if err := g.homeEncoder(ctx); err != nil {
			return false, err
		}
	// An axis with one limit switch will go till it hits the limit switch, encode that position as the
	// zero position of the singleAxis, and adds a second position limit based on the steps per length.
	// An axis with two limit switches will go till it hits the first limit switch, encode that position as the
	// zero position of the singleAxis, then go till it hits the second limit switch, then encode that position as the
	// at-length position of the singleAxis.
	case 1, 2:
		if err := g.homeLimSwitch(ctx); err != nil {
			return false, err
		}
	}

	return true, nil
}

func (g *singleAxis) homeLimSwitch(ctx context.Context) error {
	var positionA, positionB float64
	positionA, err := g.testLimit(ctx, 0)
	if err != nil {
		return err
	}

	if len(g.limitSwitchPins) > 1 {
		// Multiple limit switches, get positionB from testLimit
		positionB, err = g.testLimit(ctx, 1)
		if err != nil {
			return err
		}
	} else {
		// Only one limit switch, calculate positionB
		revPerLength := g.lengthMm / g.mmPerRevolution
		positionB = positionA + revPerLength
	}

	g.positionLimits = []float64{positionA, positionB}
	g.positionRange = positionB - positionA
	if g.positionRange == 0 {
		g.logger.CError(ctx, "positionRange is 0 or not a valid number")
	} else {
		g.logger.CDebugf(ctx, "positionA: %0.2f positionB: %0.2f range: %0.2f", positionA, positionB, g.positionRange)
	}

	// Go to start position at the middle of the axis.
	x := g.gantryToMotorPosition(0.5 * g.lengthMm)
	if err := g.motor.GoTo(ctx, g.rpm, x, nil); err != nil {
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

func (g *singleAxis) gantryToMotorPosition(positions float64) float64 {
	x := positions / g.lengthMm
	x = g.positionLimits[0] + (x * g.positionRange)
	return x
}

func (g *singleAxis) gantryToMotorSpeeds(speeds float64) float64 {
	r := (speeds / g.mmPerRevolution) * 60
	return r
}

func (g *singleAxis) testLimit(ctx context.Context, pin int) (float64, error) {
	defer utils.UncheckedErrorFunc(func() error {
		return g.motor.Stop(ctx, nil)
	})
	wrongPin := 1
	d := -1.0
	if pin != 0 {
		d = 1
		wrongPin = 0
	}

	err := g.motor.GoFor(ctx, d*g.rpm, 0, nil)
	if err != nil {
		return 0, err
	}

	// short sleep to allow pin number to switch correctly
	time.Sleep(100 * time.Millisecond)

	start := time.Now()
	for {
		hit, err := g.limitHit(ctx, pin)
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

		// check if the wrong limit switch was hit
		wrongHit, err := g.limitHit(ctx, wrongPin)
		if err != nil {
			return 0, err
		}
		if wrongHit {
			err = g.motor.Stop(ctx, nil)
			if err != nil {
				return 0, err
			}
			return 0, errors.Errorf(
				"expected limit switch %v but hit limit switch %v, try switching the order in the config",
				pin,
				wrongPin)
		}

		elapsed := time.Since(start)
		// if the parameters checked are non-zero, calculate a timeout with a safety factor of
		// 5 to complete the gantry's homing sequence to find the limit switches
		if g.mmPerRevolution != 0 && g.rpm != 0 && g.lengthMm != 0 {
			homingTimeout = time.Duration((1 / (g.rpm / 60e9 * g.mmPerRevolution / g.lengthMm) * 5))
		}
		if elapsed > (homingTimeout) {
			return 0, errors.Errorf("gantry timed out testing limit, timeout = %v", homingTimeout)
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
func (g *singleAxis) limitHit(ctx context.Context, limitPin int) (bool, error) {
	pin, err := g.board.GPIOPinByName(g.limitSwitchPins[limitPin])
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
func (g *singleAxis) MoveToPosition(ctx context.Context, positions, speeds []float64, extra map[string]interface{}) error {
	if g.positionRange == 0 {
		return errors.Errorf("cannot move to position until gantry '%v' is homed", g.Named.Name().ShortName())
	}
	ctx, done := g.opMgr.New(ctx)
	defer done()

	if len(positions) != 1 {
		return fmt.Errorf("single-axis MoveToPosition needs 1 position to move, got: %v", len(positions))
	}

	if len(speeds) > 1 {
		return fmt.Errorf("single-axis MoveToPosition needs 1 speed to move, got: %v", len(speeds))
	}

	if positions[0] < 0 || positions[0] > g.lengthMm {
		return fmt.Errorf("out of range (%.2f) min: 0 max: %.2f", positions[0], g.lengthMm)
	}

	if len(speeds) == 0 {
		speeds = append(speeds, g.rpm)
		g.logger.CDebug(ctx, "single-axis received invalid speed, using default gantry speed")
	} else if rdkutils.Float64AlmostEqual(math.Abs(speeds[0]), 0.0, 0.1) {
		if err := g.motor.Stop(ctx, nil); err != nil {
			return err
		}
		return fmt.Errorf("speed (%.2f) is too slow, stopping gantry", speeds[0])
	}

	x := g.gantryToMotorPosition(positions[0])
	r := g.gantryToMotorSpeeds(speeds[0])
	// Limit switch errors that stop the motors.
	// Currently needs to be moved by underlying gantry motor.
	if len(g.limitSwitchPins) > 0 {
		// Stops if position x is past the 0 limit switch
		if x <= (g.positionLimits[0] + limitErrorMargin) {
			g.logger.CError(ctx, "Cannot move past limit switch!")
			return g.motor.Stop(ctx, extra)
		}

		// Stops if position x is past the at-length limit switch
		if x >= (g.positionLimits[1] - limitErrorMargin) {
			g.logger.CError(ctx, "Cannot move past limit switch!")
			return g.motor.Stop(ctx, extra)
		}
	}

	g.logger.CDebugf(ctx, "going to %.2f at speed %.2f", x, r)
	if err := g.motor.GoTo(ctx, r, x, extra); err != nil {
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
	if err := g.Stop(ctx, nil); err != nil {
		return err
	}
	g.cancelFunc()
	g.activeBackgroundWorkers.Wait()
	return nil
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
	speed := []float64{}
	return g.MoveToPosition(ctx, referenceframe.InputsToFloats(goal), speed, nil)
}
