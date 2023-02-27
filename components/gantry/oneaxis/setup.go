// Package oneaxis implements a one-axis gantry.
package oneaxis

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/gantry"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
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

	if cfg.Board == "" && len(cfg.LimitSwitchPins) > 0 {
		return nil, errors.New("gantries with limit_pins require a board to sense limit hits")
	} else if cfg.Board != "" {
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
		if err = homeEncoder(ctx, oAx); err != nil {
			return nil, err
		}
		return oAx, nil
	default:
		// sets up a gantry that requires polling of external sensors, in this case limit switches
		// to find position limits and gantry range

		ls := &limitPins{
			limitSwitchPins: conf.LimitSwitchPins,
			limitHigh:       *conf.LimitPinEnabled,
			logger:          logger,
			limitsPolled:    false,
		}

		oAx, err := attachLimitSwitchesToGantry(ls, oAx)
		if err != nil {
			return nil, err
		}

		if err = homeWithLimSwitch(ctx, ls, oAx); err != nil {
			return nil, err
		}
		return oAx, nil
	}
}

func homeEncoder(ctx context.Context, oAx *oneAxis) error {
	oAx.mu.Lock()
	defer oAx.mu.Unlock()

	ctx, done := oAx.opMgr.New(ctx)
	defer done()
	// should be non-zero from creator function
	revPerLength := oAx.lengthMm / oAx.mmPerRevolution

	positionA, err := oAx.motor.Position(ctx, nil)
	if err != nil {
		return err
	}

	positionB := positionA + revPerLength

	oAx.positionLimits = []float64{positionA, positionB}

	// ensure we never create a gantry with a zero range
	if oAx.gantryRange = math.Abs(positionB - positionA); oAx.gantryRange == 0 {
		return errZeroLengthGantry
	}

	return nil
}

type limitPins struct {
	limitSwitchPins []string
	limitHigh       bool
	limitHitMu      sync.Mutex
	limitsPolled    bool

	logger golog.Logger
	generic.Unimplemented
}

// newOneAxis creates a new one axis gantry.
func attachLimitSwitchesToGantry(
	ls *limitPins,
	oAx *oneAxis,
) (*oneAxis, error) {
	if len(ls.limitSwitchPins) > 2 {
		return nil, errBadNumLimitSwitches(oAx.name, len(ls.limitSwitchPins))
	}

	if len(ls.limitSwitchPins) == 1 && (oAx.mmPerRevolution == 0 || oAx.lengthMm == 0) {
		return nil, errDimensionsNotFound(oAx.name, oAx.mmPerRevolution, oAx.mmPerRevolution)
	}

	ls.limitHitChecker(oAx)

	return oAx, nil
}

func (ls *limitPins) limitHitChecker(oAx *oneAxis) {
	// lock the mutex associated with this monitoring function
	ls.limitHitMu.Lock()
	// check if the state is false
	if !ls.limitsPolled {
		ls.limitHitMu.Unlock()
		return
	}
	// we know we are monitoring the limit switches in this gantry,
	// so we return if they are already being checked
	ls.limitsPolled = true
	ls.limitHitMu.Unlock()

	var errs error
	for {
		select {
		case <-oAx.ctx.Done():
			return
		default:
		}
		for idx := range ls.limitSwitchPins {
			hit, err := ls.limitHit(oAx.ctx, idx, oAx)
			if hit || err != nil {
				ls.limitHitMu.Lock()
				ls.limitsPolled = false
				ls.limitHitMu.Unlock()
				// motor operation manager cancels running
				errs = multierr.Combine(err, oAx.motor.Stop(oAx.ctx, nil))
				if errs != nil {
					ls.logger.Error(errs)
					return
				}
			}
		}
	}
}

func (ls *limitPins) limitHit(ctx context.Context, offset int, oAx *oneAxis) (bool, error) {
	if offset < 0 || offset > 1 {
		return true, fmt.Errorf("index out of range for gantry %s limit swith pins %d",
			oAx.name, offset)
	}
	pin, err := oAx.board.GPIOPinByName(ls.limitSwitchPins[offset])
	if err != nil {
		return false, err
	}
	high, err := pin.Get(ctx, nil)

	return high == ls.limitHigh, err
}

// helper function to return the motor rotary positoon associated with
// each limit of the oneAxis gantry.
func (ls *limitPins) testLimit(ctx context.Context, limInt int, oAx *oneAxis) (float64, error) {
	defer utils.UncheckedErrorFunc(func() error {
		return oAx.motor.Stop(ctx, nil)
	})

	d := -1.0 // go backwards to hit first limit switch pin
	if limInt > 0 {
		d *= -1 // go forwards to hit second limit switch pin
	}

	err := oAx.motor.GoFor(ctx, d*oAx.rpm, 0, nil)
	if err != nil {
		return 0, err
	}

	start := time.Now()
	for {
		hit, err := ls.limitHit(ctx, limInt, oAx)
		if err != nil {
			return 0, err
		}
		if hit {
			err = oAx.motor.Stop(ctx, nil)
			if err != nil {
				return 0, err
			}
			break
		}

		elapsed := time.Since(start)
		if elapsed > (time.Second * 15) {
			return 0, errors.New("gantry timed out testing limit")
		}

		if !utils.SelectContextOrWait(ctx, time.Millisecond*10) {
			return 0, ctx.Err()
		}
	}

	return oAx.motor.Position(ctx, nil)
}

func homeWithLimSwitch(ctx context.Context, ls *limitPins, oAx *oneAxis) error {
	ctx, done := oAx.opMgr.New(ctx)
	defer done()
	positionA, err := ls.testLimit(ctx, 0, oAx)
	if err != nil {
		return err
	}

	np := len(ls.limitSwitchPins)
	var positionB float64
	switch np {
	case 1:
		if oAx.lengthMm == 0.0 || oAx.mmPerRevolution == 0.0 {
			return errDimensionsNotFound(oAx.name, oAx.lengthMm, oAx.mmPerRevolution)
		}
		totalRevolutions := oAx.lengthMm / oAx.mmPerRevolution
		positionB = positionA + totalRevolutions
	case 2:
		positionB, err = ls.testLimit(ctx, 1, oAx)
		if err != nil {
			return err
		}
	default:
		return errBadNumLimitSwitches(oAx.name, np)
	}

	oAx.logger.Debugf("finished homing gantry with positionA: %0.2f positionB: %0.2f", positionA, positionB)

	oAx.positionLimits = []float64{positionA, positionB}

	if oAx.gantryRange = positionB - positionA; oAx.gantryRange == 0 {
		return errZeroLengthGantry
	}

	// Go backwards so limit stops are not hit.
	target := 0.9
	if len(ls.limitSwitchPins) == 1 {
		target = 0.1 // We're near the start, not the end when homing a gantry with a single limit pin
	}
	x := linearToRotational(target*(oAx.gantryRange)+positionA, positionA, oAx.lengthMm, oAx.gantryRange)
	err = oAx.motor.GoTo(ctx, oAx.rpm, x, nil)
	if err != nil {
		return err
	}
	return nil
}
