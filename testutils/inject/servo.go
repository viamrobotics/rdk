package inject

import (
	"context"

	"go.viam.com/rdk/components/servo"
)

// Servo is an injected servo.
type Servo struct {
	servo.LocalServo
	DoFunc       func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	MoveFunc     func(ctx context.Context, angleDeg uint32, extra map[string]interface{}) error
	PositionFunc func(ctx context.Context, extra map[string]interface{}) (uint32, error)
	StopFunc     func(ctx context.Context, extra map[string]interface{}) error
	IsMovingFunc func(context.Context) (bool, error)
}

// Move calls the injected Move or the real version.
func (s *Servo) Move(ctx context.Context, angleDeg uint32, extra map[string]interface{}) error {
	if s.MoveFunc == nil {
		return s.LocalServo.Move(ctx, angleDeg, extra)
	}
	return s.MoveFunc(ctx, angleDeg, extra)
}

// Position calls the injected Current or the real version.
func (s *Servo) Position(ctx context.Context, extra map[string]interface{}) (uint32, error) {
	if s.PositionFunc == nil {
		return s.LocalServo.Position(ctx, extra)
	}
	return s.PositionFunc(ctx, extra)
}

// Stop calls the injected Stop or the real version.
func (s *Servo) Stop(ctx context.Context, extra map[string]interface{}) error {
	if s.StopFunc == nil {
		return s.LocalServo.Stop(ctx, extra)
	}
	return s.StopFunc(ctx, extra)
}

// DoCommand calls the injected DoCommand or the real version.
func (s *Servo) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if s.DoFunc == nil {
		return s.LocalServo.DoCommand(ctx, cmd)
	}
	return s.DoFunc(ctx, cmd)
}

// IsMoving calls the injected IsMoving or the real version.
func (s *Servo) IsMoving(ctx context.Context) (bool, error) {
	if s.IsMovingFunc == nil {
		return s.LocalServo.IsMoving(ctx)
	}
	return s.IsMovingFunc(ctx)
}
