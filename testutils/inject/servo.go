package inject

import (
	"context"

	"go.viam.com/rdk/component/servo"
)

// Servo is an injected servo.
type Servo struct {
	servo.Servo
	DoFunc      func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	MoveFunc    func(ctx context.Context, angleDeg uint8) error
	CurrentFunc func(ctx context.Context) (uint8, error)
}

// Move calls the injected Move or the real version.
func (s *Servo) Move(ctx context.Context, angleDeg uint8) error {
	if s.MoveFunc == nil {
		return s.Servo.Move(ctx, angleDeg)
	}
	return s.MoveFunc(ctx, angleDeg)
}

// GetPosition calls the injected Current or the real version.
func (s *Servo) GetPosition(ctx context.Context) (uint8, error) {
	if s.CurrentFunc == nil {
		return s.Servo.GetPosition(ctx)
	}
	return s.CurrentFunc(ctx)
}

// Do calls the injected Do or the real version.
func (s *Servo) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if s.DoFunc == nil {
		return s.Servo.Do(ctx, cmd)
	}
	return s.DoFunc(ctx, cmd)
}
