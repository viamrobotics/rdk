package inject

import (
	"go.viam.com/robotcore/board"
	pb "go.viam.com/robotcore/proto/api/v1"
)

type Motor struct {
	board.Motor
	ForceFunc             func(force byte) error
	GoFunc                func(d pb.DirectionRelative, force byte) error
	GoForFunc             func(d pb.DirectionRelative, rpm float64, rotations float64) error
	PositionFunc          func() int64
	PositionSupportedFunc func() bool
	OffFunc               func() error
	IsOnFunc              func() bool
}

func (m *Motor) Force(force byte) error {
	if m.ForceFunc == nil {
		return m.Motor.Force(force)
	}
	return m.ForceFunc(force)
}

func (m *Motor) Go(d pb.DirectionRelative, force byte) error {
	if m.GoFunc == nil {
		return m.Motor.Go(d, force)
	}
	return m.GoFunc(d, force)
}

func (m *Motor) GoFor(d pb.DirectionRelative, rpm float64, rotations float64) error {
	if m.GoForFunc == nil {
		return m.Motor.GoFor(d, rpm, rotations)
	}
	return m.GoForFunc(d, rpm, rotations)
}

func (m *Motor) Position() int64 {
	if m.PositionFunc == nil {
		return m.Motor.Position()
	}
	return m.PositionFunc()
}

func (m *Motor) PositionSupported() bool {
	if m.PositionSupportedFunc == nil {
		return m.Motor.PositionSupported()
	}
	return m.PositionSupportedFunc()
}

func (m *Motor) Off() error {
	if m.OffFunc == nil {
		return m.Motor.Off()
	}
	return m.OffFunc()
}

func (m *Motor) IsOn() bool {
	if m.IsOnFunc == nil {
		return m.Motor.IsOn()
	}
	return m.IsOnFunc()
}
