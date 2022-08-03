// Package gantry contains a gRPC based gantry client.
package gantry

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/component/generic"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/gantry/v1"
	"go.viam.com/rdk/referenceframe"
)

// serviceClient is a client satisfies the gantry.proto contract.
type serviceClient struct {
	conn   rpc.ClientConn
	client pb.GantryServiceClient
	logger golog.Logger
}

// newSvcClientFromConn constructs a new serviceClient using the passed in connection.
func newSvcClientFromConn(conn rpc.ClientConn, logger golog.Logger) *serviceClient {
	client := pb.NewGantryServiceClient(conn)
	sc := &serviceClient{
		conn:   conn,
		client: client,
		logger: logger,
	}
	return sc
}

// client is an gantry client.
type client struct {
	*serviceClient
	name string
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Gantry {
	sc := newSvcClientFromConn(conn, logger)
	return clientFromSvcClient(sc, name)
}

func clientFromSvcClient(sc *serviceClient, name string) Gantry {
	return &client{sc, name}
}

func (c *client) GetPosition(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
	ext, err := structpb.NewStruct(extra)
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

func (c *client) GetLengths(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
	ext, err := structpb.NewStruct(extra)
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
	ext, err := structpb.NewStruct(extra)
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
	ext, err := structpb.NewStruct(extra)
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
	res, err := c.GetPosition(ctx, nil)
	if err != nil {
		return nil, err
	}
	return referenceframe.FloatsToInputs(res), nil
}

func (c *client) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	return c.MoveToPosition(ctx, referenceframe.InputsToFloats(goal), &commonpb.WorldState{}, nil)
}

func (c *client) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return generic.DoFromConnection(ctx, c.conn, c.name, cmd)
}
