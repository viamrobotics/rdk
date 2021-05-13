package fake

import (
	"context"

	"github.com/edaniels/golog"

	"go.viam.com/robotcore/base"
	"go.viam.com/robotcore/config"
	"go.viam.com/robotcore/registry"
	"go.viam.com/robotcore/robot"
)

func init() {
	registry.RegisterBase(ModelName, func(ctx context.Context, r robot.Robot, c config.Component, logger golog.Logger) (base.Base, error) {
		return &Base{Name: c.Name}, nil
	})
}

// tracks in CM
type Base struct {
	Name       string
	CloseCount int
}

func (b *Base) MoveStraight(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) (int, error) {
	return distanceMillis, nil
}

func (b *Base) Spin(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) (float64, error) {
	return angleDeg, nil
}

func (b *Base) WidthMillis(ctx context.Context) (int, error) {
	return 600, nil
}

func (b *Base) Stop(ctx context.Context) error {
	return nil
}

func (b *Base) Close() error {
	b.CloseCount++
	return nil
}
