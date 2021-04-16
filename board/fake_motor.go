package board

import (
	"context"

	pb "go.viam.com/robotcore/proto/api/v1"
)

type FakeMotor struct {
	power byte
	d     pb.DirectionRelative
}

func (m *FakeMotor) Position(ctx context.Context) (float64, error) {
	return 0, nil
}

func (m *FakeMotor) PositionSupported(ctx context.Context) (bool, error) {
	return false, nil
}

func (m *FakeMotor) Power(ctx context.Context, power byte) error {
	m.power = power
	return nil
}

func (m *FakeMotor) Go(ctx context.Context, d pb.DirectionRelative, power byte) error {
	m.d = d
	m.power = power
	return nil
}

func (m *FakeMotor) GoFor(ctx context.Context, d pb.DirectionRelative, rpm float64, revolutions float64) error {
	m.d = d
	m.power = 1
	return nil
}

func (m *FakeMotor) Off(ctx context.Context) error {
	m.d = pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED
	return nil
}

func (m *FakeMotor) IsOn(ctx context.Context) (bool, error) {
	return m.d != pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED && m.power > 0, nil
}
