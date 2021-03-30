package board

import pb "go.viam.com/robotcore/proto/api/v1"

type FakeMotor struct {
	force byte
	d     pb.DirectionRelative
}

func (m *FakeMotor) Position() int64 {
	return 0
}

func (m *FakeMotor) PositionSupported() bool {
	return false
}

func (m *FakeMotor) Force(force byte) error {
	m.force = force
	return nil
}

func (m *FakeMotor) Go(d pb.DirectionRelative, force byte) error {
	m.d = d
	m.force = force
	return nil
}

func (m *FakeMotor) GoFor(d pb.DirectionRelative, rpm float64, rotations float64) error {
	m.d = d
	m.force = 1
	return nil
}

func (m *FakeMotor) Off() error {
	m.d = pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED
	return nil
}

func (m *FakeMotor) IsOn() bool {
	return m.d != pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED && m.force > 0
}
