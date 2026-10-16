package statemachine

import (
	"context"

	pb "go.viam.com/api/service/statemachine/v1"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/logging"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/task"
)

// client is a state machine service client that talks to a gRPC state machine service.
type client struct {
	resource.Named
	resource.TriviallyCloseable
	name   string
	client pb.StateMachineServiceClient
	logger logging.Logger
}

// NewClientFromConn creates a new state machine service client from a gRPC connection.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger logging.Logger,
) (Service, error) {
	return &client{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		name:   name.Name,
		client: pb.NewStateMachineServiceClient(conn),
		logger: logger,
	}, nil
}

func (c *client) GetState(ctx context.Context, extra map[string]interface{}) (State, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return State{}, err
	}
	resp, err := c.client.GetState(ctx, &pb.GetStateRequest{Name: c.name, Extra: ext})
	if err != nil {
		return State{}, err
	}
	st := State{
		Path:         resp.Path,
		Status:       StatusFromProto(resp.Status),
		Context:      resp.Context.AsMap(),
		TaskProgress: task.ProgressFromProto(resp.TaskProgress),
	}
	if resp.EnteredAt != nil {
		st.EnteredAt = resp.EnteredAt.AsTime()
	}
	return st, nil
}

func (c *client) SendEvent(ctx context.Context, event string, payload, extra map[string]interface{}) (EventResult, error) {
	payloadPb, err := protoutils.StructToStructPb(payload)
	if err != nil {
		return EventResult{}, err
	}
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return EventResult{}, err
	}
	resp, err := c.client.SendEvent(ctx, &pb.SendEventRequest{Name: c.name, Event: event, Payload: payloadPb, Extra: ext})
	if err != nil {
		return EventResult{}, err
	}
	return EventResult{Handled: resp.Handled, Path: resp.Path}, nil
}

func (c *client) GetHistory(ctx context.Context, limit int, extra map[string]interface{}) ([]Transition, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetHistory(ctx, &pb.GetHistoryRequest{Name: c.name, Limit: int32(limit), Extra: ext})
	if err != nil {
		return nil, err
	}
	transitions := make([]Transition, 0, len(resp.Transitions))
	for _, tr := range resp.Transitions {
		t := Transition{From: tr.From, To: tr.To, Trigger: tr.Trigger, TaskResult: task.ResultFromProto(tr.TaskResult)}
		if tr.Timestamp != nil {
			t.Timestamp = tr.Timestamp.AsTime()
		}
		transitions = append(transitions, t)
	}
	return transitions, nil
}

func (c *client) GetDefinition(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetDefinition(ctx, &pb.GetDefinitionRequest{Name: c.name, Extra: ext})
	if err != nil {
		return nil, err
	}
	return resp.Definition.AsMap(), nil
}

func (c *client) Stop(ctx context.Context, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.Stop(ctx, &pb.StopRequest{Name: c.name, Extra: ext})
	return err
}

func (c *client) Reset(ctx context.Context, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.Reset(ctx, &pb.ResetRequest{Name: c.name, Extra: ext})
	return err
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return rprotoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}

func (c *client) Status(ctx context.Context) (map[string]interface{}, error) {
	return rprotoutils.GetStatusFromResourceClient(ctx, c.client, c.name)
}
