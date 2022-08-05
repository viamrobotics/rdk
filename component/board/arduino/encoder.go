package arduino

import (
	"context"
	"fmt"
	"strconv"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/encoder"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterComponent(
		encoder.Subtype,
		"arduino",
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return NewEncoder(ctx, deps, config, logger)
		}})
}

// NewEncoder creates a new HallEncoder.
func NewEncoder(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (*Encoder, error) {
	e := &Encoder{}
	if cfg, ok := config.ConvertedAttributes.(*EncoderConfig); ok {
		if cfg.BoardName == "" {
			return nil, errors.New("expected board name in config for encoder")
		}
		b, err := board.FromDependencies(deps, cfg.BoardName)
		if err != nil {
			return nil, err
		}

		e.board, ok = utils.UnwrapProxy(b).(*arduinoBoard)
		if !ok {
			return nil, errors.New("expected board to be an arduino board")
		}

		if pins, ok := cfg.Pins.(*EncoderPins); ok {
			e.A = pins.A
			e.B = pins.B
		} else {
			return nil, errors.New("Pin configuration not valid for Encoder")
		}
		e.ticksPerRotation = int64(cfg.TicksPerRotation)
		if e.ticksPerRotation <= 0 {
			return nil, errors.New("expected nonzero positive int for ticksPerRotation")
		}
		e.name = cfg.MotorName
		if e.name == "" {
			return nil, errors.New("expected non-empty string for ticksPerRotation")
		}
	}

	return e, nil
}

// Encoder keeps track of an arduino motor position.
type Encoder struct {
	board            *arduinoBoard
	A, B             string
	name             string
	ticksPerRotation int64

	generic.Unimplemented
}

// EncoderPins defines the format the pin config should be in for Encoder.
type EncoderPins struct {
	A, B string
}

// EncoderConfig describes the config of an arduino Encoder.
type EncoderConfig struct {
	Pins      interface{} `json:"pins"`
	BoardName string      `json:"board"`
	MotorName string      `json:"motor_name"`

	TicksPerRotation int `json:"ticks_per_rotation,omitempty"`
}

// GetTicksCount returns number of ticks since last zeroing
func (e *Encoder) GetTicksCount(ctx context.Context, extra map[string]interface{}) (int64, error) {
	res, err := e.board.runCommand("motor-position " + e.name)
	if err != nil {
		return 0, err
	}

	ticks, err := strconv.ParseInt(res, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("couldn't parse # ticks (%s) : %w", res, err)
	}

	return ticks, nil
}

// ResetZeroPosition resets the counted ticks to 0
func (e *Encoder) ResetZeroPosition(ctx context.Context, offset int64, extra map[string]interface{}) error {
	_, err := e.board.runCommand(fmt.Sprintf("motor-zero %s %d", e.name, offset))
	return err
}

// TicksPerRotation returns the number of ticks needed for a full rotation
func (e *Encoder) TicksPerRotation(ctx context.Context) (int64, error) {
	return e.ticksPerRotation, nil
}
