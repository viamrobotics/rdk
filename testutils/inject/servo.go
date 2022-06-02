package inject

import (
	"context"

	"go.viam.com/rdk/component/servo"
)

// Servo is an injected servo.
type Servo struct {
	servo.Servo
	DoFunc          func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	MoveFunc        func(ctx context.Context, angleDeg uint8) error
	GetPositionFunc func(ctx context.Context) (uint8, error)
	StopFunc        func(ctx context.Context) error
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
	if s.GetPositionFunc == nil {
		return s.Servo.GetPosition(ctx)
	}
	return s.GetPositionFunc(ctx)
}

// Stop calls the injected Stop or the real version.
func (s *Servo) Stop(ctx context.Context) error {
	if s.StopFunc == nil {
		return s.Servo.Stop(ctx)
	}
	return s.StopFunc(ctx)
}

// Do calls the injected Do or the real version.
func (s *Servo) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if s.DoFunc == nil {
		return s.Servo.Do(ctx, cmd)
	}
	return s.DoFunc(ctx, cmd)
}
