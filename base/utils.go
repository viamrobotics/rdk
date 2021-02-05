package base

import (
	"math"

	"github.com/viamrobotics/robotcore/sensor/compass"
)

type Move struct {
	DistanceMM int
	AngleDeg   int
	Speed      int
	Block      bool
}

func DoMove(move Move, device Device) (int, int, error) {
	if move.AngleDeg != 0 {
		if err := device.Spin(move.AngleDeg, move.Speed, move.Block); err != nil {
			// TODO(erd): Spin should report amount spun if errored
			return 0, 0, err
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

type deviceWithCompass struct {
	Device
	compass compass.Device
}

func angleDiff(a1, a2 float64) float64 {
	return float64(180) - math.Abs(math.Abs(a1-a2)-float64(180))
}

func avgHeading(device compass.Device) (float64, error) {
	numReadings := 10
	sum := 0.0
	for i := 0; i < numReadings; i++ {
		heading, err := device.Heading()
		if err != nil {
			return 0, err
		}
		sum += heading
	}
	return sum / float64(numReadings), nil
}

func (dwc deviceWithCompass) Spin(degrees int, power int, block bool) error {
	for {
		startHeading, err := avgHeading(dwc.compass)
		if err != nil {
			return err
		}
		if err := dwc.Device.Spin(degrees, power, block); err != nil {
			return err
		}
		endHeading, err := avgHeading(dwc.compass)
		if err != nil {
			return err
		}
		actual := angleDiff(startHeading, endHeading)
		offBy := math.Abs(math.Abs(float64(degrees)) - actual)
		if offBy < 1 {
			return nil
		}
		if actual > float64(degrees) {
			offBy *= -1
		}
		degrees = int(offBy)
	}
}
