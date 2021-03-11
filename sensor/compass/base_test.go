package compass_test

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/edaniels/test"
	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/sensor/compass"
	"go.viam.com/robotcore/testutils/inject"
)

func TestAugmentReduce(t *testing.T) {
	dev := &inject.Base{}
	dev.WidthMillisFunc = func(ctx context.Context) (int, error) {
		return 600, nil
	}
	test.That(t, compass.BaseWithCompass(dev, nil), test.ShouldEqual, dev)

	comp := &inject.Compass{}
	aug := compass.BaseWithCompass(dev, comp)
	test.That(t, aug, test.ShouldNotEqual, dev)
	var baseDev *api.Base = nil
	test.That(t, aug, test.ShouldImplement, baseDev)

	test.That(t, compass.ReduceBase(aug), test.ShouldEqual, dev)
}

func TestDeviceWithCompass(t *testing.T) {
	dev := &inject.Base{}
	dev.WidthMillisFunc = func(ctx context.Context) (int, error) {
		return 600, nil
	}
	comp := &inject.Compass{}
	aug := compass.BaseWithCompass(dev, comp)

	t.Run("perfect base", func(t *testing.T) {
		i := 0
		dev.SpinFunc = func(ctx context.Context, angleDeg float64, speed int, block bool) error {
			i++
			return nil
		}
		comp.HeadingFunc = func(ctx context.Context) (float64, error) {
			if i == 0 {
				return 0, nil
			}
			return 10, nil
		}
		ang, _, err := api.DoMove(context.Background(), api.Move{AngleDeg: 10}, aug)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ang, test.ShouldEqual, 10)
	})

	t.Run("off by under third", func(t *testing.T) {
		i := 0
		dev.SpinFunc = func(ctx context.Context, angleDeg float64, speed int, block bool) error {
			i++
			return nil
		}
		comp.HeadingFunc = func(ctx context.Context) (float64, error) {
			if i == 0 {
				return 0, nil
			}
			return 10 * (float64(i) / 3), nil
		}
		ang, _, err := api.DoMove(context.Background(), api.Move{AngleDeg: 10}, aug)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ang, test.ShouldEqual, 10)
	})

	t.Run("off by over third", func(t *testing.T) {
		i := 0
		dev.SpinFunc = func(ctx context.Context, angleDeg float64, speed int, block bool) error {
			i++
			return nil
		}
		comp.HeadingFunc = func(ctx context.Context) (float64, error) {
			if i == 0 {
				return 0, nil
			}
			return 10 + 10*(float64(i)/3), nil
		}
		ang, _, err := api.DoMove(context.Background(), api.Move{AngleDeg: 10}, aug)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ang, test.ShouldEqual, 10)
	})

	t.Run("error getting heading", func(t *testing.T) {
		dev.SpinFunc = func(ctx context.Context, angleDeg float64, speed int, block bool) error {
			return nil
		}
		err1 := errors.New("oh no")
		comp.HeadingFunc = func(ctx context.Context) (float64, error) {
			return 0, err1
		}
		ang, _, err := api.DoMove(context.Background(), api.Move{AngleDeg: 10}, aug)
		test.That(t, errors.Is(err, err1), test.ShouldBeTrue)
		test.That(t, math.IsNaN(ang), test.ShouldBeTrue)
	})

	t.Run("error spinning", func(t *testing.T) {
		err1 := errors.New("oh no")
		dev.SpinFunc = func(ctx context.Context, angleDeg float64, speed int, block bool) error {
			return err1
		}
		comp.HeadingFunc = func(ctx context.Context) (float64, error) {
			return 0, nil
		}
		ang, _, err := api.DoMove(context.Background(), api.Move{AngleDeg: 10}, aug)
		test.That(t, errors.Is(err, err1), test.ShouldBeTrue)
		test.That(t, math.IsNaN(ang), test.ShouldBeTrue)
	})
}
