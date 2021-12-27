package motor

import (
	"context"
	"sync"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	viamutils "go.viam.com/utils"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
)

// SubtypeName is a constant that identifies the component resource subtype string "motor".
const SubtypeName = resource.SubtypeName("motor")

// Subtype is a constant that identifies the component resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// A Motor represents a physical motor connected to a board.
type Motor interface {

	// SetPower sets the percentage of power the motor should employ between 0-1.
	SetPower(ctx context.Context, powerPct float64) error

	// Go instructs the motor to go in a specific direction at a percentage
	// of power between -1 and 1.
	Go(ctx context.Context, powerPct float64) error

	// GoFor instructs the motor to go in a specific direction for a specific amount of
	// revolutions at a given speed in revolutions per minute. Both the RPM and the revolutions
	// can be assigned negative values to move in a backwards direction. Note: if both are
	// negative the motor will spin in the forward direction.
	GoFor(ctx context.Context, rpm float64, revolutions float64) error

	// GoTo instructs the motor to go to a specific position (provided in revolutions from home/zero),
	// at a specific speed. Regardless of the directionality of the RPM this function will move the motor
	// towards the specified target/position
	GoTo(ctx context.Context, rpm float64, position float64) error

	// GoTillStop moves a motor until stopped. The "stop" mechanism is up to the underlying motor implementation.
	// Ex: EncodedMotor goes until physically stopped/stalled (detected by change in position being very small over a fixed time.)
	// Ex: TMCStepperMotor has "StallGuard" which detects the current increase when obstructed and stops when that reaches a threshold.
	// Ex: Other motors may use an endstop switch (such as via a DigitalInterrupt) or be configured with other sensors.
	GoTillStop(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error

	// Set the current position (+/- offset) to be the new zero (home) position.
	ResetZeroPosition(ctx context.Context, offset float64) error

	// Position reports the position of the motor based on its encoder. If it's not supported, the returned
	// data is undefined. The unit returned is the number of revolutions which is intended to be fed
	// back into calls of GoFor.
	Position(ctx context.Context) (float64, error)

	// PositionSupported returns whether or not the motor supports reporting of its position which
	// is reliant on having an encoder.
	PositionSupported(ctx context.Context) (bool, error)

	// Stop turns the power to the motor off immediately, without any gradual step down.
	Stop(ctx context.Context) error

	// IsOn returns whether or not the motor is currently on.
	IsOn(ctx context.Context) (bool, error)

	// PID returns underlying PID for the motor
	PID() PID
}

// Named is a helper for getting the named Motor's typed resource name.
func Named(name string) resource.Name {
	return resource.NewFromSubtype(Subtype, name)
}

var (
	_ = Motor(&reconfigurableMotor{})
	_ = resource.Reconfigurable(&reconfigurableMotor{})
)

type reconfigurableMotor struct {
	mu     sync.RWMutex
	actual Motor
}

func (r *reconfigurableMotor) ProxyFor() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual
}

func (r *reconfigurableMotor) SetPower(ctx context.Context, powerPct float64) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.SetPower(ctx, powerPct)
}

func (r *reconfigurableMotor) Go(ctx context.Context, powerPct float64) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Go(ctx, powerPct)
}

func (r *reconfigurableMotor) GoFor(ctx context.Context, rpm float64, revolutions float64) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GoFor(ctx, rpm, revolutions)
}

func (r *reconfigurableMotor) GoTo(ctx context.Context, rpm float64, position float64) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GoTo(ctx, rpm, position)
}

func (r *reconfigurableMotor) GoTillStop(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GoTillStop(ctx, rpm, stopFunc)
}

func (r *reconfigurableMotor) ResetZeroPosition(ctx context.Context, offset float64) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.ResetZeroPosition(ctx, offset)
}

func (r *reconfigurableMotor) Position(ctx context.Context) (float64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Position(ctx)
}

func (r *reconfigurableMotor) PositionSupported(ctx context.Context) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.PositionSupported(ctx)
}

func (r *reconfigurableMotor) Stop(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Stop(ctx)
}

func (r *reconfigurableMotor) IsOn(ctx context.Context) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.IsOn(ctx)
}

func (r *reconfigurableMotor) PID() PID {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.PID()
}

func (r *reconfigurableMotor) Close(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return viamutils.TryClose(ctx, r.actual)
}

func (r *reconfigurableMotor) Reconfigure(ctx context.Context, newMotor resource.Reconfigurable) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	actual, ok := newMotor.(*reconfigurableMotor)
	if !ok {
		return errors.Errorf("expected new arm to be %T but got %T", r, newMotor)
	}
	if err := viamutils.TryClose(ctx, r.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
	return nil
}

// WrapWithReconfigurable converts a regular Motor implementation to a reconfigurableMotor.
// If servo is already a reconfigurableMotor, then nothing is done.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	servo, ok := r.(Motor)
	if !ok {
		return nil, errors.Errorf("expected resource to be Motor but got %T", r)
	}
	if reconfigurable, ok := servo.(*reconfigurableMotor); ok {
		return reconfigurable, nil
	}
	return &reconfigurableMotor{actual: servo}, nil
}

// Config describes the configuration of a motor.
type Config struct {
	Pins             map[string]string `json:"pins"`
	BoardName        string            `json:"board"`    // used to get encoders
	Encoder          string            `json:"encoder"`  // name of the digital interrupt that is the encoder
	EncoderB         string            `json:"encoderB"` // name of the digital interrupt that is hall encoder b
	TicksPerRotation int               `json:"ticksPerRotation"`
	RampRate         float64           `json:"rampRate"`         // how fast to ramp power to motor when using rpm control
	MinPowerPct      float64           `json:"min_power_pct"`    // min power percentage to allow for this motor default is 0.0
	MaxPowerPct      float64           `json:"max_power_pct"`    // max power percentage to allow for this motor (0.06 - 1.0)
	MaxRPM           float64           `json:"max_rpm"`          // RPM
	MaxAcceleration  float64           `json:"max_acceleration"` // RPM per second
	PWMFreq          uint              `json:"pwmFreq"`
	StepperDelay     uint              `json:"stepperDelay"` // When using stepper motors, the time to remain high
	DirFlip          bool              `json:"dir_flip"`     // Flip the direction of the signal sent if there is a Dir pin
	PID              *PIDConfig        `json:"pid,omitempty"`
}

// RegisterConfigAttributeConverter registers a Config converter.
// Note(erd): This probably shouldn't exist since not all motors have the same config requirements.
func RegisterConfigAttributeConverter(model string) {
	config.RegisterComponentAttributeMapConverter(
		config.ComponentTypeMotor,
		model,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf Config
			decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &conf})
			if err != nil {
				return nil, err
			}
			if err := decoder.Decode(attributes); err != nil {
				return nil, err
			}
			return &conf, nil
		}, &Config{})
}
