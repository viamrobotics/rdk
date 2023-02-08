package stepper28byj48

import (
	"context"
	"sync"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	fakeboard "go.viam.com/rdk/components/board/fake"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/config"
)

func Test1(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	logger := golog.NewTestLogger(t)

	b := &fakeboard.Board{GPIOPins: make(map[string]*fakeboard.GPIOPin)}

	mc := Config{}
	c := config.Component{
		Name: "fake_28byj",
	}

	// Create motor with no board and default config
	t.Run("motor initializing test with no board and default config", func(t *testing.T) {
		_, err := newULN(nil, mc, c.Name, logger)
		test.That(t, err, test.ShouldNotBeNil)
	})

	// Create motor with board and default config
	t.Run("gpiostepper initializing test with board and default config", func(t *testing.T) {
		_, err := newULN(b, mc, c.Name, logger)
		test.That(t, err, test.ShouldNotBeNil)
	})

	mc.Pins = PinConfig{In1: "b", In2: "a", In3: "c", In4: "d"}

	_, err := newULN(b, mc, c.Name, logger)
	test.That(t, err, test.ShouldNotBeNil)

	_, err = newULN(b, mc, c.Name, logger)
	test.That(t, err, test.ShouldNotBeNil)

	mc.TicksPerRotation = 200

	mm, err := newULN(b, mc, c.Name, logger)
	test.That(t, err, test.ShouldBeNil)

	m := mm.(*uln2003)

	t.Run("motor test supports position reporting", func(t *testing.T) {
		features, err := m.Properties(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, features[motor.PositionReporting], test.ShouldBeTrue)
	})

	t.Run("motor test isOn functionality", func(t *testing.T) {
		on, powerPct, err := m.IsPowered(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldEqual, false)
		test.That(t, powerPct, test.ShouldEqual, 0.0)
	})

	t.Run("motor testing with positive rpm and positive revolutions", func(t *testing.T) {
		on, powerPct, err := m.IsPowered(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldEqual, true)
		test.That(t, powerPct, test.ShouldEqual, 1.0)

		pos, err := m.Position(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 0)
	})

	t.Run("motor testing with negative rpm and positive revolutions", func(t *testing.T) {
		on, powerPct, err := m.IsPowered(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldEqual, false)
		test.That(t, powerPct, test.ShouldEqual, 0.0)

		pos, err := m.Position(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 0)
	})

	t.Run("motor testing with positive rpm and negative revolutions", func(t *testing.T) {
		on, powerPct, err := m.IsPowered(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldEqual, true)
		test.That(t, powerPct, test.ShouldEqual, 1.0)

		pos, err := m.Position(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 0)
	})

	t.Run("motor testing with negative rpm and negative revolutions", func(t *testing.T) {
		on, powerPct, err := m.IsPowered(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldEqual, false)
		test.That(t, powerPct, test.ShouldEqual, 0.0)

		pos, err := m.Position(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 0)
	})
	t.Run("Ensure stop called when gofor is interrupted", func(t *testing.T) {
		ctx := context.Background()
		var wg sync.WaitGroup
		ctx, cancel := context.WithCancel(ctx)
		wg.Add(1)
		go func() {
			m.GoFor(ctx, 100, 100, map[string]interface{}{})
			wg.Done()
		}()
		cancel()
		wg.Wait()

		test.That(t, ctx.Err(), test.ShouldNotBeNil)
		test.That(t, m.targetStepsPerSecond, test.ShouldEqual, 0)
	})

	t.Run("motor testing with large # of revolutions", func(t *testing.T) {
		on, powerPct, err := m.IsPowered(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldEqual, true)
		test.That(t, powerPct, test.ShouldEqual, 1.0)

		err = m.Stop(ctx, nil)
		test.That(t, err, test.ShouldBeNil)

		pos, err := m.Position(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldBeGreaterThan, 2)
		test.That(t, pos, test.ShouldBeLessThan, 202)
	})

	cancel()
}
