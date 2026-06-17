package gpiostepper

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/board"
	fakeboard "go.viam.com/rdk/components/board/fake"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

const minDistanceMoved = 2

func TestConfigs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	goodConfig := Config{
		Pins:             PinConfig{Direction: "b", Step: "c", EnablePinHigh: "d", EnablePinLow: "e"},
		TicksPerRotation: 200,
		BoardName:        "brd",
		StepperDelay:     30,
	}

	logger := logging.NewTestLogger(t)
	c := resource.Config{
		Name:                "fake_gpiostepper",
		ConvertedAttributes: &goodConfig,
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

		deps, _, err := mc.Validate("")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, deps, test.ShouldResemble, []string{"brd"})

		// remove optional fields
		mc.StepperDelay = 0
		deps, _, err = mc.Validate("")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, deps, test.ShouldResemble, []string{"brd"})

		mc.Pins.EnablePinHigh = ""
		mc.Pins.EnablePinLow = ""
		deps, _, err = mc.Validate("")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, deps, test.ShouldResemble, []string{"brd"})
	})

	t.Run("config missing required pins", func(t *testing.T) {
		mc := goodConfig
		mc.Pins.Direction = ""

		_, _, err := mc.Validate("")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, resource.NewConfigValidationFieldRequiredError("", "dir"))

		mc = goodConfig
		mc.Pins.Step = ""
		_, _, err = mc.Validate("")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, resource.NewConfigValidationFieldRequiredError("", "step"))
	})

	t.Run("config missing ticks", func(t *testing.T) {
		mc := goodConfig
		mc.TicksPerRotation = 0

		_, _, err := mc.Validate("")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, resource.NewConfigValidationFieldRequiredError("", "ticks_per_rotation"))
	})

	t.Run("config missing board", func(t *testing.T) {
		mc := goodConfig
		mc.BoardName = ""

		_, _, err := mc.Validate("")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, resource.NewConfigValidationFieldRequiredError("", "board"))
	})

	deps := resource.Dependencies{resource.NewName(board.API, "brd"): &b}
	t.Run("initializing good with enable pins", func(t *testing.T) {
		m, err := newGPIOStepper(ctx, deps, c, logger)
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
		c := resource.Config{
			Name: "fake_gpiostepper",
			ConvertedAttributes: &Config{
				Pins:             PinConfig{Direction: "b", Step: "c", EnablePinHigh: "", EnablePinLow: ""},
				TicksPerRotation: 200,
				BoardName:        "brd",
			},
		}

		m, err := newGPIOStepper(ctx, deps, c, logger)
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
		c := resource.Config{
			Name:                "fake_gpiostepper",
			ConvertedAttributes: &Config{BoardName: "some_board"},
		}

		_, err := newGPIOStepper(ctx, nil, c, logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, resource.DependencyNotFoundError(resource.NewName(board.API, "some_board")))
	})

	t.Run("initializing without ticks per rotation", func(t *testing.T) {
		c := resource.Config{
			Name: "fake_gpiostepper",
			ConvertedAttributes: &Config{
				BoardName:        "brd",
				Pins:             PinConfig{Direction: "b", Step: "c", EnablePinHigh: "d", EnablePinLow: "e"},
				TicksPerRotation: 0,
			},
		}

		_, err := newGPIOStepper(ctx, deps, c, logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "expected ticks_per_rotation")
	})

	t.Run("initializing with negative stepper delay", func(t *testing.T) {
		c := resource.Config{
			Name: "fake_gpiostepper",
			ConvertedAttributes: &Config{
				BoardName:        "brd",
				Pins:             PinConfig{Direction: "b", Step: "c", EnablePinHigh: "d", EnablePinLow: "e"},
				TicksPerRotation: 1,
				StepperDelay:     -100,
			},
		}

		m, err := newGPIOStepper(ctx, deps, c, logger)
		s := m.(*gpioStepper)

		test.That(t, err, test.ShouldBeNil)
		defer m.Close(ctx)
		test.That(t, s.minDelay, test.ShouldEqual, 0*time.Microsecond)
	})

	t.Run("motor supports position reporting", func(t *testing.T) {
		m, err := newGPIOStepper(ctx, deps, c, logger)
		test.That(t, err, test.ShouldBeNil)
		defer m.Close(ctx)

		properties, err := m.Properties(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, properties.PositionReporting, test.ShouldBeTrue)
	})

	t.Run("microsteps defaults to 1 and is multiplicative on TicksPerRotation", func(t *testing.T) {
		// missing microsteps → effective stepsPerRotation == TicksPerRotation
		c := resource.Config{
			Name: "fake_gpiostepper",
			ConvertedAttributes: &Config{
				BoardName:        "brd",
				Pins:             PinConfig{Direction: "b", Step: "c"},
				TicksPerRotation: 200,
			},
		}
		m, err := newGPIOStepper(ctx, deps, c, logger)
		test.That(t, err, test.ShouldBeNil)
		defer m.Close(ctx)
		test.That(t, m.(*gpioStepper).stepsPerRotation, test.ShouldEqual, 200)

		// Microsteps=8, TicksPerRotation=200 → 1600
		c2 := resource.Config{
			Name: "fake_gpiostepper",
			ConvertedAttributes: &Config{
				BoardName:        "brd",
				Pins:             PinConfig{Direction: "b", Step: "c"},
				TicksPerRotation: 200,
				Microsteps:       8,
			},
		}
		m2, err := newGPIOStepper(ctx, deps, c2, logger)
		test.That(t, err, test.ShouldBeNil)
		defer m2.Close(ctx)
		test.That(t, m2.(*gpioStepper).stepsPerRotation, test.ShouldEqual, 1600)
	})

	t.Run("negative microsteps fails Validate", func(t *testing.T) {
		mc := goodConfig
		mc.Microsteps = -1
		_, _, err := mc.Validate("")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "microsteps")
	})

	t.Run("negative max_rpm fails Validate", func(t *testing.T) {
		mc := goodConfig
		mc.MaxRPM = -1
		_, _, err := mc.Validate("")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "max_rpm")
	})

	t.Run("max_rpm sets minDelay", func(t *testing.T) {
		// MaxRPM=60, TicksPerRotation=200, no microsteps → maxFreq=200Hz → minDelay=5ms
		c := resource.Config{
			Name: "fake_gpiostepper",
			ConvertedAttributes: &Config{
				BoardName:        "brd",
				Pins:             PinConfig{Direction: "b", Step: "c"},
				TicksPerRotation: 200,
				MaxRPM:           60,
			},
		}
		m, err := newGPIOStepper(ctx, deps, c, logger)
		test.That(t, err, test.ShouldBeNil)
		defer m.Close(ctx)
		test.That(t, m.(*gpioStepper).minDelay, test.ShouldEqual, 5*time.Millisecond)
	})

	t.Run("max_rpm wins when stepper_delay_usec is also set, logs deprecation", func(t *testing.T) {
		obsLogger, obs := logging.NewObservedTestLogger(t)
		c := resource.Config{
			Name: "fake_gpiostepper",
			ConvertedAttributes: &Config{
				BoardName:        "brd",
				Pins:             PinConfig{Direction: "b", Step: "c"},
				TicksPerRotation: 200,
				MaxRPM:           60,
				StepperDelay:     999, // would imply different minDelay if it won
			},
		}
		m, err := newGPIOStepper(ctx, deps, c, obsLogger)
		test.That(t, err, test.ShouldBeNil)
		defer m.Close(ctx)
		// minDelay derived from MaxRPM, not StepperDelay
		test.That(t, m.(*gpioStepper).minDelay, test.ShouldEqual, 5*time.Millisecond)

		// deprecation warning was logged
		var found bool
		for _, entry := range obs.All() {
			if strings.Contains(fmt.Sprint(entry), "deprecated") {
				found = true
				break
			}
		}
		test.That(t, found, test.ShouldBeTrue)
	})

	t.Run("stepper_delay_usec alone logs deprecation", func(t *testing.T) {
		obsLogger, obs := logging.NewObservedTestLogger(t)
		c := resource.Config{
			Name: "fake_gpiostepper",
			ConvertedAttributes: &Config{
				BoardName:        "brd",
				Pins:             PinConfig{Direction: "b", Step: "c"},
				TicksPerRotation: 200,
				StepperDelay:     30,
			},
		}
		m, err := newGPIOStepper(ctx, deps, c, obsLogger)
		test.That(t, err, test.ShouldBeNil)
		defer m.Close(ctx)
		test.That(t, m.(*gpioStepper).minDelay, test.ShouldEqual, 30*time.Microsecond)

		var found bool
		for _, entry := range obs.All() {
			if strings.Contains(fmt.Sprint(entry), "deprecated") {
				found = true
				break
			}
		}
		test.That(t, found, test.ShouldBeTrue)
	})
}

func TestRunning(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger, _ := logging.NewObservedTestLogger(t)
	c := resource.Config{
		Name: "fake_gpiostepper",
		ConvertedAttributes: &Config{
			Pins:             PinConfig{Direction: "b", Step: "c", EnablePinHigh: "d", EnablePinLow: "e"},
			TicksPerRotation: 200,
			BoardName:        "brd",
			StepperDelay:     30,
		},
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
	deps := resource.Dependencies{resource.NewName(board.API, "brd"): &b}

	t.Run("isPowered false after init", func(t *testing.T) {
		m, err := newGPIOStepper(ctx, deps, c, logger)
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
		m, err := newGPIOStepper(ctx, deps, c, logger)
		test.That(t, err, test.ShouldBeNil)
		defer m.Close(ctx)

		// long running GoFor in goroutine
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.GoFor(ctx, 100, 3, nil)
		}()
		defer func() {
			m.Stop(ctx, nil)
			wg.Wait()
		}()

		// the motor is running
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			on, powerPct, err := m.IsPowered(ctx, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, on, test.ShouldEqual, true)
			test.That(tb, powerPct, test.ShouldEqual, 1.0)
		})

		// the motor finished running
		testutils.WaitForAssertionWithSleep(t, 100*time.Millisecond, 100, func(tb testing.TB) {
			tb.Helper()
			on, powerPct, err := m.IsPowered(ctx, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, on, test.ShouldEqual, false)
			test.That(tb, powerPct, test.ShouldEqual, 0.0)
		})

		wg.Wait()
		pos, err := m.Position(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 3)
		test.That(t, m.(*gpioStepper).targetStepPosition.Load(), test.ShouldEqual, 600)
	})

	t.Run("motor enable", func(t *testing.T) {
		m, err := newGPIOStepper(ctx, deps, c, logger)
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
		m, err := newGPIOStepper(ctx, deps, c, logger)
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
		test.That(t, s.targetStepPosition.Load(), test.ShouldEqual, 200)
	})

	t.Run("motor testing with negative rpm and positive revolutions", func(t *testing.T) {
		m, err := newGPIOStepper(ctx, deps, c, logger)
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
		test.That(t, s.targetStepPosition.Load(), test.ShouldEqual, -200)
	})

	t.Run("motor testing with positive rpm and negative revolutions", func(t *testing.T) {
		m, err := newGPIOStepper(ctx, deps, c, logger)
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
		test.That(t, s.targetStepPosition.Load(), test.ShouldEqual, -200)
	})

	t.Run("motor testing with negative rpm and negative revolutions", func(t *testing.T) {
		m, err := newGPIOStepper(ctx, deps, c, logger)
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
		test.That(t, s.targetStepPosition.Load(), test.ShouldEqual, 200)
	})

	t.Run("motor testing with 0 rpm and 0 revolutions", func(t *testing.T) {
		m, err := newGPIOStepper(ctx, deps, c, logger)
		test.That(t, err, test.ShouldBeNil)
		defer m.Close(ctx)

		err = m.GoFor(ctx, 0, 10, nil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "RPM that is nearly 0")

		err = m.GoFor(ctx, 10, 0, nil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "0 revolutions")
	})

	t.Run("Ensure stop called when gofor is interrupted", func(t *testing.T) {
		m, err := newGPIOStepper(ctx, deps, c, logger)
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
			on, _, err := m.IsPowered(context.Background(), nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, on, test.ShouldEqual, false)
		})
		test.That(t, ctx.Err(), test.ShouldNotBeNil)

		p, err := m.Position(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)

		// stop() sets targetStepPosition to the stepPosition value
		test.That(t, s.targetStepPosition.Load(), test.ShouldEqual, s.stepPosition.Load())
		test.That(t, s.targetStepPosition.Load(), test.ShouldBeBetweenOrEqual, 1, 100*200)
		test.That(t, p, test.ShouldBeBetween, 0, 100)
	})

	t.Run("enable pins handled properly during GoFor", func(t *testing.T) {
		m, err := newGPIOStepper(ctx, deps, c, logger)
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

		// Make sure it stops moving — use fresh context since old one is cancelled
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			on, _, err := m.IsPowered(context.Background(), nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, on, test.ShouldEqual, false)

			h, err := pinD.Get(context.Background(), nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, h, test.ShouldBeFalse)

			l, err := pinE.Get(context.Background(), nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, l, test.ShouldBeTrue)
		})

		ctx2 := context.Background()

		err = m.SetRPM(ctx2, 100, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)

		// Make sure it starts moving
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			on, _, err := m.IsPowered(ctx2, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, on, test.ShouldEqual, true)

			h, err := pinD.Get(ctx2, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, h, test.ShouldBeTrue)

			l, err := pinE.Get(ctx2, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, l, test.ShouldBeFalse)
		})

		err = m.SetPower(ctx2, 1, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)

		// Make sure it starts moving
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			on, _, err := m.IsPowered(ctx2, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, on, test.ShouldEqual, true)

			h, err := pinD.Get(ctx2, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, h, test.ShouldBeTrue)

			l, err := pinE.Get(ctx2, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, l, test.ShouldBeFalse)
		})

		err = m.Stop(ctx2, nil)
		test.That(t, err, test.ShouldBeNil)
	})

	// Tests SetRPM with motion, stop, and position verification. Run twice to cover
	// both the "large revolutions" and "SetRPM" original test names.
	for _, name := range []string{"motor testing with large # of revolutions", "motor testing with SetRPM"} {
		t.Run(name, func(t *testing.T) {
			m, err := newGPIOStepper(ctx, deps, c, logger)
			test.That(t, err, test.ShouldBeNil)
			defer m.Close(ctx)

			err = m.SetRPM(ctx, 1000, nil)
			test.That(t, err, test.ShouldBeNil)

			testutils.WaitForAssertion(t, func(tb testing.TB) {
				tb.Helper()

				on, _, err := m.IsPowered(ctx, nil)
				test.That(tb, err, test.ShouldBeNil)
				test.That(tb, on, test.ShouldEqual, true)

				pos, err := m.Position(ctx, nil)
				test.That(tb, err, test.ShouldBeNil)
				test.That(tb, pos, test.ShouldBeGreaterThan, minDistanceMoved)
			})

			err = m.Stop(ctx, nil)
			test.That(t, err, test.ShouldBeNil)

			on, _, err := m.IsPowered(ctx, nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, on, test.ShouldEqual, false)

			pos, err := m.Position(ctx, nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, pos, test.ShouldBeGreaterThan, minDistanceMoved)
		})
	}

	t.Run("test rpmToFreqHz", func(t *testing.T) {
		m, err := newGPIOStepper(ctx, deps, c, logger)
		test.That(t, err, test.ShouldBeNil)
		defer m.Close(ctx)

		s := m.(*gpioStepper)

		// 200 steps/rot, minDelay=30us → maxFreq = 1/30us ≈ 33333 Hz
		// 50 RPM: 50*200/60 = 166.67 → 167 Hz
		freq := s.rpmToFreqHz(50)
		test.That(t, freq, test.ShouldEqual, uint(167))

		// negative RPM uses absolute value
		freq = s.rpmToFreqHz(-50)
		test.That(t, freq, test.ShouldEqual, uint(167))

		// 1 RPM: 1*200/60 = 3.33 → 3 Hz
		freq = s.rpmToFreqHz(1)
		test.That(t, freq, test.ShouldEqual, uint(3))

		// Very high RPM should be clamped by minDelay
		freq = s.rpmToFreqHz(100000)
		test.That(t, freq, test.ShouldEqual, uint(math.Round(1.0/(30e-6))))

		// Very low RPM clamps to 1 Hz
		freq = s.rpmToFreqHz(0.001)
		test.That(t, freq, test.ShouldEqual, uint(1))
	})

	t.Run("test PWM confirmation during motion", func(t *testing.T) {
		m, err := newGPIOStepper(ctx, deps, c, logger)
		test.That(t, err, test.ShouldBeNil)
		defer m.Close(ctx)

		err = m.SetRPM(ctx, 1000, nil)
		test.That(t, err, test.ShouldBeNil)

		// Check PWM is set on step pin
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			duty, err := pinC.PWM(ctx, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, duty, test.ShouldEqual, 0.5)

			freq, err := pinC.PWMFreq(ctx, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, freq, test.ShouldBeGreaterThan, 0)
		})

		err = m.Stop(ctx, nil)
		test.That(t, err, test.ShouldBeNil)

		// PWM should be 0 after stop
		duty, err := pinC.PWM(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, duty, test.ShouldEqual, 0.0)
	})

	t.Run("rpmToFreqHz scales with microsteps", func(t *testing.T) {
		// TicksPerRotation=200, Microsteps=8 → stepsPerRotation=1600
		// 50 RPM: 50 * 1600 / 60 = 1333.33 → 1333 Hz
		c := resource.Config{
			Name: "fake_gpiostepper",
			ConvertedAttributes: &Config{
				BoardName:        "brd",
				Pins:             PinConfig{Direction: "b", Step: "c", EnablePinHigh: "d", EnablePinLow: "e"},
				TicksPerRotation: 200,
				Microsteps:       8,
				StepperDelay:     30,
			},
		}
		m, err := newGPIOStepper(ctx, deps, c, logger)
		test.That(t, err, test.ShouldBeNil)
		defer m.Close(ctx)

		s := m.(*gpioStepper)
		test.That(t, s.rpmToFreqHz(50), test.ShouldEqual, uint(1333))
	})

	t.Run("GoFor target step position scales with microsteps", func(t *testing.T) {
		// TicksPerRotation=200, Microsteps=4 → stepsPerRotation=800
		// GoFor(rpm=10000, revs=1) should target 800 pulses
		c := resource.Config{
			Name: "fake_gpiostepper",
			ConvertedAttributes: &Config{
				BoardName:        "brd",
				Pins:             PinConfig{Direction: "b", Step: "c", EnablePinHigh: "d", EnablePinLow: "e"},
				TicksPerRotation: 200,
				Microsteps:       4,
				StepperDelay:     30,
			},
		}
		m, err := newGPIOStepper(ctx, deps, c, logger)
		test.That(t, err, test.ShouldBeNil)
		defer m.Close(ctx)

		s := m.(*gpioStepper)
		err = s.GoFor(ctx, 10000, 1, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, s.targetStepPosition.Load(), test.ShouldEqual, 800)

		pos, err := m.Position(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 1)
	})

	t.Run("SetPower works with max_rpm and no stepper_delay_usec", func(t *testing.T) {
		// MaxRPM=100, TicksPerRotation=200, Microsteps=1
		// maxFreq = 100 * 200 / 60 = 333.33 Hz → minDelay = 3ms
		// SetPower(0.5): freq = 0.5 / 0.003 = 166.67 → 166 Hz (uint truncation)
		c := resource.Config{
			Name: "fake_gpiostepper",
			ConvertedAttributes: &Config{
				BoardName:        "brd",
				Pins:             PinConfig{Direction: "b", Step: "c", EnablePinHigh: "d", EnablePinLow: "e"},
				TicksPerRotation: 200,
				MaxRPM:           100,
			},
		}
		m, err := newGPIOStepper(ctx, deps, c, logger)
		test.That(t, err, test.ShouldBeNil)
		defer m.Close(ctx)

		err = m.SetPower(ctx, 0.5, nil)
		test.That(t, err, test.ShouldBeNil)
		defer m.Stop(ctx, nil)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			freq, err := pinC.PWMFreq(ctx, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, freq, test.ShouldEqual, uint(166))
		})
	})

	t.Run("test stop signals waiters", func(t *testing.T) {
		m, err := newGPIOStepper(ctx, deps, c, logger)
		test.That(t, err, test.ShouldBeNil)
		defer m.Close(ctx)

		errCh := make(chan error, 1)
		go func() {
			errCh <- m.GoFor(ctx, 100, 100, nil)
		}()

		// Wait for motion to start
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			on, _, err := m.IsPowered(ctx, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, on, test.ShouldEqual, true)
		})

		// Stop should cause GoFor to return
		err = m.Stop(ctx, nil)
		test.That(t, err, test.ShouldBeNil)

		select {
		case goForErr := <-errCh:
			test.That(t, goForErr, test.ShouldNotBeNil)
			test.That(t, goForErr.Error(), test.ShouldContainSubstring, "context cancelled")
		case <-time.After(5 * time.Second):
			t.Fatal("GoFor did not return after Stop")
		}
	})

	cancel()
}
