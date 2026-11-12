package statemachine

import (
	"context"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/statemachine/v1"
	taskpb "go.viam.com/api/service/task/v1"
	"go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/logging"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/task"
)

type serviceServer struct {
	pb.UnimplementedStateMachineServiceServer
	coll resource.APIResourceGetter[Service]
}

// NewRPCServiceServer constructs a state machine gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.APIResourceGetter[Service], logger logging.Logger) interface{} {
	return &serviceServer{coll: coll}
}

// GetState returns the machine's active state.
func (server *serviceServer) GetState(ctx context.Context, req *pb.GetStateRequest) (*pb.GetStateResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	st, err := svc.GetState(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	contextPb, err := protoutils.StructToStructPb(st.Context)
	if err != nil {
		return nil, err
	}
	resp := &pb.GetStateResponse{
		Path:         st.Path,
		Status:       StatusToProto(st.Status),
		Context:      contextPb,
		TaskProgress: task.ProgressToProto(st.TaskProgress),
	}
	if !st.EnteredAt.IsZero() {
		resp.EnteredAt = timestamppb.New(st.EnteredAt)
	}
	return resp, nil
}

// SendEvent delivers an event to the machine.
func (server *serviceServer) SendEvent(ctx context.Context, req *pb.SendEventRequest) (*pb.SendEventResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	res, err := svc.SendEvent(ctx, req.Event, req.Payload.AsMap(), req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.SendEventResponse{Handled: res.Handled, Path: res.Path}, nil
}

// GetHistory returns recent transitions.
func (server *serviceServer) GetHistory(ctx context.Context, req *pb.GetHistoryRequest) (*pb.GetHistoryResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	transitions, err := svc.GetHistory(ctx, int(req.Limit), req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	resp := &pb.GetHistoryResponse{Transitions: make([]*pb.Transition, 0, len(transitions))}
	for _, tr := range transitions {
		trPb := &pb.Transition{From: tr.From, To: tr.To, Trigger: tr.Trigger, Timestamp: timestamppb.New(tr.Timestamp)}
		if tr.TaskResult != nil {
			output, err := protoutils.StructToStructPb(tr.TaskResult.Output)
			if err != nil {
				return nil, err
			}
			trPb.TaskResult = &taskpb.Result{Outcome: tr.TaskResult.Outcome, Output: output}
		}
		resp.Transitions = append(resp.Transitions, trPb)
	}
	return resp, nil
}

// GetDefinition returns the configured graph.
func (server *serviceServer) GetDefinition(ctx context.Context, req *pb.GetDefinitionRequest) (*pb.GetDefinitionResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	def, err := svc.GetDefinition(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	defPb, err := protoutils.StructToStructPb(def)
	if err != nil {
		return nil, err
	}
	return &pb.GetDefinitionResponse{Definition: defPb}, nil
}

// Stop holds the machine.
func (server *serviceServer) Stop(ctx context.Context, req *pb.StopRequest) (*pb.StopResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	if err := svc.Stop(ctx, req.Extra.AsMap()); err != nil {
		return nil, err
	}
	return &pb.StopResponse{}, nil
}

// Reset re-enters the initial state.
func (server *serviceServer) Reset(ctx context.Context, req *pb.ResetRequest) (*pb.ResetResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	if err := svc.Reset(ctx, req.Extra.AsMap()); err != nil {
		return nil, err
	}
	return &pb.ResetResponse{}, nil
}

// DoCommand receives arbitrary commands.
func (server *serviceServer) DoCommand(ctx context.Context, req *commonpb.DoCommandRequest) (*commonpb.DoCommandResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	return rprotoutils.DoFromResourceServer(ctx, svc, req)
}

// GetStatus returns the status of the state machine resource.
func (server *serviceServer) GetStatus(ctx context.Context, req *commonpb.GetStatusRequest) (*commonpb.GetStatusResponse, error) {
	svc, err := server.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	return rprotoutils.GetStatusFromResourceServer(ctx, svc, req)
}
