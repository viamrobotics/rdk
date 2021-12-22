package tmcstepper_test

import (
	"context"
	"testing"

	"go.viam.com/utils"

	"go.viam.com/core/component/board"
	fakeboard "go.viam.com/core/component/board/fake"
	"go.viam.com/core/component/motor"
	"go.viam.com/core/component/motor/tmcstepper"
	"go.viam.com/core/config"
	"go.viam.com/core/registry"
	"go.viam.com/core/testutils/inject"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

// check is essentially test.That with tb.Error instead of tb.Fatal (Fatal exits and leaves fake.SPI stuck waiting)
func check(tb testing.TB, actual interface{}, assert func(actual interface{}, expected ...interface{}) string, expected ...interface{}) {
	tb.Helper()
	if result := assert(actual, expected...); result != "" {
		tb.Error(result)
	}
}

func checkTx(t *testing.T, c chan []byte, expects [][]byte) {
	blank := make([]byte, 5)
	for _, expected := range expects {
		tx := <-c
		check(t, tx, test.ShouldResemble, expected)
		c <- blank
	}
}

func checkRx(t *testing.T, c chan []byte, expects [][]byte, sends [][]byte) {
	for i, expected := range expects {
		tx := <-c
		check(t, tx, test.ShouldResemble, expected)
		c <- sends[i]
	}
}

func TestTMCStepperMotor(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	c := make(chan []byte)
	b := &fakeboard.Board{}
	b.SPIs = map[string]*fakeboard.SPI{}
	b.SPIs["main"] = &fakeboard.SPI{FIFO: c}
	r := inject.Robot{}
	r.BoardByNameFunc = func(name string) (board.Board, bool) {
		return b, true
	}

	mc := tmcstepper.TMC5072Config{
		SPIBus:     "main",
		ChipSelect: "40",
		Index:      0,
		SGThresh:   0,
		CalFactor:  1.0,
		Config: motor.Config{
			MaxAcceleration:  500,
			MaxRPM:           500,
			TicksPerRotation: 200,
		},
	}

	motorReg := registry.ComponentLookup(motor.Subtype, "TMC5072")
	test.That(t, motorReg, test.ShouldNotBeNil)

	// These are the setup register writes
	go checkTx(t, c, [][]byte{
		{236, 0, 1, 0, 195},
		{176, 0, 8, 15, 10},
		{237, 0, 0, 0, 0},
		{164, 0, 0, 21, 8},
		{166, 0, 0, 21, 8},
		{170, 0, 0, 21, 8},
		{168, 0, 0, 21, 8},
		{163, 0, 0, 0, 1},
		{171, 0, 0, 0, 10},
		{165, 0, 2, 17, 149},
		{177, 0, 0, 105, 234},
		{167, 0, 0, 0, 0},
		{160, 0, 0, 0, 1},
		{161, 0, 0, 0, 0},
	})

	m, err := motorReg.Constructor(context.Background(), &r, config.Component{Name: "motor1", ConvertedAttributes: &mc}, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, utils.TryClose(m), test.ShouldBeNil)
	}()
	_motor := m.(motor.Motor)

	t.Run("motor Go testing", func(t *testing.T) {
		// Test Go forward at half speed
		go checkTx(t, c, [][]byte{
			{160, 0, 0, 0, 1},
			{167, 0, 4, 35, 42},
		})
		test.That(t, _motor.Go(ctx, 0.5), test.ShouldBeNil)

		// Test Go backward at quarter speed
		go checkTx(t, c, [][]byte{
			{160, 0, 0, 0, 2},
			{167, 0, 2, 17, 149},
		})
		test.That(t, _motor.Go(ctx, -0.25), test.ShouldBeNil)
	})

	t.Run("motor Off testing", func(t *testing.T) {
		go checkTx(t, c, [][]byte{
			{160, 0, 0, 0, 1},
			{167, 0, 0, 0, 0},
		})
		test.That(t, _motor.Stop(ctx), test.ShouldBeNil)
	})

	t.Run("motor position testing", func(t *testing.T) {
		// Check at 4.0 revolutions
		go checkRx(t, c,
			[][]byte{
				{33, 0, 0, 0, 0},
				{33, 0, 0, 0, 0},
			},
			[][]byte{
				{0, 1, 6, 18, 3}, // Can be gibberish, only second register is valid
				{0, 0, 3, 32, 0},
			},
		)
		pos, err := _motor.Position(ctx)
		test.That(t, pos, test.ShouldEqual, 4.0)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("motor GoFor with positive rpm and positive revolutions", func(t *testing.T) {
		// Check with position at 0.0 revolutions
		go checkTx(t, c, [][]byte{
			{33, 0, 0, 0, 0},
			{33, 0, 0, 0, 0},
			{160, 0, 0, 0, 0},
			{167, 0, 0, 211, 213},
			{173, 0, 2, 128, 0},
		})
		test.That(t, _motor.GoFor(ctx, 50.0, 3.2), test.ShouldBeNil)

		// Check with position at 4.0 revolutions
		go checkRx(t, c,
			[][]byte{
				{33, 0, 0, 0, 0},
				{33, 0, 0, 0, 0},
				{160, 0, 0, 0, 0},
				{167, 0, 0, 211, 213},
				{173, 0, 5, 160, 0},
			},
			[][]byte{
				{0, 8, 98, 98, 7}, // Can be gibberish
				{0, 0, 3, 32, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
			},
		)
		test.That(t, _motor.GoFor(ctx, 50.0, 3.2), test.ShouldBeNil)

		// Check with position at 1.2 revolutions
		go checkRx(t, c,
			[][]byte{
				{33, 0, 0, 0, 0},
				{33, 0, 0, 0, 0},
				{160, 0, 0, 0, 0},
				{167, 0, 0, 211, 213},
				{173, 0, 6, 24, 0},
			},
			[][]byte{
				{0, 0, 0, 0, 0},
				{0, 0, 0, 240, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
			},
		)
		test.That(t, _motor.GoFor(ctx, 50.0, 6.6), test.ShouldBeNil)
	})

	t.Run("motor GoFor with negative rpm and positive revolutions", func(t *testing.T) {
		// Check with position at 0.0 revolutions
		go checkTx(t, c, [][]byte{
			{33, 0, 0, 0, 0},
			{33, 0, 0, 0, 0},
			{160, 0, 0, 0, 0},
			{167, 0, 0, 211, 213},
			{173, 255, 253, 128, 0},
		})
		test.That(t, _motor.GoFor(ctx, -50.0, 3.2), test.ShouldBeNil)

		// Check with position at 4.0 revolutions
		go checkRx(t, c,
			[][]byte{
				{33, 0, 0, 0, 0},
				{33, 0, 0, 0, 0},
				{160, 0, 0, 0, 0},
				{167, 0, 0, 211, 213},
				{173, 0, 0, 159, 255},
			},
			[][]byte{
				{0, 8, 98, 98, 7}, // Can be gibberish
				{0, 0, 3, 32, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
			},
		)
		test.That(t, _motor.GoFor(ctx, -50.0, 3.2), test.ShouldBeNil)

		// Check with position at 1.2 revolutions
		go checkRx(t, c,
			[][]byte{
				{33, 0, 0, 0, 0},
				{33, 0, 0, 0, 0},
				{160, 0, 0, 0, 0},
				{167, 0, 0, 211, 213},
				{173, 255, 251, 200, 0},
			},
			[][]byte{
				{0, 0, 0, 0, 0},
				{0, 0, 0, 240, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
			},
		)
		test.That(t, _motor.GoFor(ctx, -50.0, 6.6), test.ShouldBeNil)
	})

	t.Run("motor GoFor with positive rpm and negative revolutions", func(t *testing.T) {
		// Check with position at 0.0 revolutions
		go checkTx(t, c, [][]byte{
			{33, 0, 0, 0, 0},
			{33, 0, 0, 0, 0},
			{160, 0, 0, 0, 0},
			{167, 0, 0, 211, 213},
			{173, 255, 253, 128, 0},
		})
		test.That(t, _motor.GoFor(ctx, 50.0, -3.2), test.ShouldBeNil)

		// Check with position at 4.0 revolutions
		go checkRx(t, c,
			[][]byte{
				{33, 0, 0, 0, 0},
				{33, 0, 0, 0, 0},
				{160, 0, 0, 0, 0},
				{167, 0, 0, 211, 213},
				{173, 0, 0, 159, 255},
			},
			[][]byte{
				{0, 8, 98, 98, 7}, // Can be gibberish
				{0, 0, 3, 32, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
			},
		)
		test.That(t, _motor.GoFor(ctx, 50.0, -3.2), test.ShouldBeNil)

		// Check with position at 1.2 revolutions
		go checkRx(t, c,
			[][]byte{
				{33, 0, 0, 0, 0},
				{33, 0, 0, 0, 0},
				{160, 0, 0, 0, 0},
				{167, 0, 0, 211, 213},
				{173, 255, 251, 200, 0},
			},
			[][]byte{
				{0, 0, 0, 0, 0},
				{0, 0, 0, 240, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
			},
		)
		test.That(t, _motor.GoFor(ctx, 50.0, -6.6), test.ShouldBeNil)
	})

	t.Run("motor GoFor with negative rpm and negative revolutions", func(t *testing.T) {
		// Check with position at 0.0 revolutions
		go checkTx(t, c, [][]byte{
			{33, 0, 0, 0, 0},
			{33, 0, 0, 0, 0},
			{160, 0, 0, 0, 0},
			{167, 0, 0, 211, 213},
			{173, 0, 2, 128, 0},
		})
		test.That(t, _motor.GoFor(ctx, -50.0, -3.2), test.ShouldBeNil)

		// Check with position at 4.0 revolutions
		go checkRx(t, c,
			[][]byte{
				{33, 0, 0, 0, 0},
				{33, 0, 0, 0, 0},
				{160, 0, 0, 0, 0},
				{167, 0, 0, 211, 213},
				{173, 0, 5, 160, 0},
			},
			[][]byte{
				{0, 8, 98, 98, 7}, // Can be gibberish
				{0, 0, 3, 32, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
			},
		)
		test.That(t, _motor.GoFor(ctx, -50.0, -3.2), test.ShouldBeNil)

		// Check with position at 1.2 revolutions
		go checkRx(t, c,
			[][]byte{
				{33, 0, 0, 0, 0},
				{33, 0, 0, 0, 0},
				{160, 0, 0, 0, 0},
				{167, 0, 0, 211, 213},
				{173, 0, 6, 24, 0},
			},
			[][]byte{
				{0, 0, 0, 0, 0},
				{0, 0, 0, 240, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
			},
		)
		test.That(t, _motor.GoFor(ctx, -50.0, -6.6), test.ShouldBeNil)
	})

	t.Run("motor is on testing", func(t *testing.T) {
		// Off
		go checkRx(t, c,
			[][]byte{
				{34, 0, 0, 0, 0},
				{34, 0, 0, 0, 0},
			},
			[][]byte{
				{0, 0, 8, 18, 3}, // Can be gibberish
				{0, 0, 0, 0, 0},  // Zero velocity == "off"
			},
		)
		on, err := _motor.IsOn(ctx)
		test.That(t, on, test.ShouldEqual, false)
		test.That(t, err, test.ShouldBeNil)

		// On
		go checkRx(t, c,
			[][]byte{
				{34, 0, 0, 0, 0},
				{34, 0, 0, 0, 0},
			},
			[][]byte{
				{0, 1, 6, 18, 3}, // Can be gibberish
				{0, 0, 0, 50, 0}, // Non-Zero velocity == "on"
			},
		)
		on, err = _motor.IsOn(ctx)
		test.That(t, on, test.ShouldEqual, true)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("motor zero testing", func(t *testing.T) {
		// No offset (and when actually off)
		go checkTx(t, c, [][]byte{
			{34, 0, 0, 0, 0},
			{34, 0, 0, 0, 0},
			{160, 0, 0, 0, 3},
			{173, 0, 0, 0, 0},
			{161, 0, 0, 0, 0},
		})
		test.That(t, _motor.ResetZeroPosition(ctx, 0), test.ShouldBeNil)

		// No offset (and when actually on)
		go checkRx(t, c,
			[][]byte{
				{34, 0, 0, 0, 0},
				{34, 0, 0, 0, 0},
			},
			[][]byte{
				{0, 0, 128, 0, 0},
				{0, 0, 128, 0, 0},
			},
		)
		test.That(t, _motor.ResetZeroPosition(ctx, 0), test.ShouldNotBeNil)

		// 3.1 offset (and when actually off)
		go checkTx(t, c, [][]byte{
			{34, 0, 0, 0, 0},
			{34, 0, 0, 0, 0},
			{160, 0, 0, 0, 3},
			{173, 0, 2, 108, 0},
			{161, 0, 2, 108, 0},
		})
		test.That(t, _motor.ResetZeroPosition(ctx, 3.1), test.ShouldBeNil)
	})

	t.Run("motor gotillstop testing", func(t *testing.T) {
		go func() {
			// GoFor
			checkTx(t, c, [][]byte{
				{33, 0, 0, 0, 0},
				{33, 0, 0, 0, 0},
				{160, 0, 0, 0, 0},
				{167, 0, 0, 105, 234},
				{173, 252, 242, 192, 0},
			})

			// RampStat (for velocity reached)
			checkRx(t, c,
				[][]byte{
					{53, 0, 0, 0, 0},
					{53, 0, 0, 0, 0},
				},
				[][]byte{
					{0, 0, 0, 1, 0},
					{0, 0, 0, 1, 0},
				},
			)

			// Enable SG
			checkTx(t, c, [][]byte{
				{180, 0, 0, 4, 0},
			})

			// RampStat (for velocity zero reached)
			checkRx(t, c,
				[][]byte{
					{53, 0, 0, 0, 0},
					{53, 0, 0, 0, 0},
				},
				[][]byte{
					{0, 0, 0, 4, 0},
					{0, 0, 0, 4, 0},
				},
			)

			// Deferred off and SG disable
			checkTx(t, c, [][]byte{
				{180, 0, 0, 0, 0},
				{160, 0, 0, 0, 1},
				{167, 0, 0, 0, 0},
			})

		}()
		// No stop func
		test.That(t, _motor.GoTillStop(ctx, -25.0, nil), test.ShouldBeNil)

		go func() {
			// GoFor
			checkTx(t, c, [][]byte{
				{33, 0, 0, 0, 0},
				{33, 0, 0, 0, 0},
				{160, 0, 0, 0, 0},
				{167, 0, 0, 105, 234},
				{173, 252, 242, 192, 0},
			})

			// RampStat (for velocity reached)
			checkRx(t, c,
				[][]byte{
					{53, 0, 0, 0, 0},
					{53, 0, 0, 0, 0},
				},
				[][]byte{
					{0, 0, 0, 1, 0},
					{0, 0, 0, 1, 0},
				},
			)

			// Enable SG
			checkTx(t, c, [][]byte{
				{180, 0, 0, 4, 0},
			})

			// RampStat (for velocity zero reached)
			checkRx(t, c,
				[][]byte{
					{53, 0, 0, 0, 0},
					{53, 0, 0, 0, 0},
				},
				[][]byte{
					{0, 0, 0, 4, 0},
					{0, 0, 0, 4, 0},
				},
			)

			// Deferred off and SG disable
			checkTx(t, c, [][]byte{
				{180, 0, 0, 0, 0},
				{160, 0, 0, 0, 1},
				{167, 0, 0, 0, 0},
			})

		}()
		// Always-false stopFunc
		test.That(t, _motor.GoTillStop(ctx, -25.0, func(ctx context.Context) bool { return false }), test.ShouldBeNil)

		go func() {
			// GoFor
			checkTx(t, c, [][]byte{
				{33, 0, 0, 0, 0},
				{33, 0, 0, 0, 0},
				{160, 0, 0, 0, 0},
				{167, 0, 0, 105, 234},
				{173, 252, 242, 192, 0},
			})

			// Deferred off and SG disable
			checkTx(t, c, [][]byte{
				{180, 0, 0, 0, 0},
				{160, 0, 0, 0, 1},
				{167, 0, 0, 0, 0},
			})

		}()
		// Always true stopFunc
		test.That(t, _motor.GoTillStop(ctx, -25.0, func(ctx context.Context) bool { return true }), test.ShouldBeNil)
	})
}
