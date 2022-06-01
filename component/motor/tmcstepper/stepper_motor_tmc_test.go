package tmcstepper_test

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils"

	fakeboard "go.viam.com/rdk/component/board/fake"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/component/motor/tmcstepper"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

// check is essentially test.That with tb.Error instead of tb.Fatal (Fatal exits and leaves fake.SPI stuck waiting).
func check(tb testing.TB, actual interface{}, assert func(actual interface{}, expected ...interface{}) string, expected ...interface{}) {
	tb.Helper()
	if result := assert(actual, expected...); result != "" {
		tb.Error(result)
	}
}

func checkTx(t *testing.T, c chan []byte, expects [][]byte) {
	t.Helper()
	blank := make([]byte, 5)
	for _, expected := range expects {
		tx := <-c
		check(t, tx, test.ShouldResemble, expected)
		c <- blank
	}
}

func checkRx(t *testing.T, c chan []byte, expects [][]byte, sends [][]byte) {
	t.Helper()
	for i, expected := range expects {
		tx := <-c
		check(t, tx, test.ShouldResemble, expected)
		c <- sends[i]
	}
}

const maxRpm = 500

func TestTMCStepperMotor(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	c := make(chan []byte)
	b := &fakeboard.Board{}
	b.GPIOPins = map[string]*fakeboard.GPIOPin{}
	b.SPIs = map[string]*fakeboard.SPI{}
	b.SPIs["main"] = &fakeboard.SPI{FIFO: c}
	r := inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return b, nil
	}

	mc := tmcstepper.TMC5072Config{
		SPIBus:     "main",
		ChipSelect: "40",
		Index:      0,
		SGThresh:   0,
		CalFactor:  1.0,
		Config: motor.Config{
			MaxAcceleration:  500,
			MaxRPM:           maxRpm,
			TicksPerRotation: 200,
		},
	}

	motorReg := registry.ComponentLookup(motor.Subtype, "TMC5072")
	test.That(t, motorReg, test.ShouldNotBeNil)

	// These are the setup register writes
	go checkTx(t, c, [][]byte{
		{236, 0, 1, 0, 195},
		{176, 0, 6, 15, 8},
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
		test.That(t, utils.TryClose(context.Background(), m), test.ShouldBeNil)
	}()
	_motor, ok := m.(motor.Motor)
	test.That(t, ok, test.ShouldBeTrue)
	stoppableMotor, ok := _motor.(motor.LocalMotor)
	test.That(t, ok, test.ShouldBeTrue)

	t.Run("motor supports position reporting", func(t *testing.T) {
		features, err := _motor.GetFeatures(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, features[motor.PositionReporting], test.ShouldBeTrue)
	})

	t.Run("motor SetPower testing", func(t *testing.T) {
		// Test Go forward at half speed
		go checkTx(t, c, [][]byte{
			{160, 0, 0, 0, 1},
			{167, 0, 4, 35, 42},
		})
		test.That(t, _motor.SetPower(ctx, 0.5), test.ShouldBeNil)

		// Test Go backward at quarter speed
		go checkTx(t, c, [][]byte{
			{160, 0, 0, 0, 2},
			{167, 0, 2, 17, 149},
		})
		test.That(t, _motor.SetPower(ctx, -0.25), test.ShouldBeNil)
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
		pos, err := _motor.GetPosition(ctx)
		test.That(t, pos, test.ShouldEqual, 4.0)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("motor GoFor with positive rpm and positive revolutions", func(t *testing.T) {
		// Check with position at 0.0 revolutions
		go checkRx(t, c,
			[][]byte{
				{33, 0, 0, 0, 0},
				{33, 0, 0, 0, 0},
				{160, 0, 0, 0, 0},
				{167, 0, 0, 211, 213},
				{173, 0, 2, 128, 0},
				{53, 0, 0, 0, 0},
				{53, 0, 0, 0, 0},
			},
			[][]byte{
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 4, 0},
				{0, 0, 0, 4, 0},
			},
		)
		test.That(t, _motor.GoFor(ctx, 50.0, 3.2), test.ShouldBeNil)

		// Check with position at 4.0 revolutions
		go checkRx(t, c,
			[][]byte{
				{33, 0, 0, 0, 0},
				{33, 0, 0, 0, 0},
				{160, 0, 0, 0, 0},
				{167, 0, 0, 211, 213},
				{173, 0, 5, 160, 0},
				{53, 0, 0, 0, 0},
				{53, 0, 0, 0, 0},
			},
			[][]byte{
				{0, 8, 98, 98, 7}, // Can be gibberish
				{0, 0, 3, 32, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 4, 0},
				{0, 0, 0, 4, 0},
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
				{53, 0, 0, 0, 0},
				{53, 0, 0, 0, 0},
			},
			[][]byte{
				{0, 0, 0, 0, 0},
				{0, 0, 0, 240, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 4, 0},
				{0, 0, 0, 4, 0},
			},
		)
		test.That(t, _motor.GoFor(ctx, 50.0, 6.6), test.ShouldBeNil)
	})

	t.Run("motor GoFor with negative rpm and positive revolutions", func(t *testing.T) {
		// Check with position at 0.0 revolutions
		go checkRx(t, c,
			[][]byte{
				{33, 0, 0, 0, 0},
				{33, 0, 0, 0, 0},
				{160, 0, 0, 0, 0},
				{167, 0, 0, 211, 213},
				{173, 255, 253, 128, 0},
				{53, 0, 0, 0, 0},
				{53, 0, 0, 0, 0},
			},
			[][]byte{
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 4, 0},
				{0, 0, 0, 4, 0},
			},
		)
		test.That(t, _motor.GoFor(ctx, -50.0, 3.2), test.ShouldBeNil)

		// Check with position at 4.0 revolutions
		go checkRx(t, c,
			[][]byte{
				{33, 0, 0, 0, 0},
				{33, 0, 0, 0, 0},
				{160, 0, 0, 0, 0},
				{167, 0, 0, 211, 213},
				{173, 0, 0, 159, 255},
				{53, 0, 0, 0, 0},
				{53, 0, 0, 0, 0},
			},
			[][]byte{
				{0, 8, 98, 98, 7}, // Can be gibberish
				{0, 0, 3, 32, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 4, 0},
				{0, 0, 0, 4, 0},
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
				{53, 0, 0, 0, 0},
				{53, 0, 0, 0, 0},
			},
			[][]byte{
				{0, 0, 0, 0, 0},
				{0, 0, 0, 240, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 4, 0},
				{0, 0, 0, 4, 0},
			},
		)
		test.That(t, _motor.GoFor(ctx, -50.0, 6.6), test.ShouldBeNil)
	})

	t.Run("motor GoFor with positive rpm and negative revolutions", func(t *testing.T) {
		// Check with position at 0.0 revolutions
		go checkRx(t, c,
			[][]byte{
				{33, 0, 0, 0, 0},
				{33, 0, 0, 0, 0},
				{160, 0, 0, 0, 0},
				{167, 0, 0, 211, 213},
				{173, 255, 253, 128, 0},
				{53, 0, 0, 0, 0},
				{53, 0, 0, 0, 0},
			},
			[][]byte{
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 4, 0},
				{0, 0, 0, 4, 0},
			},
		)
		test.That(t, _motor.GoFor(ctx, 50.0, -3.2), test.ShouldBeNil)

		// Check with position at 4.0 revolutions
		go checkRx(t, c,
			[][]byte{
				{33, 0, 0, 0, 0},
				{33, 0, 0, 0, 0},
				{160, 0, 0, 0, 0},
				{167, 0, 0, 211, 213},
				{173, 0, 0, 159, 255},
				{53, 0, 0, 0, 0},
				{53, 0, 0, 0, 0},
			},
			[][]byte{
				{0, 8, 98, 98, 7}, // Can be gibberish
				{0, 0, 3, 32, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 4, 0},
				{0, 0, 0, 4, 0},
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
				{53, 0, 0, 0, 0},
				{53, 0, 0, 0, 0},
			},
			[][]byte{
				{0, 0, 0, 0, 0},
				{0, 0, 0, 240, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 4, 0},
				{0, 0, 0, 4, 0},
			},
		)
		test.That(t, _motor.GoFor(ctx, 50.0, -6.6), test.ShouldBeNil)
	})

	t.Run("motor GoFor with negative rpm and negative revolutions", func(t *testing.T) {
		// Check with position at 0.0 revolutions
		go checkRx(t, c,
			[][]byte{
				{33, 0, 0, 0, 0},
				{33, 0, 0, 0, 0},
				{160, 0, 0, 0, 0},
				{167, 0, 0, 211, 213},
				{173, 0, 2, 128, 0},
				{53, 0, 0, 0, 0},
				{53, 0, 0, 0, 0},
			},
			[][]byte{
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 4, 0},
				{0, 0, 0, 4, 0},
			},
		)
		test.That(t, _motor.GoFor(ctx, -50.0, -3.2), test.ShouldBeNil)

		// Check with position at 4.0 revolutions
		go checkRx(t, c,
			[][]byte{
				{33, 0, 0, 0, 0},
				{33, 0, 0, 0, 0},
				{160, 0, 0, 0, 0},
				{167, 0, 0, 211, 213},
				{173, 0, 5, 160, 0},
				{53, 0, 0, 0, 0},
				{53, 0, 0, 0, 0},
			},
			[][]byte{
				{0, 8, 98, 98, 7}, // Can be gibberish
				{0, 0, 3, 32, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 4, 0},
				{0, 0, 0, 4, 0},
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
				{53, 0, 0, 0, 0},
				{53, 0, 0, 0, 0},
			},
			[][]byte{
				{0, 0, 0, 0, 0},
				{0, 0, 0, 240, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 4, 0},
				{0, 0, 0, 4, 0},
			},
		)
		test.That(t, _motor.GoFor(ctx, -50.0, -6.6), test.ShouldBeNil)
	})

	t.Run("motor is on testing", func(t *testing.T) {
		// Off
		go checkRx(t, c,
			[][]byte{
				{53, 0, 0, 0, 0},
				{53, 0, 0, 0, 0},
			},
			[][]byte{
				{0, 0, 0, 4, 0},
				{0, 0, 0, 4, 0},
			},
		)
		on, err := _motor.IsPowered(ctx)
		test.That(t, on, test.ShouldEqual, false)
		test.That(t, err, test.ShouldBeNil)

		// On
		go checkRx(t, c,
			[][]byte{
				{53, 0, 0, 0, 0},
				{53, 0, 0, 0, 0},
			},
			[][]byte{
				{0, 0, 0, 5, 0},
				{0, 0, 0, 0, 0},
			},
		)
		on, err = _motor.IsPowered(ctx)
		test.That(t, on, test.ShouldEqual, true)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("motor zero testing", func(t *testing.T) {
		// No offset (and when actually off)
		go checkRx(t, c,
			[][]byte{
				{53, 0, 0, 0, 0},
				{53, 0, 0, 0, 0},
				{160, 0, 0, 0, 3},
				{173, 0, 0, 0, 0},
				{161, 0, 0, 0, 0},
			},
			[][]byte{
				{0, 0, 0, 4, 0},
				{0, 0, 0, 4, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
			},
		)
		test.That(t, _motor.ResetZeroPosition(ctx, 0), test.ShouldBeNil)

		// No offset (and when actually on)
		go checkRx(t, c,
			[][]byte{
				{53, 0, 0, 0, 0},
				{53, 0, 0, 0, 0},
			},
			[][]byte{
				{0, 0, 128, 0, 4},
				{0, 0, 0, 0, 0},
			},
		)
		test.That(t, _motor.ResetZeroPosition(ctx, 0), test.ShouldNotBeNil)

		// 3.1 offset (and when actually off)
		go checkRx(t, c,
			[][]byte{
				{53, 0, 0, 0, 0},
				{53, 0, 0, 0, 0},
				{160, 0, 0, 0, 3},
				{173, 0, 2, 108, 0},
				{161, 0, 2, 108, 0},
			},
			[][]byte{
				{0, 0, 0, 4, 0},
				{0, 0, 0, 4, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
			},
		)
		test.That(t, _motor.ResetZeroPosition(ctx, 3.1), test.ShouldBeNil)
	})

	t.Run("motor gotillstop testing", func(t *testing.T) {
		go func() {
			// Jog
			checkTx(t, c,
				[][]byte{
					{160, 0, 0, 0, 2},
					{167, 0, 0, 105, 234},
				},
			)

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

			// Deferred SG disable
			checkTx(t, c, [][]byte{
				{180, 0, 0, 0, 0},
				{160, 0, 0, 0, 1},
				{167, 0, 0, 0, 0},
			})
		}()
		// No stop func
		test.That(t, stoppableMotor.GoTillStop(ctx, -25.0, nil), test.ShouldBeNil)

		go func() {
			// Jog
			checkTx(t, c,
				[][]byte{
					{160, 0, 0, 0, 2},
					{167, 0, 0, 105, 234},
				},
			)

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
		test.That(t, stoppableMotor.GoTillStop(ctx, -25.0, func(ctx context.Context) bool { return false }), test.ShouldBeNil)

		go func() {
			// Jog
			checkTx(t, c,
				[][]byte{
					{160, 0, 0, 0, 2},
					{167, 0, 0, 105, 234},
				},
			)

			// Deferred off and SG disable
			checkTx(t, c, [][]byte{
				{180, 0, 0, 0, 0},
				{160, 0, 0, 0, 1},
				{167, 0, 0, 0, 0},
			})
		}()
		// Always true stopFunc
		test.That(t, stoppableMotor.GoTillStop(ctx, -25.0, func(ctx context.Context) bool { return true }), test.ShouldBeNil)
	})

	//nolint:dupl
	t.Run("test over-limit current settings", func(*testing.T) {
		mc.HoldDelay = 9999
		mc.RunCurrent = 9999
		mc.HoldCurrent = 9999

		// These are the setup register writes
		go checkTx(t, c, [][]byte{
			{236, 0, 1, 0, 195},
			{176, 0, 15, 31, 31}, // Last three are delay, run, and hold
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
		test.That(t, utils.TryClose(context.Background(), m), test.ShouldBeNil)
	})

	t.Run("test under-limit current settings", func(*testing.T) {
		mc.HoldDelay = -9999
		mc.RunCurrent = -9999
		mc.HoldCurrent = -9999

		// These are the setup register writes
		go checkTx(t, c, [][]byte{
			{236, 0, 1, 0, 195},
			{176, 0, 0, 0, 0}, // Last three are delay, run, and hold
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
		test.That(t, utils.TryClose(context.Background(), m), test.ShouldBeNil)
	})

	//nolint:dupl
	t.Run("test explicit current settings", func(*testing.T) {
		mc.HoldDelay = 12
		// Currents will be passed as one less, as default is repurposed
		mc.RunCurrent = 27
		mc.HoldCurrent = 14

		// These are the setup register writes
		go checkTx(t, c, [][]byte{
			{236, 0, 1, 0, 195},
			{176, 0, 12, 26, 13}, // Last three are delay, run, and hold
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
		test.That(t, utils.TryClose(context.Background(), m), test.ShouldBeNil)
	})
}
