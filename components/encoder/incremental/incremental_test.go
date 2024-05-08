package incremental

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
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

	i1, err := b.DigitalInterruptByName("11")
	test.That(t, err, test.ShouldBeNil)
	i2, err := b.DigitalInterruptByName("13")
	test.That(t, err, test.ShouldBeNil)

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

		err = i2.(*inject.DigitalInterrupt).Tick(context.Background(), true, uint64(time.Now().UnixNano()))
		test.That(t, err, test.ShouldBeNil)
		err = i1.(*inject.DigitalInterrupt).Tick(context.Background(), true, uint64(time.Now().UnixNano()))
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

		err = i1.(*inject.DigitalInterrupt).Tick(context.Background(), true, uint64(time.Now().UnixNano()))
		test.That(t, err, test.ShouldBeNil)
		err = i2.(*inject.DigitalInterrupt).Tick(context.Background(), true, uint64(time.Now().UnixNano()))
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

func MakeBoard(t *testing.T) board.Board {
	b := inject.NewBoard("test-board")
	i1 := &inject.DigitalInterrupt{}
	i2 := &inject.DigitalInterrupt{}
	callbacks := make(map[board.DigitalInterrupt]chan board.Tick)
	i1.NameFunc = func() string {
		return "11"
	}
	i2.NameFunc = func() string {
		return "13"
	}
	i1.TickFunc = func(ctx context.Context, high bool, nanoseconds uint64) error {
		ch, ok := callbacks[i1]
		test.That(t, ok, test.ShouldBeTrue)
		ch <- board.Tick{Name: i1.Name(), High: high, TimestampNanosec: nanoseconds}
		return nil
	}
	i2.TickFunc = func(ctx context.Context, high bool, nanoseconds uint64) error {
		ch, ok := callbacks[i2]
		test.That(t, ok, test.ShouldBeTrue)
		ch <- board.Tick{Name: i2.Name(), High: high, TimestampNanosec: nanoseconds}
		return nil
	}
	i1.ValueFunc = func(ctx context.Context, extra map[string]interface{}) (int64, error) {
		return 0, nil
	}
	i2.ValueFunc = func(ctx context.Context, extra map[string]interface{}) (int64, error) {
		return 0, nil
	}

	b.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, error) {
		if name == "11" {
			return i1, nil
		} else if name == "13" {
			return i2, nil
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

	return b
}
