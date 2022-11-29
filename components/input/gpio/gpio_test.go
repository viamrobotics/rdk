package gpio_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.uber.org/zap"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/board"
	fakeboard "go.viam.com/rdk/components/board/fake"
	"go.viam.com/rdk/components/input"
	"go.viam.com/rdk/components/input/gpio"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
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
	s.logger = golog.NewTestLogger(t)

	s.b = &fakeboard.Board{
		Digitals: map[string]board.DigitalInterrupt{},
		Analogs:  map[string]*fakeboard.Analog{},
	}

	s.b.Analogs["analog1"] = &fakeboard.Analog{}
	s.b.Analogs["analog2"] = &fakeboard.Analog{}
	s.b.Analogs["analog3"] = &fakeboard.Analog{}

	var err error
	s.b.Digitals["interrupt1"], err = board.CreateDigitalInterrupt(board.DigitalInterruptConfig{})
	test.That(t, err, test.ShouldBeNil)
	s.b.Digitals["interrupt2"], err = board.CreateDigitalInterrupt(board.DigitalInterruptConfig{})
	test.That(t, err, test.ShouldBeNil)

	deps := make(registry.Dependencies)
	deps[board.Named("main")] = s.b

	ic := gpio.Config{
		Board: "main",
		Buttons: map[string]gpio.ButtonConfig{
			"interrupt1": {
				Control:    input.ButtonNorth,
				Invert:     false,
				DebounceMs: 0,
			},
			"interrupt2": {
				Control:    input.ButtonSouth,
				Invert:     true,
				DebounceMs: -1,
			},
		},
		Axes: map[string]gpio.AxisConfig{
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

	inputReg := registry.ComponentLookup(input.Subtype, "gpio")
	test.That(t, inputReg, test.ShouldNotBeNil)

	res, err := inputReg.Constructor(context.Background(), deps, config.Component{Name: "input1", ConvertedAttributes: &ic}, s.logger)
	test.That(t, err, test.ShouldBeNil)

	var ok bool
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
	test.That(t, utils.TryClose(context.Background(), s.dev), test.ShouldBeNil)
}

func TestGPIOInput(t *testing.T) {
	// Test initial button state
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

	// Test normal button press
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

	// Test debounce at 5ms (default)
	t.Run("button press debounce at 5ms (default)", func(t *testing.T) {
		s := setup(t)
		defer teardown(t, s)

		// time race: loop must complete within the debounce time
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

	// Test inverted button press
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

	// Test with debounce disabled
	t.Run("inverted button press with debounce disabled", func(t *testing.T) {
		s := setup(t)
		defer teardown(t, s)

		for i := 0; i < 20; i++ {
			err := s.b.Digitals["interrupt2"].Tick(s.ctx, true, uint64(time.Now().UnixNano()))
			test.That(t, err, test.ShouldBeNil)
			time.Sleep(time.Millisecond)
			err = s.b.Digitals["interrupt2"].Tick(s.ctx, false, uint64(time.Now().UnixNano()))
			test.That(t, err, test.ShouldBeNil)
			time.Sleep(time.Millisecond)
		}

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["ButtonSouth"].Value, test.ShouldEqual, 1)
			test.That(tb, state["ButtonSouth"].Event, test.ShouldEqual, input.ButtonPress)
			test.That(tb, atomic.LoadInt64(&s.btn2Callbacks), test.ShouldEqual, 40)
		})

		time.Sleep(time.Millisecond * 10)
		test.That(t, atomic.LoadInt64(&s.btn2Callbacks), test.ShouldEqual, 40)
	})

	// Test axis1 (default)
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

		s.b.Analogs["analog1"].Set(1023)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteX"].Value, test.ShouldAlmostEqual, 1, 0.005)
			test.That(tb, state["AbsoluteX"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis1Callbacks), test.ShouldEqual, 1)
		})

		s.b.Analogs["analog1"].Set(511)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteX"].Value, test.ShouldAlmostEqual, 0.5, 0.005)
			test.That(tb, state["AbsoluteX"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis1Callbacks), test.ShouldEqual, 2)
		})
	})

	// Test deadzone
	t.Run("axis deadzone", func(t *testing.T) {
		s := setup(t)
		defer teardown(t, s)

		s.b.Analogs["analog2"].Set(511)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, 0, 0.005)
			test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis2Callbacks), test.ShouldEqual, 1)
		})

		s.b.Analogs["analog2"].Set(511 + 20)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, 0.04, 0.005)
			test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis2Callbacks), test.ShouldEqual, 2)
		})

		s.b.Analogs["analog2"].Set(511 - 20)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, -0.04, 0.005)
			test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis2Callbacks), test.ShouldEqual, 3)
		})

		s.b.Analogs["analog2"].Set(511 + 19)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, 0, 0.005)
			test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis2Callbacks), test.ShouldEqual, 4)
		})

		s.b.Analogs["analog2"].Set(511 - 19)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, 0, 0.005)
			test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis2Callbacks), test.ShouldEqual, 4)
		})
	})

	// Test min change (default)
	t.Run("axis min change (default)", func(t *testing.T) {
		s := setup(t)
		defer teardown(t, s)

		s.b.Analogs["analog2"].Set(600)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, 0.17, 0.005)
			test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis2Callbacks), test.ShouldEqual, 1)
		})

		s.b.Analogs["analog2"].Set(600 + 14)
		time.Sleep(time.Millisecond * 30)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, 0.17, 0.005)
			test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis2Callbacks), test.ShouldEqual, 1)
		})

		s.b.Analogs["analog2"].Set(600 - 14)
		time.Sleep(time.Millisecond * 30)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, 0.17, 0.005)
			test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis2Callbacks), test.ShouldEqual, 1)
		})

		s.b.Analogs["analog2"].Set(600 - 15)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, 0.14, 0.005)
			test.That(tb, state["AbsoluteY"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis2Callbacks), test.ShouldEqual, 2)
		})
	})
	// Test negative input and inversion
	t.Run("axis negative input and inversion", func(t *testing.T) {
		s := setup(t)
		defer teardown(t, s)

		s.b.Analogs["analog3"].Set(5000)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteRX"].Value, test.ShouldAlmostEqual, -1, 0.005)
			test.That(tb, state["AbsoluteRX"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis3Callbacks), test.ShouldEqual, 1)
		})

		s.b.Analogs["analog3"].Set(-1000)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteRX"].Value, test.ShouldAlmostEqual, 0.2, 0.005)
			test.That(tb, state["AbsoluteRX"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis3Callbacks), test.ShouldEqual, 2)
		})
	})

	// Test range capping
	t.Run("axis range capping", func(t *testing.T) {
		s := setup(t)
		defer teardown(t, s)

		s.b.Analogs["analog3"].Set(-6000)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteRX"].Value, test.ShouldAlmostEqual, 1, 0.005)
			test.That(tb, state["AbsoluteRX"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis3Callbacks), test.ShouldEqual, 1)
		})

		s.b.Analogs["analog3"].Set(6000)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteRX"].Value, test.ShouldAlmostEqual, -1, 0.005)
			test.That(tb, state["AbsoluteRX"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis3Callbacks), test.ShouldEqual, 2)
		})

		s.b.Analogs["analog3"].Set(0)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteRX"].Value, test.ShouldAlmostEqual, 0, 0.005)
			test.That(tb, state["AbsoluteRX"].Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(tb, atomic.LoadInt64(&s.axis3Callbacks), test.ShouldEqual, 3)
		})
	})

	// Test poll frequency
	t.Run("axis poll frequency", func(t *testing.T) {
		s := setup(t)
		defer teardown(t, s)

		s.b.Analogs["analog1"].Set(1023)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteX"].Value, test.ShouldAlmostEqual, 1, 0.005)
			test.That(tb, atomic.LoadInt64(&s.axis1Callbacks), test.ShouldEqual, 1)
		})

		target := 1
		for i := 1; i < 10; i++ {
			startTime := time.Now()
			if target == 0 {
				target = 1
				s.b.Analogs["analog1"].Set(1023)
			} else {
				target = 0
				s.b.Analogs["analog1"].Set(0)
			}
			testutils.WaitForAssertion(t, func(tb testing.TB) {
				tb.Helper()
				state, err := s.dev.Events(s.ctx, map[string]interface{}{})
				test.That(tb, err, test.ShouldBeNil)
				test.That(tb, state["AbsoluteX"].Value, test.ShouldAlmostEqual, target, 0.005)
				test.That(tb, atomic.LoadInt64(&s.axis1Callbacks), test.ShouldEqual, i+1)
			})
			s.axisMu.RLock()
			test.That(t, s.axis1Time.Sub(startTime), test.ShouldBeBetween, 0*time.Millisecond, 110*time.Millisecond)
			s.axisMu.RUnlock()
		}

		s.b.Analogs["analog2"].Set(1023)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			state, err := s.dev.Events(s.ctx, map[string]interface{}{})
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, 1, 0.005)
			test.That(tb, atomic.LoadInt64(&s.axis2Callbacks), test.ShouldEqual, 1)
		})

		target = 1
		for i := 1; i < 20; i++ {
			startTime := time.Now()
			if target == -1 {
				target = 1
				s.b.Analogs["analog2"].Set(1023)
			} else {
				target = -1
				s.b.Analogs["analog2"].Set(0)
			}
			testutils.WaitForAssertion(t, func(tb testing.TB) {
				tb.Helper()
				state, err := s.dev.Events(s.ctx, map[string]interface{}{})
				test.That(tb, err, test.ShouldBeNil)
				test.That(tb, state["AbsoluteY"].Value, test.ShouldAlmostEqual, target, 0.005)
				test.That(tb, atomic.LoadInt64(&s.axis2Callbacks), test.ShouldEqual, i+1)
			})
			s.axisMu.RLock()
			test.That(t, s.axis2Time.Sub(startTime), test.ShouldBeBetween, 0*time.Millisecond, 40*time.Millisecond)
			s.axisMu.RUnlock()
		}
	})
}
