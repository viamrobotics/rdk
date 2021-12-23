package base

import (
	"context"
	"math"
	"time"

	"github.com/edaniels/golog"

	goutils "go.viam.com/utils"

	"go.viam.com/rdk/sensor/compass"
	"go.viam.com/rdk/utils"
)

// AugmentWithCompass augments the given base with the given compass in order
// to correct its movements.
func AugmentWithCompass(base Base, cmp compass.Compass, logger golog.Logger) Base {
	if cmp == nil {
		return base
	}
	return baseWithCompass{base, cmp, logger}
}

// Reduce extracts the underlying base from the given base. If there is
// no underlying base, the argument itself is returned.
func Reduce(b Base) Base {
	x, ok := b.(baseWithCompass)
	if ok {
		return x.Base
	}
	return b
}

// baseWithCompass is an augmented base that has its movements corrected by
// a compass.
type baseWithCompass struct {
	Base
	compass compass.Compass
	logger  golog.Logger
}

// Spin attempts to perform an accurate spin by utilizing the underlying compass. In short,
// the base makes small spins until it gets very close to the target angle.
func (wc baseWithCompass) Spin(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) error {
	rel, _ := wc.compass.(compass.RelativeCompass)
	if rel != nil {
		if err := rel.Mark(ctx); err != nil {
			return err
		}
	}

	for {
		startHeading, err := compass.MedianHeading(ctx, wc.compass)
		if err != nil {
			return err
		}
		wc.logger.Debugf("start heading %f", startHeading)
		err = wc.Base.Spin(ctx, angleDeg, degsPerSec, block)
		if err != nil {
			return err
		}
		if !goutils.SelectContextOrWait(ctx, time.Second) {
			return ctx.Err()
		}
		endHeading, err := compass.MedianHeading(ctx, wc.compass)
		if err != nil {
			return err
		}
		wc.logger.Debugf("end heading %f", endHeading)
		actual := utils.AngleDiffDeg(startHeading, endHeading)
		offBy := math.Abs(math.Abs(angleDeg) - actual)
		wc.logger.Debugf("off by %f", offBy)
		if offBy < 1 {
			return nil
		}
		if actual > angleDeg {
			offBy *= -1
		}
		wc.logger.Debugf("next %f", offBy)
		angleDeg = offBy
	}
}
