package unipolarfivewirestepper

import (
	"context"
	"errors"
	"testing"
	"time"

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
	in1 := &mockGPIOPin{}
	in2 := &mockGPIOPin{}
	in3 := &mockGPIOPin{}
	in4 := &mockGPIOPin{}

	testBoard.GPIOPinByNameFunc = func(pin string) (board.GPIOPin, error) {
		switch pin {
		case "1":
			return in1, nil
		case "2":
			return in2, nil
		case "3":
			return in3, nil
		case "4":
			return in4, nil
		}
		return nil, errors.New("pin name not found")
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
			In1: "1",
			In2: "2",
			In3: "3",
			In4: "4",
		},
		BoardName: testBoardName,
	}

	c := config.Component{
		Name:                "fake_28byj",
		ConvertedAttributes: &mc,
	}

	// Create motor with no board and default config
	t.Run("motor initializing test with no board and default config", func(t *testing.T) {
		_, err := new28byj(ctx, deps, c, logger)
		test.That(t, err, test.ShouldNotBeNil)
	})

	// Create motor with board and default config
	t.Run("gpiostepper initializing test with board and default config", func(t *testing.T) {
		_, err := new28byj(ctx, deps, c, logger)
		test.That(t, err, test.ShouldNotBeNil)
	})
	_, err := new28byj(ctx, deps, c, logger)
	test.That(t, err, test.ShouldNotBeNil)

	mc.TicksPerRotation = 200

	mm, err := new28byj(ctx, deps, c, logger)
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

func TestFunctions(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	logger := golog.NewTestLogger(t)
	deps := setupDependencies(t)

	mc := Config{
		Pins: PinConfig{
			In1: "1",
			In2: "2",
			In3: "3",
			In4: "4",
		},
		BoardName:        testBoardName,
		TicksPerRotation: 100,
	}

	c := config.Component{
		Name:                "fake_28byj",
		ConvertedAttributes: &mc,
	}
	mm, _ := new28byj(ctx, deps, c, logger)
	m := mm.(*uln28byj)

	t.Run("test goMath", func(t *testing.T) {
		targetPos, stepperdelay := m.goMath(100, 100)
		test.That(t, targetPos, test.ShouldEqual, 10000)
		test.That(t, stepperdelay, test.ShouldEqual, (6 * time.Millisecond))

		targetPos, stepperdelay = m.goMath(-100, 100)
		test.That(t, targetPos, test.ShouldEqual, -10000)
		test.That(t, stepperdelay, test.ShouldEqual, (6 * time.Millisecond))

		targetPos, stepperdelay = m.goMath(-100, -100)
		test.That(t, targetPos, test.ShouldEqual, 10000)
		test.That(t, stepperdelay, test.ShouldEqual, (6 * time.Millisecond))

		targetPos, stepperdelay = m.goMath(-2, 50)
		test.That(t, targetPos, test.ShouldEqual, -5000)
		test.That(t, stepperdelay, test.ShouldEqual, (300 * time.Millisecond))

		targetPos, stepperdelay = m.goMath(1, 400)
		test.That(t, targetPos, test.ShouldEqual, 40000)
		test.That(t, stepperdelay, test.ShouldEqual, (600 * time.Millisecond))

		targetPos, stepperdelay = m.goMath(400, 2)
		test.That(t, targetPos, test.ShouldEqual, 200)
		test.That(t, stepperdelay, test.ShouldEqual, (1500 * time.Microsecond))

		targetPos, stepperdelay = m.goMath(0, 2)
		test.That(t, targetPos, test.ShouldEqual, 200)
		test.That(t, stepperdelay, test.ShouldEqual, (100 * time.Microsecond))
	})

	t.Run("test position", func(t *testing.T) {
		m.stepPosition = 3
		pos, err := m.Position(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 0.03)

		m.stepPosition = -3
		pos, err = m.Position(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, -0.03)

		m.stepPosition = 0
		pos, err = m.Position(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 0)
	})

	t.Run("test GoFor", func(t *testing.T) {
		err := m.GoFor(ctx, 0, 1, nil)
		test.That(t, err, test.ShouldBeError)

		err = m.GoFor(ctx, -.009, 1, nil)
		test.That(t, err, test.ShouldBeNil)
	})

	cancel()
}

func TestState(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	logger := golog.NewTestLogger(t)
	deps := setupDependencies(t)

	mc := Config{
		Pins: PinConfig{
			In1: "1",
			In2: "2",
			In3: "3",
			In4: "4",
		},
		BoardName:        testBoardName,
		TicksPerRotation: 100,
	}

	c := config.Component{
		Name:                "fake_28byj",
		ConvertedAttributes: &mc,
	}
	mm, _ := new28byj(ctx, deps, c, logger)
	m := mm.(*uln28byj)
	m.logger.Info("passed m")

	t.Run("test state", func(t *testing.T) {
		m.stepPosition = 9

		err := m.doStep(ctx, true)
		test.That(t, err, test.ShouldBeNil)
		b := m.theBoard

		arrstep2 := [4]bool{
			true,
			false,
			false,
			false,
		}

		pinOutput, err := b.GPIOPinByName(mc.Pins.In1)
		test.That(t, err, test.ShouldBeNil)
		pinStruct, ok := pinOutput.(*mockGPIOPin)
		test.That(t, ok, test.ShouldBeTrue)
		currstate := pinStruct.pinStates
		test.That(t, currstate[0], test.ShouldEqual, arrstep2[0])

		pinOutput2, err := b.GPIOPinByName("2")
		test.That(t, err, test.ShouldBeNil)
		pinStruct, ok = pinOutput2.(*mockGPIOPin)
		test.That(t, ok, test.ShouldBeTrue)
		currstate2 := pinStruct.pinStates
		test.That(t, currstate2[0], test.ShouldEqual, arrstep2[1])

		pinOutput3, err := b.GPIOPinByName("3")
		test.That(t, err, test.ShouldBeNil)
		pinStruct, ok = pinOutput3.(*mockGPIOPin)
		test.That(t, ok, test.ShouldBeTrue)
		currstate2 = pinStruct.pinStates
		test.That(t, currstate2[0], test.ShouldEqual, arrstep2[2])

		pinOutput4, err := b.GPIOPinByName("4")
		test.That(t, err, test.ShouldBeNil)
		pinStruct, ok = pinOutput4.(*mockGPIOPin)
		test.That(t, ok, test.ShouldBeTrue)
		currstate3 := pinStruct.pinStates
		test.That(t, currstate3[0], test.ShouldEqual, arrstep2[3])

		err = m.doStep(ctx, true)
		test.That(t, err, test.ShouldBeNil)

		// arr_step3 := [4]bool{
		// 	true,
		// 	true,
		// 	false,
		// 	false,
		// }

		// pinOutput, _ = b.GPIOPinByName("1")
		// pinStruct, ok = pinOutput.(*mockGPIOPin)
		// test.That(t, ok, test.ShouldBeTrue)
		// curr_state = pinStruct.pinStates
		// test.That(t, curr_state[0], test.ShouldEqual, arr_step3[0])

		// pinOutput2, err = b.GPIOPinByName("2")
		// test.That(t, err, test.ShouldBeNil)
		// pinStruct, ok = pinOutput2.(*mockGPIOPin)
		// test.That(t, ok, test.ShouldBeTrue)
		// curr_state2 = pinStruct.pinStates
		// test.That(t, curr_state2[0], test.ShouldEqual, arr_step3[1])

		// pinOutput3, err = b.GPIOPinByName("3")
		// test.That(t, err, test.ShouldBeNil)
		// pinStruct, ok = pinOutput3.(*mockGPIOPin)
		// test.That(t, ok, test.ShouldBeTrue)
		// curr_state2 = pinStruct.pinStates
		// test.That(t, curr_state2[0], test.ShouldEqual, arr_step3[2])

		// pinOutput4, err = b.GPIOPinByName("4")
		// test.That(t, err, test.ShouldBeNil)
		// pinStruct, ok = pinOutput4.(*mockGPIOPin)
		// test.That(t, ok, test.ShouldBeTrue)
		// curr_state3 = pinStruct.pinStates
		// test.That(t, curr_state3[0], test.ShouldEqual, arr_step3[3])
	})

	cancel()
}

type mockGPIOPin struct {
	board.GPIOPin
	pinStates []bool
}

func (m *mockGPIOPin) Set(ctx context.Context, high bool, extra map[string]interface{}) error {
	m.pinStates = append(m.pinStates, high)
	golog.Global().Info(m.pinStates)
	return nil
}
