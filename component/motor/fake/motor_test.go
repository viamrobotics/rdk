package fake

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/test"
)

func TestEncoder(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	e := fakeEncoder{logger: logger}

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

func TestMotor(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	mcfg := motor.Config{TicksPerRotation: 1, MaxRPM: 60}
	m := &Motor{logger: logger, encoder: fakeEncoder{logger: logger}, positionReporting: true, cfg: mcfg}

	// Test initial position/features
	pos, err := m.GetPosition(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldEqual, 0)

	featureMap, err := m.GetFeatures(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, featureMap[motor.PositionReporting], test.ShouldBeTrue)

	// Test GoFor
	err = m.GoFor(ctx, 1, 1)
	test.That(t, err, test.ShouldBeNil)

	pos, err = m.GetPosition(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldEqual, 1)

	// Test GoTo
	err = m.GoTo(ctx, 1, 2)
	test.That(t, err, test.ShouldBeNil)

	pos, err = m.GetPosition(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldEqual, 2)

	// Test GoTillStop
	err = m.GoTillStop(ctx, 0, func(ctx context.Context) bool {return false})
	test.That(t, err, test.ShouldNotBeNil)

	// Test ResetZeroPosition
	err = m.ResetZeroPosition(ctx, 0)
	test.That(t, err, test.ShouldBeNil)

	pos, err = m.GetPosition(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldEqual, 0)

	// Test SetPower
	err = m.SetPower(ctx, 1.0)
	test.That(t, err, test.ShouldBeNil)

	powerPct := m.PowerPct()
	test.That(t, powerPct, test.ShouldEqual, 1.0)

	dir := m.Direction()
	test.That(t, dir, test.ShouldEqual, 1)

	isPowered, err := m.IsPowered(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, isPowered, test.ShouldEqual, true)

	isMoving, err := m.IsMoving(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, isMoving, test.ShouldEqual, true)

	err = m.Stop(ctx)
	test.That(t, err, test.ShouldBeNil)

	powerPct = m.PowerPct()
	test.That(t, powerPct, test.ShouldEqual, 0.0)
}