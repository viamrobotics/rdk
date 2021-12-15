package input_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"go.viam.com/utils"
	rpcclient "go.viam.com/utils/rpc/client"
	"go.viam.com/utils/rpc/dialer"

	"go.viam.com/core/component/input"
	pb "go.viam.com/core/proto/api/component/v1"
	"go.viam.com/core/resource"
	"go.viam.com/core/subtype"
	"go.viam.com/core/testutils/inject"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"google.golang.org/grpc"

	viamgrpc "go.viam.com/core/grpc"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, err, test.ShouldBeNil)
	gServer1 := grpc.NewServer()

	inputController1 := "inputController1"
	injectInputController := &inject.InjectableInputController{}
	injectInputController.ControlsFunc = func(ctx context.Context) ([]input.Control, error) {
		return []input.Control{input.AbsoluteX, input.ButtonStart}, nil
	}
	injectInputController.LastEventsFunc = func(ctx context.Context) (map[input.Control]input.Event, error) {
		eventsOut := make(map[input.Control]input.Event)
		eventsOut[input.AbsoluteX] = input.Event{Time: time.Now(), Event: input.PositionChangeAbs, Control: input.AbsoluteX, Value: 0.7}
		eventsOut[input.ButtonStart] = input.Event{Time: time.Now(), Event: input.ButtonPress, Control: input.ButtonStart, Value: 1.0}
		return eventsOut, nil
	}
	evStream := make(chan input.Event)
	injectInputController.RegisterControlCallbackFunc = func(ctx context.Context, control input.Control, triggers []input.EventType, ctrlFunc input.ControlFunction) error {
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

	inputController2 := "inputController2"
	injectInputController2 := &inject.InputController{}
	injectInputController2.ControlsFunc = func(ctx context.Context) ([]input.Control, error) {
		return nil, errors.New("can't get controls")
	}
	injectInputController2.LastEventsFunc = func(ctx context.Context) (map[input.Control]input.Event, error) {
		return nil, errors.New("can't get last events")
	}
	injectInputController2.RegisterControlCallbackFunc = func(ctx context.Context, control input.Control, triggers []input.EventType, ctrlFunc input.ControlFunction) error {
		return errors.New("can't register callbacks")
	}

	resources := map[resource.Name]interface{}{
		input.Named(inputController1): injectInputController,
		input.Named(inputController2): injectInputController2,
	}
	inputControllerSvc, err := subtype.New(resources)
	test.That(t, err, test.ShouldBeNil)
	pb.RegisterInputControllerServiceServer(gServer1, input.NewServer(inputControllerSvc))

	go gServer1.Serve(listener1)
	defer gServer1.Stop()

	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = input.NewClient(cancelCtx, inputController1, listener1.Addr().String(), rpcclient.DialOptions{Insecure: true}, logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	t.Run("input controller client 1", func(t *testing.T) {
		inputController1Client, err := input.NewClient(context.Background(), inputController1, listener1.Addr().String(), rpcclient.DialOptions{Insecure: true}, logger)
		test.That(t, err, test.ShouldBeNil)

		controlList, err := inputController1Client.Controls(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, controlList, test.ShouldResemble, []input.Control{input.AbsoluteX, input.ButtonStart})

		startTime := time.Now()
		outState, err := inputController1Client.LastEvents(context.Background())
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
		err = inputController1Client.RegisterControlCallback(context.Background(), input.ButtonStart, []input.EventType{input.ButtonRelease}, ctrlFuncIn)
		test.That(t, err, test.ShouldBeNil)
		ev := <-evStream
		test.That(t, ev.Event, test.ShouldEqual, input.ButtonRelease)
		test.That(t, ev.Control, test.ShouldEqual, input.ButtonStart)
		test.That(t, ev.Value, test.ShouldEqual, 0.0)
		test.That(t, ev.Time.After(startTime), test.ShouldBeTrue)
		test.That(t, ev.Time.Before(time.Now()), test.ShouldBeTrue)

		err = inputController1Client.RegisterControlCallback(context.Background(), input.AbsoluteX, []input.EventType{input.PositionChangeAbs}, ctrlFuncIn)
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

		err = inputController1Client.RegisterControlCallback(context.Background(), input.AbsoluteX, []input.EventType{input.PositionChangeAbs}, nil)
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

		injectInputController.InjectEventFunc = func(ctx context.Context, event input.Event) error {
			return errors.New("can't inject event")
		}
		event1 := input.Event{
			Time:    time.Now().UTC(),
			Event:   input.PositionChangeAbs,
			Control: input.AbsoluteX,
			Value:   0.7,
		}
		injectable, ok := inputController1Client.(input.Injectable)
		test.That(t, ok, test.ShouldBeTrue)
		err = injectable.InjectEvent(context.Background(), event1)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't inject event")

		var injectedEvent input.Event

		injectInputController.InjectEventFunc = func(ctx context.Context, event input.Event) error {
			injectedEvent = event
			return nil
		}
		err = injectable.InjectEvent(context.Background(), event1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, injectedEvent, test.ShouldResemble, event1)
		injectInputController.InjectEventFunc = nil

		test.That(t, utils.TryClose(inputController1Client), test.ShouldBeNil)
	})

	t.Run("input controller client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), rpcclient.DialOptions{Insecure: true}, logger)
		test.That(t, err, test.ShouldBeNil)
		inputController2Client := input.NewClientFromConn(conn, inputController2, logger)
		test.That(t, err, test.ShouldBeNil)

		_, err = inputController2Client.Controls(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get controls")

		_, err = inputController2Client.LastEvents(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get last events")

		err = inputController2Client.RegisterControlCallback(
			context.Background(),
			input.AbsoluteX,
			[]input.EventType{input.PositionChangeAbs},
			func(ctx context.Context, ev input.Event) {},
		)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failed to connect")

		event1 := input.Event{
			Time:    time.Now().UTC(),
			Event:   input.PositionChangeAbs,
			Control: input.AbsoluteX,
			Value:   0.7,
		}
		injectable, ok := inputController2Client.(input.Injectable)
		test.That(t, ok, test.ShouldBeTrue)
		err = injectable.InjectEvent(context.Background(), event1)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not of type Injectable")

		test.That(t, utils.TryClose(inputController2Client), test.ShouldBeNil)
	})
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectInputController := &inject.InputController{}
	inputController1 := "inputController1"

	inputControllerSvc, err := subtype.New((map[resource.Name]interface{}{input.Named(inputController1): injectInputController}))
	test.That(t, err, test.ShouldBeNil)
	pb.RegisterInputControllerServiceServer(gServer, input.NewServer(inputControllerSvc))

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &trackingDialer{Dialer: dialer.NewCachedDialer()}
	ctx := dialer.ContextWithDialer(context.Background(), td)
	client1, err := input.NewClient(ctx, inputController1, listener.Addr().String(), rpcclient.DialOptions{Insecure: true}, logger)
	test.That(t, err, test.ShouldBeNil)
	client2, err := input.NewClient(ctx, inputController1, listener.Addr().String(), rpcclient.DialOptions{Insecure: true}, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.dialCalled, test.ShouldEqual, 2)

	err = utils.TryClose(client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(client2)
	test.That(t, err, test.ShouldBeNil)
}

type trackingDialer struct {
	dialer.Dialer
	dialCalled int
}

func (td *trackingDialer) DialDirect(ctx context.Context, target string, opts ...grpc.DialOption) (dialer.ClientConn, error) {
	td.dialCalled++
	return td.Dialer.DialDirect(ctx, target, opts...)
}

func (td *trackingDialer) DialFunc(proto string, target string, f func() (dialer.ClientConn, error)) (dialer.ClientConn, error) {
	td.dialCalled++
	return td.Dialer.DialFunc(proto, target, f)
}
