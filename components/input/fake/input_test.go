package fake

import (
	"context"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/components/input"
	"go.viam.com/rdk/resource"
)

func setupDefaultInput(t *testing.T) *InputController {
	t.Helper()
	return setupInputWithCfg(t, Config{})
}

var (
	controls = []input.Control{input.AbsoluteHat0X}
	value    = 0.7
	delay    = 200 * time.Millisecond
)

func setupDefinedInput(t *testing.T) *InputController {
	t.Helper()
	conf := Config{
		controls:         controls,
		EventValue:       &value,
		CallbackDelaySec: float64(delay/time.Millisecond) / 1000,
	}
	return setupInputWithCfg(t, conf)
}

func setupInputWithCfg(t *testing.T, conf Config) *InputController {
	t.Helper()
	var logger golog.Logger
	input, err := NewInputController(context.Background(), resource.Config{ConvertedAttributes: &conf}, logger)
	test.That(t, err, test.ShouldBeNil)
	return input.(*InputController)
}

func TestControl(t *testing.T) {
	for _, tc := range []struct {
		TestName string
		Default  bool
		Expected []input.Control
	}{
		{
			"default",
			true,
			[]input.Control{input.AbsoluteX, input.ButtonStart},
		},
		{
			"defined",
			false,
			controls,
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			var i input.Controller
			if tc.Default {
				i = setupDefaultInput(t)
			} else {
				i = setupDefinedInput(t)
			}
			defer func() {
				test.That(t, i.Close(context.Background()), test.ShouldBeNil)
			}()
			actual, err := i.Controls(context.Background(), nil)
			test.That(t, err, test.ShouldBeNil)

			test.That(t, actual, test.ShouldResemble, tc.Expected)
		})
	}
}

func TestEvents(t *testing.T) {
	for _, useDefaultInput := range []bool{true, false} {
		var tName string
		if useDefaultInput {
			tName = "default"
		} else {
			tName = "defined"
		}

		t.Run(tName, func(t *testing.T) {
			var i input.Controller
			if useDefaultInput {
				i = setupDefaultInput(t)
			} else {
				i = setupDefinedInput(t)
			}
			defer func() {
				test.That(t, i.Close(context.Background()), test.ShouldBeNil)
			}()
			actual, err := i.Events(context.Background(), nil)
			test.That(t, err, test.ShouldBeNil)

			test.That(t, len(actual), test.ShouldEqual, 1)

			event, ok := actual[input.AbsoluteX]
			test.That(t, ok, test.ShouldBeTrue)

			test.That(t, event.Event, test.ShouldEqual, input.PositionChangeAbs)
			test.That(t, event.Control, test.ShouldEqual, input.AbsoluteX)

			if useDefaultInput {
				test.That(t, event.Value, test.ShouldBeBetween, 0, 1)
			} else {
				test.That(t, event.Value, test.ShouldAlmostEqual, value)
			}
		})
	}
}

func TestRegisterControlCallback(t *testing.T) {
	i := setupDefinedInput(t)
	defer func() {
		test.That(t, i.Close(context.Background()), test.ShouldBeNil)
	}()
	calledEnough := make(chan struct{})
	var (
		callCount int
		v         float64
	)

	ctrlFunc := func(ctx context.Context, event input.Event) {
		callCount++
		if callCount == 5 {
			v = event.Value
			close(calledEnough)
		}
	}

	start := time.Now()
	err := i.RegisterControlCallback(context.Background(), input.AbsoluteHat0X, []input.EventType{input.ButtonPress}, ctrlFunc, nil)
	test.That(t, err, test.ShouldBeNil)
	<-calledEnough
	test.That(t, time.Since(start), test.ShouldBeGreaterThanOrEqualTo, 5*delay)
	test.That(t, time.Since(start), test.ShouldBeLessThanOrEqualTo, 7*delay)
	test.That(t, callCount, test.ShouldEqual, 5)
	test.That(t, v, test.ShouldAlmostEqual, value)
}

func TestTriggerEvent(t *testing.T) {
	i := setupDefaultInput(t)
	defer func() {
		test.That(t, i.Close(context.Background()), test.ShouldBeNil)
	}()
	err := i.TriggerEvent(context.Background(), input.Event{}, nil)
	test.That(t, err, test.ShouldBeError, errors.New("unsupported"))
}
