//go:build linux

// Package tmcstepper contains the TMC stepper motor driver. This file contains unit tests for it.
package tmcstepper

import (
	"context"
	"fmt"
	"testing"

	"go.viam.com/test"

	fakeboard "go.viam.com/rdk/components/board/fake"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
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

func checkRx(t *testing.T, c chan []byte, expects, sends [][]byte) {
	t.Helper()
	for i, expected := range expects {
		tx := <-c
		check(t, tx, test.ShouldResemble, expected)
		c <- sends[i]
	}
}

const maxRpm = 500

func TestRPMBounds(t *testing.T) {
	ctx := context.Background()
	logger, obs := logging.NewObservedTestLogger(t)
	c := make(chan []byte)
	fakeSpi := fakeboard.SPI{FIFO: c}

	var deps resource.Dependencies

	mc := TMC5072Config{
		SPIBus:           "3",
		ChipSelect:       "40",
		Index:            0,
		SGThresh:         0,
		CalFactor:        1.0,
		MaxAcceleration:  500,
		MaxRPM:           maxRpm,
		TicksPerRotation: 200,
	}

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

	name := resource.NewName(motor.API, "motor1")
	motorDep, err := makeMotor(ctx, deps, mc, name, logger, &fakeSpi)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, motorDep.Close(context.Background()), test.ShouldBeNil)
	}()

	test.That(t, motorDep.GoFor(ctx, 0.05, 6.6, nil), test.ShouldNotBeNil)
	allObs := obs.All()
	latestLoggedEntry := allObs[len(allObs)-1]
	test.That(t, fmt.Sprint(latestLoggedEntry), test.ShouldContainSubstring, "nearly 0")

	// Check with position at 0.0 revolutions
	go checkRx(t, c,
		[][]byte{
			{33, 0, 0, 0, 0},
			{33, 0, 0, 0, 0},
			{160, 0, 0, 0, 0},
			{167, 0, 8, 70, 85},
			{173, 0, 5, 40, 0},
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
	test.That(t, motorDep.GoFor(ctx, 500, 6.6, nil), test.ShouldBeNil)
	allObs = obs.All()
	latestLoggedEntry = allObs[len(allObs)-1]
	test.That(t, fmt.Sprint(latestLoggedEntry), test.ShouldContainSubstring, "nearly the max")
}

func TestTMCStepperMotor(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	c := make(chan []byte)
	fakeSpi := fakeboard.SPI{FIFO: c}

	var deps resource.Dependencies

	mc := TMC5072Config{
		SPIBus:           "main",
		ChipSelect:       "40",
		Index:            0,
		SGThresh:         0,
		CalFactor:        1.0,
		MaxAcceleration:  500,
		MaxRPM:           maxRpm,
		TicksPerRotation: 200,
	}

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

	name := resource.NewName(motor.API, "motor1")
	motorDep, err := makeMotor(ctx, deps, mc, name, logger, &fakeSpi)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, motorDep.Close(context.Background()), test.ShouldBeNil)
	}()

	t.Run("motor supports position reporting", func(t *testing.T) {
		properties, err := motorDep.Properties(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, properties.PositionReporting, test.ShouldBeTrue)
	})

	t.Run("motor SetPower testing", func(t *testing.T) {
		// Test Go forward at half speed
		go checkTx(t, c, [][]byte{
			{160, 0, 0, 0, 1},
			{167, 0, 4, 35, 42},
		})
		test.That(t, motorDep.SetPower(ctx, 0.5, nil), test.ShouldBeNil)

		// Test Go backward at quarter speed
		go checkTx(t, c, [][]byte{
			{160, 0, 0, 0, 2},
			{167, 0, 2, 17, 149},
		})
		test.That(t, motorDep.SetPower(ctx, -0.25, nil), test.ShouldBeNil)
	})

	t.Run("motor Off testing", func(t *testing.T) {
		go checkTx(t, c, [][]byte{
			{160, 0, 0, 0, 1},
			{167, 0, 0, 0, 0},
		})
		test.That(t, motorDep.Stop(ctx, nil), test.ShouldBeNil)
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
		pos, err := motorDep.Position(ctx, nil)
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
		test.That(t, motorDep.GoFor(ctx, 50.0, 3.2, nil), test.ShouldBeNil)

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
		test.That(t, motorDep.GoFor(ctx, 50.0, 3.2, nil), test.ShouldBeNil)

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
		test.That(t, motorDep.GoFor(ctx, 50.0, 6.6, nil), test.ShouldBeNil)
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
		test.That(t, motorDep.GoFor(ctx, -50.0, 3.2, nil), test.ShouldBeNil)

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
		test.That(t, motorDep.GoFor(ctx, -50.0, 3.2, nil), test.ShouldBeNil)

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
		test.That(t, motorDep.GoFor(ctx, -50.0, 6.6, nil), test.ShouldBeNil)
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
		test.That(t, motorDep.GoFor(ctx, 50.0, -3.2, nil), test.ShouldBeNil)

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
		test.That(t, motorDep.GoFor(ctx, 50.0, -3.2, nil), test.ShouldBeNil)

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
		test.That(t, motorDep.GoFor(ctx, 50.0, -6.6, nil), test.ShouldBeNil)
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
		test.That(t, motorDep.GoFor(ctx, -50.0, -3.2, nil), test.ShouldBeNil)

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
		test.That(t, motorDep.GoFor(ctx, -50.0, -3.2, nil), test.ShouldBeNil)

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
		test.That(t, motorDep.GoFor(ctx, -50.0, -6.6, nil), test.ShouldBeNil)
	})

	t.Run("motor GoFor with zero rpm", func(t *testing.T) {
		test.That(t, motorDep.GoFor(ctx, 0, 1, nil), test.ShouldBeError, motor.NewZeroRPMError())
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
		on, powerPct, err := motorDep.IsPowered(ctx, nil)
		test.That(t, on, test.ShouldEqual, false)
		test.That(t, powerPct, test.ShouldEqual, -0.25)
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
		on, powerPct, err = motorDep.IsPowered(ctx, nil)
		test.That(t, on, test.ShouldEqual, true)
		test.That(t, powerPct, test.ShouldEqual, -0.25)
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
		test.That(t, motorDep.ResetZeroPosition(ctx, 0, nil), test.ShouldBeNil)

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
		test.That(t, motorDep.ResetZeroPosition(ctx, 0, nil), test.ShouldNotBeNil)

		// 3.1 offset (and when actually off)
		go checkRx(t, c,
			[][]byte{
				{53, 0, 0, 0, 0},
				{53, 0, 0, 0, 0},
				{160, 0, 0, 0, 3},
				{173, 255, 253, 148, 0},
				{161, 255, 253, 148, 0},
			},
			[][]byte{
				{0, 0, 0, 4, 0},
				{0, 0, 0, 4, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0},
			},
		)
		test.That(t, motorDep.ResetZeroPosition(ctx, 3.1, nil), test.ShouldBeNil)
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

		m, err := makeMotor(ctx, deps, mc, name, logger, &fakeSpi)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, m.Close(context.Background()), test.ShouldBeNil)
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

		m, err := makeMotor(ctx, deps, mc, name, logger, &fakeSpi)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, m.Close(context.Background()), test.ShouldBeNil)
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

		m, err := makeMotor(ctx, deps, mc, name, logger, &fakeSpi)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, m.Close(context.Background()), test.ShouldBeNil)
	})
}
