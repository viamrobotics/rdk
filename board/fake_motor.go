package board

import (
	"fmt"
)

type FakeMotor struct {
	force byte
	d     Direction
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

func (m *FakeMotor) Go(d Direction, force byte) error {
	m.d = d
	m.force = force
	return nil
}

func (m *FakeMotor) GoFor(d Direction, rpm float64, rotations float64) error {
	return fmt.Errorf("should not be called")
}

func (m *FakeMotor) Off() error {
	m.d = DirNone
	return nil
}

func (m *FakeMotor) IsOn() bool {
	return m.d != DirNone && m.force > 0
}
