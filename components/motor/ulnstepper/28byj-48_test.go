package unipolarfivewirestepper

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testBoardName = "fake_board"
)

func setupDependencies(t *testing.T) registry.Dependencies {
	t.Helper()

	testBoard := &inject.Board{}
	injectGPIOPin := &inject.GPIOPin{}
	testBoard.GPIOPinByNameFunc = func(pin string) (board.GPIOPin, error) {
		return injectGPIOPin, nil
	}
	injectGPIOPin.GetFunc = func(ctx context.Context, extra map[string]interface{}) (bool, error) {
		return true, nil
	}
	deps := make(registry.Dependencies)
	deps[board.Named(testBoardName)] = testBoard
	return deps
}

func TestValid(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	logger := golog.NewTestLogger(t)
	deps := setupDependencies(t)

	mc := Config{
		Pins: PinConfig{
			In1: "b",
			In2: "a",
			In3: "c",
			In4: "d",
		},
		BoardName: testBoardName,
	}

	c := config.Component{
		Name:                "fake_28byj",
		ConvertedAttributes: &mc,
	}

	// Create motor with no board and default config
	t.Run("motor initializing test with no board and default config", func(t *testing.T) {
		_, err := new28byj(deps, c, c.Name, logger)
		test.That(t, err, test.ShouldNotBeNil)
	})

	// Create motor with board and default config
	t.Run("gpiostepper initializing test with board and default config", func(t *testing.T) {
		_, err := new28byj(deps, c, c.Name, logger)
		test.That(t, err, test.ShouldNotBeNil)
	})
	_, err := new28byj(deps, c, c.Name, logger)
	test.That(t, err, test.ShouldNotBeNil)

	mc.TicksPerRotation = 200

	mm, err := new28byj(deps, c, c.Name, logger)
	test.That(t, err, test.ShouldBeNil)

	m := mm.(*uln28byj)

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
		test.That(t, on, test.ShouldEqual, false)
		test.That(t, powerPct, test.ShouldEqual, 0.0)

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
		test.That(t, on, test.ShouldEqual, false)
		test.That(t, powerPct, test.ShouldEqual, 0.0)

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

	t.Run("motor testing with large # of revolutions", func(t *testing.T) {
		on, powerPct, err := m.IsPowered(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldEqual, false)
		test.That(t, powerPct, test.ShouldEqual, 0.0)

		err = m.Stop(ctx, nil)
		test.That(t, err, test.ShouldBeNil)

		pos, err := m.Position(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldBeGreaterThanOrEqualTo, 0)
		test.That(t, pos, test.ShouldBeLessThan, 202)
	})

	cancel()
}
