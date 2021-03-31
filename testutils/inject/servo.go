package inject

import (
	"go.viam.com/robotcore/board"
)

type Servo struct {
	board.Servo
	MoveFunc    func(angle uint8) error
	CurrentFunc func() uint8
}

func (s *Servo) Move(angle uint8) error {
	if s.MoveFunc == nil {
		return s.Servo.Move(angle)
	}
	return s.MoveFunc(angle)
}

func (s *Servo) Current() uint8 {
	if s.CurrentFunc == nil {
		return s.Servo.Current()
	}
	return s.CurrentFunc()
}
