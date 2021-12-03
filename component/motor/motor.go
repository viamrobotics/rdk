package motor

import (
	"context"
	"sync"

	"github.com/go-errors/errors"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/resource"
	"go.viam.com/core/rlog"
	viamutils "go.viam.com/utils"
)

// SubtypeName is a constant that identifies the component resource subtype string "motor"
const SubtypeName = resource.SubtypeName("motor")

// Subtype is a constant that identifies the component resource subtype
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceCore,
	resource.ResourceTypeComponent,
	SubtypeName,
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

	//PID returns underlying PID for the motor
	PID() PID
}

var (
	_ = Motor(&reconfigurableMotor{})
	_ = resource.Reconfigurable(&reconfigurableMotor{})
)

type reconfigurableMotor struct {
	mu     sync.RWMutex
	actual Motor
}

func (_motor *reconfigurableMotor) ProxyFor() interface{} {
	_motor.mu.RLock()
	defer _motor.mu.RUnlock()
	return _motor.actual
}

func (_motor *reconfigurableMotor) Power(ctx context.Context, powerPct float32) error {
	_motor.mu.RLock()
	defer _motor.mu.RUnlock()
	return _motor.actual.Power(ctx, powerPct)
}

func (_motor *reconfigurableMotor) Go(ctx context.Context, d pb.DirectionRelative, powerPct float32) error {
	_motor.mu.RLock()
	defer _motor.mu.RUnlock()
	return _motor.actual.Go(ctx, d, powerPct)
}

func (_motor *reconfigurableMotor) GoFor(ctx context.Context, d pb.DirectionRelative, rpm float64, revolutions float64) error {
	_motor.mu.RLock()
	defer _motor.mu.RUnlock()
	return _motor.actual.GoFor(ctx, d, rpm, revolutions)
}

func (_motor *reconfigurableMotor) GoTo(ctx context.Context, rpm float64, position float64) error {
	_motor.mu.RLock()
	defer _motor.mu.RUnlock()
	return _motor.actual.GoTo(ctx, rpm, position)
}

func (_motor *reconfigurableMotor) GoTillStop(ctx context.Context, d pb.DirectionRelative, rpm float64, stopFunc func(ctx context.Context) bool) error {
	_motor.mu.RLock()
	defer _motor.mu.RUnlock()
	return _motor.actual.GoTillStop(ctx, d, rpm, stopFunc)
}

func (_motor *reconfigurableMotor) Zero(ctx context.Context, offset float64) error {
	_motor.mu.RLock()
	defer _motor.mu.RUnlock()
	return _motor.actual.Zero(ctx, offset)
}

func (_motor *reconfigurableMotor) Position(ctx context.Context) (float64, error) {
	_motor.mu.RLock()
	defer _motor.mu.RUnlock()
	return _motor.actual.Position(ctx)
}

func (_motor *reconfigurableMotor) PositionSupported(ctx context.Context) (bool, error) {
	_motor.mu.RLock()
	defer _motor.mu.RUnlock()
	return _motor.actual.PositionSupported(ctx)
}

func (_motor *reconfigurableMotor) Off(ctx context.Context) error {
	_motor.mu.RLock()
	defer _motor.mu.RUnlock()
	return _motor.actual.Off(ctx)
}

func (_motor *reconfigurableMotor) IsOn(ctx context.Context) (bool, error) {
	_motor.mu.RLock()
	defer _motor.mu.RUnlock()
	return _motor.actual.IsOn(ctx)
}

func (_motor *reconfigurableMotor) PID() PID {
	_motor.mu.RLock()
	defer _motor.mu.RUnlock()
	return _motor.actual.PID()
}

func (r *reconfigurableMotor) Close() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return viamutils.TryClose(r.actual)
}

func (r *reconfigurableMotor) Reconfigure(newMotor resource.Reconfigurable) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	actual, ok := newMotor.(*reconfigurableMotor)
	if !ok {
		return errors.Errorf("expected new arm to be %T but got %T", r, newMotor)
	}
	if err := viamutils.TryClose(r.actual); err != nil {
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
