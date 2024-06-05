package fake

import (
	"context"
	"fmt"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/encoder/fake"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/resource"
)

func TestMotorInit(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	enc, err := fake.NewEncoder(context.Background(), resource.Config{
		ConvertedAttributes: &fake.Config{},
	}, logger)
	test.That(t, err, test.ShouldBeNil)
	m := &Motor{
		Encoder:           enc.(fake.Encoder),
		Logger:            logger,
		PositionReporting: true,
		MaxRPM:            60,
		TicksPerRotation:  1,
		OpMgr:             operation.NewSingleOperationManager(),
	}

	pos, err := m.Position(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldEqual, 0)

	properties, err := m.Properties(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, properties.PositionReporting, test.ShouldBeTrue)
}

func TestGoFor(t *testing.T) {
	logger, obs := logging.NewObservedTestLogger(t)
	ctx := context.Background()

	enc, err := fake.NewEncoder(context.Background(), resource.Config{
		ConvertedAttributes: &fake.Config{},
	}, logger)
	test.That(t, err, test.ShouldBeNil)
	m := &Motor{
		Encoder:           enc.(fake.Encoder),
		Logger:            logger,
		PositionReporting: true,
		MaxRPM:            60,
		TicksPerRotation:  1,
		OpMgr:             operation.NewSingleOperationManager(),
	}

	err = m.GoFor(ctx, 0, 1, nil)
	allObs := obs.All()
	latestLoggedEntry := allObs[len(allObs)-1]
	test.That(t, fmt.Sprint(latestLoggedEntry), test.ShouldContainSubstring, "nearly 0")
	test.That(t, err, test.ShouldBeError, motor.NewZeroRPMError())

	err = m.GoFor(ctx, 60, 1, nil)
	test.That(t, err, test.ShouldBeNil)
	allObs = obs.All()
	latestLoggedEntry = allObs[1]
	test.That(t, fmt.Sprint(latestLoggedEntry), test.ShouldContainSubstring, "nearly the max")

	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		pos, err := m.Position(ctx, nil)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, pos, test.ShouldEqual, 1)
	})
}

func TestGoTo(t *testing.T) {
	logger, obs := logging.NewObservedTestLogger(t)
	ctx := context.Background()

	enc, err := fake.NewEncoder(context.Background(), resource.Config{
		ConvertedAttributes: &fake.Config{},
	}, logger)
	test.That(t, err, test.ShouldBeNil)
	m := &Motor{
		Encoder:           enc.(fake.Encoder),
		Logger:            logger,
		PositionReporting: true,
		MaxRPM:            60,
		TicksPerRotation:  1,
		OpMgr:             operation.NewSingleOperationManager(),
	}

	err = m.GoTo(ctx, 60, 1, nil)
	test.That(t, err, test.ShouldBeNil)
	allObs := obs.All()
	latestLoggedEntry := allObs[0]
	test.That(t, fmt.Sprint(latestLoggedEntry), test.ShouldContainSubstring, "nearly the max")

	err = m.GoTo(ctx, 0, 1, nil)
	test.That(t, err, test.ShouldBeNil)
	allObs = obs.All()
	latestLoggedEntry = allObs[3]
	test.That(t, fmt.Sprint(latestLoggedEntry), test.ShouldContainSubstring, "nearly 0")

	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		pos, err := m.Position(ctx, nil)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, pos, test.ShouldEqual, 1)
	})
}

func TestResetZeroPosition(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	enc, err := fake.NewEncoder(context.Background(), resource.Config{
		ConvertedAttributes: &fake.Config{},
	}, logger)
	test.That(t, err, test.ShouldBeNil)
	m := &Motor{
		Encoder:           enc.(fake.Encoder),
		Logger:            logger,
		PositionReporting: true,
		MaxRPM:            60,
		TicksPerRotation:  1,
		OpMgr:             operation.NewSingleOperationManager(),
	}

	err = m.ResetZeroPosition(ctx, 0, nil)
	test.That(t, err, test.ShouldBeNil)

	pos, err := m.Position(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldEqual, 0)
}

func TestPower(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	enc, err := fake.NewEncoder(context.Background(), resource.Config{
		ConvertedAttributes: &fake.Config{},
	}, logger)
	test.That(t, err, test.ShouldBeNil)
	m := &Motor{
		Encoder:           enc.(fake.Encoder),
		Logger:            logger,
		PositionReporting: true,
		MaxRPM:            60,
		TicksPerRotation:  1,
		OpMgr:             operation.NewSingleOperationManager(),
	}

	err = m.SetPower(ctx, 1.0, nil)
	test.That(t, err, test.ShouldBeNil)

	powerPct := m.PowerPct()
	test.That(t, powerPct, test.ShouldEqual, 1.0)

	dir := m.Direction()
	test.That(t, dir, test.ShouldEqual, 1)

	isPowered, powerPctReported, err := m.IsPowered(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, isPowered, test.ShouldEqual, true)
	test.That(t, powerPctReported, test.ShouldEqual, powerPct)

	isMoving, err := m.IsMoving(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, isMoving, test.ShouldEqual, true)

	err = m.Stop(ctx, nil)
	test.That(t, err, test.ShouldBeNil)

	powerPct = m.PowerPct()
	test.That(t, powerPct, test.ShouldEqual, 0.0)
}
