package inject

import (
	"context"

	"go.viam.com/robotcore/board"
)

type Servo struct {
	board.Servo
	MoveFunc    func(ctx context.Context, angle uint8) error
	CurrentFunc func(ctx context.Context) (uint8, error)
}

func (s *Servo) Move(ctx context.Context, angle uint8) error {
	if s.MoveFunc == nil {
		return s.Servo.Move(ctx, angle)
	}
	return s.MoveFunc(ctx, angle)
}

func (s *Servo) Current(ctx context.Context) (uint8, error) {
	if s.CurrentFunc == nil {
		return s.Servo.Current(ctx)
	}
	return s.CurrentFunc(ctx)
}
