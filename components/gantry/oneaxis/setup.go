// Package oneaxis implements a one-axis gantry.
package oneaxis

import (
	"context"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/rdk/resource"
	utils "go.viam.com/utils"
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
		return nil, utils.NewConfigValidationFieldRequiredError(path, "length_mm")
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

	if cfg.Axis.X == 1 && cfg.Axis.Y == 1 ||
		cfg.Axis.X == 1 && cfg.Axis.Z == 1 || cfg.Axis.Y == 1 && cfg.Axis.Z == 1 {
		return nil, errors.New("only one translational axis of movement allowed for single axis gantry")
	}

	return deps, nil
}

func (g *oneAxis) home(ctx context.Context) error {
	ctx, done := g.opMgr.New(ctx)
	defer done()

	// Mapping one limit switch motor0->limsw0, motor1 ->limsw1, motor 2 -> limsw2
	// Mapping two limit switch motor0->limSw0,limSw1; motor1->limSw2,limSw3; motor2->limSw4,limSw5
	switch len(g.limitSwitchPins) {
	// An axis with one limit switch will go till it hits the limit switch, encode that position as the
	// zero position of the one-axis, and adds a second position limit based on the steps per length.
	case 1:
		if err := g.homeOneLimSwitch(ctx); err != nil {
			return err
		}
	// An axis with two limit switches will go till it hits the first limit switch, encode that position as the
	// zero position of the one-axis, then go till it hits the second limit switch, then encode that position as the
	// at-length position of the one-axis.
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

	// Go backwards so limit stops are not hit.
	x := g.rotationalToLinear(0.8 * g.lengthMm)
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

	return nil
}

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
