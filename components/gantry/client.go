// Package gantry contains a gRPC based gantry client.
package gantry

import (
	"context"

	"github.com/edaniels/golog"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/gantry/v1"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
)

// client implements GantryServiceClient.
type client struct {
	name   string
	conn   rpc.ClientConn
	client pb.GantryServiceClient
	logger golog.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Gantry {
	c := pb.NewGantryServiceClient(conn)
	return &client{
		name:   name,
		conn:   conn,
		client: c,
		logger: logger,
	}
}

func (c *client) Position(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetPosition(ctx, &pb.GetPositionRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return nil, err
	}
	return resp.PositionsMm, nil
}

func (c *client) Lengths(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	lengths, err := c.client.GetLengths(ctx, &pb.GetLengthsRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return nil, err
	}
	return lengths.LengthsMm, nil
}

func (c *client) MoveToPosition(
	ctx context.Context,
	positionsMm []float64,
	worldState *commonpb.WorldState,
	extra map[string]interface{},
) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.MoveToPosition(ctx, &pb.MoveToPositionRequest{
		Name:        c.name,
		PositionsMm: positionsMm,
		WorldState:  worldState,
		Extra:       ext,
	})
	return err
}

func (c *client) Stop(ctx context.Context, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.Stop(ctx, &pb.StopRequest{Name: c.name, Extra: ext})
	return err
}

func (c *client) ModelFrame() referenceframe.Model {
	// TODO(erh): this feels wrong
	return nil
}

func (c *client) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	res, err := c.Position(ctx, nil)
	if err != nil {
		return nil, err
	}
	return referenceframe.FloatsToInputs(res), nil
}

func (c *client) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	return c.MoveToPosition(ctx, referenceframe.InputsToFloats(goal), &commonpb.WorldState{}, nil)
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return generic.DoFromConnection(ctx, c.conn, c.name, cmd)
}
