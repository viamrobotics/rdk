package gpiostepper

import (
	"context"
	"testing"

	"go.viam.com/core/motor"
	"go.viam.com/core/robots/fake"

	"github.com/edaniels/golog"

	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	pb "go.viam.com/core/proto/api/v1"
)

func Test1(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	logger := golog.NewTestLogger(t)

	b := &fake.Board{}

	mc := motor.Config{}

	_, err := newGPIOStepper(ctx, nil, mc, logger)
	test.That(t, err, test.ShouldNotBeNil)

	_, err = newGPIOStepper(ctx, b, mc, logger)
	test.That(t, err, test.ShouldNotBeNil)

	mc.Pins = map[string]string{"dir": "b"}

	_, err = newGPIOStepper(ctx, b, mc, logger)
	test.That(t, err, test.ShouldNotBeNil)

	mc.Pins["step"] = "c"

	m, err := newGPIOStepper(ctx, b, mc, logger)
	test.That(t, err, test.ShouldBeNil)

	on, err := m.IsOn(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, on, test.ShouldEqual, false)

	err = m.GoFor(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 100, 2)
	test.That(t, err, test.ShouldBeNil)
	on, err = m.IsOn(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, on, test.ShouldEqual, true)

	testutils.WaitForAssertion(t, func(t testing.TB) {
		on, err = m.IsOn(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldEqual, false)
	})

	pos, err := m.Position(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldEqual, 2)

	err = m.GoFor(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 100, 200)
	test.That(t, err, test.ShouldBeNil)
	on, err = m.IsOn(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, on, test.ShouldEqual, true)

	testutils.WaitForAssertion(t, func(t testing.TB) {
		pos, err = m.Position(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldBeGreaterThan, 2)
	})

	err = m.Off(ctx)
	test.That(t, err, test.ShouldBeNil)

	pos, err = m.Position(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldBeGreaterThan, 2)
	test.That(t, pos, test.ShouldBeLessThan, 202)

	cancel()

}
