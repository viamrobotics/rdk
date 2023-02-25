// Package oneaxis implements a one-axis gantry.
package oneaxis

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/gantry"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	rdkutils "go.viam.com/rdk/utils"
)

var modelname = resource.NewDefaultModel("oneaxis")

func errBadNumLimitSwitches(name string, numLimPins int) error {
	return fmt.Errorf("bad number of limit switch pins for gantry %s: can only be 0, 1 or 2, have %d",
		name, numLimPins)
}

func errDimensionsNotFound(name string, length, mmPerRev float64) error {
	return fmt.Errorf("zero dimension found for gantry %s, length: %.2f, mmPerRev %.2f",
		name, length, mmPerRev)
}

var errZeroLengthGantry = errors.New("zero length gantry found between position limits")

// AttrConfig is used for converting oneAxis config attributes.
type AttrConfig struct {
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
func (cfg *AttrConfig) Validate(path string) ([]string, error) {
	var deps []string

	if len(cfg.Motor) == 0 {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "motor")
	}
	deps = append(deps, cfg.Motor)

	if cfg.LengthMm <= 0 {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "length_mm")
	}

	if len(cfg.LimitSwitchPins) == 0 && len(cfg.Board) > 0 {
		return nil, errors.New("gantry with encoders have to assign boards or controllers to motors")
	}

	if len(cfg.Board) == 0 && len(cfg.LimitSwitchPins) > 0 {
		return nil, errors.New("gantries with limit_pins require a board to sense limit hits")
	} else {
		deps = append(deps, cfg.Board)
	}

	if len(cfg.LimitSwitchPins) == 1 && cfg.MmPerRevolution == 0 {
		return nil, errors.New("the one-axis gantry has one limit switch axis, needs pulley radius to set position limits")
	}

	if len(cfg.LimitSwitchPins) > 0 && cfg.LimitPinEnabled == nil {
		return nil, errors.New("limit pin enabled must be set to true or false")
	}

	if cfg.Axis.X == 0 && cfg.Axis.Y == 0 && cfg.Axis.Z == 0 {
		return nil, errors.New("gantry axis undefined, need one translational axis")
	}

	if cfg.Axis.X == 1 && cfg.Axis.Y == 1 ||
		cfg.Axis.X == 1 && cfg.Axis.Z == 1 || cfg.Axis.Y == 1 && cfg.Axis.Z == 1 {
		return nil, errors.New("only one translational axis of movement allowed for single axis gantry")
	}

	if cfg.GantryRPM == 0 {
		cfg.GantryRPM = float64(100)
	}

	return deps, nil
}

func init() {
	registry.RegisterComponent(gantry.Subtype, modelname, registry.Component{Constructor: setUpGantry})

	config.RegisterComponentAttributeMapConverter(gantry.Subtype, modelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&AttrConfig{})
}

func setUpGantry(
	ctx context.Context,
	deps registry.Dependencies,
	cfg config.Component,
	logger golog.Logger,
) (interface{}, error) {
	// sets up a single axis that uses the motor to set the gantry limits and range
	// the motor is position reporting and can return the absolute position along its axis
	oAx, err := newOneAxis(ctx, deps, cfg, logger)
	if err != nil {
		return nil, err
	}

	conf, ok := cfg.ConvertedAttributes.(*AttrConfig)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(conf, cfg.ConvertedAttributes)
	}

	switch len(conf.LimitSwitchPins) {
	case 0:
		// homes the motor and finds limits using the motor's position reporting
		if err = oAx.homeEncoder(ctx); err != nil {
			return nil, err
		}
		return oAx, nil
	default:
		// sets up a gantry that requires polling of external sensors, in this case limit switches
		// to find position limits and gantry range
		lAx, err := newLimitSwitchGantry(ctx, cfg, logger, oAx)
		if err != nil {
			return nil, err
		}

		if err = lAx.homeWithLimSwitch(ctx, conf.LimitSwitchPins); err != nil {
			return nil, err
		}
		return lAx, nil
	}
}

type limitSwitchGantry struct {
	ctx context.Context
	oAx *oneAxis

	limitSwitchPins []string
	limitHigh       bool
	limitHitMu      sync.Mutex
	limitsPolled    bool

	logger golog.Logger
	generic.Unimplemented
}

// newOneAxis creates a new one axis gantry.
func newLimitSwitchGantry(
	ctx context.Context,
	cfg config.Component,
	logger golog.Logger,
	oneAxis *oneAxis,
) (*limitSwitchGantry, error) {
	conf, ok := cfg.ConvertedAttributes.(*AttrConfig)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(conf, cfg.ConvertedAttributes)
	}

	if len(conf.LimitSwitchPins) > 2 {
		return nil, errBadNumLimitSwitches(cfg.Name, len(conf.LimitSwitchPins))
	}

	g := &limitSwitchGantry{
		ctx:             ctx,
		limitSwitchPins: conf.LimitSwitchPins,
		limitHigh:       *conf.LimitPinEnabled,
		oAx:             oneAxis,
		logger:          logger,
		limitsPolled:    false,
	}

	if len(g.limitSwitchPins) == 1 && (g.oAx.mmPerRevolution == 0 || g.oAx.lengthMm == 0) {
		return nil, errDimensionsNotFound(g.oAx.name, g.oAx.mmPerRevolution, g.oAx.mmPerRevolution)
	}

	g.limitHitChecker()

	return g, nil
}

func (g *limitSwitchGantry) limitHitChecker() {
	// lock the mutex associated with this monitoring function
	g.limitHitMu.Lock()
	// check if the state is false
	if !g.limitsPolled {
		g.limitHitMu.Unlock()
		return
	}
	// we know we are monitoring the limit switches in this gantry,
	// so we return if they are already being checked
	g.limitsPolled = true
	g.limitHitMu.Unlock()

	var errs error
	for {
		select {
		case <-g.ctx.Done():
			return
		default:
		}
		for idx := range g.limitSwitchPins {
			hit, err := g.limitHit(g.ctx, idx)
			if hit || err != nil {
				g.limitHitMu.Lock()
				g.limitsPolled = false
				g.limitHitMu.Unlock()
				// motor operation manager cancels running
				errs = multierr.Combine(err, g.oAx.motor.Stop(g.ctx, nil))
				if errs != nil {
					g.logger.Error(errs)
					return
				}
			}
		}
	}
}

func (g *limitSwitchGantry) limitHit(ctx context.Context, offset int) (bool, error) {
	if offset < 0 || offset > 1 {
		return true, fmt.Errorf("index out of range for gantry %s limit swith pins %d",
			g.oAx.name, offset)
	}
	pin, err := g.oAx.board.GPIOPinByName(g.limitSwitchPins[offset])
	if err != nil {
		return false, err
	}
	high, err := pin.Get(ctx, nil)

	return high == g.limitHigh, err
}

// helper function to return the motor rotary positoon associated with
// each limit of the oneAxis gantry
func (g *limitSwitchGantry) testLimit(ctx context.Context, limInt int) (float64, error) {
	defer utils.UncheckedErrorFunc(func() error {
		return g.oAx.motor.Stop(ctx, nil)
	})

	d := -1.0 // go backwards to hit first limit switch pin
	if limInt > 0 {
		d *= -1 // go forwards to hit second limit switch pin
	}

	err := g.oAx.motor.GoFor(ctx, d*g.oAx.rpm, 0, nil)
	if err != nil {
		return 0, err
	}

	start := time.Now()
	for {
		hit, err := g.limitHit(ctx, limInt)
		if err != nil {
			return 0, err
		}
		if hit {
			err = g.oAx.motor.Stop(ctx, nil)
			if err != nil {
				return 0, err
			}
			break
		}

		elapsed := time.Now().Sub(start)
		if elapsed > (time.Second * 15) {
			return 0, errors.New("gantry timed out testing limit")
		}

		if !utils.SelectContextOrWait(ctx, time.Millisecond*10) {
			return 0, ctx.Err()
		}
	}

	return g.oAx.motor.Position(ctx, nil)
}

func (g *limitSwitchGantry) homeWithLimSwitch(ctx context.Context, limSwitchPins []string) error {
	ctx, done := g.oAx.opMgr.New(ctx)
	defer done()
	positionA, err := g.testLimit(ctx, 0)
	if err != nil {
		return err
	}

	np := len(limSwitchPins)
	var positionB float64
	switch np {
	case 1:
		if g.oAx.lengthMm == 0.0 || g.oAx.mmPerRevolution == 0.0 {
			return errDimensionsNotFound(g.oAx.name, g.oAx.lengthMm, g.oAx.mmPerRevolution)
		}
		totalRevolutions := g.oAx.lengthMm / g.oAx.mmPerRevolution
		positionB = positionA + totalRevolutions
	case 2:
		positionB, err = g.testLimit(ctx, 1)
		if err != nil {
			return err
		}
	default:
		return errBadNumLimitSwitches(g.oAx.name, np)
	}

	g.oAx.logger.Debugf("finished homing gantry with positionA: %0.2f positionB: %0.2f", positionA, positionB)

	g.oAx.positionLimits = []float64{positionA, positionB}

	if g.oAx.gantryRange = positionB - positionA; g.oAx.gantryRange == 0 {
		return errZeroLengthGantry
	}

	// Go backwards so limit stops are not hit.
	target := 0.9
	if len(limSwitchPins) == 1 {
		target = 0.1 // We're near the start, not the end when homing a gantry with a single limit pin
	}
	x := g.oAx.linearToRotational(target*(g.oAx.gantryRange) + positionA)
	err = g.oAx.motor.GoTo(ctx, g.oAx.rpm, x, nil)
	if err != nil {
		return err
	}
	return nil
}

func (g *limitSwitchGantry) Position(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
	return g.oAx.Position(ctx, extra)
}

func (g *limitSwitchGantry) MoveToPosition(
	ctx context.Context,
	positionsMm []float64,
	worldstate *referenceframe.WorldState,
	extra map[string]interface{},
) error {
	return g.oAx.MoveToPosition(ctx, positionsMm, worldstate, extra)
}

func (g *limitSwitchGantry) Lengths(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
	return g.oAx.Lengths(ctx, extra)
}

func (g *limitSwitchGantry) Stop(ctx context.Context, extra map[string]interface{}) error {
	return g.oAx.Stop(ctx, extra)
}

func (g *limitSwitchGantry) ModelFrame() referenceframe.Model {
	return g.oAx.ModelFrame()
}

func (g *limitSwitchGantry) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	return g.oAx.CurrentInputs(ctx)
}

func (g *limitSwitchGantry) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	return g.oAx.GoToInputs(ctx, goal)
}

func (g *limitSwitchGantry) IsMoving(ctx context.Context) (bool, error) {
	return g.oAx.IsMoving(ctx)
}
