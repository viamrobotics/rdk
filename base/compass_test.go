package base_test

import (
	"context"
	"math"
	"testing"

	"github.com/pkg/errors"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/base"
	"go.viam.com/rdk/testutils/inject"
)

func TestAugmentReduce(t *testing.T) {
	logger := golog.NewTestLogger(t)
	dev := &inject.Base{}
	dev.WidthMillisFunc = func(ctx context.Context) (int, error) {
		return 600, nil
	}
	test.That(t, base.AugmentWithCompass(dev, nil, logger), test.ShouldEqual, dev)

	comp := &inject.Compass{}
	aug := base.AugmentWithCompass(dev, comp, logger)
	test.That(t, aug, test.ShouldNotEqual, dev)
	var baseDev *base.Base = nil
	test.That(t, aug, test.ShouldImplement, baseDev)

	test.That(t, base.Reduce(aug), test.ShouldEqual, dev)
}

func TestDeviceWithCompass(t *testing.T) {
	logger := golog.NewTestLogger(t)
	dev := &inject.Base{}
	dev.WidthMillisFunc = func(ctx context.Context) (int, error) {
		return 600, nil
	}
	comp := &inject.Compass{}
	aug := base.AugmentWithCompass(dev, comp, logger)

	t.Run("perfect base", func(t *testing.T) {
		i := 0
		dev.SpinFunc = func(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) error {
			i++
			return nil
		}
		comp.HeadingFunc = func(ctx context.Context) (float64, error) {
			if i == 0 {
				return 0, nil
			}
			return 10, nil
		}
		err := base.DoMove(context.Background(), base.Move{AngleDeg: 10}, aug)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("off by under third", func(t *testing.T) {
		i := 0
		dev.SpinFunc = func(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) error {
			i++
			return nil
		}
		comp.HeadingFunc = func(ctx context.Context) (float64, error) {
			if i == 0 {
				return 0, nil
			}
			return 10 * (float64(i) / 3), nil
		}
		err := base.DoMove(context.Background(), base.Move{AngleDeg: 10}, aug)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("off by over third", func(t *testing.T) {
		i := 0
		dev.SpinFunc = func(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) error {
			i++
			return nil
		}
		comp.HeadingFunc = func(ctx context.Context) (float64, error) {
			if i == 0 {
				return 0, nil
			}
			return 10 + 10*(float64(i)/3), nil
		}
		err := base.DoMove(context.Background(), base.Move{AngleDeg: 10}, aug)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("error getting heading", func(t *testing.T) {
		dev.SpinFunc = func(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) error {
			return nil
		}
		err1 := errors.New("oh no")
		comp.HeadingFunc = func(ctx context.Context) (float64, error) {
			return math.NaN(), err1
		}
		err := base.DoMove(context.Background(), base.Move{AngleDeg: 10}, aug)
		test.That(t, errors.Is(err, err1), test.ShouldBeTrue)
	})

	t.Run("error spinning", func(t *testing.T) {
		err1 := errors.New("oh no")
		dev.SpinFunc = func(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) error {
			return err1
		}
		comp.HeadingFunc = func(ctx context.Context) (float64, error) {
			return 0, nil
		}
		err := base.DoMove(context.Background(), base.Move{AngleDeg: 10}, aug)
		test.That(t, errors.Is(err, err1), test.ShouldBeTrue)
	})
}
