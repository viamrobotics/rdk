package compass

import (
	"context"
	"math"
	"time"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
)

func BaseWithCompass(device api.Base, cmp Device) api.Base {
	if cmp == nil {
		return device
	}
	return baseDeviceWithCompass{device, cmp}
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
		golog.Global.Debugf("start heading %f", startHeading)
		if err := wc.Base.Spin(ctx, angleDeg, speed, block); err != nil {
			return err
		}
		time.Sleep(1 * time.Second)
		endHeading, err := MedianHeading(ctx, wc.compass)
		if err != nil {
			return err
		}
		golog.Global.Debugf("end heading %f", endHeading)
		actual := utils.AngleDiffDeg(startHeading, endHeading)
		offBy := math.Abs(math.Abs(angleDeg) - actual)
		golog.Global.Debugf("off by %f", offBy)
		if offBy < 1 {
			return nil
		}
		if actual > angleDeg {
			offBy *= -1
		}
		golog.Global.Debugf("next %f", offBy)
		angleDeg = offBy
	}
}
