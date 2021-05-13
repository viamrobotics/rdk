package hellorobot

import (
	"context"
	"errors"
	"math"
	"time"

	"go.viam.com/robotcore/base"
	"go.viam.com/robotcore/config"
	"go.viam.com/robotcore/registry"
	"go.viam.com/robotcore/robot"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"github.com/sbinet/go-python"
)

func init() {
	registry.RegisterBase(ModelName, func(ctx context.Context, r robot.Robot, c config.Component, logger golog.Logger) (base.Base, error) {
		t := r.ProviderByName(ModelName)
		if t == nil {
			return nil, errors.New("no provider created for hellorobot")
		}
		return t.(*Robot).Base()
	})
}

type Base struct {
	robot   *Robot
	baseObj *python.PyObject
}

func (b *Base) MoveStraight(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) (int, error) {
	if millisPerSec != 0 {
		b.robot.logger.Info("Base.MoveStraight does not support speed")
	}
	if err := b.TranslateBy(ctx, float64(distanceMillis)/1000, block); err != nil {
		return 0, err
	}
	return distanceMillis, nil
}

func (b *Base) Spin(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) (float64, error) {
	if degsPerSec != 0 {
		b.robot.logger.Info("Base.Spin does not support degsPerSec")
	}
	if err := b.RotateBy(ctx, angleDeg, block); err != nil {
		return math.NaN(), err
	}
	return angleDeg, nil
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

func (b *Base) TranslateBy(ctx context.Context, meters float64, block bool) error {
	b.baseObj.CallMethod("translate_by", python.PyFloat_FromDouble(meters))
	if err := checkPythonErr(); err != nil {
		return err
	}
	if err := b.robot.pushCommand(); err != nil {
		return err
	}
	if block {
		if !utils.SelectContextOrWait(ctx, time.Duration(math.Ceil(math.Abs(meters)/baseTranslateSpeed))*time.Second) {
			return ctx.Err()
		}
	}
	return nil
}

const baseRotateSpeed = 2 * math.Pi / 5 // rad/sec

func (b *Base) RotateBy(ctx context.Context, angleDeg float64, block bool) error {
	rads := -utils.DegToRad(angleDeg)
	b.baseObj.CallMethod("rotate_by", python.PyFloat_FromDouble(rads))
	if err := checkPythonErr(); err != nil {
		return err
	}
	if err := b.robot.pushCommand(); err != nil {
		return err
	}
	if block {
		if !utils.SelectContextOrWait(ctx, time.Duration(math.Ceil(math.Abs(rads)/baseRotateSpeed))*time.Second) {
			return ctx.Err()
		}
	}
	return nil
}
