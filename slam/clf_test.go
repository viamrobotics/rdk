package slam

import (
	"os"
	"testing"

	"go.viam.com/utils/artifact"
	"go.viam.com/utils/testutils"

	"go.viam.com/core/lidar"
	"go.viam.com/core/rimage"
	"go.viam.com/core/rlog"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"
	"go.viam.com/test"
)

// http://ais.informatik.uni-freiburg.de/slamevaluation/index.php

func TestAcesCLF(t *testing.T) {
	fn := artifact.MustPath("slam/aces.clf")
	f, err := os.Open(fn)
	test.That(t, err, test.ShouldBeNil)
	defer f.Close()

	unitsPerMeter := 20.

	area, err := NewSquareArea(200, unitsPerMeter, rlog.Logger)
	test.That(t, err, test.ShouldBeNil)

	clf := utils.NewCLFReader(f)

	i := 0
	err = clf.Process(func(message utils.CLFMessage) error {
		i++
		if message.Type() != utils.CLFMessageTypeFrontLaser {
			return nil
		}
		laserMessage := message.(*utils.CLFOldLaserMessage)
		if len(laserMessage.RangeReadings) != 180 {
			return errors.Errorf("len(rangeReadings) != 180 : %d", len(laserMessage.RangeReadings))
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
				xx := x * unitsPerMeter
				yy := y * unitsPerMeter
				err = area.Set(xx, yy, 1)
			})

			if err != nil {
				return err
			}
		}

		return nil
	})
	test.That(t, err, test.ShouldBeNil)

	outDir := testutils.TempDirT(t, "", "slam")
	golog.NewTestLogger(t).Debugf("out dir: %q", outDir)
	err = rimage.WriteImageToFile(outDir+"/foo.png", AreaToImage(area))
	test.That(t, err, test.ShouldBeNil)
}
