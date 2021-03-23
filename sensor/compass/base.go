package compass

import (
	"context"
	"math"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/utils"
)

func BaseWithCompass(device api.Base, cmp Device, logger golog.Logger) api.Base {
	if cmp == nil {
		return device
	}
	return baseDeviceWithCompass{device, cmp, logger}
}

func ReduceBase(b api.Base) api.Base {
	x, ok := b.(baseDeviceWithCompass)
	if ok {
		return x.Base
	}
	return b
}

type baseDeviceWithCompass struct {
	api.Base
	compass Device
	logger  golog.Logger
}

func (wc baseDeviceWithCompass) Spin(ctx context.Context, angleDeg float64, speed int, block bool) error {
	rel, _ := wc.compass.(RelativeDevice)
	if rel != nil {
		if err := rel.Mark(ctx); err != nil {
			return err
		}
	}
	for {
		startHeading, err := MedianHeading(ctx, wc.compass)
		if err != nil {
			return err
		}
		wc.logger.Debugf("start heading %f", startHeading)
		if err := wc.Base.Spin(ctx, angleDeg, speed, block); err != nil {
			return err
		}
		time.Sleep(1 * time.Second)
		endHeading, err := MedianHeading(ctx, wc.compass)
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
