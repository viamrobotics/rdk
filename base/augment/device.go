package augment

import (
	"context"
	"math"
	"time"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/sensor/compass"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
)

func Device(device api.Base, with interface{}) api.Base {
	switch v := with.(type) {
	case compass.Device:
		return baseDeviceWithCompass{device, v}
	}
	return device
}

func ReduceDevice(device api.Base) api.Base {
	switch v := device.(type) {
	case baseDeviceWithCompass:
		return v.Base
	}
	return device
}

type baseDeviceWithCompass struct {
	api.Base
	compass compass.Device
}

func (wc baseDeviceWithCompass) Spin(ctx context.Context, angleDeg float64, speed int, block bool) error {
	rel, _ := wc.compass.(compass.RelativeDevice)
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
		golog.Global.Debugf("start heading %f", startHeading)
		if err := wc.Base.Spin(ctx, angleDeg, speed, block); err != nil {
			return err
		}
		time.Sleep(1 * time.Second)
		endHeading, err := compass.MedianHeading(ctx, wc.compass)
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
