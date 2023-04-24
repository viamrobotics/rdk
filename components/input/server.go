// Package input contains a gRPC based input controller service server.
package input

import (
	"context"

	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/inputcontroller/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// serviceServer implements the InputControllerService from proto.
type serviceServer struct {
	pb.UnimplementedInputControllerServiceServer
	coll resource.APIResourceCollection[Controller]
}

// NewRPCServiceServer constructs an input controller gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.APIResourceCollection[Controller]) interface{} {
	return &serviceServer{coll: coll}
}

// GetControls lists the inputs of an Controller.
func (s *serviceServer) GetControls(
	ctx context.Context,
	req *pb.GetControlsRequest,
) (*pb.GetControlsResponse, error) {
	controller, err := s.coll.Resource(req.Controller)
	if err != nil {
		return nil, err
	}

	controlList, err := controller.Controls(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}

	resp := &pb.GetControlsResponse{}

	for _, control := range controlList {
		resp.Controls = append(resp.Controls, string(control))
	}
	return resp, nil
}

// GetEvents returns the last Event (current state) of each control.
func (s *serviceServer) GetEvents(
	ctx context.Context,
	req *pb.GetEventsRequest,
) (*pb.GetEventsResponse, error) {
	controller, err := s.coll.Resource(req.Controller)
	if err != nil {
		return nil, err
	}

	eventsIn, err := controller.Events(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}

	resp := &pb.GetEventsResponse{}

	for _, eventIn := range eventsIn {
		resp.Events = append(resp.Events, &pb.Event{
			Time:    timestamppb.New(eventIn.Time),
			Event:   string(eventIn.Event),
			Control: string(eventIn.Control),
			Value:   eventIn.Value,
		})
	}

	return resp, nil
}

// TriggerEvent allows directly sending an Event (such as a button press) from external code.
func (s *serviceServer) TriggerEvent(
	ctx context.Context,
	req *pb.TriggerEventRequest,
) (*pb.TriggerEventResponse, error) {
	controller, err := s.coll.Resource(req.Controller)
	if err != nil {
		return nil, err
	}
	injectController, ok := controller.(Triggerable)
	if !ok {
		return nil, errors.Errorf("input controller is not of type Triggerable (%s)", req.Controller)
	}

	err = injectController.TriggerEvent(
		ctx,
		Event{
			Time:    req.Event.Time.AsTime(),
			Event:   EventType(req.Event.Event),
			Control: Control(req.Event.Control),
			Value:   req.Event.Value,
		},
		req.Extra.AsMap(),
	)
	if err != nil {
		return nil, err
	}

	return &pb.TriggerEventResponse{}, nil
}

// StreamEvents returns a stream of Event.
func (s *serviceServer) StreamEvents(
	req *pb.StreamEventsRequest,
	server pb.InputControllerService_StreamEventsServer,
) error {
	controller, err := s.coll.Resource(req.Controller)
	if err != nil {
		return err
	}
	eventsChan := make(chan *pb.Event, 1024)

	ctrlFunc := func(ctx context.Context, eventIn Event) {
		resp := &pb.Event{
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
			err := controller.RegisterControlCallback(server.Context(), Control(ev.Control), triggers, ctrlFunc, req.Extra.AsMap())
			if err != nil {
				return err
			}
		}

		var cancelledTriggers []EventType
		for _, v := range ev.CancelledEvents {
			cancelledTriggers = append(cancelledTriggers, EventType(v))
		}
		if len(cancelledTriggers) > 0 {
			err := controller.RegisterControlCallback(server.Context(), Control(ev.Control), cancelledTriggers, nil, req.Extra.AsMap())
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
			err := server.Send(&pb.StreamEventsResponse{Event: msg})
			if err != nil {
				return err
			}
		}
	}
}

// DoCommand receives arbitrary commands.
func (s *serviceServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	controller, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, controller, req)
}
