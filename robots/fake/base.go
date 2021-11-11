package fake

import (
	"context"

	"github.com/edaniels/golog"

	"go.viam.com/core/base"
	"go.viam.com/core/config"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
)

func init() {
	registry.RegisterBase(ModelName, registry.Base{Constructor: func(ctx context.Context, r robot.Robot, c config.Component, logger golog.Logger) (base.Base, error) {
		return &Base{Name: c.Name}, nil
	}})
}

// Base is a fake base that returns what it was provided in each method.
type Base struct {
	Name       string
	CloseCount int
}

// MoveStraight returns that it moved the given distance.
func (b *Base) MoveStraight(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) (int, error) {
	return distanceMillis, nil
}

// MoveArc returns that it moved the given distance at a certain angle.
func (b *Base) MoveArc(ctx context.Context, distanceMillis int, millisPerSec float64, angleDeg float64, block bool) (int, error) {
	return distanceMillis, nil
}

// Spin returns that it spun the given angle.
func (b *Base) Spin(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) (float64, error) {
	return angleDeg, nil
}

// WidthMillis returns some arbitrary width.
func (b *Base) WidthMillis(ctx context.Context) (int, error) {
	return 600, nil
}

// Stop does nothing.
func (b *Base) Stop(ctx context.Context) error {
	return nil
}

// Close does nothing.
func (b *Base) Close() error {
	b.CloseCount++
	return nil
}
