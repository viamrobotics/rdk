package base_test

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/base"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

const (
	testBaseName    = "base1"
	testBaseName2   = "base2"
	failBaseName    = "base3"
	fakeBaseName    = "base4"
	missingBaseName = "base5"
)

func setupInjectRobot() *inject.Robot {
	base1 := &mock{Name: testBaseName}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		switch name {
		case base.Named(testBaseName):
			return base1, nil
		case base.Named(fakeBaseName):
			return "not a base", nil
		default:
			return nil, rutils.NewResourceNotFoundError(name)
		}
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{base.Named(testBaseName), arm.Named("arm1")}
	}
	return r
}

func TestGenericDo(t *testing.T) {
	r := setupInjectRobot()

	b, err := base.FromRobot(r, testBaseName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, b, test.ShouldNotBeNil)

	command := map[string]interface{}{"cmd": "test", "data1": 500}
	ret, err := b.Do(context.Background(), command)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ret, test.ShouldEqual, command)
}

func TestFromRobot(t *testing.T) {
	r := setupInjectRobot()

	res, err := base.FromRobot(r, testBaseName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldNotBeNil)

	result, err := res.(base.LocalBase).GetWidth(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldEqual, width)

	res, err = base.FromRobot(r, fakeBaseName)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("Base", "string"))
	test.That(t, res, test.ShouldBeNil)

	res, err = base.FromRobot(r, missingBaseName)
	test.That(t, err, test.ShouldBeError, rutils.NewResourceNotFoundError(base.Named(missingBaseName)))
	test.That(t, res, test.ShouldBeNil)
}

func TestNamesFromRobot(t *testing.T) {
	r := setupInjectRobot()

	names := base.NamesFromRobot(r)
	test.That(t, names, test.ShouldResemble, []string{testBaseName})
}

func TestBaseNamed(t *testing.T) {
	baseName := base.Named("test_base")
	test.That(t, baseName.String(), test.ShouldResemble, "rdk:component:base/test_base")
	test.That(t, baseName.Subtype, test.ShouldResemble, base.Subtype)
}

func TestWrapWithReconfigurable(t *testing.T) {
	var actualBase1 base.Base = &mock{Name: testBaseName}
	reconfBase1, err := base.WrapWithReconfigurable(actualBase1)
	test.That(t, err, test.ShouldBeNil)

	_, err = base.WrapWithReconfigurable(nil)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("LocalBase", nil))

	reconfBase2, err := base.WrapWithReconfigurable(reconfBase1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfBase2, test.ShouldEqual, reconfBase1)
}

func TestReconfigurableBase(t *testing.T) {
	actualBase1 := &mock{Name: testBaseName}
	reconfBase1, err := base.WrapWithReconfigurable(actualBase1)
	test.That(t, err, test.ShouldBeNil)

	actualBase2 := &mock{Name: testBaseName2}
	reconfBase2, err := base.WrapWithReconfigurable(actualBase2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualBase1.reconfCount, test.ShouldEqual, 0)

	err = reconfBase1.Reconfigure(context.Background(), reconfBase2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfBase1, test.ShouldResemble, reconfBase2)
	test.That(t, actualBase1.reconfCount, test.ShouldEqual, 1)

	test.That(t, actualBase1.widthCount, test.ShouldEqual, 0)
	test.That(t, actualBase2.widthCount, test.ShouldEqual, 0)
	result, err := reconfBase1.(base.LocalBase).GetWidth(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldEqual, width)
	test.That(t, actualBase1.widthCount, test.ShouldEqual, 0)
	test.That(t, actualBase2.widthCount, test.ShouldEqual, 1)

	err = reconfBase1.Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected *base.reconfigurableBase")
}

func TestClose(t *testing.T) {
	actualBase1 := &mock{Name: testBaseName}
	reconfBase1, _ := base.WrapWithReconfigurable(actualBase1)

	test.That(t, actualBase1.reconfCount, test.ShouldEqual, 0)
	test.That(t, utils.TryClose(context.Background(), reconfBase1), test.ShouldBeNil)
	test.That(t, actualBase1.reconfCount, test.ShouldEqual, 1)
}

const width = 10

type mock struct {
	base.LocalBase
	Name        string
	widthCount  int
	reconfCount int
}

func (m *mock) GetWidth(ctx context.Context) (int, error) {
	m.widthCount++
	return width, nil
}

func (m *mock) Close() { m.reconfCount++ }

func (m *mock) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
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
