package hellorobot

import (
	"math"
	"time"

	"github.com/viamrobotics/robotcore/utils"

	"github.com/edaniels/golog"
	"github.com/sbinet/go-python"
)

type Base struct {
	robot   *Robot
	baseObj *python.PyObject
}

func (b *Base) MoveStraight(distanceMM int, speed int, block bool) error {
	if speed != 0 {
		golog.Global.Info("Base.MoveStraight does not support speed")
	}
	b.TranslateBy(float64(distanceMM)/1000, block)
	return nil
}

func (b *Base) Spin(degrees float64, power int, block bool) error {
	if power != 0 {
		golog.Global.Info("Base.Spin does not support power")
	}
	b.RotateBy(degrees, block)
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

func (b *Base) RotateBy(degrees float64, block bool) {
	rads := -utils.DegToRad(degrees)
	b.baseObj.CallMethod("rotate_by", python.PyFloat_FromDouble(rads))
	b.robot.pushCommand()
	if block {
		time.Sleep(time.Duration(math.Ceil(math.Abs(rads)/baseRotateSpeed)) * time.Second)
	}
}
