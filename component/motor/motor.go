package motor

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/control"
	pb "go.viam.com/rdk/proto/api/component/motor/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		Reconfigurable: WrapWithReconfigurable,
		Status: func(ctx context.Context, resource interface{}) (interface{}, error) {
			return CreateStatus(ctx, resource)
		},
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.MotorService_ServiceDesc,
				NewServer(subtypeSvc),
				pb.RegisterMotorServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})
}

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
	// SetPower sets the percentage of power the motor should employ between -1 and 1.
	// Negative power implies a backward directional rotational
	SetPower(ctx context.Context, powerPct float64) error

	// GoFor instructs the motor to go in a specific direction for a specific amount of
	// revolutions at a given speed in revolutions per minute. Both the RPM and the revolutions
	// can be assigned negative values to move in a backwards direction. Note: if both are
	// negative the motor will spin in the forward direction.
	GoFor(ctx context.Context, rpm float64, revolutions float64) error

	// GoTo instructs the motor to go to a specific position (provided in revolutions from home/zero),
	// at a specific speed. Regardless of the directionality of the RPM this function will move the motor
	// towards the specified target/position
	GoTo(ctx context.Context, rpm float64, positionRevolutions float64) error

	// Set the current position (+/- offset) to be the new zero (home) position.
	ResetZeroPosition(ctx context.Context, offset float64) error

	// GetPosition reports the position of the motor based on its encoder. If it's not supported, the returned
	// data is undefined. The unit returned is the number of revolutions which is intended to be fed
	// back into calls of GoFor.
	GetPosition(ctx context.Context) (float64, error)

	// GetFeatures returns whether or not the motor supports certain optional features.
	GetFeatures(ctx context.Context) (map[Feature]bool, error)

	// Stop turns the power to the motor off immediately, without any gradual step down.
	Stop(ctx context.Context) error

	// IsPowered returns whether or not the motor is currently on.
	IsPowered(ctx context.Context) (bool, error)
}

// A LocalMotor is a motor that supports additional features provided by RDK
// (e.g. GoTillStop).
type LocalMotor interface {
	Motor
	// GoTillStop moves a motor until stopped. The "stop" mechanism is up to the underlying motor implementation.
	// Ex: EncodedMotor goes until physically stopped/stalled (detected by change in position being very small over a fixed time.)
	// Ex: TMCStepperMotor has "StallGuard" which detects the current increase when obstructed and stops when that reaches a threshold.
	// Ex: Other motors may use an endstop switch (such as via a DigitalInterrupt) or be configured with other sensors.
	GoTillStop(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error
}

// Named is a helper for getting the named Motor's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

var (
	_ = LocalMotor(&reconfigurableMotor{})
	_ = resource.Reconfigurable(&reconfigurableMotor{})
)

// FromRobot is a helper for getting the named motor from the given Robot.
func FromRobot(r robot.Robot, name string) (Motor, error) {
	res, err := r.ResourceByName(Named(name))
	if err != nil {
		return nil, err
	}
	part, ok := res.(Motor)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("Motor", res)
	}
	return part, nil
}

// NamesFromRobot is a helper for getting all motor names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesBySubtype(r, Subtype)
}

// CreateStatus creates a status from the motor.
func CreateStatus(ctx context.Context, resource interface{}) (*pb.Status, error) {
	motor, ok := resource.(Motor)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("Motor", resource)
	}
	on, err := motor.IsPowered(ctx)
	if err != nil {
		return nil, err
	}
	features, err := motor.GetFeatures(ctx)
	if err != nil {
		return nil, err
	}
	var position float64
	if features[PositionReporting] {
		position, err = motor.GetPosition(ctx)
		if err != nil {
			return nil, err
		}
	}
	return &pb.Status{
		IsOn:              on,
		PositionReporting: features[PositionReporting],
		Position:          position,
	}, nil
}

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

func (r *reconfigurableMotor) GoFor(ctx context.Context, rpm float64, revolutions float64) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GoFor(ctx, rpm, revolutions)
}

func (r *reconfigurableMotor) GoTo(ctx context.Context, rpm float64, positionRevolutions float64) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GoTo(ctx, rpm, positionRevolutions)
}

func (r *reconfigurableMotor) ResetZeroPosition(ctx context.Context, offset float64) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.ResetZeroPosition(ctx, offset)
}

func (r *reconfigurableMotor) GetPosition(ctx context.Context) (float64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GetPosition(ctx)
}

func (r *reconfigurableMotor) GetFeatures(ctx context.Context) (map[Feature]bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GetFeatures(ctx)
}

func (r *reconfigurableMotor) Stop(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Stop(ctx)
}

func (r *reconfigurableMotor) IsPowered(ctx context.Context) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.IsPowered(ctx)
}

func (r *reconfigurableMotor) GoTillStop(
	ctx context.Context, rpm float64,
	stopFunc func(ctx context.Context) bool,
) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	localMotor, ok := r.actual.(LocalMotor)
	if !ok {
		return NewGoTillStopUnsupportedError("(name unavailable)")
	}
	return localMotor.GoTillStop(ctx, rpm, stopFunc)
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
		return utils.NewUnexpectedTypeError(r, newMotor)
	}
	if err := viamutils.TryClose(ctx, r.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
	return nil
}

// WrapWithReconfigurable converts a regular Motor implementation to a reconfigurableMotor.
// If motor is already a reconfigurableMotor, then nothing is done.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	motor, ok := r.(Motor)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("Motor", r)
	}
	if reconfigurable, ok := motor.(*reconfigurableMotor); ok {
		return reconfigurable, nil
	}
	return &reconfigurableMotor{actual: motor}, nil
}

// PinConfig defines the mapping of where motor are wired.
// Standard Configurations:
// - A/B       [EnablePinHigh/EnablePinLow]
// - A/B + PWM [EnablePinHigh/EnablePinLow]
// - Dir + PWM [EnablePinHigh/EnablePinLow].
type PinConfig struct {
	A             string `json:"a"`
	B             string `json:"b"`
	Direction     string `json:"dir"`
	PWM           string `json:"pwm"`
	EnablePinHigh string `json:"en_high,omitempty"`
	EnablePinLow  string `json:"en_low,omitempty"`
	Step          string `json:"step,omitempty"`
}

// Config describes the configuration of a motor.
type Config struct {
	Pins          PinConfig             `json:"pins"`
	BoardName     string                `json:"board"`                   // used to get encoders
	MinPowerPct   float64               `json:"min_power_pct,omitempty"` // min power percentage to allow for this motor default is 0.0
	MaxPowerPct   float64               `json:"max_power_pct,omitempty"` // max power percentage to allow for this motor (0.06 - 1.0)
	PWMFreq       uint                  `json:"pwm_freq,omitempty"`
	DirectionFlip bool                  `json:"dir_flip,omitempty"`       // Flip the direction of the signal sent if there is a Dir pin
	StepperDelay  uint                  `json:"stepper_delay,omitempty"`  // When using stepper motors, the time to remain high
	ControlLoop   control.ControlConfig `json:"control_config,omitempty"` // Optional control loop

	// Encoder Config
	EncoderBoard     string  `json:"encoder_board,omitempty"`    // name of the board where encoders are; default is same as 'board'
	EncoderA         string  `json:"encoder,omitempty"`          // name of the digital interrupt that is the encoder a
	EncoderB         string  `json:"encoder_b,omitempty"`        // name of the digital interrupt that is hall encoder b
	RampRate         float64 `json:"ramp_rate,omitempty"`        // how fast to ramp power to motor when using rpm control
	MaxRPM           float64 `json:"max_rpm"`                    // RPM
	MaxAcceleration  float64 `json:"max_acceleration,omitempty"` // RPM per second
	TicksPerRotation int     `json:"ticks_per_rotation"`
}

// RegisterConfigAttributeConverter registers a Config converter.
// Note(erd): This probably shouldn't exist since not all motors have the same config requirements.
func RegisterConfigAttributeConverter(model string) {
	config.RegisterComponentAttributeMapConverter(
		config.ComponentTypeMotor,
		model,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf Config
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&Config{})
}
