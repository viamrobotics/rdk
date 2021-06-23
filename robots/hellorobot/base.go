package hellorobot

import (
	"context"
	"math"
	"time"

	"github.com/go-errors/errors"

	"go.viam.com/core/base"
	"go.viam.com/core/config"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"github.com/sbinet/go-python"
	goutils "go.viam.com/utils"
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

// Base is the base of the robot.
type Base struct {
	robot   *Robot
	baseObj *python.PyObject
}

// MoveStraight moves the base straight but does not support speed.
func (b *Base) MoveStraight(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) (int, error) {
	if millisPerSec != 0 {
		b.robot.logger.Info("Base.MoveStraight does not support speed")
	}
	if err := b.TranslateBy(ctx, float64(distanceMillis)/1000, block); err != nil {
		return 0, err
	}
	return distanceMillis, nil
}

// Spin spins the base but does not support speed.
func (b *Base) Spin(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) (float64, error) {
	if degsPerSec != 0 {
		b.robot.logger.Info("Base.Spin does not support degsPerSec")
	}
	if err := b.RotateBy(ctx, angleDeg, block); err != nil {
		return math.NaN(), err
	}
	return angleDeg, nil
}

// WidthMillis returns the width of the base as if it were a square.
func (b *Base) WidthMillis(ctx context.Context) (int, error) {
	return 600, nil
}

// Stop stops the base in its place.
func (b *Base) Stop(ctx context.Context) error {
	b.baseObj.CallMethod("stop")
	return checkPythonErr()
}

// Close calls stop.
func (b *Base) Close() error {
	return b.Stop(context.Background())
}

const baseTranslateSpeed = 1.0 / 6 // m/sec

// TranslateBy has the robot move a given amount of meters using a speed we found to
// be safe.
func (b *Base) TranslateBy(ctx context.Context, meters float64, block bool) error {
	b.baseObj.CallMethod("translate_by", python.PyFloat_FromDouble(meters))
	if err := checkPythonErr(); err != nil {
		return err
	}
	if err := b.robot.pushCommand(); err != nil {
		return err
	}
	if block {
		if !goutils.SelectContextOrWait(ctx, time.Duration(math.Ceil(math.Abs(meters)/baseTranslateSpeed))*time.Second) {
			return ctx.Err()
		}
	}
	return nil
}

const baseRotateSpeed = 2 * math.Pi / 5 // rad/sec

// RotateBy has the robot spin a given amount of degrees clockwise using a speed we
// found to be safe.
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
		if !goutils.SelectContextOrWait(ctx, time.Duration(math.Ceil(math.Abs(rads)/baseRotateSpeed))*time.Second) {
			return ctx.Err()
		}
	}
	return nil
}
