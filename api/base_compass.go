package api

import (
	"context"
	"math"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/robotcore/sensor/compass"
	"go.viam.com/robotcore/utils"
)

func BaseWithCompass(device Base, cmp compass.Device, logger golog.Logger) Base {
	if cmp == nil {
		return device
	}
	return baseDeviceWithCompass{device, cmp, logger}
}

func ReduceBase(b Base) Base {
	x, ok := b.(baseDeviceWithCompass)
	if ok {
		return x.Base
	}
	return b
}

type baseDeviceWithCompass struct {
	Base
	compass compass.Device
	logger  golog.Logger
}

func (wc baseDeviceWithCompass) Spin(ctx context.Context, angleDeg float64, speed int, block bool) (float64, error) {
	rel, _ := wc.compass.(compass.RelativeDevice)
	if rel != nil {
		if err := rel.Mark(ctx); err != nil {
			return math.NaN(), err
		}
	}
	origAngleDeg := angleDeg
	var totalSpin float64
	for {
		startHeading, err := compass.MedianHeading(ctx, wc.compass)
		if err != nil {
			return totalSpin, err
		}
		wc.logger.Debugf("start heading %f", startHeading)
		spun, err := wc.Base.Spin(ctx, angleDeg, speed, block)
		totalSpin += spun
		if err != nil {
			return totalSpin, err
		}
		time.Sleep(1 * time.Second)
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
