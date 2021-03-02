package base_test

import (
	"context"
	"errors"
	"math"
	"testing"

	"go.viam.com/robotcore/base"
	"go.viam.com/robotcore/testutils/inject"

	"github.com/edaniels/test"
)

func TestDoMove(t *testing.T) {
	dev := &inject.Base{}
	ang, dist, err := base.DoMove(base.Move{}, dev)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ang, test.ShouldEqual, 0)
	test.That(t, dist, test.ShouldEqual, 0)

	err1 := errors.New("oh no")
	dev.MoveStraightFunc = func(distanceMM int, speed float64, block bool) error {
		return err1
	}

	m := base.Move{DistanceMM: 1}
	ang, dist, err = base.DoMove(m, dev)
	test.That(t, errors.Is(err, err1), test.ShouldBeTrue)
	test.That(t, ang, test.ShouldEqual, 0)
	test.That(t, dist, test.ShouldEqual, 0)

	dev.MoveStraightFunc = func(distanceMM int, speed float64, block bool) error {
		test.That(t, distanceMM, test.ShouldEqual, m.DistanceMM)
		test.That(t, speed, test.ShouldEqual, m.Speed)
		test.That(t, block, test.ShouldEqual, m.Block)
		return nil
	}
	ang, dist, err = base.DoMove(m, dev)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ang, test.ShouldEqual, 0)
	test.That(t, dist, test.ShouldEqual, m.DistanceMM)

	m = base.Move{DistanceMM: 1, Block: true, Speed: 5}
	ang, dist, err = base.DoMove(m, dev)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ang, test.ShouldEqual, 0)
	test.That(t, dist, test.ShouldEqual, m.DistanceMM)

	dev.SpinFunc = func(angleDeg float64, speed int, block bool) error {
		return err1
	}

	m = base.Move{AngleDeg: 10}
	ang, dist, err = base.DoMove(m, dev)
	test.That(t, errors.Is(err, err1), test.ShouldBeTrue)
	test.That(t, math.IsNaN(ang), test.ShouldBeTrue)
	test.That(t, dist, test.ShouldEqual, 0)

	dev.SpinFunc = func(angleDeg float64, speed int, block bool) error {
		test.That(t, angleDeg, test.ShouldEqual, m.AngleDeg)
		test.That(t, speed, test.ShouldEqual, m.Speed)
		test.That(t, block, test.ShouldEqual, m.Block)
		return nil
	}

	m = base.Move{AngleDeg: 10}
	ang, dist, err = base.DoMove(m, dev)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ang, test.ShouldEqual, m.AngleDeg)
	test.That(t, dist, test.ShouldEqual, 0)

	m = base.Move{AngleDeg: 10, Block: true, Speed: 5}
	ang, dist, err = base.DoMove(m, dev)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ang, test.ShouldEqual, m.AngleDeg)
	test.That(t, dist, test.ShouldEqual, 0)

	m = base.Move{DistanceMM: 2, AngleDeg: 10, Block: true, Speed: 5}
	ang, dist, err = base.DoMove(m, dev)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ang, test.ShouldEqual, m.AngleDeg)
	test.That(t, dist, test.ShouldEqual, m.DistanceMM)

	t.Run("if rotation succeeds but moving straight fails, report rotation", func(t *testing.T) {
		dev.MoveStraightFunc = func(distanceMM int, speed float64, block bool) error {
			return err1
		}

		m = base.Move{DistanceMM: 2, AngleDeg: 10, Block: true, Speed: 5}
		ang, dist, err = base.DoMove(m, dev)
		test.That(t, errors.Is(err, err1), test.ShouldBeTrue)
		test.That(t, ang, test.ShouldEqual, m.AngleDeg)
		test.That(t, dist, test.ShouldEqual, 0)
	})
}

func TestAugmentReduce(t *testing.T) {
	dev := &inject.Base{}
	test.That(t, base.Augment(dev, nil), test.ShouldEqual, dev)

	comp := &inject.Compass{}
	aug := base.Augment(dev, comp)
	test.That(t, aug, test.ShouldNotEqual, dev)
	var baseDev *base.Device = nil
	test.That(t, aug, test.ShouldImplement, baseDev)

	test.That(t, base.Reduce(aug), test.ShouldEqual, dev)
}

func TestDeviceWithCompass(t *testing.T) {
	dev := &inject.Base{}
	comp := &inject.Compass{}
	aug := base.Augment(dev, comp)

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
