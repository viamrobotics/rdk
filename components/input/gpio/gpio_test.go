package gpio

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/board"
	fakeboard "go.viam.com/rdk/components/board/fake"
	"go.viam.com/rdk/components/input"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

type setupResult struct {
	ctx                                            context.Context
	logger                                         *zap.SugaredLogger
	b                                              *fakeboard.Board
	dev                                            input.Controller
	btn1Callbacks, btn2Callbacks                   int64
	axis1Callbacks, axis2Callbacks, axis3Callbacks int64
	axis1Time, axis2Time                           time.Time
	axisMu                                         sync.RWMutex
}

func setup(t *testing.T) *setupResult {
	t.Helper()
	s := setupResult{}

	s.ctx = context.Background()
	s.logger = logging.NewTestLogger(t)

	s.b = &fakeboard.Board{
		Digitals:      map[string]*fakeboard.DigitalInterruptWrapper{},
		AnalogReaders: map[string]*fakeboard.AnalogReader{},
	}

	s.b.AnalogReaders["analog1"] = &fakeboard.AnalogReader{}
	s.b.AnalogReaders["analog2"] = &fakeboard.AnalogReader{}
	s.b.AnalogReaders["analog3"] = &fakeboard.AnalogReader{}

	var err error
	s.b.Digitals["interrupt1"], err = fakeboard.NewDigitalInterruptWrapper(board.DigitalInterruptConfig{})
	test.That(t, err, test.ShouldBeNil)
	s.b.Digitals["interrupt2"], err = fakeboard.NewDigitalInterruptWrapper(board.DigitalInterruptConfig{})
	test.That(t, err, test.ShouldBeNil)

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

		err := s.b.Digitals["interrupt1"].Tick(s.ctx, true, uint64(time.Now().UnixNano()))
		test.That(t, err, test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["ButtonNorth"].Value, test.ShouldEqual, 1)
			test.That(tb, state["ButtonNorth"].Event, test.ShouldEqual, input.ButtonPress)
			test.That(tb, atomic.LoadInt64(&s.btn1Callbacks), test.ShouldEqual, 1)
		})

		err = s.b.Digitals["interrupt1"].Tick(s.ctx, false, uint64(time.Now().UnixNano()))
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
			err := s.b.Digitals["interrupt1"].Tick(s.ctx, false, uint64(time.Now().UnixNano()))
			test.That(t, err, test.ShouldBeNil)
			err = s.b.Digitals["interrupt1"].Tick(s.ctx, true, uint64(time.Now().UnixNano()))
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

		err := s.b.Digitals["interrupt2"].Tick(s.ctx, true, uint64(time.Now().UnixNano()))
		test.That(t, err, test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["ButtonSouth"].Value, test.ShouldEqual, 0)
			test.That(tb, state["ButtonSouth"].Event, test.ShouldEqual, input.ButtonRelease)
			test.That(tb, atomic.LoadInt64(&s.btn2Callbacks), test.ShouldEqual, 1)
		})

		err = s.b.Digitals["interrupt2"].Tick(s.ctx, false, uint64(time.Now().UnixNano()))
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
			err := s.b.Digitals["interrupt2"].Tick(s.ctx, true, uint64(time.Now().UnixNano()))
			test.That(t, err, test.ShouldBeNil)
			err = s.b.Digitals["interrupt2"].Tick(s.ctx, false, uint64(time.Now().UnixNano()))
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

		s.b.AnalogReaders["analog1"].Set(1023)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteX"].Value, test.ShouldAlmostEqual, 1, 0.005)
			test.That(tb, state["AbsoluteX"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis1Callbacks), test.ShouldEqual, 1)
		})

		s.b.AnalogReaders["analog1"].Set(511)
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

		s.b.AnalogReaders["analog2"].Set(511)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, 0, 0.005)
			test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis2Callbacks), test.ShouldEqual, 1)
		})

		s.b.AnalogReaders["analog2"].Set(511 + 20)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, 0.04, 0.005)
			test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis2Callbacks), test.ShouldEqual, 2)
		})

		s.b.AnalogReaders["analog2"].Set(511 - 20)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, -0.04, 0.005)
			test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis2Callbacks), test.ShouldEqual, 3)
		})

		s.b.AnalogReaders["analog2"].Set(511 + 19)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, 0, 0.005)
			test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis2Callbacks), test.ShouldEqual, 4)
		})

		s.b.AnalogReaders["analog2"].Set(511 - 19)
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

		s.b.AnalogReaders["analog2"].Set(600)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, 0.17, 0.005)
			test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis2Callbacks), test.ShouldEqual, 1)
		})

		s.b.AnalogReaders["analog2"].Set(600 + 14)
		time.Sleep(time.Millisecond * 30)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, 0.17, 0.005)
			test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis2Callbacks), test.ShouldEqual, 1)
		})

		s.b.AnalogReaders["analog2"].Set(600 - 14)
		time.Sleep(time.Millisecond * 30)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, 0.17, 0.005)
			test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis2Callbacks), test.ShouldEqual, 1)
		})

		s.b.AnalogReaders["analog2"].Set(600 - 15)
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

		s.b.AnalogReaders["analog3"].Set(5000)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteRX"].Value, test.ShouldAlmostEqual, -1, 0.005)
			test.That(tb, state["AbsoluteRX"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis3Callbacks), test.ShouldEqual, 1)
		})

		s.b.AnalogReaders["analog3"].Set(-1000)
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

		s.b.AnalogReaders["analog3"].Set(-6000)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteRX"].Value, test.ShouldAlmostEqual, 1, 0.005)
			test.That(tb, state["AbsoluteRX"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis3Callbacks), test.ShouldEqual, 1)
		})

		s.b.AnalogReaders["analog3"].Set(6000)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteRX"].Value, test.ShouldAlmostEqual, -1, 0.005)
			test.That(tb, state["AbsoluteRX"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis3Callbacks), test.ShouldEqual, 2)
		})

		s.b.AnalogReaders["analog3"].Set(0)
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
