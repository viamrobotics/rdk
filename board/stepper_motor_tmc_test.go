package board

import (
	"context"
	"testing"

	"go.viam.com/utils"

	pb "go.viam.com/core/proto/api/v1"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

// check is essentially test.That with tb.Error instead of tb.Fatal (Fatal exits and leaves fakeSPI stuck waiting)
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
	b := &FakeBoard{}
	logger := golog.NewTestLogger(t)
	c := make(chan []byte)
	b.spis = map[string]*fakeSPI{}
	b.spis["main"] = &fakeSPI{fifo: c}
	mc := MotorConfig{
		Model:            "TMC5072",
		MaxAcceleration:  500,
		MaxRPM:           500,
		TicksPerRotation: 200,
		Attributes: map[string]string{
			"spi_bus":     "main",
			"chip_select": "40",
			"index":       "0",
			"sg_thresh":   "0",
			"cal_factor":  "1.0",
		},
	}

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
	m, err := NewTMCStepperMotor(context.Background(), b, mc, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, utils.TryClose(m), test.ShouldBeNil)
	}()

	// Test Go forward at half speed
	go checkTx(t, c, [][]byte{
		{160, 0, 0, 0, 1},
		{167, 0, 4, 35, 42},
	})
	test.That(t, m.Go(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 0.5), test.ShouldBeNil)

	// Test Go backward at quarter speed
	go checkTx(t, c, [][]byte{
		{160, 0, 0, 0, 2},
		{167, 0, 2, 17, 149},
	})
	test.That(t, m.Go(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, 0.25), test.ShouldBeNil)

	// Test Off
	go checkTx(t, c, [][]byte{
		{160, 0, 0, 0, 1},
		{167, 0, 0, 0, 0},
	})
	test.That(t, m.Off(ctx), test.ShouldBeNil)

	// Test GoFor (which calls Position and GoTo) with position at zero
	go checkTx(t, c, [][]byte{
		{33, 0, 0, 0, 0},
		{33, 0, 0, 0, 0},
		{160, 0, 0, 0, 0},
		{167, 0, 0, 211, 213},
		{173, 0, 2, 128, 0},
	})
	test.That(t, m.GoFor(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 50.0, 3.2), test.ShouldBeNil)

	// Test Position at 4.0 revolutions
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
	pos, err := m.Position(ctx)
	test.That(t, pos, test.ShouldEqual, 4.0)
	test.That(t, err, test.ShouldBeNil)

	// Test GoFor (which calls Position and GoTo) with position at 4.0 revolutions
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
	test.That(t, m.GoFor(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 50.0, 3.2), test.ShouldBeNil)

	// Test GoFor (which calls Position and GoTo) with position at 1.2 revolutions
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
	test.That(t, m.GoFor(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 50.0, 6.6), test.ShouldBeNil)

	// Test IsOn when off
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
	on, err := m.IsOn(ctx)
	test.That(t, on, test.ShouldEqual, false)
	test.That(t, err, test.ShouldBeNil)

	// Test IsOn when on
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
	on, err = m.IsOn(ctx)
	test.That(t, on, test.ShouldEqual, true)
	test.That(t, err, test.ShouldBeNil)

	// Test Zero with no offset (and when actually off)
	go checkTx(t, c, [][]byte{
		{34, 0, 0, 0, 0},
		{34, 0, 0, 0, 0},
		{160, 0, 0, 0, 3},
		{173, 0, 0, 0, 0},
		{161, 0, 0, 0, 0},
	})
	test.That(t, m.Zero(ctx, 0), test.ShouldBeNil)

	// Test Zero with no offset (and when actually on)
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
	test.That(t, m.Zero(ctx, 0), test.ShouldNotBeNil)

	// Test Zero with 3.1 offset (and when actually off)
	go checkTx(t, c, [][]byte{
		{34, 0, 0, 0, 0},
		{34, 0, 0, 0, 0},
		{160, 0, 0, 0, 3},
		{173, 0, 2, 108, 0},
		{161, 0, 2, 108, 0},
	})
	test.That(t, m.Zero(ctx, 3.1), test.ShouldBeNil)

	// Test GoTillStop with no extra stop func
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
	test.That(t, m.GoTillStop(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, 25.0, nil), test.ShouldBeNil)

	// Test GoTillStop with always-false stopFunc
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
	test.That(t, m.GoTillStop(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, 25.0, func(ctx context.Context) bool { return false }), test.ShouldBeNil)

	// Test GoTillStop with always true stopFunc
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
	test.That(t, m.GoTillStop(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, 25.0, func(ctx context.Context) bool { return true }), test.ShouldBeNil)

}
