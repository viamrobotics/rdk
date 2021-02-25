package base

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/viamrobotics/robotcore/sensor/compass"

	"github.com/edaniels/test"
)

type injectDevice struct {
	Device
	MoveStraightFunc func(distanceMM int, speed float64, block bool) error
	SpinFunc         func(angleDeg float64, speed int, block bool) error
	StopFunc         func() error
	CloseFunc        func()
}

func (id *injectDevice) MoveStraight(distanceMM int, speed float64, block bool) error {
	if id.MoveStraightFunc == nil {
		return id.Device.MoveStraight(distanceMM, speed, block)
	}
	return id.MoveStraightFunc(distanceMM, speed, block)
}

func (id *injectDevice) Spin(angleDeg float64, speed int, block bool) error {
	if id.SpinFunc == nil {
		return id.Device.Spin(angleDeg, speed, block)
	}
	return id.SpinFunc(angleDeg, speed, block)
}

func (id *injectDevice) Stop() error {
	if id.StopFunc == nil {
		return id.Device.Stop()
	}
	return id.StopFunc()
}

func (id *injectDevice) Close() {
	if id.CloseFunc == nil {
		id.Device.Close()
		return
	}
	id.CloseFunc()
}

func TestDoMove(t *testing.T) {
	dev := &injectDevice{}
	ang, dist, err := DoMove(Move{}, dev)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ang, test.ShouldEqual, 0)
	test.That(t, dist, test.ShouldEqual, 0)

	err1 := errors.New("oh no")
	dev.MoveStraightFunc = func(distanceMM int, speed float64, block bool) error {
		return err1
	}

	m := Move{DistanceMM: 1}
	ang, dist, err = DoMove(m, dev)
	test.That(t, errors.Is(err, err1), test.ShouldBeTrue)
	test.That(t, ang, test.ShouldEqual, 0)
	test.That(t, dist, test.ShouldEqual, 0)

	dev.MoveStraightFunc = func(distanceMM int, speed float64, block bool) error {
		test.That(t, distanceMM, test.ShouldEqual, m.DistanceMM)
		test.That(t, speed, test.ShouldEqual, m.Speed)
		test.That(t, block, test.ShouldEqual, m.Block)
		return nil
	}
	ang, dist, err = DoMove(m, dev)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ang, test.ShouldEqual, 0)
	test.That(t, dist, test.ShouldEqual, m.DistanceMM)

	m = Move{DistanceMM: 1, Block: true, Speed: 5}
	ang, dist, err = DoMove(m, dev)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ang, test.ShouldEqual, 0)
	test.That(t, dist, test.ShouldEqual, m.DistanceMM)

	dev.SpinFunc = func(angleDeg float64, speed int, block bool) error {
		return err1
	}

	m = Move{AngleDeg: 10}
	ang, dist, err = DoMove(m, dev)
	test.That(t, errors.Is(err, err1), test.ShouldBeTrue)
	test.That(t, math.IsNaN(ang), test.ShouldBeTrue)
	test.That(t, dist, test.ShouldEqual, 0)

	dev.SpinFunc = func(angleDeg float64, speed int, block bool) error {
		test.That(t, angleDeg, test.ShouldEqual, m.AngleDeg)
		test.That(t, speed, test.ShouldEqual, m.Speed)
		test.That(t, block, test.ShouldEqual, m.Block)
		return nil
	}

	m = Move{AngleDeg: 10}
	ang, dist, err = DoMove(m, dev)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ang, test.ShouldEqual, m.AngleDeg)
	test.That(t, dist, test.ShouldEqual, 0)

	m = Move{AngleDeg: 10, Block: true, Speed: 5}
	ang, dist, err = DoMove(m, dev)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ang, test.ShouldEqual, m.AngleDeg)
	test.That(t, dist, test.ShouldEqual, 0)

	m = Move{DistanceMM: 2, AngleDeg: 10, Block: true, Speed: 5}
	ang, dist, err = DoMove(m, dev)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ang, test.ShouldEqual, m.AngleDeg)
	test.That(t, dist, test.ShouldEqual, m.DistanceMM)

	t.Run("if rotation succeeds but moving straight fails, report rotation", func(t *testing.T) {
		dev.MoveStraightFunc = func(distanceMM int, speed float64, block bool) error {
			return err1
		}

		m = Move{DistanceMM: 2, AngleDeg: 10, Block: true, Speed: 5}
		ang, dist, err = DoMove(m, dev)
		test.That(t, errors.Is(err, err1), test.ShouldBeTrue)
		test.That(t, ang, test.ShouldEqual, m.AngleDeg)
		test.That(t, dist, test.ShouldEqual, 0)
	})
}

type injectCompass struct {
	compass.Device
	ReadingsFunc         func(ctx context.Context) ([]interface{}, error)
	HeadingFunc          func(ctx context.Context) (float64, error)
	StartCalibrationFunc func(ctx context.Context) error
	StopCalibrationFunc  func(ctx context.Context) error
	CloseFunc            func(ctx context.Context) error
}

func (ic *injectCompass) Readings(ctx context.Context) ([]interface{}, error) {
	if ic.ReadingsFunc == nil {
		return ic.Device.Readings(ctx)
	}
	return ic.ReadingsFunc(ctx)
}

func (ic *injectCompass) Heading(ctx context.Context) (float64, error) {
	if ic.HeadingFunc == nil {
		return ic.Device.Heading(ctx)
	}
	return ic.HeadingFunc(ctx)
}

func (ic *injectCompass) StartCalibration(ctx context.Context) error {
	if ic.StartCalibrationFunc == nil {
		return ic.Device.StartCalibration(ctx)
	}
	return ic.StartCalibrationFunc(ctx)
}

func (ic *injectCompass) StopCalibration(ctx context.Context) error {
	if ic.StopCalibrationFunc == nil {
		return ic.Device.StopCalibration(ctx)
	}
	return ic.StopCalibrationFunc(ctx)
}

func (ic *injectCompass) Close(ctx context.Context) error {
	if ic.CloseFunc == nil {
		return ic.Device.Close(ctx)
	}
	return ic.CloseFunc(ctx)
}

func TestAugmentReduce(t *testing.T) {
	dev := &injectDevice{}
	test.That(t, Augment(dev, nil), test.ShouldEqual, dev)

	comp := &injectCompass{}
	aug := Augment(dev, comp)
	test.That(t, aug, test.ShouldNotEqual, dev)
	var baseDev *Device = nil
	test.That(t, aug, test.ShouldImplement, baseDev)

	test.That(t, Reduce(aug), test.ShouldEqual, dev)
}

func TestDeviceWithCompass(t *testing.T) {
	dev := &injectDevice{}
	comp := &injectCompass{}
	aug := Augment(dev, comp)

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
		ang, _, err := DoMove(Move{AngleDeg: 10}, aug)
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
		ang, _, err := DoMove(Move{AngleDeg: 10}, aug)
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
		ang, _, err := DoMove(Move{AngleDeg: 10}, aug)
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
		ang, _, err := DoMove(Move{AngleDeg: 10}, aug)
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
		ang, _, err := DoMove(Move{AngleDeg: 10}, aug)
		test.That(t, errors.Is(err, err1), test.ShouldBeTrue)
		test.That(t, math.IsNaN(ang), test.ShouldBeTrue)
	})
}
