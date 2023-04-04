package single

import (
	"context"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/board"
	fakeboard "go.viam.com/rdk/components/board/fake"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
)

func TestConfig(t *testing.T) {
	ctx := context.Background()

	b := MakeBoard(t)

	deps := make(registry.Dependencies)
	deps[board.Named("main")] = b

	t.Run("valid config", func(t *testing.T) {
		ic := AttrConfig{
			BoardName: "main",
			Pins:      Pin{I: "10"},
		}

		rawcfg := config.Component{Name: "enc1", ConvertedAttributes: &ic}

		_, err := NewSingleEncoder(ctx, deps, rawcfg, golog.NewTestLogger(t))

		test.That(t, err, test.ShouldBeNil)
	})
	t.Run("invalid config", func(t *testing.T) {
		ic := AttrConfig{
			BoardName: "pi",
			// Pins intentionally missing
		}

		rawcfg := config.Component{Name: "enc1", ConvertedAttributes: &ic}

		_, err := NewSingleEncoder(ctx, deps, rawcfg, golog.NewTestLogger(t))
		test.That(t, err, test.ShouldNotBeNil)
	})
}

func TestEnconder(t *testing.T) {
	ctx := context.Background()

	b := MakeBoard(t)

	deps := make(registry.Dependencies)
	deps[board.Named("main")] = b

	ic := AttrConfig{
		BoardName: "main",
		Pins:      Pin{I: "10"},
	}

	rawcfg := config.Component{Name: "enc1", ConvertedAttributes: &ic}

	t.Run("run forward", func(t *testing.T) {
		enc, err := NewSingleEncoder(ctx, deps, rawcfg, golog.NewTestLogger(t))
		test.That(t, err, test.ShouldBeNil)
		enc2 := enc.(*Encoder)
		defer enc2.Close()

		m := &FakeDir{1} // forward
		enc2.AttachDirectionalAwareness(m)

		err = enc2.I.Tick(context.Background(), true, uint64(time.Now().UnixNano()))
		test.That(t, err, test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			ticks, err := enc.GetPosition(context.Background(), nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, ticks, test.ShouldEqual, 1)
		})
	})

	t.Run("run backward", func(t *testing.T) {
		enc, err := NewSingleEncoder(ctx, deps, rawcfg, golog.NewTestLogger(t))
		test.That(t, err, test.ShouldBeNil)
		enc2 := enc.(*Encoder)
		defer enc2.Close()

		m := &FakeDir{-1} // backward
		enc2.AttachDirectionalAwareness(m)

		err = enc2.I.Tick(context.Background(), true, uint64(time.Now().UnixNano()))
		test.That(t, err, test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			ticks, err := enc.GetPosition(context.Background(), nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, ticks, test.ShouldEqual, -1)
		})
	})

	// this test ensures that digital interrupts are ignored if AttachDirectionalAwareness
	// is never called
	t.Run("run no direction", func(t *testing.T) {
		enc, err := NewSingleEncoder(ctx, deps, rawcfg, golog.NewTestLogger(t))
		test.That(t, err, test.ShouldBeNil)
		enc2 := enc.(*Encoder)
		defer enc2.Close()

		err = enc2.I.Tick(context.Background(), true, uint64(time.Now().UnixNano()))
		test.That(t, err, test.ShouldBeNil)

		// Give the tick time to propagate to encoder
		// Warning: theres a race condition if the tick has not been processed
		// by the encoder worker
		time.Sleep(50 * time.Millisecond)

		ticks, err := enc.GetPosition(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ticks, test.ShouldEqual, 0)
	})

	t.Run("reset position", func(t *testing.T) {
		enc, err := NewSingleEncoder(ctx, deps, rawcfg, golog.NewTestLogger(t))
		test.That(t, err, test.ShouldBeNil)
		enc2 := enc.(*Encoder)
		defer enc2.Close()

		// set to a positive int
		err = enc.ResetPosition(context.Background(), 5.0, nil)
		test.That(t, err, test.ShouldBeNil)
		ticks, err := enc.GetPosition(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ticks, test.ShouldEqual, 5)

		// set to a negative int
		err = enc.ResetPosition(context.Background(), -5.0, nil)
		test.That(t, err, test.ShouldBeNil)
		ticks, err = enc.GetPosition(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ticks, test.ShouldEqual, -5)

		// set to a positive float
		err = enc.ResetPosition(context.Background(), 1.2, nil)
		test.That(t, err, test.ShouldBeNil)
		ticks, err = enc.GetPosition(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ticks, test.ShouldEqual, 1)

		// set to a negative float
		err = enc.ResetPosition(context.Background(), -1000.999, nil)
		test.That(t, err, test.ShouldBeNil)
		ticks, err = enc.GetPosition(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ticks, test.ShouldEqual, -1001)

		// set to 09
		err = enc.ResetPosition(context.Background(), 0, nil)
		test.That(t, err, test.ShouldBeNil)
		ticks, err = enc.GetPosition(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ticks, test.ShouldEqual, 0)
	})

	t.Run("reset position and tick", func(t *testing.T) {
		enc, err := NewSingleEncoder(ctx, deps, rawcfg, golog.NewTestLogger(t))
		test.That(t, err, test.ShouldBeNil)
		enc2 := enc.(*Encoder)
		defer enc2.Close()

		m := &FakeDir{1} // forward
		enc2.AttachDirectionalAwareness(m)

		// move forward
		err = enc2.I.Tick(context.Background(), true, uint64(time.Now().UnixNano()))
		test.That(t, err, test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			ticks, err := enc.GetPosition(context.Background(), nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, ticks, test.ShouldEqual, 1)
		})

		// reset tick
		err = enc.ResetPosition(context.Background(), 0, nil)
		test.That(t, err, test.ShouldBeNil)
		ticks, err := enc.GetPosition(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ticks, test.ShouldEqual, 0)

		// now tick up again
		err = enc2.I.Tick(context.Background(), true, uint64(time.Now().UnixNano()))
		test.That(t, err, test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			ticks, err := enc.GetPosition(context.Background(), nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, ticks, test.ShouldEqual, 1)
		})
	})
}

func MakeBoard(t *testing.T) *fakeboard.Board {
	interrupt, _ := board.CreateDigitalInterrupt(board.DigitalInterruptConfig{
		Name: "10",
		Pin:  "10",
		Type: "basic",
	})

	interrupts := map[string]board.DigitalInterrupt{
		"10": interrupt,
	}

	b := fakeboard.Board{
		GPIOPins: map[string]*fakeboard.GPIOPin{},
		Digitals: interrupts,
	}

	return &b
}

type FakeDir struct {
	dir int
}

func (f *FakeDir) DirectionMoving() int64 {
	return int64(f.dir)
}
