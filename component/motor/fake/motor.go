package fake

import (
	"context"
	"math"
	"sync"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"

	"go.viam.com/core/component/motor"
	"go.viam.com/core/config"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
)

func init() {
	_motor := registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			mcfg := config.ConvertedAttributes.(*motor.Config)
			if mcfg.PID != nil {
				pid, err := motor.CreatePID(mcfg.PID)
				if err != nil {
					return nil, err
				}
				return &Motor{Name: config.Name, pid: pid}, nil
			}
			return &Motor{Name: config.Name, pid: nil}, nil
		},
	}
	registry.RegisterComponent(motor.Subtype, "fake", _motor)

	motor.RegisterConfigAttributeConverter("fake")
}

// A Motor allows setting and reading a set power percentage and
// direction.
type Motor struct {
	Name     string
	mu       sync.Mutex
	powerPct float64
	pid      motor.PID
}

// PID Return the underlying PID
func (m *Motor) PID() motor.PID {
	return m.pid
}

// Position always returns 0.
func (m *Motor) Position(ctx context.Context) (float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return 0, nil
}

// PositionSupported returns false.
func (m *Motor) PositionSupported(ctx context.Context) (bool, error) {
	return false, nil
}

// SetPower sets the given power percentage.
func (m *Motor) SetPower(ctx context.Context, powerPct float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setPowerPct(powerPct)
	return nil
}

func (m *Motor) setPowerPct(powerPct float64) {
	m.powerPct = powerPct
}

// PowerPct returns the set power percentage.
func (m *Motor) PowerPct() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.powerPct
}

// Direction returns the set direction.
func (m *Motor) Direction() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.powerPct > 0 {
		return 1
	}
	if m.powerPct < 0 {
		return -1
	}
	return 0
}

// Go sets the given direction and power.
func (m *Motor) Go(ctx context.Context, powerPct float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.setPowerPct(powerPct)
	return nil
}

// GoFor sets the given direction and an arbitrary power percentage.
func (m *Motor) GoFor(ctx context.Context, rpm float64, revolutions float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Backward
	if math.Signbit(rpm) == math.Signbit(revolutions) {
		m.setPowerPct(0.01)
	} else {
		m.setPowerPct(-0.01)
	}
	return nil
}

// GoTo sets the given direction and an arbitrary power percentage for now.
func (m *Motor) GoTo(ctx context.Context, rpm float64, position float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setPowerPct(.01)
	return nil
}

// GoTillStop always returns an error
func (m *Motor) GoTillStop(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error {
	return errors.New("unsupported")
}

// SetToZeroPosition always returns an error
func (m *Motor) SetToZeroPosition(ctx context.Context, offset float64) error {
	return errors.New("unsupported")
}

// Off has the motor pretend to be off.
func (m *Motor) Off(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setPowerPct(0.0)
	return nil
}

// IsOn returns if the motor is pretending to be on or not.
func (m *Motor) IsOn(ctx context.Context) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return math.Abs(m.powerPct) >= 0.005, nil
}
