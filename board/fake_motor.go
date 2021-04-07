package board

import (
	"context"

	pb "go.viam.com/robotcore/proto/api/v1"
)

type FakeMotor struct {
	force byte
	d     pb.DirectionRelative
}

func (m *FakeMotor) Position(ctx context.Context) (float64, error) {
	return 0, nil
}

func (m *FakeMotor) PositionSupported(ctx context.Context) (bool, error) {
	return false, nil
}

func (m *FakeMotor) Force(ctx context.Context, force byte) error {
	m.force = force
	return nil
}

func (m *FakeMotor) Go(ctx context.Context, d pb.DirectionRelative, force byte) error {
	m.d = d
	m.force = force
	return nil
}

func (m *FakeMotor) GoFor(ctx context.Context, d pb.DirectionRelative, rpm float64, rotations float64) error {
	m.d = d
	m.force = 1
	return nil
}

func (m *FakeMotor) Off(ctx context.Context) error {
	m.d = pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED
	return nil
}

func (m *FakeMotor) IsOn(ctx context.Context) (bool, error) {
	return m.d != pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED && m.force > 0, nil
}
