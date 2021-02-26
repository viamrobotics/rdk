package base

import (
	"context"
	"math"
	"time"

	"go.viam.com/robotcore/sensor/compass"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
)

type Move struct {
	DistanceMM int
	AngleDeg   float64
	Speed      float64
	Block      bool
}

func DoMove(move Move, device Device) (float64, int, error) {
	if move.AngleDeg != 0 {
		// TODO(erh): speed is wrong
		if err := device.Spin(move.AngleDeg, int(move.Speed), move.Block); err != nil {
			// TODO(erd): Spin should report amount spun if errored
			return math.NaN(), 0, err
		}
	}

	if move.DistanceMM != 0 {
		if err := device.MoveStraight(move.DistanceMM, move.Speed, move.Block); err != nil {
			// TODO(erd): MoveStraight should report amount moved if errored
			return move.AngleDeg, 0, err
		}
	}

	return move.AngleDeg, move.DistanceMM, nil
}

// TODO(erd): probably belongs in some other util area
// that won't cause inter-device package cycles...
func Augment(device Device, with interface{}) Device {
	switch v := with.(type) {
	case compass.Device:
		return deviceWithCompass{device, v}
	}
	return device
}

func Reduce(device Device) Device {
	switch v := device.(type) {
	case deviceWithCompass:
		return v.Device
	}
	return device
}

type deviceWithCompass struct {
	Device
	compass compass.Device
}

func (dwc deviceWithCompass) Spin(angleDeg float64, speed int, block bool) error {
	rel, _ := dwc.compass.(compass.RelativeDevice)
	if rel != nil {
		if err := rel.Mark(); err != nil {
			return err
		}
	}
	for {
		startHeading, err := compass.MedianHeading(context.TODO(), dwc.compass)
		if err != nil {
			return err
		}
		golog.Global.Debugf("start heading %f", startHeading)
		if err := dwc.Device.Spin(angleDeg, speed, block); err != nil {
			return err
		}
		time.Sleep(1 * time.Second)
		endHeading, err := compass.MedianHeading(context.TODO(), dwc.compass)
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
