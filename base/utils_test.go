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
	dev.WidthMillisFunc = func(ctx context.Context) (int, error) {
		return 600, nil
	}
	ang, dist, err := base.DoMove(context.Background(), base.Move{}, dev)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ang, test.ShouldEqual, 0)
	test.That(t, dist, test.ShouldEqual, 0)

	err1 := errors.New("oh no")
	dev.MoveStraightFunc = func(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) error {
		return err1
	}

	m := base.Move{DistanceMillis: 1}
	ang, dist, err = base.DoMove(context.Background(), m, dev)
	test.That(t, errors.Is(err, err1), test.ShouldBeTrue)
	test.That(t, ang, test.ShouldEqual, 0)
	test.That(t, dist, test.ShouldEqual, 0)

	dev.MoveStraightFunc = func(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) error {
		test.That(t, distanceMillis, test.ShouldEqual, m.DistanceMillis)
		test.That(t, millisPerSec, test.ShouldEqual, m.MillisPerSec)
		test.That(t, block, test.ShouldEqual, m.Block)
		return nil
	}
	ang, dist, err = base.DoMove(context.Background(), m, dev)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ang, test.ShouldEqual, 0)
	test.That(t, dist, test.ShouldEqual, m.DistanceMillis)

	m = base.Move{DistanceMillis: 1, Block: true, MillisPerSec: 5}
	ang, dist, err = base.DoMove(context.Background(), m, dev)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ang, test.ShouldEqual, 0)
	test.That(t, dist, test.ShouldEqual, m.DistanceMillis)

	dev.SpinFunc = func(ctx context.Context, angleDeg float64, speed int, block bool) error {
		return err1
	}

	m = base.Move{AngleDeg: 10}
	ang, dist, err = base.DoMove(context.Background(), m, dev)
	test.That(t, errors.Is(err, err1), test.ShouldBeTrue)
	test.That(t, math.IsNaN(ang), test.ShouldBeTrue)
	test.That(t, dist, test.ShouldEqual, 0)

	dev.SpinFunc = func(ctx context.Context, angleDeg float64, speed int, block bool) error {
		test.That(t, angleDeg, test.ShouldEqual, m.AngleDeg)
		test.That(t, speed, test.ShouldEqual, m.MillisPerSec)
		test.That(t, block, test.ShouldEqual, m.Block)
		return nil
	}

	m = base.Move{AngleDeg: 10}
	ang, dist, err = base.DoMove(context.Background(), m, dev)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ang, test.ShouldEqual, m.AngleDeg)
	test.That(t, dist, test.ShouldEqual, 0)

	m = base.Move{AngleDeg: 10, Block: true, MillisPerSec: 5}
	ang, dist, err = base.DoMove(context.Background(), m, dev)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ang, test.ShouldEqual, m.AngleDeg)
	test.That(t, dist, test.ShouldEqual, 0)

	m = base.Move{DistanceMillis: 2, AngleDeg: 10, Block: true, MillisPerSec: 5}
	ang, dist, err = base.DoMove(context.Background(), m, dev)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ang, test.ShouldEqual, m.AngleDeg)
	test.That(t, dist, test.ShouldEqual, m.DistanceMillis)

	t.Run("if rotation succeeds but moving straight fails, report rotation", func(t *testing.T) {
		dev.MoveStraightFunc = func(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) error {
			return err1
		}

		m = base.Move{DistanceMillis: 2, AngleDeg: 10, Block: true, MillisPerSec: 5}
		ang, dist, err = base.DoMove(context.Background(), m, dev)
		test.That(t, errors.Is(err, err1), test.ShouldBeTrue)
		test.That(t, ang, test.ShouldEqual, m.AngleDeg)
		test.That(t, dist, test.ShouldEqual, 0)
	})
}
