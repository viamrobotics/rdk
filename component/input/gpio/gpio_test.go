package gpio_test

import (
	"context"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/component/board"
	fakeboard "go.viam.com/rdk/component/board/fake"
	"go.viam.com/rdk/component/input"
	"go.viam.com/rdk/component/input/gpio"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

func TestGPIOInput(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	b := &fakeboard.Board{
		Digitals: map[string]board.DigitalInterrupt{},
		Analogs:  map[string]*fakeboard.Analog{},
	}

	b.Analogs["analog1"] = &fakeboard.Analog{}
	b.Analogs["analog2"] = &fakeboard.Analog{}
	b.Analogs["analog3"] = &fakeboard.Analog{}
	b.Analogs["analog4"] = &fakeboard.Analog{}

	var err error
	b.Digitals["interrupt1"], err = board.CreateDigitalInterrupt(board.DigitalInterruptConfig{})
	test.That(t, err, test.ShouldBeNil)
	b.Digitals["interrupt2"], err = board.CreateDigitalInterrupt(board.DigitalInterruptConfig{})
	test.That(t, err, test.ShouldBeNil)

	r := inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return b, nil
	}

	ic := gpio.Config{
		Board: "main",
		Buttons: map[string]gpio.ButtonConfig{
			"interrupt1": gpio.ButtonConfig{
				Control:    input.ButtonNorth,
				Invert:     false,
				DebounceMs: 0,
			},
			"interrupt2": gpio.ButtonConfig{
				Control:    input.ButtonSouth,
				Invert:     true,
				DebounceMs: -1,
			},
		},
		Axes: map[string]gpio.AxisConfig{
			"analog1": gpio.AxisConfig{
				Control:       input.AbsoluteX,
				Min:           0,
				Max:           1023,
				Bidirectional: false,
				Deadzone:      0,
				MinChange:     0,
				PollHz:        0,
				Invert:        false,
			},
			"analog2": gpio.AxisConfig{
				Control:       input.AbsoluteY,
				Min:           0,
				Max:           1023,
				Bidirectional: true,
				Deadzone:      20,
				MinChange:     15,
				PollHz:        50,
				Invert:        false,
			},
			"analog3": gpio.AxisConfig{
				Control:       input.AbsoluteRX,
				Min:           -5000,
				Max:           5000,
				Bidirectional: true,
				Deadzone:      0,
				MinChange:     0,
				PollHz:        0,
				Invert:        true,
			},
			"analog4": gpio.AxisConfig{
				Control:       input.AbsoluteRY,
				Min:           0,
				Max:           1024,
				Bidirectional: false,
				Deadzone:      0,
				MinChange:     0,
				PollHz:        0,
				Invert:        false,
			},
		},
	}

	inputReg := registry.ComponentLookup(input.Subtype, "gpio")
	test.That(t, inputReg, test.ShouldNotBeNil)

	res, err := inputReg.Constructor(context.Background(), &r, config.Component{Name: "input1", ConvertedAttributes: &ic}, logger)
	test.That(t, err, test.ShouldBeNil)
	dev, ok := res.(input.Controller)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, dev, test.ShouldNotBeNil)

	var btn1Callbacks, btn2Callbacks int64
	var axis1Callbacks, axis2Callbacks, axis3Callbacks, axis4Callbacks int64
	var axis1Time, axis2Time time.Time

	err = dev.RegisterControlCallback(ctx, input.ButtonNorth, []input.EventType{input.ButtonChange},
		func(ctx context.Context, event input.Event) {
			btn1Callbacks++
		})
	test.That(t, err, test.ShouldBeNil)

	err = dev.RegisterControlCallback(ctx, input.ButtonSouth, []input.EventType{input.ButtonChange},
		func(ctx context.Context, event input.Event) {
			btn2Callbacks++
		})
	test.That(t, err, test.ShouldBeNil)

	err = dev.RegisterControlCallback(ctx, input.AbsoluteX, []input.EventType{input.PositionChangeAbs},
		func(ctx context.Context, event input.Event) {
			axis1Time = time.Now()
			axis1Callbacks++
		})
	test.That(t, err, test.ShouldBeNil)

	err = dev.RegisterControlCallback(ctx, input.AbsoluteY, []input.EventType{input.PositionChangeAbs},
		func(ctx context.Context, event input.Event) {
			axis2Time = time.Now()
			axis2Callbacks++
		})
	test.That(t, err, test.ShouldBeNil)

	err = dev.RegisterControlCallback(ctx, input.AbsoluteRX, []input.EventType{input.PositionChangeAbs},
		func(ctx context.Context, event input.Event) {
			axis3Callbacks++
		})
	test.That(t, err, test.ShouldBeNil)

	err = dev.RegisterControlCallback(ctx, input.AbsoluteRY, []input.EventType{input.PositionChangeAbs},
		func(ctx context.Context, event input.Event) {
			axis4Callbacks++
		})
	test.That(t, err, test.ShouldBeNil)

	defer func() {
		test.That(t, utils.TryClose(context.Background(), dev), test.ShouldBeNil)
	}()

	// Test initial button state
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		state, err := dev.GetEvents(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, state["ButtonNorth"].Value, test.ShouldEqual, 0)
		test.That(tb, state["ButtonNorth"].Event, test.ShouldEqual, input.Connect)
	})

	// Test normal button press
	err = b.Digitals["interrupt1"].Tick(ctx, true, uint64(time.Now().UnixNano()))
	test.That(t, err, test.ShouldBeNil)

	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		state, err := dev.GetEvents(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, state["ButtonNorth"].Value, test.ShouldEqual, 1)
		test.That(tb, state["ButtonNorth"].Event, test.ShouldEqual, input.ButtonPress)
		test.That(tb, btn1Callbacks, test.ShouldEqual, 1)
	})

	err = b.Digitals["interrupt1"].Tick(ctx, false, uint64(time.Now().UnixNano()))
	test.That(t, err, test.ShouldBeNil)

	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		state, err := dev.GetEvents(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, state["ButtonNorth"].Value, test.ShouldEqual, 0)
		test.That(tb, state["ButtonNorth"].Event, test.ShouldEqual, input.ButtonRelease)
		test.That(tb, btn1Callbacks, test.ShouldEqual, 2)
	})

	// Test debounce at 5ms (default)
	for i := 0; i < 20; i++ {
		err = b.Digitals["interrupt1"].Tick(ctx, false, uint64(time.Now().UnixNano()))
		test.That(t, err, test.ShouldBeNil)
		err = b.Digitals["interrupt1"].Tick(ctx, true, uint64(time.Now().UnixNano()))
		test.That(t, err, test.ShouldBeNil)
	}

	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		state, err := dev.GetEvents(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, state["ButtonNorth"].Value, test.ShouldEqual, 1)
		test.That(tb, state["ButtonNorth"].Event, test.ShouldEqual, input.ButtonPress)
		test.That(tb, btn1Callbacks, test.ShouldEqual, 3)
	})

	time.Sleep(time.Millisecond * 10)
	test.That(t, btn1Callbacks, test.ShouldEqual, 3)

	// Test inverted, non-debounced button

	// Test initial button state
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		state, err := dev.GetEvents(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, state["ButtonSouth"].Value, test.ShouldEqual, 0)
		test.That(tb, state["ButtonSouth"].Event, test.ShouldEqual, input.Connect)
	})

	// Test inverted button press
	err = b.Digitals["interrupt2"].Tick(ctx, true, uint64(time.Now().UnixNano()))
	test.That(t, err, test.ShouldBeNil)

	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		state, err := dev.GetEvents(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, state["ButtonSouth"].Value, test.ShouldEqual, 0)
		test.That(tb, state["ButtonSouth"].Event, test.ShouldEqual, input.ButtonRelease)
		test.That(tb, btn2Callbacks, test.ShouldEqual, 1)
	})

	err = b.Digitals["interrupt2"].Tick(ctx, false, uint64(time.Now().UnixNano()))
	test.That(t, err, test.ShouldBeNil)

	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		state, err := dev.GetEvents(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, state["ButtonSouth"].Value, test.ShouldEqual, 1)
		test.That(tb, state["ButtonSouth"].Event, test.ShouldEqual, input.ButtonPress)
		test.That(tb, btn2Callbacks, test.ShouldEqual, 2)
	})

	// Test with debounce disabled
	for i := 0; i < 20; i++ {
		err = b.Digitals["interrupt2"].Tick(ctx, true, uint64(time.Now().UnixNano()))
		test.That(t, err, test.ShouldBeNil)
		time.Sleep(time.Millisecond)
		err = b.Digitals["interrupt2"].Tick(ctx, false, uint64(time.Now().UnixNano()))
		test.That(t, err, test.ShouldBeNil)
		time.Sleep(time.Millisecond)
	}

	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		state, err := dev.GetEvents(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, state["ButtonSouth"].Value, test.ShouldEqual, 1)
		test.That(tb, state["ButtonSouth"].Event, test.ShouldEqual, input.ButtonPress)
		test.That(tb, btn2Callbacks, test.ShouldEqual, 42)
	})

	time.Sleep(time.Millisecond * 10)
	test.That(t, btn2Callbacks, test.ShouldEqual, 42)

	// Test axis1 (default)
	b.Analogs["analog1"].Value = 0
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		state, err := dev.GetEvents(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, state["AbsoluteX"].Value, test.ShouldAlmostEqual, 0, 0.005)
		test.That(tb, state["AbsoluteX"].Event, test.ShouldEqual, input.Connect)
		test.That(tb, axis1Callbacks, test.ShouldEqual, 0)
	})

	b.Analogs["analog1"].Value = 1023
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		state, err := dev.GetEvents(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, state["AbsoluteX"].Value, test.ShouldAlmostEqual, 1, 0.005)
		test.That(tb, state["AbsoluteX"].Event, test.ShouldEqual, input.PositionChangeAbs)
		test.That(tb, axis1Callbacks, test.ShouldEqual, 1)
	})

	b.Analogs["analog1"].Value = 511
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		state, err := dev.GetEvents(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, state["AbsoluteX"].Value, test.ShouldAlmostEqual, 0.5, 0.005)
		test.That(tb, state["AbsoluteX"].Event, test.ShouldEqual, input.PositionChangeAbs)
		test.That(tb, axis1Callbacks, test.ShouldEqual, 2)
	})

	b.Analogs["analog1"].Value = 511
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		state, err := dev.GetEvents(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, state["AbsoluteX"].Value, test.ShouldAlmostEqual, 0.5, 0.005)
		test.That(tb, state["AbsoluteX"].Event, test.ShouldEqual, input.PositionChangeAbs)
		test.That(tb, axis1Callbacks, test.ShouldEqual, 2)
	})

	// Test deadzone
	b.Analogs["analog2"].Value = 511
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		state, err := dev.GetEvents(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, 0, 0.005)
		test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
		test.That(tb, axis2Callbacks, test.ShouldEqual, 1)
	})

	b.Analogs["analog2"].Value = 511 + 20
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		state, err := dev.GetEvents(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, 0.04, 0.005)
		test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
		test.That(tb, axis2Callbacks, test.ShouldEqual, 2)
	})

	b.Analogs["analog2"].Value = 511 - 20
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		state, err := dev.GetEvents(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, -0.04, 0.005)
		test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
		test.That(tb, axis2Callbacks, test.ShouldEqual, 3)
	})

	b.Analogs["analog2"].Value = 511 + 19
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		state, err := dev.GetEvents(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, 0, 0.005)
		test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
		test.That(tb, axis2Callbacks, test.ShouldEqual, 4)
	})

	b.Analogs["analog2"].Value = 511 - 19
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		state, err := dev.GetEvents(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, 0, 0.005)
		test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
		test.That(tb, axis2Callbacks, test.ShouldEqual, 4)
	})

	// Test min change (default)

	b.Analogs["analog2"].Value = 600
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		state, err := dev.GetEvents(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, 0.17, 0.005)
		test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
		test.That(tb, axis2Callbacks, test.ShouldEqual, 5)
	})

	b.Analogs["analog2"].Value += 14
	time.Sleep(time.Millisecond * 30)
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		state, err := dev.GetEvents(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, 0.17, 0.005)
		test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
		test.That(tb, axis2Callbacks, test.ShouldEqual, 5)
	})

	b.Analogs["analog2"].Value -= 28
	time.Sleep(time.Millisecond * 30)
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		state, err := dev.GetEvents(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, 0.17, 0.005)
		test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
		test.That(tb, axis2Callbacks, test.ShouldEqual, 5)
	})

	b.Analogs["analog2"].Value--
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		state, err := dev.GetEvents(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, 0.14, 0.005)
		test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
		test.That(tb, axis2Callbacks, test.ShouldEqual, 6)
	})

	// Test negative input and inversion

	b.Analogs["analog3"].Value = 5000
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		state, err := dev.GetEvents(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, state["AbsoluteRX"].Value, test.ShouldAlmostEqual, -1, 0.005)
		test.That(tb, state["AbsoluteRX"].Event, test.ShouldEqual, input.PositionChangeAbs)
		test.That(tb, axis3Callbacks, test.ShouldEqual, 1)
	})

	b.Analogs["analog3"].Value = -1000
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		state, err := dev.GetEvents(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, state["AbsoluteRX"].Value, test.ShouldAlmostEqual, 0.2, 0.005)
		test.That(tb, state["AbsoluteRX"].Event, test.ShouldEqual, input.PositionChangeAbs)
		test.That(tb, axis3Callbacks, test.ShouldEqual, 2)
	})

	// Test range capping
	b.Analogs["analog3"].Value = -6000
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		state, err := dev.GetEvents(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, state["AbsoluteRX"].Value, test.ShouldAlmostEqual, 1, 0.005)
		test.That(tb, state["AbsoluteRX"].Event, test.ShouldEqual, input.PositionChangeAbs)
		test.That(tb, axis3Callbacks, test.ShouldEqual, 3)
	})

	b.Analogs["analog3"].Value = 6000
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		state, err := dev.GetEvents(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, state["AbsoluteRX"].Value, test.ShouldAlmostEqual, -1, 0.005)
		test.That(tb, state["AbsoluteRX"].Event, test.ShouldEqual, input.PositionChangeAbs)
		test.That(tb, axis3Callbacks, test.ShouldEqual, 4)
	})

	b.Analogs["analog3"].Value = 0
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		state, err := dev.GetEvents(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, state["AbsoluteRX"].Value, test.ShouldAlmostEqual, 0, 0.005)
		test.That(tb, state["AbsoluteRX"].Event, test.ShouldEqual, input.PositionChangeAbs)
		test.That(tb, axis3Callbacks, test.ShouldEqual, 5)
	})

	// Test poll frequency

	b.Analogs["analog1"].Value = 0
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		state, err := dev.GetEvents(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, state["AbsoluteX"].Value, test.ShouldAlmostEqual, 0, 0.005)
		test.That(tb, axis1Callbacks, test.ShouldEqual, 3)
	})

	for i := 1; i < 10; i++ {
		var target float64
		startTime := time.Now()
		if b.Analogs["analog1"].Value == 0 {
			target = 1
			b.Analogs["analog1"].Value = 1023
		} else {
			target = 0
			b.Analogs["analog1"].Value = 0
		}
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := dev.GetEvents(ctx)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteX"].Value, test.ShouldAlmostEqual, target, 0.005)
			test.That(tb, axis1Callbacks, test.ShouldEqual, 3+i)
		})
		test.That(t, axis1Time.Sub(startTime), test.ShouldBeBetween, 0*time.Millisecond, 110*time.Millisecond)
	}

	b.Analogs["analog2"].Value = 0
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		state, err := dev.GetEvents(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, -1, 0.005)
		test.That(tb, axis2Callbacks, test.ShouldEqual, 7)
	})

	for i := 1; i < 20; i++ {
		var target float64
		startTime := time.Now()
		if b.Analogs["analog2"].Value == 0 {
			target = 1
			b.Analogs["analog2"].Value = 1023
		} else {
			target = -1
			b.Analogs["analog2"].Value = 0
		}
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := dev.GetEvents(ctx)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, target, 0.005)
			test.That(tb, axis2Callbacks, test.ShouldEqual, 7+i)
		})
		test.That(t, axis2Time.Sub(startTime), test.ShouldBeBetween, 0*time.Millisecond, 22*time.Millisecond)
	}
}
