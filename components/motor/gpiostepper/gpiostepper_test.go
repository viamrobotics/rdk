package gpiostepper

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/testutils"

	fakeboard "go.viam.com/rdk/components/board/fake"
	"go.viam.com/rdk/resource"
)

func TestConfigs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := golog.NewTestLogger(t)
	c := resource.Config{
		Name: "fake_gpiostepper",
	}

	goodConfig := Config{
		Pins:             PinConfig{Direction: "b", Step: "c", EnablePinHigh: "d", EnablePinLow: "e"},
		TicksPerRotation: 200,
		BoardName:        "brd",
		StepperDelay:     30,
	}

	pinB := &fakeboard.GPIOPin{}
	pinC := &fakeboard.GPIOPin{}
	pinD := &fakeboard.GPIOPin{}
	pinE := &fakeboard.GPIOPin{}
	pinMap := map[string]*fakeboard.GPIOPin{
		"b": pinB,
		"c": pinC,
		"d": pinD,
		"e": pinE,
	}
	b := fakeboard.Board{GPIOPins: pinMap}

	t.Run("config validation good", func(t *testing.T) {
		mc := goodConfig

		deps, err := mc.Validate("")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, deps, test.ShouldResemble, []string{"brd"})

		// remove optional fields
		mc.StepperDelay = 0
		deps, err = mc.Validate("")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, deps, test.ShouldResemble, []string{"brd"})

		mc.Pins.EnablePinHigh = ""
		mc.Pins.EnablePinLow = ""
		deps, err = mc.Validate("")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, deps, test.ShouldResemble, []string{"brd"})
	})

	t.Run("config missing required pins", func(t *testing.T) {
		mc := goodConfig
		mc.Pins.Direction = ""

		_, err := mc.Validate("")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, utils.NewConfigValidationFieldRequiredError("", "dir"))

		mc = goodConfig
		mc.Pins.Step = ""
		_, err = mc.Validate("")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, utils.NewConfigValidationFieldRequiredError("", "step"))
	})

	t.Run("config missing ticks", func(t *testing.T) {
		mc := goodConfig
		mc.TicksPerRotation = 0

		_, err := mc.Validate("")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, utils.NewConfigValidationFieldRequiredError("", "ticks_per_rotation"))
	})

	t.Run("config missing board", func(t *testing.T) {
		mc := goodConfig
		mc.BoardName = ""

		_, err := mc.Validate("")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, utils.NewConfigValidationFieldRequiredError("", "board"))
	})

	t.Run("initializing good with enable pins", func(t *testing.T) {
		m, err := newGPIOStepper(ctx, &b, goodConfig, c.ResourceName(), logger)
		s := m.(*gpioStepper)

		test.That(t, err, test.ShouldBeNil)
		defer m.Close(ctx)

		test.That(t, s.minDelay, test.ShouldEqual, 30*time.Microsecond)
		test.That(t, s.stepsPerRotation, test.ShouldEqual, 200)
		test.That(t, s.dirPin, test.ShouldEqual, pinB)
		test.That(t, s.stepPin, test.ShouldEqual, pinC)
		test.That(t, s.enablePinHigh, test.ShouldEqual, pinD)
		test.That(t, s.enablePinLow, test.ShouldEqual, pinE)
	})

	t.Run("initializing good without enable pins", func(t *testing.T) {
		mc := goodConfig
		mc.Pins.EnablePinHigh = ""
		mc.Pins.EnablePinLow = ""

		m, err := newGPIOStepper(ctx, &b, mc, c.ResourceName(), logger)
		s := m.(*gpioStepper)

		test.That(t, err, test.ShouldBeNil)
		defer m.Close(ctx)

		test.That(t, s.dirPin, test.ShouldEqual, pinB)
		test.That(t, s.stepPin, test.ShouldEqual, pinC)

		// fake board auto-creates new pins by default. just make sure they're not what they would normally be.
		test.That(t, s.enablePinHigh, test.ShouldNotEqual, pinD)
		test.That(t, s.enablePinLow, test.ShouldNotEqual, pinE)
	})

	t.Run("initializing with no board", func(t *testing.T) {
		_, err := newGPIOStepper(ctx, nil, goodConfig, c.ResourceName(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "board is required")
	})

	t.Run("initializing without ticks per rotation", func(t *testing.T) {
		mc := goodConfig
		mc.TicksPerRotation = 0

		_, err := newGPIOStepper(ctx, &b, mc, c.ResourceName(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "expected ticks_per_rotation")
	})

	t.Run("initializing with negative stepper delay", func(t *testing.T) {
		mc := goodConfig
		mc.StepperDelay = -100

		m, err := newGPIOStepper(ctx, &b, mc, c.ResourceName(), logger)
		s := m.(*gpioStepper)

		test.That(t, err, test.ShouldBeNil)
		defer m.Close(ctx)
		test.That(t, s.minDelay, test.ShouldEqual, 0*time.Microsecond)
	})

	t.Run("motor supports position reporting", func(t *testing.T) {
		m, err := newGPIOStepper(ctx, &b, goodConfig, c.ResourceName(), logger)
		test.That(t, err, test.ShouldBeNil)
		defer m.Close(ctx)

		properties, err := m.Properties(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, properties.PositionReporting, test.ShouldBeTrue)
	})
}

// Warning: Tests that run goForInternal may be racy.
func TestRunning(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger, obs := golog.NewObservedTestLogger(t)
	c := resource.Config{
		Name: "fake_gpiostepper",
	}

	goodConfig := Config{
		Pins:             PinConfig{Direction: "b", Step: "c", EnablePinHigh: "d", EnablePinLow: "e"},
		TicksPerRotation: 200,
		BoardName:        "brd",
		StepperDelay:     30,
	}

	pinB := &fakeboard.GPIOPin{}
	pinC := &fakeboard.GPIOPin{}
	pinD := &fakeboard.GPIOPin{}
	pinE := &fakeboard.GPIOPin{}
	pinMap := map[string]*fakeboard.GPIOPin{
		"b": pinB,
		"c": pinC,
		"d": pinD,
		"e": pinE,
	}
	b := fakeboard.Board{GPIOPins: pinMap}

	t.Run("isPowered false after init", func(t *testing.T) {
		m, err := newGPIOStepper(ctx, &b, goodConfig, c.ResourceName(), logger)
		test.That(t, err, test.ShouldBeNil)
		defer m.Close(ctx)

		on, powerPct, err := m.IsPowered(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldEqual, false)
		test.That(t, powerPct, test.ShouldEqual, 0.0)

		h, err := pinD.Get(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, h, test.ShouldBeFalse)

		l, err := pinE.Get(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, l, test.ShouldBeTrue)
	})

	t.Run("IsPowered true", func(t *testing.T) {
		m, err := newGPIOStepper(ctx, &b, goodConfig, c.ResourceName(), logger)
		s := m.(*gpioStepper)
		test.That(t, err, test.ShouldBeNil)
		defer m.Close(ctx)

		// long running goFor
		err = s.goForInternal(ctx, 100, 3)
		defer m.Stop(ctx, nil)

		test.That(t, err, test.ShouldBeNil)

		// the motor is running
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			on, powerPct, err := m.IsPowered(ctx, nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, on, test.ShouldEqual, true)
			test.That(t, powerPct, test.ShouldEqual, 1.0)
		})

		// the motor finished running
		testutils.WaitForAssertionWithSleep(t, 100*time.Millisecond, 100, func(tb testing.TB) {
			tb.Helper()
			on, powerPct, err := m.IsPowered(ctx, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, on, test.ShouldEqual, false)
			test.That(tb, powerPct, test.ShouldEqual, 0.0)
		})

		pos, err := m.Position(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 3)
		test.That(t, s.targetStepPosition, test.ShouldEqual, 600)
	})

	t.Run("motor enable", func(t *testing.T) {
		m, err := newGPIOStepper(ctx, &b, goodConfig, c.ResourceName(), logger)
		s := m.(*gpioStepper)
		test.That(t, err, test.ShouldBeNil)
		defer m.Close(ctx)

		err = s.enable(ctx, true)
		test.That(t, err, test.ShouldBeNil)

		h, err := pinD.Get(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, h, test.ShouldBeTrue)

		l, err := pinE.Get(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, l, test.ShouldBeFalse)

		err = s.enable(ctx, false)
		test.That(t, err, test.ShouldBeNil)

		h, err = pinD.Get(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, h, test.ShouldBeFalse)

		l, err = pinE.Get(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, l, test.ShouldBeTrue)
	})

	t.Run("motor testing with positive rpm and positive revolutions", func(t *testing.T) {
		m, err := newGPIOStepper(ctx, &b, goodConfig, c.ResourceName(), logger)
		s := m.(*gpioStepper)
		test.That(t, err, test.ShouldBeNil)
		defer m.Close(ctx)

		err = s.GoFor(ctx, 10000, 1, nil)
		test.That(t, err, test.ShouldBeNil)

		on, powerPct, err := m.IsPowered(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldEqual, false)
		test.That(t, powerPct, test.ShouldEqual, 0.0)

		pos, err := m.Position(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 1)
		test.That(t, s.targetStepPosition, test.ShouldEqual, 200)
	})

	t.Run("motor testing with negative rpm and positive revolutions", func(t *testing.T) {
		m, err := newGPIOStepper(ctx, &b, goodConfig, c.ResourceName(), logger)
		s := m.(*gpioStepper)
		test.That(t, err, test.ShouldBeNil)
		defer m.Close(ctx)

		err = m.GoFor(ctx, -10000, 1, nil)
		test.That(t, err, test.ShouldBeNil)

		on, powerPct, err := m.IsPowered(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldEqual, false)
		test.That(t, powerPct, test.ShouldEqual, 0.0)

		pos, err := m.Position(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, -1)
		test.That(t, s.targetStepPosition, test.ShouldEqual, -200)
	})

	t.Run("motor testing with positive rpm and negative revolutions", func(t *testing.T) {
		m, err := newGPIOStepper(ctx, &b, goodConfig, c.ResourceName(), logger)
		s := m.(*gpioStepper)
		test.That(t, err, test.ShouldBeNil)
		defer m.Close(ctx)

		err = m.GoFor(ctx, 10000, -1, nil)
		test.That(t, err, test.ShouldBeNil)

		on, powerPct, err := m.IsPowered(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldEqual, false)
		test.That(t, powerPct, test.ShouldEqual, 0.0)

		pos, err := m.Position(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, -1)
		test.That(t, s.targetStepPosition, test.ShouldEqual, -200)
	})

	t.Run("motor testing with negative rpm and negative revolutions", func(t *testing.T) {
		m, err := newGPIOStepper(ctx, &b, goodConfig, c.ResourceName(), logger)
		s := m.(*gpioStepper)
		test.That(t, err, test.ShouldBeNil)
		defer m.Close(ctx)

		err = m.GoFor(ctx, -10000, -1, nil)
		test.That(t, err, test.ShouldBeNil)

		on, powerPct, err := m.IsPowered(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldEqual, false)
		test.That(t, powerPct, test.ShouldEqual, 0.0)

		pos, err := m.Position(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 1)
		test.That(t, s.targetStepPosition, test.ShouldEqual, 200)
	})

	t.Run("Ensure stop called when gofor is interrupted", func(t *testing.T) {
		m, err := newGPIOStepper(ctx, &b, goodConfig, c.ResourceName(), logger)
		s := m.(*gpioStepper)
		test.That(t, err, test.ShouldBeNil)
		defer m.Close(ctx)

		ctx := context.Background()
		var wg sync.WaitGroup
		ctx, cancel := context.WithCancel(ctx)
		wg.Add(1)
		go func() {
			m.GoFor(ctx, 100, 100, map[string]interface{}{})
			wg.Done()
		}()

		// Make sure it starts moving
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			on, _, err := m.IsPowered(ctx, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, on, test.ShouldEqual, true)

			p, err := m.Position(ctx, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, p, test.ShouldBeGreaterThan, 0)
		})

		cancel()
		wg.Wait()

		// Make sure it stops moving
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			on, _, err := m.IsPowered(ctx, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, on, test.ShouldEqual, false)
		})
		test.That(t, ctx.Err(), test.ShouldNotBeNil)

		p, err := m.Position(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)

		// stop() sets targetStepPosition to the stepPostion value
		test.That(t, s.targetStepPosition, test.ShouldEqual, s.stepPosition)
		test.That(t, s.targetStepPosition, test.ShouldBeBetweenOrEqual, 1, 100*200)
		test.That(t, p, test.ShouldBeBetween, 0, 100)
	})

	t.Run("enable pins handled properly during GoFor", func(t *testing.T) {
		m, err := newGPIOStepper(ctx, &b, goodConfig, c.ResourceName(), logger)
		test.That(t, err, test.ShouldBeNil)
		defer m.Close(ctx)

		ctx := context.Background()
		var wg sync.WaitGroup
		ctx, cancel := context.WithCancel(ctx)
		wg.Add(1)
		go func() {
			m.GoFor(ctx, 100, 100, map[string]interface{}{})
			wg.Done()
		}()

		// Make sure it starts moving
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			on, _, err := m.IsPowered(ctx, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, on, test.ShouldEqual, true)

			h, err := pinD.Get(ctx, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, h, test.ShouldBeTrue)

			l, err := pinE.Get(ctx, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, l, test.ShouldBeFalse)
		})

		cancel()
		wg.Wait()

		// Make sure it stops moving
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			on, _, err := m.IsPowered(ctx, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, on, test.ShouldEqual, false)

			h, err := pinD.Get(ctx, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, h, test.ShouldBeFalse)

			l, err := pinE.Get(ctx, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, l, test.ShouldBeTrue)
		})
		test.That(t, ctx.Err(), test.ShouldNotBeNil)
	})

	t.Run("motor testing with large # of revolutions", func(t *testing.T) {
		m, err := newGPIOStepper(ctx, &b, goodConfig, c.ResourceName(), logger)
		s := m.(*gpioStepper)
		test.That(t, err, test.ShouldBeNil)
		defer m.Close(ctx)

		err = s.goForInternal(ctx, 1000, 200)
		test.That(t, err, test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()

			on, _, err := m.IsPowered(ctx, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, on, test.ShouldEqual, true)

			pos, err := m.Position(ctx, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, pos, test.ShouldBeGreaterThan, 2)
		})

		err = m.Stop(ctx, nil)
		test.That(t, err, test.ShouldBeNil)

		on, _, err := m.IsPowered(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldEqual, false)

		pos, err := m.Position(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldBeGreaterThan, 2)
		test.That(t, pos, test.ShouldBeLessThan, 202)
	})

	t.Run("motor testing with 0 rpm", func(t *testing.T) {
		m, err := newGPIOStepper(ctx, &b, goodConfig, c.ResourceName(), logger)
		test.That(t, err, test.ShouldBeNil)
		defer m.Close(ctx)

		err = m.GoFor(ctx, 0, 1, nil)
		test.That(t, err, test.ShouldBeNil)
		allObs := obs.All()
		latestLoggedEntry := allObs[len(allObs)-1]
		test.That(t, fmt.Sprint(latestLoggedEntry), test.ShouldContainSubstring, "nearly 0")

		on, _, err := m.IsPowered(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldEqual, false)
	})

	cancel()
}
