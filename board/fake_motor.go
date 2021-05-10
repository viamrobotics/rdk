package board

import (
	"context"
	"sync"

	pb "go.viam.com/robotcore/proto/api/v1"
)

// A FakeMotor allows setting and reading a set power percentage and
// direction.
type FakeMotor struct {
	mu       sync.Mutex
	powerPct float32
	d        pb.DirectionRelative
}

func (m *FakeMotor) Position(ctx context.Context) (float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return 0, nil
}

func (m *FakeMotor) PositionSupported(ctx context.Context) (bool, error) {
	return false, nil
}

func (m *FakeMotor) Power(ctx context.Context, powerPct float32) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setPowerPct(powerPct)
	return nil
}

func (m *FakeMotor) setPowerPct(powerPct float32) {
	m.powerPct = powerPct
}

func (m *FakeMotor) PowerPct() float32 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.powerPct
}

func (m *FakeMotor) Direction() pb.DirectionRelative {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.d
}

func (m *FakeMotor) Go(ctx context.Context, d pb.DirectionRelative, powerPct float32) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.d = d
	m.setPowerPct(powerPct)
	return nil
}

func (m *FakeMotor) GoFor(ctx context.Context, d pb.DirectionRelative, rpm float64, revolutions float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.d = d
	m.setPowerPct(.01)
	return nil
}

func (m *FakeMotor) Off(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.d = pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED
	return nil
}

func (m *FakeMotor) IsOn(ctx context.Context) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.d != pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED && m.powerPct > 0, nil
}
