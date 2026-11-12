package task

import (
	"context"
	"errors"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/task/v1"
	"go.viam.com/utils/protoutils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/logging"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

type serviceServer struct {
	pb.UnimplementedTaskServiceServer
	coll resource.APIResourceGetter[Service]
}

// NewRPCServiceServer constructs a task gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.APIResourceGetter[Service], logger logging.Logger) interface{} {
	return &serviceServer{coll: coll}
}

// Run executes the task and returns its result.
func (server *serviceServer) Run(ctx context.Context, req *pb.RunRequest) (*pb.RunResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	res, err := svc.Run(ctx, req.Input.AsMap(), req.Extra.AsMap())
	if err != nil {
		if errors.Is(err, ErrAlreadyRunning) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		return nil, err
	}
	output, err := protoutils.StructToStructPb(res.Output)
	if err != nil {
		return nil, err
	}
	return &pb.RunResponse{Result: &pb.Result{Outcome: res.Outcome, Output: output}}, nil
}

// GetStatus reports whether the task is running and its last result.
func (server *serviceServer) GetStatus(ctx context.Context, req *pb.GetStatusRequest) (*pb.GetStatusResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	st, err := svc.GetStatus(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	resp := &pb.GetStatusResponse{Phase: pb.Phase_PHASE_IDLE, Progress: ProgressToProto(st.Progress)}
	if st.Phase == PhaseRunning {
		resp.Phase = pb.Phase_PHASE_RUNNING
	}
	if !st.StartedAt.IsZero() {
		resp.StartedAt = timestamppb.New(st.StartedAt)
	}
	if st.LastResult != nil {
		output, err := protoutils.StructToStructPb(st.LastResult.Output)
		if err != nil {
			return nil, err
		}
		resp.LastResult = &pb.Result{Outcome: st.LastResult.Outcome, Output: output}
	}
	return resp, nil
}

// DoCommand receives arbitrary commands.
func (server *serviceServer) DoCommand(ctx context.Context, req *commonpb.DoCommandRequest) (*commonpb.DoCommandResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	return rprotoutils.DoFromResourceServer(ctx, svc, req)
}
