package inject

import (
	"context"

	"go.viam.com/rdk/component/servo"
)

// Servo is an injected servo.
type Servo struct {
	servo.LocalServo
	DoFunc          func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	MoveFunc        func(ctx context.Context, angleDeg uint8) error
	GetPositionFunc func(ctx context.Context) (uint8, error)
	StopFunc        func(ctx context.Context) error
	IsMovingFunc    func(context.Context) (bool, error)
}

// Move calls the injected Move or the real version.
func (s *Servo) Move(ctx context.Context, angleDeg uint8) error {
	if s.MoveFunc == nil {
		return s.LocalServo.Move(ctx, angleDeg)
	}
	return s.MoveFunc(ctx, angleDeg)
}

// GetPosition calls the injected Current or the real version.
func (s *Servo) GetPosition(ctx context.Context) (uint8, error) {
	if s.GetPositionFunc == nil {
		return s.LocalServo.GetPosition(ctx)
	}
	return s.GetPositionFunc(ctx)
}

// Stop calls the injected Stop or the real version.
func (s *Servo) Stop(ctx context.Context) error {
	if s.StopFunc == nil {
		return s.LocalServo.Stop(ctx)
	}
	return s.StopFunc(ctx)
}

// Do calls the injected Do or the real version.
func (s *Servo) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if s.DoFunc == nil {
		return s.LocalServo.Do(ctx, cmd)
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
