package gpiostepper

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	fakeboard "go.viam.com/rdk/component/board/fake"
	"go.viam.com/rdk/component/motor"
)

func Test1(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	logger := golog.NewTestLogger(t)

	b := &fakeboard.Board{}

	mc := motor.Config{}

	// Create motor with no board and default config
	t.Run("gpiostepper initializing test with no board and default config", func(t *testing.T) {
		_, err := newGPIOStepper(ctx, nil, mc, logger)
		test.That(t, err, test.ShouldNotBeNil)
	})

	// Create motor with board and default config
	t.Run("gpiostepper initializing test with board and default config", func(t *testing.T) {
		_, err := newGPIOStepper(ctx, b, mc, logger)
		test.That(t, err, test.ShouldNotBeNil)
	})

	mc.Pins = map[string]string{"dir": "b"}

	_, err := newGPIOStepper(ctx, b, mc, logger)
	test.That(t, err, test.ShouldNotBeNil)

	mc.Pins["step"] = "c"

	m, err := newGPIOStepper(ctx, b, mc, logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("motor test isOn functionality", func(t *testing.T) {
		on, err := m.IsOn(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldEqual, false)
	})

	t.Run("motor testing with positive rpm and positive revolutions", func(t *testing.T) {
		err = m.GoFor(ctx, 100, 2)
		test.That(t, err, test.ShouldBeNil)

		on, err := m.IsOn(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldEqual, true)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			on, err = m.IsOn(ctx)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, on, test.ShouldEqual, false)
		})

		pos, err := m.Position(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 2)
	})

	t.Run("motor testing with negative rpm and positive revolutions", func(t *testing.T) {
		err = m.GoFor(ctx, -100, 2)
		test.That(t, err, test.ShouldBeNil)

		on, err := m.IsOn(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldEqual, true)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			on, err = m.IsOn(ctx)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, on, test.ShouldEqual, false)
		})

		pos, err := m.Position(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 0)
	})

	t.Run("motor testing with positive rpm and negative revolutions", func(t *testing.T) {
		err = m.GoFor(ctx, 100, -2)
		test.That(t, err, test.ShouldBeNil)

		on, err := m.IsOn(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldEqual, true)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			on, err = m.IsOn(ctx)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, on, test.ShouldEqual, false)
		})

		pos, err := m.Position(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, -2)
	})

	t.Run("motor testing with negative rpm and negative revolutions", func(t *testing.T) {
		err = m.GoFor(ctx, -100, -2)
		test.That(t, err, test.ShouldBeNil)

		on, err := m.IsOn(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldEqual, true)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			on, err = m.IsOn(ctx)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, on, test.ShouldEqual, false)
		})

		pos, err := m.Position(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 0)
	})

	t.Run("motor testing with large # of revolutions", func(t *testing.T) {
		err = m.GoFor(ctx, 100, 200)
		test.That(t, err, test.ShouldBeNil)

		on, err := m.IsOn(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldEqual, true)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos, err := m.Position(ctx)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, pos, test.ShouldBeGreaterThan, 2)
		})

		err = m.Stop(ctx)
		test.That(t, err, test.ShouldBeNil)

		pos, err := m.Position(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldBeGreaterThan, 2)
		test.That(t, pos, test.ShouldBeLessThan, 202)
	})
	cancel()
}
