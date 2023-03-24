package input_test

import (
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
	pb "go.viam.com/api/component/inputcontroller/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/input"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testInputControllerName    = "inputController1"
	testInputControllerName2   = "inputController2"
	failInputControllerName    = "inputController3"
	fakeInputControllerName    = "inputController4"
	missingInputControllerName = "inputController5"
)

func TestCreateStatus(t *testing.T) {
	_, err := input.CreateStatus(context.Background(), testutils.NewUnimplementedResource(generic.Named("foo")))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected implementation")

	timestamp := time.Now()
	event := input.Event{Time: timestamp, Event: input.PositionChangeAbs, Control: input.AbsoluteX, Value: 0.7}
	status := &pb.Status{
		Events: []*pb.Event{
			{Time: timestamppb.New(timestamp), Event: string(event.Event), Control: string(event.Control), Value: event.Value},
		},
	}
	injectInputController := &inject.InputController{}
	injectInputController.EventsFunc = func(ctx context.Context, extra map[string]interface{}) (map[input.Control]input.Event, error) {
		eventsOut := make(map[input.Control]input.Event)
		eventsOut[input.AbsoluteX] = event
		return eventsOut, nil
	}

	t.Run("working", func(t *testing.T) {
		status1, err := input.CreateStatus(context.Background(), injectInputController)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status1, test.ShouldResemble, status)
	})

	t.Run("fail on Events", func(t *testing.T) {
		errFail := errors.New("can't get events")
		injectInputController.EventsFunc = func(ctx context.Context, extra map[string]interface{}) (map[input.Control]input.Event, error) {
			return nil, errFail
		}
		_, err = input.CreateStatus(context.Background(), injectInputController)
		test.That(t, err, test.ShouldBeError, errFail)
	})
}
