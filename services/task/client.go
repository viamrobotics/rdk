package task

import (
	"context"
	"errors"

	pb "go.viam.com/api/service/task/v1"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/logging"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// client is a task service client that talks to a gRPC task service.
type client struct {
	resource.Named
	resource.TriviallyCloseable
	name   string
	client pb.TaskServiceClient
	logger logging.Logger
}

// NewClientFromConn creates a new task service client from a gRPC connection.
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
		client: pb.NewTaskServiceClient(conn),
		logger: logger,
	}, nil
}

func (c *client) Run(ctx context.Context, input, extra map[string]interface{}) (Result, error) {
	inputPb, err := protoutils.StructToStructPb(input)
	if err != nil {
		return Result{}, err
	}
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return Result{}, err
	}
	resp, err := c.client.Run(ctx, &pb.RunRequest{Name: c.name, Input: inputPb, Extra: ext})
	if err != nil {
		if status.Code(err) == codes.FailedPrecondition {
			return Result{}, errors.Join(ErrAlreadyRunning, err)
		}
		return Result{}, err
	}
	if resp.Result == nil {
		return Result{}, errors.New("task returned no result")
	}
	return *ResultFromProto(resp.Result), nil
}

func (c *client) GetStatus(ctx context.Context, extra map[string]interface{}) (Status, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return Status{}, err
	}
	resp, err := c.client.GetStatus(ctx, &pb.GetStatusRequest{Name: c.name, Extra: ext})
	if err != nil {
		return Status{}, err
	}
	st := Status{LastResult: ResultFromProto(resp.LastResult), Progress: ProgressFromProto(resp.Progress)}
	if resp.Phase == pb.Phase_PHASE_RUNNING {
		st.Phase = PhaseRunning
	}
	if resp.StartedAt != nil {
		st.StartedAt = resp.StartedAt.AsTime()
	}
	return st, nil
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return rprotoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}
