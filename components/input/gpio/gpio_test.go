package gpio

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/input"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

type setupResult struct {
	btn1Callbacks, btn2Callbacks                   int64
	axis1Callbacks, axis2Callbacks, axis3Callbacks int64
	ctx                                            context.Context
	logger                                         logging.Logger
	b                                              *inject.Board
	dev                                            input.Controller
	axis1Time, axis2Time                           time.Time
	axisMu                                         sync.RWMutex
	interrupt1, interrupt2                         *inject.DigitalInterrupt
	analog1, analog2, analog3                      *inject.Analog
	mu                                             sync.Mutex
}

func setup(t *testing.T) *setupResult {
	t.Helper()
	s := setupResult{}

	s.ctx = context.Background()
	s.logger = logging.NewTestLogger(t)

	b := inject.NewBoard("test-board")
	s.b = b
	s.interrupt1 = &inject.DigitalInterrupt{}
	s.interrupt2 = &inject.DigitalInterrupt{}

	callbacks := make(map[board.DigitalInterrupt]chan board.Tick)

	s.interrupt1.NameFunc = func() string {
		return "interrupt1"
	}
	s.interrupt1.TickFunc = func(ctx context.Context, high bool, nanoseconds uint64) error {
		ch, ok := callbacks[s.interrupt1]
		test.That(t, ok, test.ShouldBeTrue)
		ch <- board.Tick{Name: s.interrupt1.Name(), High: high, TimestampNanosec: nanoseconds}
		return nil
	}

	// interrupt2 funcs
	s.interrupt2.NameFunc = func() string {
		return "interrupt2"
	}
	s.interrupt2.TickFunc = func(ctx context.Context, high bool, nanoseconds uint64) error {
		ch, ok := callbacks[s.interrupt2]
		test.That(t, ok, test.ShouldBeTrue)
		ch <- board.Tick{Name: s.interrupt2.Name(), High: high, TimestampNanosec: nanoseconds}
		return nil
	}

	b.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, error) {
		if name == "interrupt1" {
			return s.interrupt1, nil
		} else if name == "interrupt2" {
			return s.interrupt2, nil
		}
		return nil, fmt.Errorf("unknown digital interrupt: %s", name)
	}
	b.StreamTicksFunc = func(
		ctx context.Context, interrupts []board.DigitalInterrupt, ch chan board.Tick, extra map[string]interface{},
	) error {
		for _, i := range interrupts {
			callbacks[i] = ch
		}
		return nil
	}

	s.analog1 = &inject.Analog{}
	s.analog2 = &inject.Analog{}
	s.analog3 = &inject.Analog{}
	analog1Val, analog2Val, analog3Val := 0, 0, 0

	s.analog1.WriteFunc = func(ctx context.Context, value int, extra map[string]interface{}) error {
		s.mu.Lock()
		defer s.mu.Unlock()
		analog1Val = value
		return nil
	}

	s.analog1.ReadFunc = func(ctx context.Context, extra map[string]interface{}) (int, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		return analog1Val, nil
	}
	s.analog2.ReadFunc = func(ctx context.Context, extra map[string]interface{}) (int, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		return analog2Val, nil
	}
	s.analog3.ReadFunc = func(ctx context.Context, extra map[string]interface{}) (int, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		return analog3Val, nil
	}

	s.analog2.WriteFunc = func(ctx context.Context, value int, extra map[string]interface{}) error {
		s.mu.Lock()
		defer s.mu.Unlock()
		analog2Val = value
		return nil
	}

	s.analog3.WriteFunc = func(ctx context.Context, value int, extra map[string]interface{}) error {
		s.mu.Lock()
		defer s.mu.Unlock()
		analog3Val = value
		return nil
	}

	b.AnalogByNameFunc = func(name string) (board.Analog, error) {
		switch name {
		case "analog1":
			return s.analog1, nil
		case "analog2":
			return s.analog2, nil
		case "analog3":
			return s.analog3, nil
		default:
			return nil, fmt.Errorf("unknown analog: %s", name)
		}
	}

	deps := make(resource.Dependencies)
	deps[board.Named("main")] = s.b

	ic := Config{
		Board: "main",
		Buttons: map[string]*ButtonConfig{
			"interrupt1": {
				Control:    input.ButtonNorth,
				Invert:     false,
				DebounceMs: 20,
			},
			"interrupt2": {
				Control:    input.ButtonSouth,
				Invert:     true,
				DebounceMs: -1,
			},
		},
		Axes: map[string]*AxisConfig{
			"analog1": {
				Control:       input.AbsoluteX,
				Min:           0,
				Max:           1023,
				Bidirectional: false,
				Deadzone:      0,
				MinChange:     0,
				PollHz:        0,
				Invert:        false,
			},
			"analog2": {
				Control:       input.AbsoluteY,
				Min:           0,
				Max:           1023,
				Bidirectional: true,
				Deadzone:      20,
				MinChange:     15,
				PollHz:        50,
				Invert:        false,
			},
			"analog3": {
				Control:       input.AbsoluteRX,
				Min:           -5000,
				Max:           5000,
				Bidirectional: true,
				Deadzone:      0,
				MinChange:     0,
				PollHz:        0,
				Invert:        true,
			},
		},
	}

	inputReg, ok := resource.LookupRegistration(input.API, resource.DefaultModelFamily.WithModel("gpio"))
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, inputReg, test.ShouldNotBeNil)

	res, err := inputReg.Constructor(context.Background(), deps, resource.Config{Name: "input1", ConvertedAttributes: &ic}, s.logger)
	test.That(t, err, test.ShouldBeNil)

	s.dev, ok = res.(input.Controller)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, s.dev, test.ShouldNotBeNil)

	err = s.dev.RegisterControlCallback(s.ctx, input.ButtonNorth, []input.EventType{input.ButtonChange},
		func(ctx context.Context, event input.Event) {
			atomic.AddInt64(&s.btn1Callbacks, 1)
		},
		map[string]interface{}{},
	)
	test.That(t, err, test.ShouldBeNil)

	err = s.dev.RegisterControlCallback(s.ctx, input.ButtonSouth, []input.EventType{input.ButtonChange},
		func(ctx context.Context, event input.Event) {
			atomic.AddInt64(&s.btn2Callbacks, 1)
		},
		map[string]interface{}{},
	)
	test.That(t, err, test.ShouldBeNil)

	err = s.dev.RegisterControlCallback(s.ctx, input.AbsoluteX, []input.EventType{input.PositionChangeAbs},
		func(ctx context.Context, event input.Event) {
			s.axisMu.Lock()
			s.axis1Time = time.Now()
			s.axisMu.Unlock()
			atomic.AddInt64(&s.axis1Callbacks, 1)
		},
		map[string]interface{}{},
	)
	test.That(t, err, test.ShouldBeNil)

	err = s.dev.RegisterControlCallback(s.ctx, input.AbsoluteY, []input.EventType{input.PositionChangeAbs},
		func(ctx context.Context, event input.Event) {
			s.axisMu.Lock()
			s.axis2Time = time.Now()
			s.axisMu.Unlock()
			atomic.AddInt64(&s.axis2Callbacks, 1)
		},
		map[string]interface{}{},
	)
	test.That(t, err, test.ShouldBeNil)

	err = s.dev.RegisterControlCallback(s.ctx, input.AbsoluteRX, []input.EventType{input.PositionChangeAbs},
		func(ctx context.Context, event input.Event) {
			atomic.AddInt64(&s.axis3Callbacks, 1)
		},
		map[string]interface{}{},
	)
	test.That(t, err, test.ShouldBeNil)

	return &s
}

func teardown(t *testing.T, s *setupResult) {
	t.Helper()
	test.That(t, s.dev.Close(context.Background()), test.ShouldBeNil)
}

func TestGPIOInput(t *testing.T) {
	t.Run("config defaults", func(t *testing.T) {
		c := &Config{
			Board: "main",
			Buttons: map[string]*ButtonConfig{
				"interrupt1": {
					Control:    input.ButtonNorth,
					Invert:     false,
					DebounceMs: 20,
				},
				"interrupt2": {
					Control:    input.ButtonSouth,
					Invert:     true,
					DebounceMs: -1,
				},
				"interrupt3": {
					Control:    input.ButtonWest,
					Invert:     false,
					DebounceMs: 0, // default
				},
			},
			Axes: map[string]*AxisConfig{
				"analog1": {
					Control:       input.AbsoluteX,
					Min:           0,
					Max:           1023,
					Bidirectional: false,
					Deadzone:      0,
					MinChange:     0,
					PollHz:        0,
					Invert:        false,
				},
				"analog2": {
					Control:       input.AbsoluteY,
					Min:           0,
					Max:           1023,
					Bidirectional: true,
					Deadzone:      20,
					MinChange:     15,
					PollHz:        50,
					Invert:        false,
				},
			},
		}
		err := c.validateValues()

		test.That(t, err, test.ShouldBeNil)
		test.That(t, c.Buttons["interrupt1"].DebounceMs, test.ShouldEqual, 20) // unchanged
		test.That(t, c.Buttons["interrupt2"].DebounceMs, test.ShouldEqual, -1) // unchanged
		test.That(t, c.Buttons["interrupt3"].DebounceMs, test.ShouldEqual, 5)  // default

		test.That(t, c.Axes["analog1"].PollHz, test.ShouldEqual, 10) // default
		test.That(t, c.Axes["analog2"].PollHz, test.ShouldEqual, 50) // default
	})

	t.Run("config axis min > max", func(t *testing.T) {
		c := &Config{
			Board: "main",
			Axes: map[string]*AxisConfig{
				"analog1": {
					Control:       input.AbsoluteX,
					Min:           1023,
					Max:           0,
					Bidirectional: false,
					Deadzone:      0,
					MinChange:     0,
					PollHz:        0,
					Invert:        false,
				},
			},
		}
		err := c.validateValues()

		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("initial button state", func(t *testing.T) {
		s := setup(t)
		defer teardown(t, s)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := (s.dev).Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["ButtonNorth"].Value, test.ShouldEqual, 0)
			test.That(tb, state["ButtonNorth"].Event, test.ShouldEqual, input.Connect)
			test.That(tb, state["ButtonSouth"].Value, test.ShouldEqual, 0)
			test.That(tb, state["ButtonSouth"].Event, test.ShouldEqual, input.Connect)
		})
	})

	//nolint:dupl
	t.Run("button press and release", func(t *testing.T) {
		s := setup(t)
		defer teardown(t, s)

		err := s.interrupt1.Tick(s.ctx, true, uint64(time.Now().UnixNano()))
		test.That(t, err, test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["ButtonNorth"].Value, test.ShouldEqual, 1)
			test.That(tb, state["ButtonNorth"].Event, test.ShouldEqual, input.ButtonPress)
			test.That(tb, atomic.LoadInt64(&s.btn1Callbacks), test.ShouldEqual, 1)
		})

		err = s.interrupt1.Tick(s.ctx, false, uint64(time.Now().UnixNano()))
		test.That(t, err, test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["ButtonNorth"].Value, test.ShouldEqual, 0)
			test.That(tb, state["ButtonNorth"].Event, test.ShouldEqual, input.ButtonRelease)
			test.That(tb, atomic.LoadInt64(&s.btn1Callbacks), test.ShouldEqual, 2)
		})
	})

	// Testing methodology: Issue many events within the debounce time and confirm that only one is registered
	// Note: This is a time-sensitive test and is prone to flakiness.
	t.Run("button press debounce", func(t *testing.T) {
		s := setup(t)
		defer teardown(t, s)

		// this loop must complete within the debounce time
		for i := 0; i < 20; i++ {
			err := s.interrupt1.Tick(s.ctx, false, uint64(time.Now().UnixNano()))
			test.That(t, err, test.ShouldBeNil)
			err = s.interrupt1.Tick(s.ctx, true, uint64(time.Now().UnixNano()))
			test.That(t, err, test.ShouldBeNil)
		}

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["ButtonNorth"].Value, test.ShouldEqual, 1)
			test.That(tb, state["ButtonNorth"].Event, test.ShouldEqual, input.ButtonPress)
			test.That(tb, atomic.LoadInt64(&s.btn1Callbacks), test.ShouldEqual, 1)
		})

		time.Sleep(time.Millisecond * 10)
		test.That(t, atomic.LoadInt64(&s.btn1Callbacks), test.ShouldEqual, 1)
	})

	//nolint:dupl
	t.Run("inverted button press and release", func(t *testing.T) {
		s := setup(t)
		defer teardown(t, s)

		err := s.interrupt2.Tick(s.ctx, true, uint64(time.Now().UnixNano()))
		test.That(t, err, test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["ButtonSouth"].Value, test.ShouldEqual, 0)
			test.That(tb, state["ButtonSouth"].Event, test.ShouldEqual, input.ButtonRelease)
			test.That(tb, atomic.LoadInt64(&s.btn2Callbacks), test.ShouldEqual, 1)
		})

		err = s.interrupt2.Tick(s.ctx, false, uint64(time.Now().UnixNano()))
		test.That(t, err, test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["ButtonSouth"].Value, test.ShouldEqual, 1)
			test.That(tb, state["ButtonSouth"].Event, test.ShouldEqual, input.ButtonPress)
			test.That(tb, atomic.LoadInt64(&s.btn2Callbacks), test.ShouldEqual, 2)
		})
	})

	t.Run("inverted button press with debounce disabled", func(t *testing.T) {
		s := setup(t)
		defer teardown(t, s)

		iterations := 50

		for i := 0; i < iterations; i++ {
			err := s.interrupt2.Tick(s.ctx, true, uint64(time.Now().UnixNano()))
			test.That(t, err, test.ShouldBeNil)
			err = s.interrupt2.Tick(s.ctx, false, uint64(time.Now().UnixNano()))
			test.That(t, err, test.ShouldBeNil)
		}

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["ButtonSouth"].Value, test.ShouldEqual, 1)
			test.That(tb, state["ButtonSouth"].Event, test.ShouldEqual, input.ButtonPress)
			test.That(tb, atomic.LoadInt64(&s.btn2Callbacks), test.ShouldEqual, iterations*2)
		})

		time.Sleep(time.Millisecond * 10)
		test.That(t, atomic.LoadInt64(&s.btn2Callbacks), test.ShouldEqual, iterations*2)
	})

	t.Run("axis1 (default)", func(t *testing.T) {
		s := setup(t)
		defer teardown(t, s)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteX"].Value, test.ShouldAlmostEqual, 0, 0.005)
			test.That(tb, state["AbsoluteX"].Event, test.ShouldEqual, input.Connect)
			test.That(tb, atomic.LoadInt64(&s.axis1Callbacks), test.ShouldEqual, 0)
		})

		s.analog1.Write(s.ctx, 1023, nil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteX"].Value, test.ShouldAlmostEqual, 1, 0.005)
			test.That(tb, state["AbsoluteX"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis1Callbacks), test.ShouldEqual, 1)
		})

		s.analog1.Write(s.ctx, 511, nil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteX"].Value, test.ShouldAlmostEqual, 0.5, 0.005)
			test.That(tb, state["AbsoluteX"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis1Callbacks), test.ShouldEqual, 2)
		})
	})

	t.Run("axis deadzone", func(t *testing.T) {
		s := setup(t)
		defer teardown(t, s)

		s.analog2.Write(s.ctx, 511, nil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, 0, 0.005)
			test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis2Callbacks), test.ShouldEqual, 1)
		})

		s.analog2.Write(s.ctx, 511+20, nil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, 0.04, 0.005)
			test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis2Callbacks), test.ShouldEqual, 2)
		})

		s.analog2.Write(s.ctx, 511-20, nil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, -0.04, 0.005)
			test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis2Callbacks), test.ShouldEqual, 3)
		})

		s.analog2.Write(s.ctx, 511+19, nil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, 0, 0.005)
			test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis2Callbacks), test.ShouldEqual, 4)
		})

		s.analog2.Write(s.ctx, 511-19, nil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, 0, 0.005)
			test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis2Callbacks), test.ShouldEqual, 4)
		})
	})

	t.Run("axis min change (default)", func(t *testing.T) {
		s := setup(t)
		defer teardown(t, s)

		s.analog2.Write(s.ctx, 600, nil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, 0.17, 0.005)
			test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis2Callbacks), test.ShouldEqual, 1)
		})

		s.analog2.Write(s.ctx, 600+14, nil)
		time.Sleep(time.Millisecond * 30)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, 0.17, 0.005)
			test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis2Callbacks), test.ShouldEqual, 1)
		})

		s.analog2.Write(s.ctx, 600-14, nil)
		time.Sleep(time.Millisecond * 30)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, 0.17, 0.005)
			test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis2Callbacks), test.ShouldEqual, 1)
		})

		s.analog2.Write(s.ctx, 600-15, nil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, 0.14, 0.005)
			test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis2Callbacks), test.ShouldEqual, 2)
		})
	})

	t.Run("axis negative input and inversion", func(t *testing.T) {
		s := setup(t)
		defer teardown(t, s)

		s.analog3.Write(s.ctx, 5000, nil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteRX"].Value, test.ShouldAlmostEqual, -1, 0.005)
			test.That(tb, state["AbsoluteRX"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis3Callbacks), test.ShouldEqual, 1)
		})

		s.analog3.Write(s.ctx, -1000, nil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteRX"].Value, test.ShouldAlmostEqual, 0.2, 0.005)
			test.That(tb, state["AbsoluteRX"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis3Callbacks), test.ShouldEqual, 2)
		})
	})

	t.Run("axis range capping", func(t *testing.T) {
		s := setup(t)
		defer teardown(t, s)

		s.analog3.Write(s.ctx, -6000, nil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteRX"].Value, test.ShouldAlmostEqual, 1, 0.005)
			test.That(tb, state["AbsoluteRX"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis3Callbacks), test.ShouldEqual, 1)
		})

		s.analog3.Write(s.ctx, 6000, nil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteRX"].Value, test.ShouldAlmostEqual, -1, 0.005)
			test.That(tb, state["AbsoluteRX"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis3Callbacks), test.ShouldEqual, 2)
		})

		s.analog3.Write(s.ctx, 0, nil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteRX"].Value, test.ShouldAlmostEqual, 0, 0.005)
			test.That(tb, state["AbsoluteRX"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis3Callbacks), test.ShouldEqual, 3)
		})
	})
}
