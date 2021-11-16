// Package arm contains a gRPC based arm client.
package arm

import (
	"context"
	"errors"
	"io"

	rpcclient "go.viam.com/utils/rpc/client"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc/dialer"

	"go.viam.com/core/grpc"
	commonpb "go.viam.com/core/proto/api/common/v1"
	pb "go.viam.com/core/proto/api/component/v1"
	"go.viam.com/core/referenceframe"
)

// subtypeClient is a client satisfies the arm_subtype.proto contract.
type subtypeClient struct {
	address string
	conn    dialer.ClientConn
	client  pb.ArmServiceClient
	logger  golog.Logger
}

// NewSubtypeClient constructs a new subtypeClient that is served at the given address.
func NewSubtypeClient(ctx context.Context, address string, opts rpcclient.DialOptions, logger golog.Logger) (io.Closer, error) {
	conn, err := grpc.Dial(ctx, address, opts, logger)
	if err != nil {
		return nil, err
	}

	client := pb.NewArmServiceClient(conn)
	sc := &subtypeClient{
		address: address,
		conn:    conn,
		client:  client,
		logger:  logger,
	}
	return sc, nil
}

// Close cleanly closes the underlying connections
func (sc *subtypeClient) Close() error {
	return sc.conn.Close()
}

// client is an arm client
type client struct {
	*subtypeClient
	name string
}

// NewClient constructs a new client that is served at the given address.
func NewClient(ctx context.Context, name string, address string, opts rpcclient.DialOptions, logger golog.Logger) (Arm, error) {
	sc, err := NewSubtypeClient(ctx, address, opts, logger)
	if err != nil {
		return nil, err
	}

	c, err := NewClientFromSubtypeClient(sc, name)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// NewClientFromSubtypeClient constructs a new Client from subtype client.
func NewClientFromSubtypeClient(sc interface{}, name string) (Arm, error) {
	newSc, ok := sc.(*subtypeClient)
	if !ok {
		return nil, errors.New("not an arm subtype client")
	}
	c := &client{newSc, name}
	return c, nil
}

func (c *client) CurrentPosition(ctx context.Context) (*commonpb.Pose, error) {
	resp, err := c.client.CurrentPosition(ctx, &pb.ArmServiceCurrentPositionRequest{
		Name: c.name,
	})
	if err != nil {
		return nil, err
	}
	return resp.Position, nil
}

func (c *client) MoveToPosition(ctx context.Context, pos *commonpb.Pose) error {
	_, err := c.client.MoveToPosition(ctx, &pb.ArmServiceMoveToPositionRequest{
		Name: c.name,
		To:   pos,
	})
	return err
}

func (c *client) MoveToJointPositions(ctx context.Context, pos *pb.ArmJointPositions) error {
	_, err := c.client.MoveToJointPositions(ctx, &pb.ArmServiceMoveToJointPositionsRequest{
		Name: c.name,
		To:   pos,
	})
	return err
}

func (c *client) CurrentJointPositions(ctx context.Context) (*pb.ArmJointPositions, error) {
	resp, err := c.client.CurrentJointPositions(ctx, &pb.ArmServiceCurrentJointPositionsRequest{
		Name: c.name,
	})
	if err != nil {
		return nil, err
	}
	return resp.Positions, nil
}

func (c *client) JointMoveDelta(ctx context.Context, joint int, amountDegs float64) error {
	_, err := c.client.JointMoveDelta(ctx, &pb.ArmServiceJointMoveDeltaRequest{
		Name:       c.name,
		Joint:      int32(joint),
		AmountDegs: amountDegs,
	})
	return err
}

func (c *client) ModelFrame() *referenceframe.Model {
	// TODO(erh): this feels wrong
	return nil
}

// Close cleanly closes the underlying connections
func (c *client) Close() error {
	return c.conn.Close()
}
