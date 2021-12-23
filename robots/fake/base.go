package fake

import (
	"context"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/base"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

func init() {
	registry.RegisterBase(
		ModelName,
		registry.Base{Constructor: func(
			ctx context.Context,
			r robot.Robot,
			c config.Component,
			logger golog.Logger,
		) (base.Base, error) {
			return &Base{Name: c.Name}, nil
		}})
}

// Base is a fake base that returns what it was provided in each method.
type Base struct {
	Name       string
	CloseCount int
}

// MoveStraight does nothing
func (b *Base) MoveStraight(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) error {
	return nil
}

// MoveArc does nothing
func (b *Base) MoveArc(ctx context.Context, distanceMillis int, millisPerSec float64, angleDeg float64, block bool) error {
	return nil
}

// Spin does nothing
func (b *Base) Spin(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) error {
	return nil
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
