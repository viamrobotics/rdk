package input_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/input"
	viamgrpc "go.viam.com/rdk/grpc"
	pb "go.viam.com/rdk/proto/api/component/inputcontroller/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	injectInputController := &inject.TriggerableInputController{}
	injectInputController.GetControlsFunc = func(ctx context.Context) ([]input.Control, error) {
		return []input.Control{input.AbsoluteX, input.ButtonStart}, nil
	}
	injectInputController.GetEventsFunc = func(ctx context.Context) (map[input.Control]input.Event, error) {
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
	) error {
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
	injectInputController2.GetControlsFunc = func(ctx context.Context) ([]input.Control, error) {
		return nil, errors.New("can't get controls")
	}
	injectInputController2.GetEventsFunc = func(ctx context.Context) (map[input.Control]input.Event, error) {
		return nil, errors.New("can't get last events")
	}

	resources := map[resource.Name]interface{}{
		input.Named(testInputControllerName): injectInputController,
		input.Named(failInputControllerName): injectInputController2,
	}
	inputControllerSvc, err := subtype.New(resources)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype := registry.ResourceSubtypeLookup(input.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, inputControllerSvc)

	injectInputController.DoFunc = generic.EchoFunc
	generic.RegisterService(rpcServer, inputControllerSvc)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = input.NewClient(cancelCtx, testInputControllerName, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	t.Run("input controller client 1", func(t *testing.T) {
		inputController1Client, err := input.NewClient(
			context.Background(),
			testInputControllerName,
			listener1.Addr().String(),
			logger,
		)
		test.That(t, err, test.ShouldBeNil)

		// Do
		resp, err := inputController1Client.Do(context.Background(), generic.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, generic.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, generic.TestCommand["data"])

		controlList, err := inputController1Client.GetControls(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, controlList, test.ShouldResemble, []input.Control{input.AbsoluteX, input.ButtonStart})

		startTime := time.Now()
		outState, err := inputController1Client.GetEvents(context.Background())
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

		ctrlFuncIn := func(ctx context.Context, event input.Event) { evStream <- event }
		err = inputController1Client.RegisterControlCallback(
			context.Background(),
			input.ButtonStart,
			[]input.EventType{input.ButtonRelease},
			ctrlFuncIn,
		)
		test.That(t, err, test.ShouldBeNil)
		ev := <-evStream
		test.That(t, ev.Event, test.ShouldEqual, input.ButtonRelease)
		test.That(t, ev.Control, test.ShouldEqual, input.ButtonStart)
		test.That(t, ev.Value, test.ShouldEqual, 0.0)
		test.That(t, ev.Time.After(startTime), test.ShouldBeTrue)
		test.That(t, ev.Time.Before(time.Now()), test.ShouldBeTrue)

		err = inputController1Client.RegisterControlCallback(
			context.Background(),
			input.AbsoluteX,
			[]input.EventType{input.PositionChangeAbs},
			ctrlFuncIn,
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

		err = inputController1Client.RegisterControlCallback(
			context.Background(),
			input.AbsoluteX,
			[]input.EventType{input.PositionChangeAbs},
			nil,
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

		injectInputController.TriggerEventFunc = func(ctx context.Context, event input.Event) error {
			return errors.New("can't inject event")
		}
		event1 := input.Event{
			Time:    time.Now().UTC(),
			Event:   input.PositionChangeAbs,
			Control: input.AbsoluteX,
			Value:   0.7,
		}
		injectable, ok := inputController1Client.(input.Triggerable)
		test.That(t, ok, test.ShouldBeTrue)
		err = injectable.TriggerEvent(context.Background(), event1)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't inject event")

		var injectedEvent input.Event

		injectInputController.TriggerEventFunc = func(ctx context.Context, event input.Event) error {
			injectedEvent = event
			return nil
		}
		err = injectable.TriggerEvent(context.Background(), event1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, injectedEvent, test.ShouldResemble, event1)
		injectInputController.TriggerEventFunc = nil

		test.That(t, utils.TryClose(context.Background(), inputController1Client), test.ShouldBeNil)
	})

	t.Run("input controller client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client := resourceSubtype.RPCClient(context.Background(), conn, failInputControllerName, logger)
		inputController2Client, ok := client.(input.Controller)
		test.That(t, ok, test.ShouldBeTrue)

		_, err = inputController2Client.GetControls(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get controls")

		_, err = inputController2Client.GetEvents(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get last events")

		event1 := input.Event{
			Time:    time.Now().UTC(),
			Event:   input.PositionChangeAbs,
			Control: input.AbsoluteX,
			Value:   0.7,
		}
		injectable, ok := inputController2Client.(input.Triggerable)
		test.That(t, ok, test.ShouldBeTrue)
		err = injectable.TriggerEvent(context.Background(), event1)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not of type Triggerable")

		test.That(t, utils.TryClose(context.Background(), inputController2Client), test.ShouldBeNil)
	})
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectInputController := &inject.InputController{}

	inputControllerSvc, err := subtype.New(map[resource.Name]interface{}{input.Named(testInputControllerName): injectInputController})
	test.That(t, err, test.ShouldBeNil)
	pb.RegisterInputControllerServiceServer(gServer, input.NewServer(inputControllerSvc))

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	client1, err := input.NewClient(ctx, testInputControllerName, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)
	client2, err := input.NewClient(ctx, testInputControllerName, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)
}
