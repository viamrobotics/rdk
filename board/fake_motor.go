package board

import (
	"context"
	"sync"

	"github.com/go-errors/errors"

	pb "go.viam.com/core/proto/api/v1"
)

// A FakeMotor allows setting and reading a set power percentage and
// direction.
type FakeMotor struct {
	mu       *sync.Mutex
	powerPct float32
	d        pb.DirectionRelative
}

// Position always returns 0.
func (m *FakeMotor) Position(ctx context.Context) (float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return 0, nil
}

// PositionSupported returns false.
func (m *FakeMotor) PositionSupported(ctx context.Context) (bool, error) {
	return false, nil
}

// Power sets the given power percentage.
func (m *FakeMotor) Power(ctx context.Context, powerPct float32) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setPowerPct(powerPct)
	return nil
}

func (m *FakeMotor) setPowerPct(powerPct float32) {
	m.powerPct = powerPct
}

// PowerPct returns the set power percentage.
func (m *FakeMotor) PowerPct() float32 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.powerPct
}

// Direction returns the set direction.
func (m *FakeMotor) Direction() pb.DirectionRelative {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.d
}

// Go sets the given direction and power.
func (m *FakeMotor) Go(ctx context.Context, d pb.DirectionRelative, powerPct float32) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.d = d
	m.setPowerPct(powerPct)
	return nil
}

// GoFor sets the given direction and an arbitrary power percentage.
func (m *FakeMotor) GoFor(ctx context.Context, d pb.DirectionRelative, rpm float64, revolutions float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.d = d
	m.setPowerPct(.01)
	return nil
}

// GoTo sets the given direction and an arbitrary power percentage for now.
func (m *FakeMotor) GoTo(ctx context.Context, rpm float64, position float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.d = pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD
	m.setPowerPct(.01)
	return nil
}

// GoTillStop always returns an error
func (m *FakeMotor) GoTillStop(ctx context.Context, d pb.DirectionRelative, rpm float64) error {
	return errors.New("unsupported")
}

// Zero always returns an error
func (m *FakeMotor) Zero(ctx context.Context, offset float64) error {
	return errors.New("unsupported")
}

// Off has the motor pretend to be off.
func (m *FakeMotor) Off(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.d = pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED
	return nil
}

// IsOn returns if the motor is pretending to be on or not.
func (m *FakeMotor) IsOn(ctx context.Context) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.d != pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED && m.powerPct > 0, nil
}
