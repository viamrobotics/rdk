// Package gantry contains a gRPC based gantry client.
package gantry

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/grpc"
	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/referenceframe"
)

// serviceClient is a client satisfies the gantry.proto contract.
type serviceClient struct {
	conn   rpc.ClientConn
	client pb.GantryServiceClient
	logger golog.Logger
}

// newServiceClient constructs a new serviceClient that is served at the given address.
func newServiceClient(ctx context.Context, address string, logger golog.Logger, opts ...rpc.DialOption) (*serviceClient, error) {
	conn, err := grpc.Dial(ctx, address, logger, opts...)
	if err != nil {
		return nil, err
	}
	sc := newSvcClientFromConn(conn, logger)
	return sc, nil
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

// Close cleanly closes the underlying connections.
func (sc *serviceClient) Close() error {
	return sc.conn.Close()
}

// client is an gantry client.
type client struct {
	*serviceClient
	name string
}

// NewClient constructs a new client that is served at the given address.
func NewClient(ctx context.Context, name string, address string, logger golog.Logger, opts ...rpc.DialOption) (Gantry, error) {
	sc, err := newServiceClient(ctx, address, logger, opts...)
	if err != nil {
		return nil, err
	}
	return clientFromSvcClient(sc, name), nil
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Gantry {
	sc := newSvcClientFromConn(conn, logger)
	return clientFromSvcClient(sc, name)
}

func clientFromSvcClient(sc *serviceClient, name string) Gantry {
	return &client{sc, name}
}

func (c *client) CurrentPosition(ctx context.Context) ([]float64, error) {
	resp, err := c.client.CurrentPosition(ctx, &pb.GantryServiceCurrentPositionRequest{
		Name: c.name,
	})
	if err != nil {
		return nil, err
	}
	return resp.Positions, nil
}

func (c *client) Lengths(ctx context.Context) ([]float64, error) {
	lengths, err := c.client.Lengths(ctx, &pb.GantryServiceLengthsRequest{
		Name: c.name,
	})
	if err != nil {
		return nil, err
	}
	return lengths.Lengths, nil
}

func (c *client) MoveToPosition(ctx context.Context, positions []float64) error {
	_, err := c.client.MoveToPosition(ctx, &pb.GantryServiceMoveToPositionRequest{
		Name:      c.name,
		Positions: positions,
	})
	return err
}

func (c *client) ModelFrame() referenceframe.Model {
	// TODO(erh): this feels wrong
	return nil
}

func (c *client) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	res, err := c.CurrentPosition(ctx)
	if err != nil {
		return nil, err
	}
	return referenceframe.FloatsToInputs(res), nil
}

func (c *client) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	return c.MoveToPosition(ctx, referenceframe.InputsToFloats(goal))
}

// Close cleanly closes the underlying connections.
func (c *client) Close() error {
	return c.serviceClient.Close()
}
