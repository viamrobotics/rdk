package input_test

import (
	"context"
	"net"
	"testing"
	"time"

	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/input"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func TestClient(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	var extraOptions map[string]interface{}

	injectInputController := &inject.TriggerableInputController{}
	injectInputController.ControlsFunc = func(ctx context.Context, extra map[string]interface{}) ([]input.Control, error) {
		extraOptions = extra
		return []input.Control{input.AbsoluteX, input.ButtonStart}, nil
	}
	injectInputController.EventsFunc = func(ctx context.Context, extra map[string]interface{}) (map[input.Control]input.Event, error) {
		extraOptions = extra
		eventsOut := make(map[input.Control]input.Event)
		eventsOut[input.AbsoluteX] = input.Event{Time: time.Now(), Event: input.PositionChangeAbs, Control: input.AbsoluteX, Value: 0.7}
		eventsOut[input.ButtonStart] = input.Event{Time: time.Now(), Event: input.ButtonPress, Control: input.ButtonStart, Value: 1.0}
		return eventsOut, nil
	}
	evStream := make(chan input.Event)
	injectInputController.RegisterControlCallbackFunc = func(
		ctx context.Context,
		control input.Control,
		triggers []input.EventType,
		ctrlFunc input.ControlFunction,
		extra map[string]interface{},
	) error {
		extraOptions = extra
		if ctrlFunc != nil {
			outEvent := input.Event{Time: time.Now(), Event: triggers[0], Control: input.ButtonStart, Value: 0.0}
			if control == input.AbsoluteX {
				outEvent = input.Event{Time: time.Now(), Event: input.PositionChangeAbs, Control: input.AbsoluteX, Value: 0.75}
			}
			ctrlFunc(ctx, outEvent)
		} else {
			evStream <- input.Event{}
		}
		return nil
	}

	injectInputController2 := &inject.InputController{}
	injectInputController2.ControlsFunc = func(ctx context.Context, extra map[string]interface{}) ([]input.Control, error) {
		return nil, errControlsFailed
	}
	injectInputController2.EventsFunc = func(ctx context.Context, extra map[string]interface{}) (map[input.Control]input.Event, error) {
		return nil, errEventsFailed
	}

	resources := map[resource.Name]input.Controller{
		input.Named(testInputControllerName): injectInputController,
		input.Named(failInputControllerName): injectInputController2,
	}
	inputControllerSvc, err := resource.NewAPIResourceCollection(input.API, resources)
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[input.Controller](input.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, inputControllerSvc), test.ShouldBeNil)

	injectInputController.DoFunc = testutils.EchoFunc

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := viamgrpc.Dial(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, context.Canceled)
	})

	t.Run("input controller client 1", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		inputController1Client, err := input.NewClientFromConn(context.Background(), conn, "", input.Named(testInputControllerName), logger)
		test.That(t, err, test.ShouldBeNil)

		// DoCommand
		resp, err := inputController1Client.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, testutils.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])

		extra := map[string]interface{}{"foo": "Controls"}
		controlList, err := inputController1Client.Controls(context.Background(), extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, controlList, test.ShouldResemble, []input.Control{input.AbsoluteX, input.ButtonStart})
		test.That(t, extraOptions, test.ShouldResemble, extra)

		extra = map[string]interface{}{"foo": "Events"}
		startTime := time.Now()
		outState, err := inputController1Client.Events(context.Background(), extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, outState[input.ButtonStart].Event, test.ShouldEqual, input.ButtonPress)
		test.That(t, outState[input.ButtonStart].Control, test.ShouldEqual, input.ButtonStart)
		test.That(t, outState[input.ButtonStart].Value, test.ShouldEqual, 1)
		test.That(t, outState[input.ButtonStart].Time.After(startTime), test.ShouldBeTrue)
		test.That(t, outState[input.ButtonStart].Time.Before(time.Now()), test.ShouldBeTrue)

		test.That(t, outState[input.AbsoluteX].Event, test.ShouldEqual, input.PositionChangeAbs)
		test.That(t, outState[input.AbsoluteX].Control, test.ShouldEqual, input.AbsoluteX)
		test.That(t, outState[input.AbsoluteX].Value, test.ShouldEqual, 0.7)
		test.That(t, outState[input.AbsoluteX].Time.After(startTime), test.ShouldBeTrue)
		test.That(t, outState[input.AbsoluteX].Time.Before(time.Now()), test.ShouldBeTrue)
		test.That(t, extraOptions, test.ShouldResemble, extra)

		extra = map[string]interface{}{"foo": "RegisterControlCallback"}
		ctrlFuncIn := func(ctx context.Context, event input.Event) { evStream <- event }
		err = inputController1Client.RegisterControlCallback(
			context.Background(),
			input.ButtonStart,
			[]input.EventType{input.ButtonRelease},
			ctrlFuncIn,
			extra,
		)
		test.That(t, err, test.ShouldBeNil)
		ev := <-evStream
		test.That(t, ev.Event, test.ShouldEqual, input.ButtonRelease)
		test.That(t, ev.Control, test.ShouldEqual, input.ButtonStart)
		test.That(t, ev.Value, test.ShouldEqual, 0.0)
		test.That(t, ev.Time.After(startTime), test.ShouldBeTrue)
		test.That(t, ev.Time.Before(time.Now()), test.ShouldBeTrue)
		test.That(t, extraOptions, test.ShouldResemble, extra)

		err = inputController1Client.RegisterControlCallback(
			context.Background(),
			input.AbsoluteX,
			[]input.EventType{input.PositionChangeAbs},
			ctrlFuncIn,
			map[string]interface{}{},
		)
		test.That(t, err, test.ShouldBeNil)
		ev1 := <-evStream
		ev2 := <-evStream

		var btnEv, posEv input.Event
		if ev1.Control == input.ButtonStart {
			btnEv = ev1
			posEv = ev2
		} else {
			btnEv = ev2
			posEv = ev1
		}

		test.That(t, btnEv.Event, test.ShouldEqual, input.ButtonRelease)
		test.That(t, btnEv.Control, test.ShouldEqual, input.ButtonStart)
		test.That(t, btnEv.Value, test.ShouldEqual, 0.0)
		test.That(t, btnEv.Time.After(startTime), test.ShouldBeTrue)
		test.That(t, btnEv.Time.Before(time.Now()), test.ShouldBeTrue)

		test.That(t, posEv.Event, test.ShouldEqual, input.PositionChangeAbs)
		test.That(t, posEv.Control, test.ShouldEqual, input.AbsoluteX)
		test.That(t, posEv.Value, test.ShouldEqual, 0.75)
		test.That(t, posEv.Time.After(startTime), test.ShouldBeTrue)
		test.That(t, posEv.Time.Before(time.Now()), test.ShouldBeTrue)
		test.That(t, extraOptions, test.ShouldResemble, map[string]interface{}{})

		err = inputController1Client.RegisterControlCallback(
			context.Background(),
			input.AbsoluteX,
			[]input.EventType{input.PositionChangeAbs},
			nil,
			map[string]interface{}{},
		)
		test.That(t, err, test.ShouldBeNil)

		ev1 = <-evStream
		ev2 = <-evStream

		if ev1.Control == input.ButtonStart {
			btnEv = ev1
			posEv = ev2
		} else {
			btnEv = ev2
			posEv = ev1
		}

		test.That(t, posEv, test.ShouldResemble, input.Event{})

		test.That(t, btnEv.Event, test.ShouldEqual, input.ButtonRelease)
		test.That(t, btnEv.Control, test.ShouldEqual, input.ButtonStart)
		test.That(t, btnEv.Value, test.ShouldEqual, 0.0)
		test.That(t, btnEv.Time.After(startTime), test.ShouldBeTrue)
		test.That(t, btnEv.Time.Before(time.Now()), test.ShouldBeTrue)

		injectInputController.TriggerEventFunc = func(ctx context.Context, event input.Event, extra map[string]interface{}) error {
			return errTriggerEvent
		}
		event1 := input.Event{
			Time:    time.Now().UTC(),
			Event:   input.PositionChangeAbs,
			Control: input.AbsoluteX,
			Value:   0.7,
		}
		injectable, ok := inputController1Client.(input.Triggerable)
		test.That(t, ok, test.ShouldBeTrue)
		err = injectable.TriggerEvent(context.Background(), event1, map[string]interface{}{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errTriggerEvent.Error())

		var injectedEvent input.Event
		extra = map[string]interface{}{"foo": "TriggerEvent"}
		injectInputController.TriggerEventFunc = func(ctx context.Context, event input.Event, extra map[string]interface{}) error {
			injectedEvent = event
			extraOptions = extra
			return nil
		}
		err = injectable.TriggerEvent(context.Background(), event1, extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, injectedEvent, test.ShouldResemble, event1)
		test.That(t, extraOptions, test.ShouldResemble, extra)
		injectInputController.TriggerEventFunc = nil

		test.That(t, inputController1Client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("input controller client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client2, err := resourceAPI.RPCClient(context.Background(), conn, "", input.Named(failInputControllerName), logger)
		test.That(t, err, test.ShouldBeNil)

		_, err = client2.Controls(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errControlsFailed.Error())

		_, err = client2.Events(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errEventsFailed.Error())

		event1 := input.Event{
			Time:    time.Now().UTC(),
			Event:   input.PositionChangeAbs,
			Control: input.AbsoluteX,
			Value:   0.7,
		}
		injectable, ok := client2.(input.Triggerable)
		test.That(t, ok, test.ShouldBeTrue)
		err = injectable.TriggerEvent(context.Background(), event1, map[string]interface{}{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not of type Triggerable")

		test.That(t, client2.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}
