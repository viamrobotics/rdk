package base_test

import (
	"context"
	"testing"

	"github.com/pkg/errors"

	"go.viam.com/core/base"
	"go.viam.com/core/testutils/inject"

	"go.viam.com/test"
)

func TestDoMove(t *testing.T) {
	dev := &inject.Base{}
	dev.WidthMillisFunc = func(ctx context.Context) (int, error) {
		return 600, nil
	}
	err := base.DoMove(context.Background(), base.Move{}, dev)
	test.That(t, err, test.ShouldBeNil)

	err1 := errors.New("oh no")
	dev.MoveStraightFunc = func(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) error {
		return err1
	}

	m := base.Move{DistanceMillis: 1}
	err = base.DoMove(context.Background(), m, dev)
	test.That(t, errors.Is(err, err1), test.ShouldBeTrue)

	dev.MoveStraightFunc = func(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) error {
		test.That(t, distanceMillis, test.ShouldEqual, m.DistanceMillis)
		test.That(t, millisPerSec, test.ShouldEqual, m.MillisPerSec)
		test.That(t, block, test.ShouldEqual, m.Block)
		return nil
	}
	err = base.DoMove(context.Background(), m, dev)
	test.That(t, err, test.ShouldBeNil)

	m = base.Move{DistanceMillis: 1, Block: true, MillisPerSec: 5}
	err = base.DoMove(context.Background(), m, dev)
	test.That(t, err, test.ShouldBeNil)

	dev.SpinFunc = func(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) error {
		return err1
	}

	m = base.Move{AngleDeg: 10}
	err = base.DoMove(context.Background(), m, dev)
	test.That(t, errors.Is(err, err1), test.ShouldBeTrue)

	dev.SpinFunc = func(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) error {
		test.That(t, angleDeg, test.ShouldEqual, m.AngleDeg)
		test.That(t, degsPerSec, test.ShouldEqual, m.DegsPerSec)
		test.That(t, block, test.ShouldEqual, m.Block)
		return nil
	}

	m = base.Move{AngleDeg: 10}
	err = base.DoMove(context.Background(), m, dev)
	test.That(t, err, test.ShouldBeNil)

	m = base.Move{AngleDeg: 10, Block: true, DegsPerSec: 5}
	err = base.DoMove(context.Background(), m, dev)
	test.That(t, err, test.ShouldBeNil)

	m = base.Move{DistanceMillis: 2, AngleDeg: 10, Block: true, DegsPerSec: 5}
	err = base.DoMove(context.Background(), m, dev)
	test.That(t, err, test.ShouldBeNil)

	t.Run("if rotation succeeds but moving straight fails, report rotation", func(t *testing.T) {
		dev.MoveStraightFunc = func(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) error {
			return err1
		}

		m = base.Move{DistanceMillis: 2, AngleDeg: 10, Block: true, MillisPerSec: 5}
		err = base.DoMove(context.Background(), m, dev)
		test.That(t, errors.Is(err, err1), test.ShouldBeTrue)
	})
}
