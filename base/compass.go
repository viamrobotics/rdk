package base

import (
	"context"
	"math"
	"time"

	"github.com/edaniels/golog"

	"go.viam.com/core/sensor/compass"
	"go.viam.com/core/utils"
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
// no underlying base, the argument iteslf is returned.
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
func (wc baseWithCompass) Spin(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) (float64, error) {
	rel, _ := wc.compass.(compass.RelativeCompass)
	if rel != nil {
		if err := rel.Mark(ctx); err != nil {
			return math.NaN(), err
		}
	}
	origAngleDeg := angleDeg
	// track the total amount spun in case we fail along the way
	var totalSpin float64
	for {
		startHeading, err := compass.MedianHeading(ctx, wc.compass)
		if err != nil {
			return totalSpin, err
		}
		wc.logger.Debugf("start heading %f", startHeading)
		spun, err := wc.Base.Spin(ctx, angleDeg, degsPerSec, block)
		totalSpin += spun
		if err != nil {
			return totalSpin, err
		}
		if !utils.SelectContextOrWait(ctx, time.Second) {
			return totalSpin, ctx.Err()
		}
		endHeading, err := compass.MedianHeading(ctx, wc.compass)
		if err != nil {
			return totalSpin, err
		}
		wc.logger.Debugf("end heading %f", endHeading)
		actual := utils.AngleDiffDeg(startHeading, endHeading)
		offBy := math.Abs(math.Abs(angleDeg) - actual)
		wc.logger.Debugf("off by %f", offBy)
		if offBy < 1 {
			return origAngleDeg, nil
		}
		if actual > angleDeg {
			offBy *= -1
		}
		wc.logger.Debugf("next %f", offBy)
		angleDeg = offBy
	}
}
