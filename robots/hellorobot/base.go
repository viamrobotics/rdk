package hellorobot

import (
	"math"
	"time"

	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"github.com/sbinet/go-python"
)

type Base struct {
	robot   *Robot
	baseObj *python.PyObject
}

func (b *Base) MoveStraight(distanceMM int, mmPerSec float64, block bool) error {
	if mmPerSec != 0 {
		golog.Global.Info("Base.MoveStraight does not support speed")
	}
	b.TranslateBy(float64(distanceMM)/1000, block)
	return nil
}

func (b *Base) Spin(angleDeg float64, speed int, block bool) error {
	if speed != 0 {
		golog.Global.Info("Base.Spin does not support speed")
	}
	b.RotateBy(angleDeg, block)
	return nil
}

func (b *Base) Stop() error {
	b.baseObj.CallMethod("stop")
	return nil
}

func (b *Base) Close() {
	if err := b.Stop(); err != nil {
		golog.Global.Errorw("error stopping base", "error", err)
	}
}

const baseTranslateSpeed = 1.0 / 4 // m/sec

func (b *Base) TranslateBy(meters float64, block bool) {
	b.baseObj.CallMethod("translate_by", python.PyFloat_FromDouble(meters))
	b.robot.pushCommand()
	if block {
		time.Sleep(time.Duration(math.Ceil(math.Abs(meters)/baseTranslateSpeed)) * time.Second)
	}
}

const baseRotateSpeed = 2 * math.Pi / 5 // rad/sec

func (b *Base) RotateBy(angleDeg float64, block bool) {
	rads := -utils.DegToRad(angleDeg)
	b.baseObj.CallMethod("rotate_by", python.PyFloat_FromDouble(rads))
	b.robot.pushCommand()
	if block {
		time.Sleep(time.Duration(math.Ceil(math.Abs(rads)/baseRotateSpeed)) * time.Second)
	}
}
