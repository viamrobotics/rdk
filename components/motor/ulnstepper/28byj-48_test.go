package uln28byj

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testBoardName = "fake_board"
)

func setupDependencies(t *testing.T) resource.Dependencies {
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
	deps := make(resource.Dependencies)
	deps[board.Named(testBoardName)] = testBoard
	return deps
}

func TestValid(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	logger := logging.NewTestLogger(t)
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

	c := resource.Config{
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
		properties, err := m.Properties(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, properties.PositionReporting, test.ShouldBeTrue)
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
	logger, obs := logging.NewObservedTestLogger(t)
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

	c := resource.Config{
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
		allObs := obs.All()
		latestLoggedEntry := allObs[len(allObs)-1]
		test.That(t, fmt.Sprint(latestLoggedEntry), test.ShouldContainSubstring, "nearly 0")

		err = m.GoFor(ctx, -.009, 1, nil)
		test.That(t, err, test.ShouldNotBeNil)

		err = m.GoFor(ctx, 146, 1, nil)
		test.That(t, err, test.ShouldBeNil)
		allObs = obs.All()
		latestLoggedEntry = allObs[len(allObs)-1]
		test.That(t, fmt.Sprint(latestLoggedEntry), test.ShouldContainSubstring, "nearly the max")
	})

	cancel()
}

func TestState(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	logger := logging.NewTestLogger(t)
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

	c := resource.Config{
		Name:                "fake_28byj",
		ConvertedAttributes: &mc,
	}
	mm, _ := new28byj(ctx, deps, c, logger)
	m := mm.(*uln28byj)

	t.Run("test state", func(t *testing.T) {
		m.stepPosition = 9
		b := m.theBoard
		var pin1Arr []bool
		var pin2Arr []bool
		var pin3Arr []bool
		var pin4Arr []bool

		arrIn1 := []bool{true, true, false, false, false, false, false, true}
		arrIn2 := []bool{false, true, true, true, false, false, false, false}
		arrIn3 := []bool{false, false, false, true, true, true, false, false}
		arrIn4 := []bool{false, false, false, false, false, true, true, true}

		for i := 0; i < 8; i++ {
			// moving forward.
			err := m.doStep(ctx, true)
			test.That(t, err, test.ShouldBeNil)

			PinOut1, err := b.GPIOPinByName("1")
			test.That(t, err, test.ShouldBeNil)
			pinStruct, ok := PinOut1.(*mockGPIOPin)
			test.That(t, ok, test.ShouldBeTrue)
			pin1Arr = pinStruct.pinStates

			PinOut2, err := b.GPIOPinByName("2")
			test.That(t, err, test.ShouldBeNil)
			pinStruct2, ok := PinOut2.(*mockGPIOPin)
			test.That(t, ok, test.ShouldBeTrue)
			pin2Arr = pinStruct2.pinStates

			PinOut3, err := b.GPIOPinByName("3")
			test.That(t, err, test.ShouldBeNil)
			pinStruct3, ok := PinOut3.(*mockGPIOPin)
			test.That(t, ok, test.ShouldBeTrue)
			pin3Arr = pinStruct3.pinStates

			PinOut4, err := b.GPIOPinByName("4")
			test.That(t, err, test.ShouldBeNil)
			pinStruct4, ok := PinOut4.(*mockGPIOPin)
			test.That(t, ok, test.ShouldBeTrue)
			pin4Arr = pinStruct4.pinStates
		}

		m.logger.Info("pin1Arr ", pin1Arr)
		m.logger.Info("pin2Arr ", pin2Arr)
		m.logger.Info("pin3Arr ", pin3Arr)
		m.logger.Info("pin4Arr ", pin4Arr)

		test.That(t, pin1Arr, test.ShouldResemble, arrIn1)
		test.That(t, pin2Arr, test.ShouldResemble, arrIn2)
		test.That(t, pin3Arr, test.ShouldResemble, arrIn3)
		test.That(t, pin4Arr, test.ShouldResemble, arrIn4)
	})

	cancel()
}

type mockGPIOPin struct {
	board.GPIOPin
	pinStates []bool
}

func (m *mockGPIOPin) Set(ctx context.Context, high bool, extra map[string]interface{}) error {
	m.pinStates = append(m.pinStates, high)
	return nil
}
