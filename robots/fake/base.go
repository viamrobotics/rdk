package fake

import (
	"context"
	"fmt"

	"github.com/edaniels/golog"

	"go.viam.com/core/base"
	"go.viam.com/core/config"
	"go.viam.com/core/registry"
	"go.viam.com/core/rlog"
	"go.viam.com/core/robot"
)

func init() {
	registry.RegisterBase(ModelName, func(ctx context.Context, r robot.Robot, c config.Component, logger golog.Logger) (base.Base, error) {
		return &Base{Name: c.Name}, nil
	})
}

// Base is a fake base that returns what it was provided in each method.
type Base struct {
	Name             string
	CloseCount       int
	ReconfigureCount int
}

// MoveStraight returns that it moved the given distance.
func (b *Base) MoveStraight(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) (int, error) {
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

// Reconfigure replaces this base with the given base.
func (b *Base) Reconfigure(newBase base.Base) {
	actual, ok := newBase.(*Base)
	if !ok {
		panic(fmt.Errorf("expected new base to be %T but got %T", actual, newBase))
	}
	if err := b.Close(); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	oldCloseCount := b.CloseCount
	oldReconfigureCount := b.ReconfigureCount + 1
	*b = *actual
	b.CloseCount += oldCloseCount
	b.ReconfigureCount += oldReconfigureCount
}

// Close does nothing.
func (b *Base) Close() error {
	b.CloseCount++
	return nil
}
