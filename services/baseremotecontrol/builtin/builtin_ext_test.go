package builtin_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/google/uuid"
	"go.viam.com/test"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/base/fake"
	"go.viam.com/rdk/components/input"
	"go.viam.com/rdk/components/input/webgamepad"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/baseremotecontrol/builtin"
	"go.viam.com/rdk/session"
	"go.viam.com/rdk/testutils/inject"
)

func TestSafetyMonitoring(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	gamepadName := input.Named("barf")
	gamepad, err := webgamepad.NewController(ctx, nil, resource.Config{}, logger)
	test.That(t, err, test.ShouldBeNil)

	myBaseName := base.Named("warf")
	injectBase := inject.NewBase(myBaseName.ShortName())

	setPowerFirst := make(chan struct{})
	injectBase.SetPowerFunc = func(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
		close(setPowerFirst)
		return nil
	}

	svc, err := builtin.NewBuiltIn(ctx, resource.Dependencies{
		gamepadName: gamepad,
		myBaseName:  injectBase,
	}, resource.Config{
		ConvertedAttributes: &builtin.Config{
			BaseName:            myBaseName.Name,
			InputControllerName: gamepadName.Name,
		},
	}, logger)
	test.That(t, err, test.ShouldBeNil)

	type triggerer interface {
		TriggerEvent(ctx context.Context, event input.Event, extra map[string]interface{}) error
	}
	test.That(t, gamepad.(triggerer).TriggerEvent(ctx, input.Event{
		Event:   input.PositionChangeAbs,
		Control: input.AbsoluteHat0X,
		Value:   1,
	}, nil), test.ShouldBeNil)

	<-setPowerFirst

	safetyFirst := make(chan struct{})
	setPowerSecond := make(chan struct{})
	injectBase.SetPowerFunc = func(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
		<-safetyFirst
		close(setPowerSecond)
		return nil
	}

	var stored sync.Once
	var storedCount int32
	var storedID uuid.UUID
	var storedResourceName resource.Name
	sess1 := session.New(context.Background(), "ownerID", time.Minute, func(id uuid.UUID, resourceName resource.Name) {
		atomic.AddInt32(&storedCount, 1)
		stored.Do(func() {
			storedID = id
			storedResourceName = resourceName
			close(safetyFirst)
		})
	})
	nextCtx := session.ToContext(context.Background(), sess1)

	test.That(t, gamepad.(triggerer).TriggerEvent(nextCtx, input.Event{
		Event:   input.PositionChangeAbs,
		Control: input.AbsoluteHat0X,
		Value:   0,
	}, nil), test.ShouldBeNil)

	<-setPowerSecond

	test.That(t, storedID, test.ShouldEqual, sess1.ID())
	test.That(t, storedResourceName, test.ShouldResemble, myBaseName)
	test.That(t, atomic.LoadInt32(&storedCount), test.ShouldEqual, 1)

	test.That(t, svc.Close(ctx), test.ShouldBeNil)
}

func TestConnectStopsBase(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	gamepadName := input.Named("barf")
	gamepad, err := webgamepad.NewController(ctx, nil, resource.Config{}, logger)
	test.That(t, err, test.ShouldBeNil)

	myBaseName := base.Named("warf")
	injectBase := &inject.Base{Base: &fake.Base{
		Named: myBaseName.AsNamed(),
	}}

	//nolint:dupl
	t.Run("connect", func(t *testing.T) {
		// Use an injected Stop function and a channel to ensure stop is called on connect.
		stop := make(chan struct{})
		injectBase.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
			close(stop)
			return nil
		}

		svc, err := builtin.NewBuiltIn(ctx, resource.Dependencies{
			gamepadName: gamepad,
			myBaseName:  injectBase,
		}, resource.Config{
			ConvertedAttributes: &builtin.Config{
				BaseName:            myBaseName.Name,
				InputControllerName: gamepadName.Name,
			},
		}, logger)
		test.That(t, err, test.ShouldBeNil)

		type triggerer interface {
			TriggerEvent(ctx context.Context, event input.Event, extra map[string]interface{}) error
		}
		test.That(t, gamepad.(triggerer).TriggerEvent(ctx, input.Event{
			Event:   input.Connect,
			Control: input.AbsoluteHat0X,
		}, nil), test.ShouldBeNil)

		<-stop
		test.That(t, svc.Close(ctx), test.ShouldBeNil)
	})

	//nolint:dupl
	t.Run("disconnect", func(t *testing.T) {
		// Use an injected Stop function and a channel to ensure stop is called on disconnect.
		stop := make(chan struct{})
		injectBase.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
			close(stop)
			return nil
		}

		svc, err := builtin.NewBuiltIn(ctx, resource.Dependencies{
			gamepadName: gamepad,
			myBaseName:  injectBase,
		}, resource.Config{
			ConvertedAttributes: &builtin.Config{
				BaseName:            myBaseName.Name,
				InputControllerName: gamepadName.Name,
			},
		}, logger)
		test.That(t, err, test.ShouldBeNil)

		type triggerer interface {
			TriggerEvent(ctx context.Context, event input.Event, extra map[string]interface{}) error
		}
		test.That(t, gamepad.(triggerer).TriggerEvent(ctx, input.Event{
			Event:   input.Disconnect,
			Control: input.AbsoluteHat0X,
		}, nil), test.ShouldBeNil)

		<-stop
		test.That(t, svc.Close(ctx), test.ShouldBeNil)
	})
}

func TestReconfigure(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	gamepadName := input.Named("barf")
	gamepad, err := webgamepad.NewController(ctx, nil, resource.Config{}, logger)
	test.That(t, err, test.ShouldBeNil)

	myBaseName := base.Named("warf")
	injectBase := inject.NewBase(myBaseName.ShortName())

	setPowerVal := make(chan r3.Vector)
	injectBase.SetPowerFunc = func(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
		setPowerVal <- angular
		return nil
	}

	svc, err := builtin.NewBuiltIn(ctx, resource.Dependencies{
		gamepadName: gamepad,
		myBaseName:  injectBase,
	}, resource.Config{
		ConvertedAttributes: &builtin.Config{
			BaseName:            myBaseName.Name,
			InputControllerName: gamepadName.Name,
		},
	}, logger)
	test.That(t, err, test.ShouldBeNil)

	type triggerer interface {
		TriggerEvent(ctx context.Context, event input.Event, extra map[string]interface{}) error
	}

	test.That(t, gamepad.(triggerer).TriggerEvent(ctx, input.Event{
		Event:   input.PositionChangeAbs,
		Control: input.AbsoluteHat0X,
		Value:   1,
	}, nil), test.ShouldBeNil)
	test.That(t, <-setPowerVal, test.ShouldResemble, r3.Vector{0, 0, -1})

	err = svc.Reconfigure(ctx, nil, resource.Config{
		ConvertedAttributes: &builtin.Config{},
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "missing from")

	test.That(t, gamepad.(triggerer).TriggerEvent(ctx, input.Event{
		Event:   input.PositionChangeAbs,
		Control: input.AbsoluteHat0X,
		Value:   2,
	}, nil), test.ShouldBeNil)
	test.That(t, <-setPowerVal, test.ShouldResemble, r3.Vector{0, 0, -2})

	err = svc.Reconfigure(ctx, resource.Dependencies{
		gamepadName: gamepad,
		myBaseName:  injectBase,
	}, resource.Config{
		ConvertedAttributes: &builtin.Config{
			BaseName:            myBaseName.Name,
			InputControllerName: gamepadName.Name,
		},
	})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, gamepad.(triggerer).TriggerEvent(ctx, input.Event{
		Event:   input.PositionChangeAbs,
		Control: input.AbsoluteHat0X,
		Value:   3,
	}, nil), test.ShouldBeNil)
	test.That(t, <-setPowerVal, test.ShouldResemble, r3.Vector{0, 0, -3})

	gamepadName2 := input.Named("hello")
	gamepad2, err := webgamepad.NewController(ctx, nil, resource.Config{}, logger)
	test.That(t, err, test.ShouldBeNil)

	err = svc.Reconfigure(ctx, resource.Dependencies{
		gamepadName2: gamepad2,
		myBaseName:   injectBase,
	}, resource.Config{
		ConvertedAttributes: &builtin.Config{
			BaseName:            myBaseName.Name,
			InputControllerName: gamepadName2.Name,
		},
	})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, gamepad.(triggerer).TriggerEvent(ctx, input.Event{
		Event:   input.PositionChangeAbs,
		Control: input.AbsoluteHat0X,
		Value:   3,
	}, nil), test.ShouldBeNil)
	test.That(t, gamepad2.(triggerer).TriggerEvent(ctx, input.Event{
		Event:   input.PositionChangeAbs,
		Control: input.AbsoluteHat0X,
		Value:   4,
	}, nil), test.ShouldBeNil)

	test.That(t, <-setPowerVal, test.ShouldResemble, r3.Vector{0, 0, -4})

	close(setPowerVal)

	myBaseName2 := base.Named("warf2")
	injectBase2 := inject.NewBase(myBaseName2.ShortName())

	setPowerVal2 := make(chan r3.Vector)
	injectBase2.SetPowerFunc = func(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
		setPowerVal2 <- angular
		return nil
	}

	err = svc.Reconfigure(ctx, resource.Dependencies{
		gamepadName2: gamepad2,
		myBaseName2:  injectBase2,
	}, resource.Config{
		ConvertedAttributes: &builtin.Config{
			BaseName:            myBaseName2.Name,
			InputControllerName: gamepadName2.Name,
		},
	})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, gamepad2.(triggerer).TriggerEvent(ctx, input.Event{
		Event:   input.PositionChangeAbs,
		Control: input.AbsoluteHat0X,
		Value:   5,
	}, nil), test.ShouldBeNil)

	test.That(t, <-setPowerVal2, test.ShouldResemble, r3.Vector{0, 0, -5})

	test.That(t, svc.Close(ctx), test.ShouldBeNil)
}
