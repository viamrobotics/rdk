package base_test

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/component/base"
	"go.viam.com/rdk/testutils/inject"
)

func TestBaseNamed(t *testing.T) {
	baseName := base.Named("test_base")
	test.That(t, baseName.String(), test.ShouldResemble, "rdk:component:base/test_base")
	test.That(t, baseName.Subtype, test.ShouldResemble, base.Subtype)
	test.That(t, baseName.UUID, test.ShouldResemble, "026551c7-e5d4-55bd-ba08-61bcdc643bce")
}

func TestDoMove(t *testing.T) {
	dev := &inject.Base{}
	dev.GetWidthFunc = func(ctx context.Context) (int, error) {
		return 600, nil
	}
	err := base.DoMove(context.Background(), base.Move{}, dev)
	test.That(t, err, test.ShouldBeNil)

	err1 := errors.New("oh no")
	dev.MoveStraightFunc = func(ctx context.Context, distanceMm int, mmPerSec float64, block bool) error {
		return err1
	}

	m := base.Move{DistanceMm: 1}
	err = base.DoMove(context.Background(), m, dev)
	test.That(t, errors.Is(err, err1), test.ShouldBeTrue)

	dev.MoveStraightFunc = func(ctx context.Context, distanceMm int, mmPerSec float64, block bool) error {
		test.That(t, distanceMm, test.ShouldEqual, m.DistanceMm)
		test.That(t, mmPerSec, test.ShouldEqual, m.MmPerSec)
		test.That(t, block, test.ShouldEqual, m.Block)
		return nil
	}
	err = base.DoMove(context.Background(), m, dev)
	test.That(t, err, test.ShouldBeNil)

	m = base.Move{DistanceMm: 1, Block: true, MmPerSec: 5}
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

	m = base.Move{DistanceMm: 2, AngleDeg: 10, Block: true, DegsPerSec: 5}
	err = base.DoMove(context.Background(), m, dev)
	test.That(t, err, test.ShouldBeNil)

	t.Run("if rotation succeeds but moving straight fails, report rotation", func(t *testing.T) {
		dev.MoveStraightFunc = func(ctx context.Context, distanceMm int, mmPerSec float64, block bool) error {
			return err1
		}

		m = base.Move{DistanceMm: 2, AngleDeg: 10, Block: true, MmPerSec: 5}
		err = base.DoMove(context.Background(), m, dev)
		test.That(t, errors.Is(err, err1), test.ShouldBeTrue)
	})
}
