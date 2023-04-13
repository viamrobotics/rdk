package incremental

import (
	"context"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/board"
	fakeboard "go.viam.com/rdk/components/board/fake"
	"go.viam.com/rdk/components/encoder"
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
			Pins:      Pins{A: "11", B: "13"},
		}

		rawcfg := config.Component{Name: "enc1", ConvertedAttributes: &ic}

		_, err := NewIncrementalEncoder(ctx, deps, rawcfg, golog.NewTestLogger(t))

		test.That(t, err, test.ShouldBeNil)
	})
	t.Run("invalid config", func(t *testing.T) {
		ic := AttrConfig{
			BoardName: "pi",
			// Pins intentionally missing
		}

		rawcfg := config.Component{Name: "enc1", ConvertedAttributes: &ic}

		_, err := NewIncrementalEncoder(ctx, deps, rawcfg, golog.NewTestLogger(t))
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
		Pins:      Pins{A: "11", B: "13"},
	}

	rawcfg := config.Component{Name: "enc1", ConvertedAttributes: &ic}

	t.Run("run forward", func(t *testing.T) {
		enc, err := NewIncrementalEncoder(ctx, deps, rawcfg, golog.NewTestLogger(t))
		test.That(t, err, test.ShouldBeNil)
		enc2 := enc.(*Encoder)
		defer enc2.Close()

		err = enc2.B.Tick(context.Background(), true, uint64(time.Now().UnixNano()))
		test.That(t, err, test.ShouldBeNil)
		err = enc2.A.Tick(context.Background(), true, uint64(time.Now().UnixNano()))
		test.That(t, err, test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			ticks, _, err := enc.GetPosition(context.Background(), nil, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, ticks, test.ShouldEqual, 1)
		})
	})

	t.Run("run backward", func(t *testing.T) {
		enc, err := NewIncrementalEncoder(ctx, deps, rawcfg, golog.NewTestLogger(t))
		test.That(t, err, test.ShouldBeNil)
		enc2 := enc.(*Encoder)
		defer enc2.Close()

		err = enc2.A.Tick(context.Background(), false, uint64(time.Now().UnixNano()))
		test.That(t, err, test.ShouldBeNil)
		err = enc2.B.Tick(context.Background(), false, uint64(time.Now().UnixNano()))
		test.That(t, err, test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			ticks, _, err := enc.GetPosition(context.Background(), nil, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, ticks, test.ShouldEqual, -1)
		})
	})

	t.Run("reset position", func(t *testing.T) {
		enc, err := NewIncrementalEncoder(ctx, deps, rawcfg, golog.NewTestLogger(t))
		test.That(t, err, test.ShouldBeNil)
		enc2 := enc.(*Encoder)
		defer enc2.Close()

		// reset position to 0
		err = enc.ResetPosition(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		ticks, _, err := enc.GetPosition(context.Background(), nil, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ticks, test.ShouldEqual, 0)
	})

	t.Run("specify correct position type", func(t *testing.T) {
		enc, err := NewIncrementalEncoder(ctx, deps, rawcfg, golog.NewTestLogger(t))
		test.That(t, err, test.ShouldBeNil)
		enc2 := enc.(*Encoder)
		defer enc2.Close()

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			ticks, positionType, err := enc.GetPosition(context.Background(), encoder.PositionTypeTICKS.Enum(), nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, ticks, test.ShouldEqual, 0)
			test.That(tb, positionType, test.ShouldEqual, encoder.PositionTypeTICKS)
		})
	})
	t.Run("specify wrong position type", func(t *testing.T) {
		enc, err := NewIncrementalEncoder(ctx, deps, rawcfg, golog.NewTestLogger(t))
		test.That(t, err, test.ShouldBeNil)
		enc2 := enc.(*Encoder)
		defer enc2.Close()

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			ticks, positionType, err := enc.GetPosition(context.Background(), encoder.PositionTypeDEGREES.Enum(), nil)
			test.That(tb, err, test.ShouldNotBeNil)
			test.That(tb, ticks, test.ShouldEqual, 0)
			test.That(tb, positionType, test.ShouldEqual, encoder.PositionTypeDEGREES)
		})
	})

	t.Run("get properties", func(t *testing.T) {
		enc, err := NewIncrementalEncoder(ctx, deps, rawcfg, golog.NewTestLogger(t))
		test.That(t, err, test.ShouldBeNil)
		enc2 := enc.(*Encoder)
		defer enc2.Close()

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			props, err := enc.GetProperties(ctx, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, props[encoder.TicksCountSupported], test.ShouldBeTrue)
			test.That(tb, props[encoder.AngleDegreesSupported], test.ShouldBeFalse)
		})
	})
}

func MakeBoard(t *testing.T) *fakeboard.Board {
	interrupt, _ := board.CreateDigitalInterrupt(board.DigitalInterruptConfig{
		Name: "11",
		Pin:  "11",
		Type: "basic",
	})

	interrupt1, _ := board.CreateDigitalInterrupt(board.DigitalInterruptConfig{
		Name: "13",
		Pin:  "13",
		Type: "basic",
	})

	interrupts := map[string]board.DigitalInterrupt{
		"11": interrupt,
		"13": interrupt1,
	}

	b := fakeboard.Board{
		GPIOPins: map[string]*fakeboard.GPIOPin{},
		Digitals: interrupts,
	}

	return &b
}
