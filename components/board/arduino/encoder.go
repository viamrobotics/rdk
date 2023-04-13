package arduino

import (
	"context"
	"fmt"
	"strconv"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	rdkutils "go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterComponent(
		encoder.Subtype,
		resource.NewDefaultModel("arduino"),
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return NewEncoder(ctx, deps, config, logger)
		}})

	config.RegisterComponentAttributeMapConverter(
		encoder.Subtype,
		resource.NewDefaultModel("arduino"),
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf EncoderConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&EncoderConfig{})
}

// NewEncoder creates a new incremental Encoder.
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

		e.A = cfg.Pins.A
		e.B = cfg.Pins.B

		e.name = cfg.MotorName
		if e.name == "" {
			return nil, errors.New("expected non-empty string for motor_name")
		}
	}

	return e, nil
}

// Encoder keeps track of an arduino motor position.
type Encoder struct {
	board *arduinoBoard
	A, B  string
	name  string

	positionType encoder.PositionType
	generic.Unimplemented
}

// EncoderPins defines the format the pin config should be in for Encoder.
type EncoderPins struct {
	A string `json:"a"`
	B string `json:"b"`
}

// EncoderConfig describes the config of an arduino Encoder.
type EncoderConfig struct {
	Pins      EncoderPins `json:"pins"`
	BoardName string      `json:"board"`
	MotorName string      `json:"motor_name"`
}

// Validate ensures all parts of the config are valid.
func (cfg *EncoderConfig) Validate(path string) ([]string, error) {
	var deps []string
	if cfg.BoardName == "" {
		return nil, rdkutils.NewConfigValidationFieldRequiredError(path, "board")
	}
	deps = append(deps, cfg.BoardName)
	return deps, nil
}

// GetPosition returns the current position in terms of ticks or
// degrees, and whether it is a relative or absolute position.
func (e *Encoder) GetPosition(
	ctx context.Context,
	positionType *encoder.PositionType,
	extra map[string]interface{},
) (float64, encoder.PositionType, error) {
	if positionType != nil && *positionType == encoder.PositionTypeDEGREES {
		err := errors.New("Encoder does not support PositionType Angle Degrees, use a different PositionType")
		return 0, *positionType, err
	}
	res, err := e.board.runCommand("motor-position " + e.name)
	if err != nil {
		return 0, e.positionType, err
	}

	ticks, err := strconv.ParseInt(res, 10, 64)
	if err != nil {
		return 0, e.positionType, fmt.Errorf("couldn't parse # ticks (%s) : %w", res, err)
	}

	return float64(ticks), e.positionType, nil
}

// ResetPosition sets the current position of the motor (adjusted by a given offset)
// to be its new zero position.
func (e *Encoder) ResetPosition(ctx context.Context, extra map[string]interface{}) error {
	offsetInt := int64(0)
	_, err := e.board.runCommand(fmt.Sprintf("motor-zero %s %d", e.name, offsetInt))
	return err
}

// GetProperties returns a list of all the position types that are supported by a given encoder.
func (e *Encoder) GetProperties(ctx context.Context, extra map[string]interface{}) (map[encoder.Feature]bool, error) {
	return map[encoder.Feature]bool{
		encoder.TicksCountSupported:   true,
		encoder.AngleDegreesSupported: false,
	}, nil
}
