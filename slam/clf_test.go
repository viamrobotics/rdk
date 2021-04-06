package slam

import (
	"fmt"
	"os"
	"testing"

	"github.com/edaniels/golog"

	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/testutils"
	"go.viam.com/robotcore/utils"
)

// http://ais.informatik.uni-freiburg.de/slamevaluation/index.php

func TestAcesCLF(t *testing.T) {
	fn := testutils.LargeFileTestPath("slam/aces.clf")
	f, err := os.Open(fn)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	unitsPerMeter := 20

	area, err := NewSquareArea(200, unitsPerMeter, golog.Global)
	if err != nil {
		t.Fatal(err)
	}

	clf := utils.NewCLFReader(f)

	i := 0
	err = clf.Process(func(message utils.CLFMessage) error {
		i++
		if message.Type() != utils.CLFMessageTypeFrontLaser {
			return nil
		}
		laserMessage := message.(*utils.CLFOldLaserMessage)
		if len(laserMessage.RangeReadings) != 180 {
			return fmt.Errorf("len(rangeReadings) != 180 : %d", len(laserMessage.RangeReadings))
		}

		theta := utils.RadToDeg(laserMessage.Theta)

		for pos, distance := range laserMessage.RangeReadings {
			// TODO(erh): this is possibly wrong?
			angleDegrees := pos

			if distance > 4 {
				continue
			}

			correctedDegrees := float64(angleDegrees) + theta
			m := lidar.NewMeasurement(correctedDegrees, distance)
			x, y := m.Coords()

			x += laserMessage.X
			y += laserMessage.Y

			area.Mutate(func(area MutableArea) {
				xx := int(x * float64(unitsPerMeter))
				yy := int(y * float64(unitsPerMeter))
				err = area.Set(xx, yy, 1)
			})

			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	os.Mkdir("out", 0755)
	err = rimage.WriteImageToFile("out/foo.png", AreaToImage(area))
	if err != nil {
		t.Fatal(err)
	}
}
