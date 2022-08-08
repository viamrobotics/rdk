package fake

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/component/motor"
)

func TestEncoder(t *testing.T) {
	ctx := context.Background()

	e := Encoder{}

	// Get and set position
	pos, err := e.GetPosition(ctx)
	test.That(t, pos, test.ShouldEqual, 0)
	test.That(t, err, test.ShouldBeNil)

	err = e.SetPosition(ctx, 1)
	test.That(t, err, test.ShouldBeNil)

	pos, err = e.GetPosition(ctx)
	test.That(t, pos, test.ShouldEqual, 1)
	test.That(t, err, test.ShouldBeNil)

	// ResetZeroPosition
	err = e.ResetZeroPosition(ctx, 0)
	test.That(t, err, test.ShouldBeNil)

	pos, err = e.GetPosition(ctx)
	test.That(t, pos, test.ShouldEqual, 0)
	test.That(t, err, test.ShouldBeNil)

	// Set Speed
	err = e.SetSpeed(ctx, 1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, e.speed, test.ShouldEqual, 1)
}

func TestMotorInit(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	m := &Motor{
		Encoder:           &Encoder{},
		Logger:            logger,
		PositionReporting: true,
		TicksPerRotation:  1,
		MaxRPM:            60,
	}

	pos, err := m.GetPosition(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldEqual, 0)

	featureMap, err := m.GetFeatures(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, featureMap[motor.PositionReporting], test.ShouldBeTrue)
}

func TestGoFor(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	m := &Motor{
		Encoder:           &Encoder{},
		Logger:            logger,
		PositionReporting: true,
		TicksPerRotation:  1,
		MaxRPM:            60,
	}

	m.Encoder.Start(ctx, &m.activeBackgroundWorkers)
	err := m.GoFor(ctx, 60, 1, nil)
	test.That(t, err, test.ShouldBeNil)
}

func TestGoTo(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	m := &Motor{
		Encoder:           &Encoder{},
		Logger:            logger,
		PositionReporting: true,
		TicksPerRotation:  1,
		MaxRPM:            60,
	}

	m.Encoder.Start(ctx, &m.activeBackgroundWorkers)
	err := m.GoTo(ctx, 60, 1, nil)
	test.That(t, err, test.ShouldBeNil)
}

func TestGoTillStop(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	m := &Motor{
		Encoder:           &Encoder{},
		Logger:            logger,
		PositionReporting: true,
		TicksPerRotation:  1,
		MaxRPM:            60,
	}

	err := m.GoTillStop(ctx, 0, func(ctx context.Context) bool { return false })
	test.That(t, err, test.ShouldNotBeNil)
}

func TestResetZeroPosition(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	m := &Motor{
		Encoder:           &Encoder{},
		Logger:            logger,
		PositionReporting: true,
		TicksPerRotation:  1,
		MaxRPM:            60,
	}

	err := m.ResetZeroPosition(ctx, 0, nil)
	test.That(t, err, test.ShouldBeNil)

	pos, err := m.GetPosition(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldEqual, 0)
}

func TestPower(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	m := &Motor{
		Encoder:           &Encoder{},
		Logger:            logger,
		PositionReporting: true,
		TicksPerRotation:  1,
		MaxRPM:            60,
	}

	err := m.SetPower(ctx, 1.0, nil)
	test.That(t, err, test.ShouldBeNil)

	powerPct := m.PowerPct()
	test.That(t, powerPct, test.ShouldEqual, 1.0)

	dir := m.Direction()
	test.That(t, dir, test.ShouldEqual, 1)

	isPowered, err := m.IsPowered(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, isPowered, test.ShouldEqual, true)

	isMoving, err := m.IsMoving(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, isMoving, test.ShouldEqual, true)

	err = m.Stop(ctx, nil)
	test.That(t, err, test.ShouldBeNil)

	powerPct = m.PowerPct()
	test.That(t, powerPct, test.ShouldEqual, 0.0)
}
