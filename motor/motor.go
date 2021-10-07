package motor

import (
	"context"

	"github.com/mitchellh/mapstructure"

	"go.viam.com/core/config"
	pb "go.viam.com/core/proto/api/v1"
)

// A Motor represents a physical motor connected to a board.
type Motor interface {

	// Power sets the percentage of power the motor should employ between 0-1.
	Power(ctx context.Context, powerPct float32) error

	// Go instructs the motor to go in a specific direction at a percentage
	// of power between 0-1.
	Go(ctx context.Context, d pb.DirectionRelative, powerPct float32) error

	// GoFor instructs the motor to go in a specific direction for a specific amount of
	// revolutions at a given speed in revolutions per minute.
	GoFor(ctx context.Context, d pb.DirectionRelative, rpm float64, revolutions float64) error

	// GoTo instructs the motor to go to a specific position (provided in revolutions from home/zero), at a specific speed.
	GoTo(ctx context.Context, rpm float64, position float64) error

	// GoTillStop moves a motor until stopped. The "stop" mechanism is up to the underlying motor implementation.
	// Ex: EncodedMotor goes until physically stopped/stalled (detected by change in position being very small over a fixed time.)
	// Ex: TMCStepperMotor has "StallGuard" which detects the current increase when obstructed and stops when that reaches a threshold.
	// Ex: Other motors may use an endstop switch (such as via a DigitalInterrupt) or be configured with other sensors.
	GoTillStop(ctx context.Context, d pb.DirectionRelative, rpm float64, stopFunc func(ctx context.Context) bool) error

	// Set the current position (+/- offset) to be the new zero (home) position.
	Zero(ctx context.Context, offset float64) error

	// Position reports the position of the motor based on its encoder. If it's not supported, the returned
	// data is undefined. The unit returned is the number of revolutions which is intended to be fed
	// back into calls of GoFor.
	Position(ctx context.Context) (float64, error)

	// PositionSupported returns whether or not the motor supports reporting of its position which
	// is reliant on having an encoder.
	PositionSupported(ctx context.Context) (bool, error)

	// Off turns the motor off.
	Off(ctx context.Context) error

	// IsOn returns whether or not the motor is currently on.
	IsOn(ctx context.Context) (bool, error)
}

// Config describes the configuration of a motor.
type Config struct {
	Pins             map[string]string `json:"pins"`
	BoardName        string            `json:"board"`    // used to get encoders
	Encoder          string            `json:"encoder"`  // name of the digital interrupt that is the encoder
	EncoderB         string            `json:"encoderB"` // name of the digital interrupt that is hall encoder b
	TicksPerRotation int               `json:"ticksPerRotation"`
	RampRate         float32           `json:"rampRate"`         // how fast to ramp power to motor when using rpm control
	MinPowerPct      float32           `json:"min_power_pct"`    // min power percentage to allow for this motor default is 0.0
	MaxPowerPct      float32           `json:"max_power_pct"`    // max power percentage to allow for this motor (0.06 - 1.0)
	MaxRPM           float64           `json:"max_rpm"`          // RPM
	MaxAcceleration  float64           `json:"max_acceleration"` // RPM per second
	PWMFreq          uint              `json:"pwmFreq"`
}

// RegisterConfigAttributeConverter registers a Config converter.
// Note(erd): This probably shouldn't exist since not all motors have the same config requirements.
func RegisterConfigAttributeConverter(model string) {
	config.RegisterComponentAttributeMapConverter(config.ComponentTypeMotor, model, func(attributes config.AttributeMap) (interface{}, error) {
		var conf Config
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &conf})
		if err != nil {
			return nil, err
		}
		if err := decoder.Decode(attributes); err != nil {
			return nil, err
		}
		return &conf, nil
	})
}
