package single

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

const (
	testBoardName = "main"
	testPinName   = "10"
)

func TestConfig(t *testing.T) {
	ctx := context.Background()

	b := MakeBoard(t, testBoardName, testPinName)

	deps := make(resource.Dependencies)
	deps[board.Named(testBoardName)] = b

	t.Run("valid config", func(t *testing.T) {
		ic := Config{
			BoardName: testBoardName,
			Pins:      Pin{I: testPinName},
		}

		rawcfg := resource.Config{Name: "enc1", ConvertedAttributes: &ic}

		_, err := NewSingleEncoder(ctx, deps, rawcfg, logging.NewTestLogger(t))

		test.That(t, err, test.ShouldBeNil)
	})
	t.Run("invalid config", func(t *testing.T) {
		ic := Config{
			BoardName: "pi",
			// Pins intentionally missing
		}

		rawcfg := resource.Config{Name: "enc1", ConvertedAttributes: &ic}

		_, err := NewSingleEncoder(ctx, deps, rawcfg, logging.NewTestLogger(t))
		test.That(t, err, test.ShouldNotBeNil)
	})
}

func TestEncoder(t *testing.T) {
	ctx := context.Background()

	b := MakeBoard(t, testBoardName, testPinName)
	i, ok := b.DigitalInterruptByName(testPinName)
	test.That(t, ok, test.ShouldBeTrue)
	ii, ok := i.(*inject.DigitalInterrupt)
	test.That(t, ok, test.ShouldBeTrue)

	deps := make(resource.Dependencies)
	deps[board.Named(testBoardName)] = b

	ic := Config{
		BoardName: "main",
		Pins:      Pin{I: testPinName},
	}

	rawcfg := resource.Config{Name: "enc1", ConvertedAttributes: &ic}

	t.Run("run forward", func(t *testing.T) {
		enc, err := NewSingleEncoder(ctx, deps, rawcfg, logging.NewTestLogger(t))
		test.That(t, err, test.ShouldBeNil)
		enc2 := enc.(*Encoder)
		defer enc2.Close(context.Background())

		m := &FakeDir{1} // forward
		enc2.AttachDirectionalAwareness(m)

		err = ii.Tick(context.Background(), true, uint64(time.Now().UnixNano()))
		test.That(t, err, test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			ticks, _, err := enc.Position(context.Background(), encoder.PositionTypeUnspecified, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, ticks > 0, test.ShouldBeTrue)
		})
	})

	t.Run("run backward", func(t *testing.T) {
		enc, err := NewSingleEncoder(ctx, deps, rawcfg, logging.NewTestLogger(t))
		test.That(t, err, test.ShouldBeNil)
		enc2 := enc.(*Encoder)
		defer enc2.Close(context.Background())

		m := &FakeDir{-1} // backward
		enc2.AttachDirectionalAwareness(m)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			ticks, _, err := enc.Position(context.Background(), encoder.PositionTypeUnspecified, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, ticks < 0, test.ShouldBeTrue)
		})
	})

	// this test ensures that digital interrupts are ignored if AttachDirectionalAwareness
	// is never called
	t.Run("run no direction", func(t *testing.T) {
		enc, err := NewSingleEncoder(ctx, deps, rawcfg, logging.NewTestLogger(t))
		test.That(t, err, test.ShouldBeNil)
		enc2 := enc.(*Encoder)
		defer enc2.Close(context.Background())

		// Give the tick time to propagate to encoder
		// Warning: theres a race condition if the tick has not been processed
		// by the encoder worker
		time.Sleep(50 * time.Millisecond)

		ticks, _, err := enc.Position(context.Background(), encoder.PositionTypeUnspecified, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ticks, test.ShouldEqual, 0)
	})

	t.Run("reset position", func(t *testing.T) {
		enc, err := NewSingleEncoder(ctx, deps, rawcfg, logging.NewTestLogger(t))
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

	t.Run("reset position and tick", func(t *testing.T) {
		enc, err := NewSingleEncoder(ctx, deps, rawcfg, logging.NewTestLogger(t))
		test.That(t, err, test.ShouldBeNil)
		enc2 := enc.(*Encoder)
		defer enc2.Close(context.Background())

		m := &FakeDir{1} // forward
		enc2.AttachDirectionalAwareness(m)

		// move forward
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			ticks, _, err := enc.Position(context.Background(), encoder.PositionTypeUnspecified, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, ticks > 0, test.ShouldBeTrue)
		})

		// reset tick
		err = enc.ResetPosition(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		ticks, _, err := enc.Position(context.Background(), encoder.PositionTypeUnspecified, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ticks, test.ShouldEqual, 0)

		// now tick up again
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			ticks, _, err := enc.Position(context.Background(), encoder.PositionTypeUnspecified, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, ticks, test.ShouldEqual, 1)
		})
	})
	t.Run("specify correct position type", func(t *testing.T) {
		enc, err := NewSingleEncoder(ctx, deps, rawcfg, logging.NewTestLogger(t))
		test.That(t, err, test.ShouldBeNil)
		enc2 := enc.(*Encoder)
		defer enc2.Close(context.Background())

		m := &FakeDir{1} // forward
		enc2.AttachDirectionalAwareness(m)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			ticks, positionType, err := enc.Position(context.Background(), encoder.PositionTypeTicks, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, ticks > 0, test.ShouldBeTrue)
			test.That(tb, positionType, test.ShouldEqual, encoder.PositionTypeTicks)
		})
	})
	t.Run("specify wrong position type", func(t *testing.T) {
		enc, err := NewSingleEncoder(ctx, deps, rawcfg, logging.NewTestLogger(t))
		test.That(t, err, test.ShouldBeNil)
		enc2 := enc.(*Encoder)
		defer enc2.Close(context.Background())

		m := &FakeDir{-1} // backward
		enc2.AttachDirectionalAwareness(m)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			_, _, err := enc.Position(context.Background(), encoder.PositionTypeDegrees, nil)
			test.That(tb, err.Error(), test.ShouldContainSubstring, "encoder does not support")
			test.That(tb, err.Error(), test.ShouldContainSubstring, "degrees")
		})
	})
	t.Run("get properties", func(t *testing.T) {
		enc, err := NewSingleEncoder(ctx, deps, rawcfg, logging.NewTestLogger(t))
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

func MakeBoard(t *testing.T, name, pinname string) board.Board {

	b := inject.NewBoard(name)
	i := inject.DigitalInterrupt{}
	i.NameFunc = func() string {
		return testPinName
	}
	i.TickFunc = func(ctx context.Context, high bool, nanoseconds uint64) error {
		time.Sleep(50 * time.Microsecond)
		return nil

	}
	i.RemoveCallbackFunc = func(c chan board.Tick) {}

	b.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) {
		return &i, true
	}
	b.StreamTicksFunc = func(ctx context.Context, interrupts []board.DigitalInterrupt, ch chan board.Tick, extra map[string]interface{}) error {
		// Check if the channel is ready to receive
		select {
		case ch <- board.Tick{Name: "dummy", High: true, TimestampNanosec: 10000000}:
			// Value sent successfully
		default:
			// Channel not ready to receive; do nothing
		}

		// Receive the value from the channel if available
		select {
		case tick := <-ch:
			fmt.Println("Received tick:", tick)
		default:
			// No value available on the channel; do nothing
		}

		// Close the channel
		close(ch)
		return nil
	}

	return b
}

type FakeDir struct {
	dir int
}

func (f *FakeDir) DirectionMoving() int64 {
	return int64(f.dir)
}
