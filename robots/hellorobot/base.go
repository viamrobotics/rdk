package hellorobot

import (
	"context"
	"fmt"
	"math"
	"time"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"github.com/sbinet/go-python"
)

func init() {
	api.RegisterBase(ModelName, func(ctx context.Context, r api.Robot, c api.Component, logger golog.Logger) (api.Base, error) {
		t := r.ProviderByModel(ModelName)
		if t == nil {
			return nil, fmt.Errorf("no provider created for hellorobot")
		}
		return t.(*Robot).Base()
	})
}

type Base struct {
	robot   *Robot
	baseObj *python.PyObject
}

func (b *Base) MoveStraight(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) error {
	if millisPerSec != 0 {
		b.robot.logger.Info("Base.MoveStraight does not support speed")
	}
	return b.TranslateBy(float64(distanceMillis)/1000, block)
}

func (b *Base) Spin(ctx context.Context, angleDeg float64, speed int, block bool) error {
	if speed != 0 {
		b.robot.logger.Info("Base.Spin does not support speed")
	}
	return b.RotateBy(angleDeg, block)
}

func (b *Base) WidthMillis(ctx context.Context) (int, error) {
	return 600, nil
}

func (b *Base) Stop(ctx context.Context) error {
	b.baseObj.CallMethod("stop")
	return checkPythonErr()
}

func (b *Base) Close() error {
	return b.Stop(context.Background())
}

const baseTranslateSpeed = 1.0 / 6 // m/sec

func (b *Base) TranslateBy(meters float64, block bool) error {
	b.baseObj.CallMethod("translate_by", python.PyFloat_FromDouble(meters))
	if err := checkPythonErr(); err != nil {
		return err
	}
	if err := b.robot.pushCommand(); err != nil {
		return err
	}
	if block {
		time.Sleep(time.Duration(math.Ceil(math.Abs(meters)/baseTranslateSpeed)) * time.Second)
	}
	return nil
}

const baseRotateSpeed = 2 * math.Pi / 5 // rad/sec

func (b *Base) RotateBy(angleDeg float64, block bool) error {
	rads := -utils.DegToRad(angleDeg)
	b.baseObj.CallMethod("rotate_by", python.PyFloat_FromDouble(rads))
	if err := checkPythonErr(); err != nil {
		return err
	}
	if err := b.robot.pushCommand(); err != nil {
		return err
	}
	if block {
		time.Sleep(time.Duration(math.Ceil(math.Abs(rads)/baseRotateSpeed)) * time.Second)
	}
	return nil
}
