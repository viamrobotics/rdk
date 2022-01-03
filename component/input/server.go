// Package input contains a gRPC based input controller service server.
package input

import (
	"context"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/subtype"
)

// subtypeServer implements the contract from proto.
type subtypeServer struct {
	pb.UnimplementedInputControllerServiceServer
	s subtype.Service
}

// NewServer constructs an input controller gRPC service server.
func NewServer(s subtype.Service) pb.InputControllerServiceServer {
	return &subtypeServer{s: s}
}

// getInputController returns the input controller specified, nil if not.
func (s *subtypeServer) getInputController(name string) (Controller, error) {
	resource := s.s.Resource(name)
	if resource == nil {
		return nil, errors.Errorf("no input controller with name (%s)", name)
	}
	input, ok := resource.(Controller)
	if !ok {
		return nil, errors.Errorf("resource with name (%s) is not an input controller", name)
	}
	return input, nil
}

// Controls lists the inputs of an Controller.
func (s *subtypeServer) Controls(
	ctx context.Context,
	req *pb.InputControllerServiceControlsRequest,
) (*pb.InputControllerServiceControlsResponse, error) {
	controller, err := s.getInputController(req.Controller)
	if err != nil {
		return nil, err
	}

	controlList, err := controller.Controls(ctx)
	if err != nil {
		return nil, err
	}

	resp := &pb.InputControllerServiceControlsResponse{}

	for _, control := range controlList {
		resp.Controls = append(resp.Controls, string(control))
	}

	return resp, nil
}

// LastEvents returns the last Event (current state) of each control.
func (s *subtypeServer) LastEvents(
	ctx context.Context,
	req *pb.InputControllerServiceLastEventsRequest,
) (*pb.InputControllerServiceLastEventsResponse, error) {
	controller, err := s.getInputController(req.Controller)
	if err != nil {
		return nil, err
	}

	eventsIn, err := controller.LastEvents(ctx)
	if err != nil {
		return nil, err
	}

	resp := &pb.InputControllerServiceLastEventsResponse{}

	for _, eventIn := range eventsIn {
		resp.Events = append(resp.Events, &pb.InputControllerServiceEvent{
			Time:    timestamppb.New(eventIn.Time),
			Event:   string(eventIn.Event),
			Control: string(eventIn.Control),
			Value:   eventIn.Value,
		})
	}

	return resp, nil
}

// InjectEvent allows directly sending an Event (such as a button press) from external code.
func (s *subtypeServer) InjectEvent(
	ctx context.Context,
	req *pb.InputControllerServiceInjectEventRequest,
) (*pb.InputControllerServiceInjectEventResponse, error) {
	controller, err := s.getInputController(req.Controller)
	if err != nil {
		return nil, err
	}
	injectController, ok := controller.(Injectable)
	if !ok {
		return nil, errors.Errorf("input controller is not of type Injectable (%s)", req.Controller)
	}

	err = injectController.InjectEvent(ctx, Event{
		Time:    req.Event.Time.AsTime(),
		Event:   EventType(req.Event.Event),
		Control: Control(req.Event.Control),
		Value:   req.Event.Value,
	})
	if err != nil {
		return nil, err
	}

	return &pb.InputControllerServiceInjectEventResponse{}, nil
}

// EventStream returns a stream of Event.
func (s *subtypeServer) EventStream(
	req *pb.InputControllerServiceEventStreamRequest,
	server pb.InputControllerService_EventStreamServer,
) error {
	controller, err := s.getInputController(req.Controller)
	if err != nil {
		return err
	}
	eventsChan := make(chan *pb.InputControllerServiceEvent, 1024)

	ctrlFunc := func(ctx context.Context, eventIn Event) {
		resp := &pb.InputControllerServiceEvent{
			Time:    timestamppb.New(eventIn.Time),
			Event:   string(eventIn.Event),
			Control: string(eventIn.Control),
			Value:   eventIn.Value,
		}
		select {
		case eventsChan <- resp:
		case <-ctx.Done():
		}
	}

	for _, ev := range req.Events {
		var triggers []EventType
		for _, v := range ev.Events {
			triggers = append(triggers, EventType(v))
		}
		if len(triggers) > 0 {
			err := controller.RegisterControlCallback(server.Context(), Control(ev.Control), triggers, ctrlFunc)
			if err != nil {
				return err
			}
		}

		var cancelledTriggers []EventType
		for _, v := range ev.CancelledEvents {
			cancelledTriggers = append(cancelledTriggers, EventType(v))
		}
		if len(cancelledTriggers) > 0 {
			err := controller.RegisterControlCallback(server.Context(), Control(ev.Control), cancelledTriggers, nil)
			if err != nil {
				return err
			}
		}
	}

	for {
		select {
		case <-server.Context().Done():
			return server.Context().Err()
		case msg := <-eventsChan:
			err := server.Send(&pb.InputControllerServiceEventStreamResponse{Event: msg})
			if err != nil {
				return err
			}
		}
	}
}
