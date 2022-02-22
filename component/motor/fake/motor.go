// Package fake implements a fake motor.
package fake

import (
	"context"
	"math"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

func init() {
	_motor := registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			return &Motor{Name: config.Name}, nil
		},
	}
	registry.RegisterComponent(motor.Subtype, "fake", _motor)

	motor.RegisterConfigAttributeConverter("fake")
}

// A Motor allows setting and reading a set power percentage and
// direction.
type Motor struct {
	Name              string
	mu                sync.Mutex
	powerPct          float64
	DefaultPosition   float64
	PositionSupported bool
}

// GetPosition always returns 0.
func (m *Motor) GetPosition(ctx context.Context) (float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.DefaultPosition, nil
}

// GetFeatures returns the status of whether the motor supports certain optional features.
func (m *Motor) GetFeatures(ctx context.Context) (map[motor.Feature]bool, error) {
	return map[motor.Feature]bool{
		motor.PositionReporting: m.PositionSupported,
	}, nil
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
func (m *Motor) GoTo(ctx context.Context, rpm float64, positionRevolutions float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setPowerPct(.01)
	return nil
}

// GoTillStop always returns an error.
func (m *Motor) GoTillStop(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error {
	return errors.New("unsupported")
}

// ResetZeroPosition always returns an error.
func (m *Motor) ResetZeroPosition(ctx context.Context, offset float64) error {
	return motor.NewResetZeroPositionUnsupportedError(m.Name)
}

// Stop has the motor pretend to be off.
func (m *Motor) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setPowerPct(0.0)
	return nil
}

// IsPowered returns if the motor is pretending to be on or not.
func (m *Motor) IsPowered(ctx context.Context) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return math.Abs(m.powerPct) >= 0.005, nil
}
