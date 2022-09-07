package input_test

import (
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/test"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/components/input"
	pb "go.viam.com/rdk/proto/api/component/inputcontroller/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
)

type streamServer struct {
	grpc.ServerStream
	ctx       context.Context
	messageCh chan<- *pb.StreamEventsResponse
	fail      bool
}

func (x *streamServer) Context() context.Context {
	return x.ctx
}

func (x *streamServer) Send(m *pb.StreamEventsResponse) error {
	if x.fail {
		return errors.New("send fail")
	}
	if x.messageCh == nil {
		return nil
	}
	x.messageCh <- m
	return nil
}

func newServer() (pb.InputControllerServiceServer, *inject.TriggerableInputController, *inject.InputController, error) {
	injectInputController := &inject.TriggerableInputController{}
	injectInputController2 := &inject.InputController{}
	inputControllers := map[resource.Name]interface{}{
		input.Named(testInputControllerName): injectInputController,
		input.Named(failInputControllerName): injectInputController2,
		input.Named(fakeInputControllerName): "notInputController",
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

	injectInputController.GetControlsFunc = func(ctx context.Context) ([]input.Control, error) {
		return []input.Control{input.AbsoluteX, input.ButtonStart}, nil
	}
	injectInputController.GetEventsFunc = func(ctx context.Context) (map[input.Control]input.Event, error) {
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

	injectInputController2.GetControlsFunc = func(ctx context.Context) ([]input.Control, error) {
		return nil, errors.New("can't get controls")
	}
	injectInputController2.GetEventsFunc = func(ctx context.Context) (map[input.Control]input.Event, error) {
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

	t.Run("GetControls", func(t *testing.T) {
		_, err := inputControllerServer.GetControls(
			context.Background(),
			&pb.GetControlsRequest{Controller: missingInputControllerName},
		)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no input controller")

		_, err = inputControllerServer.GetControls(
			context.Background(),
			&pb.GetControlsRequest{Controller: fakeInputControllerName},
		)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not an input controller")

		resp, err := inputControllerServer.GetControls(
			context.Background(),
			&pb.GetControlsRequest{Controller: testInputControllerName},
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Controls, test.ShouldResemble, []string{"AbsoluteX", "ButtonStart"})

		_, err = inputControllerServer.GetControls(
			context.Background(),
			&pb.GetControlsRequest{Controller: failInputControllerName},
		)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get controls")
	})

	t.Run("GetEvents", func(t *testing.T) {
		_, err := inputControllerServer.GetEvents(
			context.Background(),
			&pb.GetEventsRequest{Controller: missingInputControllerName},
		)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no input controller")

		startTime := time.Now()
		time.Sleep(time.Millisecond)
		resp, err := inputControllerServer.GetEvents(
			context.Background(),
			&pb.GetEventsRequest{Controller: testInputControllerName},
		)
		test.That(t, err, test.ShouldBeNil)
		var absEv, buttonEv *pb.Event
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

		_, err = inputControllerServer.GetEvents(
			context.Background(),
			&pb.GetEventsRequest{Controller: failInputControllerName},
		)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get last events")
	})

	t.Run("StreamEvents", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		messageCh := make(chan *pb.StreamEventsResponse, 1024)
		s := &streamServer{
			ctx:       cancelCtx,
			messageCh: messageCh,
		}

		startTime := time.Now()
		err := inputControllerServer.StreamEvents(&pb.StreamEventsRequest{Controller: missingInputControllerName}, s)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no input controller")

		eventReqList := &pb.StreamEventsRequest{
			Controller: testInputControllerName,
			Events: []*pb.StreamEventsRequest_Events{
				{
					Control: string(input.ButtonStart),
					Events: []string{
						string(input.ButtonRelease),
					},
				},
			},
		}
		relayFunc := func(ctx context.Context, event input.Event) {
			messageCh <- &pb.StreamEventsResponse{
				Event: &pb.Event{
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

		err = inputControllerServer.StreamEvents(eventReqList, s)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "send fail")

		var streamErr error
		done := make(chan struct{})
		s.fail = false

		go func() {
			streamErr = inputControllerServer.StreamEvents(eventReqList, s)
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

		eventReqList.Controller = failInputControllerName
		err = inputControllerServer.StreamEvents(eventReqList, s)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't register callbacks")
	})

	t.Run("TriggerEvent", func(t *testing.T) {
		_, err := inputControllerServer.TriggerEvent(
			context.Background(),
			&pb.TriggerEventRequest{Controller: missingInputControllerName},
		)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no input controller")

		injectInputController.TriggerEventFunc = func(ctx context.Context, event input.Event) error {
			return errors.New("can't inject event")
		}

		event1 := input.Event{
			Time:    time.Now().UTC(),
			Event:   input.PositionChangeAbs,
			Control: input.AbsoluteX,
			Value:   0.7,
		}
		pbEvent := &pb.Event{
			Time:    timestamppb.New(event1.Time),
			Event:   string(event1.Event),
			Control: string(event1.Control),
			Value:   event1.Value,
		}
		_, err = inputControllerServer.TriggerEvent(
			context.Background(),
			&pb.TriggerEventRequest{
				Controller: testInputControllerName,
				Event:      pbEvent,
			},
		)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't inject event")

		var injectedEvent input.Event

		injectInputController.TriggerEventFunc = func(ctx context.Context, event input.Event) error {
			injectedEvent = event
			return nil
		}

		_, err = inputControllerServer.TriggerEvent(
			context.Background(),
			&pb.TriggerEventRequest{Controller: testInputControllerName, Event: pbEvent},
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, injectedEvent, test.ShouldResemble, event1)
		injectInputController.TriggerEventFunc = nil

		_, err = inputControllerServer.TriggerEvent(
			context.Background(),
			&pb.TriggerEventRequest{Controller: failInputControllerName, Event: pbEvent},
		)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "is not of type Triggerable")
	})
}
