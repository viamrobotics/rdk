// Package arm contains a gRPC based arm client.
package arm

import (
	"context"
	"errors"

	rpcclient "go.viam.com/utils/rpc/client"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc/dialer"

	"go.viam.com/core/grpc"
	"go.viam.com/core/kinematics"
	commonpb "go.viam.com/core/proto/api/common/v1"
	pb "go.viam.com/core/proto/api/component/v1"
)

// SubtypeClient is a client satisfies the arm_subtype.proto contract.
type SubtypeClient struct {
	conn   dialer.ClientConn
	client pb.ArmSubtypeServiceClient

	logger golog.Logger
}

// NewSubtypeClient constructs a new SubtypeClient that is served at the given address.
func NewSubtypeClient(ctx context.Context, address string, opts rpcclient.DialOptions, logger golog.Logger) (*SubtypeClient, error) {
	conn, err := grpc.Dial(ctx, address, opts, logger)
	if err != nil {
		return nil, err
	}

	client := pb.NewArmSubtypeServiceClient(conn)
	ac := &SubtypeClient{
		conn:   conn,
		client: client,
		logger: logger,
	}
	return ac, nil
}

// Close cleanly closes the underlying connections
func (ac *SubtypeClient) Close() error {
	return ac.conn.Close()
}

// client is an arm client
type client struct {
	sc   *SubtypeClient
	name string
}

// NewClient constructs a new client that is served at the given address.
func NewClient(ctx context.Context, name string, address string, opts rpcclient.DialOptions, logger golog.Logger) (Arm, error) {
	sc, err := NewSubtypeClient(ctx, address, opts, logger)
	if err != nil {
		return nil, err
	}

	ac, err := NewClientFromSubtypeClient(sc, name)
	if err != nil {
		return nil, err
	}
	return ac, nil
}

// NewClientFromSubtypeClient constructs a new Client from subtype client.
func NewClientFromSubtypeClient(sc interface{}, name string) (Arm, error) {
	newSc, ok := sc.(*SubtypeClient)
	if !ok {
		return nil, errors.New("not an arm subtype client")
	}
	ac := &client{newSc, name}
	return ac, nil
}

func (ac *client) CurrentPosition(ctx context.Context) (*commonpb.Pose, error) {
	resp, err := ac.sc.client.CurrentPosition(ctx, &pb.ArmSubtypeServiceCurrentPositionRequest{
		Name: ac.name,
	})
	if err != nil {
		return nil, err
	}
	return resp.Position, nil
}

func (ac *client) MoveToPosition(ctx context.Context, c *commonpb.Pose) error {
	_, err := ac.sc.client.MoveToPosition(ctx, &pb.ArmSubtypeServiceMoveToPositionRequest{
		Name: ac.name,
		To:   c,
	})
	return err
}

func (ac *client) MoveToJointPositions(ctx context.Context, pos *pb.ArmJointPositions) error {
	_, err := ac.sc.client.MoveToJointPositions(ctx, &pb.ArmSubtypeServiceMoveToJointPositionsRequest{
		Name: ac.name,
		To:   pos,
	})
	return err
}

func (ac *client) CurrentJointPositions(ctx context.Context) (*pb.ArmJointPositions, error) {
	resp, err := ac.sc.client.CurrentJointPositions(ctx, &pb.ArmSubtypeServiceCurrentJointPositionsRequest{
		Name: ac.name,
	})
	if err != nil {
		return nil, err
	}
	return resp.Positions, nil
}

func (ac *client) JointMoveDelta(ctx context.Context, joint int, amountDegs float64) error {
	_, err := ac.sc.client.JointMoveDelta(ctx, &pb.ArmSubtypeServiceJointMoveDeltaRequest{
		Name:       ac.name,
		Joint:      int32(joint),
		AmountDegs: amountDegs,
	})
	return err
}

func (ac *client) ModelFrame() *kinematics.Model {
	// TODO(erh): this feels wrong
	return nil
}
