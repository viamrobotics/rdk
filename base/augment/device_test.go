package augment_test

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/edaniels/test"
	"go.viam.com/robotcore/base"
	"go.viam.com/robotcore/base/augment"
	"go.viam.com/robotcore/testutils/inject"
)

func TestAugmentReduce(t *testing.T) {
	dev := &inject.Base{}
	dev.WidthFunc = func() float64 {
		return 0.6
	}
	test.That(t, augment.Device(dev, nil), test.ShouldEqual, dev)

	comp := &inject.Compass{}
	aug := augment.Device(dev, comp)
	test.That(t, aug, test.ShouldNotEqual, dev)
	var baseDev *base.Device = nil
	test.That(t, aug, test.ShouldImplement, baseDev)

	test.That(t, augment.ReduceDevice(aug), test.ShouldEqual, dev)
}

func TestDeviceWithCompass(t *testing.T) {
	dev := &inject.Base{}
	dev.WidthFunc = func() float64 {
		return 0.6
	}
	comp := &inject.Compass{}
	aug := augment.Device(dev, comp)

	t.Run("perfect base", func(t *testing.T) {
		i := 0
		dev.SpinFunc = func(angleDeg float64, speed int, block bool) error {
			i++
			return nil
		}
		comp.HeadingFunc = func(ctx context.Context) (float64, error) {
			if i == 0 {
				return 0, nil
			}
			return 10, nil
		}
		ang, _, err := base.DoMove(base.Move{AngleDeg: 10}, aug)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ang, test.ShouldEqual, 10)
	})

	t.Run("off by under third", func(t *testing.T) {
		i := 0
		dev.SpinFunc = func(angleDeg float64, speed int, block bool) error {
			i++
			return nil
		}
		comp.HeadingFunc = func(ctx context.Context) (float64, error) {
			if i == 0 {
				return 0, nil
			}
			return 10 * (float64(i) / 3), nil
		}
		ang, _, err := base.DoMove(base.Move{AngleDeg: 10}, aug)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ang, test.ShouldEqual, 10)
	})

	t.Run("off by over third", func(t *testing.T) {
		i := 0
		dev.SpinFunc = func(angleDeg float64, speed int, block bool) error {
			i++
			return nil
		}
		comp.HeadingFunc = func(ctx context.Context) (float64, error) {
			if i == 0 {
				return 0, nil
			}
			return 10 + 10*(float64(i)/3), nil
		}
		ang, _, err := base.DoMove(base.Move{AngleDeg: 10}, aug)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ang, test.ShouldEqual, 10)
	})

	t.Run("error getting heading", func(t *testing.T) {
		dev.SpinFunc = func(angleDeg float64, speed int, block bool) error {
			return nil
		}
		err1 := errors.New("oh no")
		comp.HeadingFunc = func(ctx context.Context) (float64, error) {
			return 0, err1
		}
		ang, _, err := base.DoMove(base.Move{AngleDeg: 10}, aug)
		test.That(t, errors.Is(err, err1), test.ShouldBeTrue)
		test.That(t, math.IsNaN(ang), test.ShouldBeTrue)
	})

	t.Run("error spinning", func(t *testing.T) {
		err1 := errors.New("oh no")
		dev.SpinFunc = func(angleDeg float64, speed int, block bool) error {
			return err1
		}
		comp.HeadingFunc = func(ctx context.Context) (float64, error) {
			return 0, nil
		}
		ang, _, err := base.DoMove(base.Move{AngleDeg: 10}, aug)
		test.That(t, errors.Is(err, err1), test.ShouldBeTrue)
		test.That(t, math.IsNaN(ang), test.ShouldBeTrue)
	})
}
