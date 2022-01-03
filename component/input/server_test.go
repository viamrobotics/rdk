package input_test

import (
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/test"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/component/input"
	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
)

type streamServer struct {
	grpc.ServerStream
	ctx       context.Context
	messageCh chan<- *pb.InputControllerServiceEventStreamResponse
	fail      bool
}

func (x *streamServer) Context() context.Context {
	return x.ctx
}

func (x *streamServer) Send(m *pb.InputControllerServiceEventStreamResponse) error {
	if x.fail {
		return errors.New("send fail")
	}
	if x.messageCh == nil {
		return nil
	}
	x.messageCh <- m
	return nil
}

func newServer() (pb.InputControllerServiceServer, *inject.InjectableInputController, *inject.InputController, error) {
	injectInputController := &inject.InjectableInputController{}
	injectInputController2 := &inject.InputController{}
	inputControllers := map[resource.Name]interface{}{
		input.Named("inputController1"): injectInputController,
		input.Named("inputController2"): injectInputController2,
		input.Named("inputController3"): "notInputController",
	}
	inputControllerSvc, err := subtype.New(inputControllers)
	if err != nil {
		return nil, nil, nil, err
	}
	return input.NewServer(inputControllerSvc), injectInputController, injectInputController2, nil
}

func TestServer(t *testing.T) {
	inputControllerServer, injectInputController, injectInputController2, err := newServer()
	test.That(t, err, test.ShouldBeNil)

	inputController1 := "inputController1"
	injectInputController.ControlsFunc = func(ctx context.Context) ([]input.Control, error) {
		return []input.Control{input.AbsoluteX, input.ButtonStart}, nil
	}
	injectInputController.LastEventsFunc = func(ctx context.Context) (map[input.Control]input.Event, error) {
		eventsOut := make(map[input.Control]input.Event)
		eventsOut[input.AbsoluteX] = input.Event{Time: time.Now(), Event: input.PositionChangeAbs, Control: input.AbsoluteX, Value: 0.7}
		eventsOut[input.ButtonStart] = input.Event{Time: time.Now(), Event: input.ButtonPress, Control: input.ButtonStart, Value: 1.0}
		return eventsOut, nil
	}
	injectInputController.RegisterControlCallbackFunc = func(
		ctx context.Context,
		control input.Control,
		triggers []input.EventType,
		ctrlFunc input.ControlFunction,
	) error {
		outEvent := input.Event{Time: time.Now(), Event: triggers[0], Control: input.ButtonStart, Value: 0.0}
		ctrlFunc(ctx, outEvent)
		return nil
	}

	inputController2 := "inputController2"
	injectInputController2.ControlsFunc = func(ctx context.Context) ([]input.Control, error) {
		return nil, errors.New("can't get controls")
	}
	injectInputController2.LastEventsFunc = func(ctx context.Context) (map[input.Control]input.Event, error) {
		return nil, errors.New("can't get last events")
	}
	injectInputController2.RegisterControlCallbackFunc = func(
		ctx context.Context,
		control input.Control,
		triggers []input.EventType,
		ctrlFunc input.ControlFunction,
	) error {
		return errors.New("can't register callbacks")
	}

	t.Run("Controls", func(t *testing.T) {
		_, err := inputControllerServer.Controls(context.Background(), &pb.InputControllerServiceControlsRequest{Controller: "i4"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no input controller")

		_, err = inputControllerServer.Controls(context.Background(), &pb.InputControllerServiceControlsRequest{Controller: "inputController3"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not an input controller")

		resp, err := inputControllerServer.Controls(context.Background(), &pb.InputControllerServiceControlsRequest{Controller: inputController1})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Controls, test.ShouldResemble, []string{"AbsoluteX", "ButtonStart"})

		_, err = inputControllerServer.Controls(context.Background(), &pb.InputControllerServiceControlsRequest{Controller: inputController2})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get controls")
	})

	t.Run("LastEvents", func(t *testing.T) {
		_, err := inputControllerServer.LastEvents(context.Background(), &pb.InputControllerServiceLastEventsRequest{Controller: "i4"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no input controller")

		startTime := time.Now()
		resp, err := inputControllerServer.LastEvents(
			context.Background(),
			&pb.InputControllerServiceLastEventsRequest{Controller: inputController1},
		)
		test.That(t, err, test.ShouldBeNil)
		var absEv, buttonEv *pb.InputControllerServiceEvent
		if resp.Events[0].Control == "AbsoluteX" {
			absEv = resp.Events[0]
			buttonEv = resp.Events[1]
		} else {
			absEv = resp.Events[1]
			buttonEv = resp.Events[0]
		}

		test.That(t, absEv.Event, test.ShouldEqual, input.PositionChangeAbs)
		test.That(t, absEv.Control, test.ShouldEqual, input.AbsoluteX)
		test.That(t, absEv.Value, test.ShouldEqual, 0.7)
		test.That(t, absEv.Time.AsTime().After(startTime), test.ShouldBeTrue)
		test.That(t, absEv.Time.AsTime().Before(time.Now()), test.ShouldBeTrue)

		test.That(t, buttonEv.Event, test.ShouldEqual, input.ButtonPress)
		test.That(t, buttonEv.Control, test.ShouldEqual, input.ButtonStart)
		test.That(t, buttonEv.Value, test.ShouldEqual, 1)
		test.That(t, buttonEv.Time.AsTime().After(startTime), test.ShouldBeTrue)
		test.That(t, buttonEv.Time.AsTime().Before(time.Now()), test.ShouldBeTrue)

		_, err = inputControllerServer.LastEvents(context.Background(), &pb.InputControllerServiceLastEventsRequest{Controller: inputController2})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get last events")
	})

	t.Run("EventStream", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		messageCh := make(chan *pb.InputControllerServiceEventStreamResponse, 1024)
		s := &streamServer{
			ctx:       cancelCtx,
			messageCh: messageCh,
		}

		startTime := time.Now()
		err := inputControllerServer.EventStream(&pb.InputControllerServiceEventStreamRequest{Controller: "i4"}, s)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no input controller")

		eventReqList := &pb.InputControllerServiceEventStreamRequest{
			Controller: inputController1,
			Events: []*pb.InputControllerServiceEventStreamRequest_Events{
				{
					Control: string(input.ButtonStart),
					Events: []string{
						string(input.ButtonRelease),
					},
				},
			},
		}
		relayFunc := func(ctx context.Context, event input.Event) {
			messageCh <- &pb.InputControllerServiceEventStreamResponse{
				Event: &pb.InputControllerServiceEvent{
					Time:    timestamppb.New(event.Time),
					Event:   string(event.Event),
					Control: string(event.Control),
					Value:   event.Value,
				},
			}
		}

		err = injectInputController.RegisterControlCallback(cancelCtx, input.ButtonStart, []input.EventType{input.ButtonRelease}, relayFunc)
		test.That(t, err, test.ShouldBeNil)

		s.fail = true

		err = inputControllerServer.EventStream(eventReqList, s)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "send fail")

		var streamErr error
		done := make(chan struct{})
		s.fail = false

		go func() {
			streamErr = inputControllerServer.EventStream(eventReqList, s)
			close(done)
		}()

		resp := <-messageCh
		event := resp.Event
		test.That(t, event.Control, test.ShouldEqual, string(input.ButtonStart))
		test.That(t, event.Event, test.ShouldEqual, input.ButtonRelease)
		test.That(t, event.Value, test.ShouldEqual, 0)
		test.That(t, event.Time.AsTime().After(startTime), test.ShouldBeTrue)
		test.That(t, event.Time.AsTime().Before(time.Now()), test.ShouldBeTrue)

		cancel()
		<-done
		test.That(t, streamErr, test.ShouldEqual, context.Canceled)

		eventReqList.Controller = inputController2
		err = inputControllerServer.EventStream(eventReqList, s)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't register callbacks")
	})

	t.Run("InjectEvent", func(t *testing.T) {
		_, err := inputControllerServer.InjectEvent(context.Background(), &pb.InputControllerServiceInjectEventRequest{Controller: "i4"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no input controller")

		injectInputController.InjectEventFunc = func(ctx context.Context, event input.Event) error {
			return errors.New("can't inject event")
		}

		event1 := input.Event{
			Time:    time.Now().UTC(),
			Event:   input.PositionChangeAbs,
			Control: input.AbsoluteX,
			Value:   0.7,
		}
		pbEvent := &pb.InputControllerServiceEvent{
			Time:    timestamppb.New(event1.Time),
			Event:   string(event1.Event),
			Control: string(event1.Control),
			Value:   event1.Value,
		}
		_, err = inputControllerServer.InjectEvent(
			context.Background(),
			&pb.InputControllerServiceInjectEventRequest{
				Controller: inputController1,
				Event:      pbEvent,
			},
		)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't inject event")

		var injectedEvent input.Event

		injectInputController.InjectEventFunc = func(ctx context.Context, event input.Event) error {
			injectedEvent = event
			return nil
		}

		_, err = inputControllerServer.InjectEvent(
			context.Background(),
			&pb.InputControllerServiceInjectEventRequest{Controller: inputController1, Event: pbEvent},
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, injectedEvent, test.ShouldResemble, event1)
		injectInputController.InjectEventFunc = nil

		_, err = inputControllerServer.InjectEvent(
			context.Background(),
			&pb.InputControllerServiceInjectEventRequest{Controller: inputController2, Event: pbEvent},
		)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "is not of type Injectable")
	})
}
