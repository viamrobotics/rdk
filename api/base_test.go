package api_test

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/testutils/inject"

	"go.viam.com/test"
)

func TestDoMove(t *testing.T) {
	dev := &inject.Base{}
	dev.WidthMillisFunc = func(ctx context.Context) (int, error) {
		return 600, nil
	}
	ang, dist, err := api.DoMove(context.Background(), api.Move{}, dev)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ang, test.ShouldEqual, 0)
	test.That(t, dist, test.ShouldEqual, 0)

	err1 := errors.New("oh no")
	dev.MoveStraightFunc = func(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) (int, error) {
		return 2, err1
	}

	m := api.Move{DistanceMillis: 1}
	ang, dist, err = api.DoMove(context.Background(), m, dev)
	test.That(t, errors.Is(err, err1), test.ShouldBeTrue)
	test.That(t, ang, test.ShouldEqual, 0)
	test.That(t, dist, test.ShouldEqual, 2)

	dev.MoveStraightFunc = func(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) (int, error) {
		test.That(t, distanceMillis, test.ShouldEqual, m.DistanceMillis)
		test.That(t, millisPerSec, test.ShouldEqual, m.MillisPerSec)
		test.That(t, block, test.ShouldEqual, m.Block)
		return distanceMillis, nil
	}
	ang, dist, err = api.DoMove(context.Background(), m, dev)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ang, test.ShouldEqual, 0)
	test.That(t, dist, test.ShouldEqual, m.DistanceMillis)

	m = api.Move{DistanceMillis: 1, Block: true, MillisPerSec: 5}
	ang, dist, err = api.DoMove(context.Background(), m, dev)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ang, test.ShouldEqual, 0)
	test.That(t, dist, test.ShouldEqual, m.DistanceMillis)

	dev.SpinFunc = func(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) (float64, error) {
		return 2.2, err1
	}

	m = api.Move{AngleDeg: 10}
	ang, dist, err = api.DoMove(context.Background(), m, dev)
	test.That(t, errors.Is(err, err1), test.ShouldBeTrue)
	test.That(t, ang, test.ShouldEqual, 2.2)
	test.That(t, dist, test.ShouldEqual, 0)

	dev.SpinFunc = func(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) (float64, error) {
		test.That(t, angleDeg, test.ShouldEqual, m.AngleDeg)
		test.That(t, degsPerSec, test.ShouldEqual, m.DegsPerSec)
		test.That(t, block, test.ShouldEqual, m.Block)
		return angleDeg, nil
	}

	m = api.Move{AngleDeg: 10}
	ang, dist, err = api.DoMove(context.Background(), m, dev)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ang, test.ShouldEqual, m.AngleDeg)
	test.That(t, dist, test.ShouldEqual, 0)

	m = api.Move{AngleDeg: 10, Block: true, DegsPerSec: 5}
	ang, dist, err = api.DoMove(context.Background(), m, dev)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ang, test.ShouldEqual, m.AngleDeg)
	test.That(t, dist, test.ShouldEqual, 0)

	m = api.Move{DistanceMillis: 2, AngleDeg: 10, Block: true, DegsPerSec: 5}
	ang, dist, err = api.DoMove(context.Background(), m, dev)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ang, test.ShouldEqual, m.AngleDeg)
	test.That(t, dist, test.ShouldEqual, m.DistanceMillis)

	t.Run("if rotation succeeds but moving straight fails, report rotation", func(t *testing.T) {
		dev.MoveStraightFunc = func(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) (int, error) {
			return 2, err1
		}

		m = api.Move{DistanceMillis: 2, AngleDeg: 10, Block: true, MillisPerSec: 5}
		ang, dist, err = api.DoMove(context.Background(), m, dev)
		test.That(t, errors.Is(err, err1), test.ShouldBeTrue)
		test.That(t, ang, test.ShouldEqual, m.AngleDeg)
		test.That(t, dist, test.ShouldEqual, 2)
	})
}
