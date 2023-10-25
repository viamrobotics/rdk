package incremental

import (
	"context"
	"testing"
	"time"

	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/board"
	fakeboard "go.viam.com/rdk/components/board/fake"
	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

func TestConfig(t *testing.T) {
	ctx := context.Background()

	b := MakeBoard(t)

	deps := make(resource.Dependencies)
	deps[board.Named("main")] = b

	t.Run("valid config", func(t *testing.T) {
		ic := Config{
			BoardName: "main",
			Pins:      Pins{A: "11", B: "13"},
		}

		rawcfg := resource.Config{Name: "enc1", ConvertedAttributes: &ic}

		_, err := NewIncrementalEncoder(ctx, deps, rawcfg, logging.NewTestLogger(t))

		test.That(t, err, test.ShouldBeNil)
	})
	t.Run("invalid config", func(t *testing.T) {
		ic := Config{
			BoardName: "pi",
			// Pins intentionally missing
		}

		rawcfg := resource.Config{Name: "enc1", ConvertedAttributes: &ic}

		_, err := NewIncrementalEncoder(ctx, deps, rawcfg, logging.NewTestLogger(t))
		test.That(t, err, test.ShouldNotBeNil)
	})
}

func TestEncoder(t *testing.T) {
	ctx := context.Background()

	b := MakeBoard(t)

	deps := make(resource.Dependencies)
	deps[board.Named("main")] = b

	ic := Config{
		BoardName: "main",
		Pins:      Pins{A: "11", B: "13"},
	}

	rawcfg := resource.Config{Name: "enc1", ConvertedAttributes: &ic}

	t.Run("run forward", func(t *testing.T) {
		enc, err := NewIncrementalEncoder(ctx, deps, rawcfg, logging.NewTestLogger(t))
		test.That(t, err, test.ShouldBeNil)
		enc2 := enc.(*Encoder)
		defer enc2.Close(context.Background())

		err = enc2.B.Tick(context.Background(), true, uint64(time.Now().UnixNano()))
		test.That(t, err, test.ShouldBeNil)
		err = enc2.A.Tick(context.Background(), true, uint64(time.Now().UnixNano()))
		test.That(t, err, test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			ticks, _, err := enc.Position(context.Background(), encoder.PositionTypeUnspecified, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, ticks, test.ShouldEqual, 1)
		})
	})

	t.Run("run backward", func(t *testing.T) {
		enc, err := NewIncrementalEncoder(ctx, deps, rawcfg, logging.NewTestLogger(t))
		test.That(t, err, test.ShouldBeNil)
		enc2 := enc.(*Encoder)
		defer enc2.Close(context.Background())

		err = enc2.A.Tick(context.Background(), false, uint64(time.Now().UnixNano()))
		test.That(t, err, test.ShouldBeNil)
		err = enc2.B.Tick(context.Background(), false, uint64(time.Now().UnixNano()))
		test.That(t, err, test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			ticks, _, err := enc.Position(context.Background(), encoder.PositionTypeUnspecified, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, ticks, test.ShouldEqual, -1)
		})
	})

	t.Run("reset position", func(t *testing.T) {
		enc, err := NewIncrementalEncoder(ctx, deps, rawcfg, logging.NewTestLogger(t))
		test.That(t, err, test.ShouldBeNil)
		enc2 := enc.(*Encoder)
		defer enc2.Close(context.Background())

		// reset position to 0
		err = enc.ResetPosition(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		ticks, _, err := enc.Position(context.Background(), encoder.PositionTypeUnspecified, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ticks, test.ShouldEqual, 0)
	})

	t.Run("specify correct position type", func(t *testing.T) {
		enc, err := NewIncrementalEncoder(ctx, deps, rawcfg, logging.NewTestLogger(t))
		test.That(t, err, test.ShouldBeNil)
		enc2 := enc.(*Encoder)
		defer enc2.Close(context.Background())

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			ticks, positionType, err := enc.Position(context.Background(), encoder.PositionTypeTicks, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, ticks, test.ShouldEqual, 0)
			test.That(tb, positionType, test.ShouldEqual, encoder.PositionTypeTicks)
		})
	})
	t.Run("specify wrong position type", func(t *testing.T) {
		enc, err := NewIncrementalEncoder(ctx, deps, rawcfg, logging.NewTestLogger(t))
		test.That(t, err, test.ShouldBeNil)
		enc2 := enc.(*Encoder)
		defer enc2.Close(context.Background())

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			_, _, err := enc.Position(context.Background(), encoder.PositionTypeDegrees, nil)
			test.That(tb, err, test.ShouldNotBeNil)
			test.That(tb, err.Error(), test.ShouldContainSubstring, "encoder does not support")
			test.That(tb, err.Error(), test.ShouldContainSubstring, "degrees")
		})
	})

	t.Run("get properties", func(t *testing.T) {
		enc, err := NewIncrementalEncoder(ctx, deps, rawcfg, logging.NewTestLogger(t))
		test.That(t, err, test.ShouldBeNil)
		enc2 := enc.(*Encoder)
		defer enc2.Close(context.Background())

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			props, err := enc.Properties(ctx, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, props.TicksCountSupported, test.ShouldBeTrue)
			test.That(tb, props.AngleDegreesSupported, test.ShouldBeFalse)
		})
	})
}

func MakeBoard(t *testing.T) *fakeboard.Board {
	interrupt11, _ := fakeboard.NewDigitalInterruptWrapper(board.DigitalInterruptConfig{
		Name: "11",
		Pin:  "11",
		Type: "basic",
	})

	interrupt13, _ := fakeboard.NewDigitalInterruptWrapper(board.DigitalInterruptConfig{
		Name: "13",
		Pin:  "13",
		Type: "basic",
	})

	interrupts := map[string]*fakeboard.DigitalInterruptWrapper{
		"11": interrupt11,
		"13": interrupt13,
	}

	b := fakeboard.Board{
		GPIOPins: map[string]*fakeboard.GPIOPin{},
		Digitals: interrupts,
	}

	return &b
}
