// Package arm contains a gRPC based arm client.
package arm

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/edaniels/golog"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/component/generic"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/arm/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/utils"
)

var errArmClientInputsNotSupport = errors.New("arm client does not support inputs directly")

// serviceClient is a client satisfies the arm.proto contract.
type serviceClient struct {
	conn   rpc.ClientConn
	client pb.ArmServiceClient
	logger golog.Logger
}

// newSvcClientFromConn constructs a new serviceClient using the passed in connection.
func newSvcClientFromConn(conn rpc.ClientConn, logger golog.Logger) *serviceClient {
	client := pb.NewArmServiceClient(conn)
	sc := &serviceClient{
		conn:   conn,
		client: client,
		logger: logger,
	}
	return sc
}

// client is an arm client.
type client struct {
	*serviceClient
	name string
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Arm {
	sc := newSvcClientFromConn(conn, logger)
	arm := clientFromSvcClient(sc, name)
	return &reconfigurableClient{actual: arm}
}

func clientFromSvcClient(sc *serviceClient, name string) Arm {
	return &client{sc, name}
}

type reconfigurableClient struct {
	mu     sync.RWMutex
	actual Arm
}

func (c *reconfigurableClient) Reconfigure(ctx context.Context, newClient resource.Reconfigurable) error {
	fmt.Println("call reconfigure in arm")
	client, ok := newClient.(*reconfigurableClient)
	if !ok {
		return utils.NewUnexpectedTypeError(c, newClient)
	}
	if err := viamutils.TryClose(ctx, c.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	c.actual = client.actual
	return nil
}

func (c *reconfigurableClient) GetEndPosition(ctx context.Context, extra map[string]interface{}) (*commonpb.Pose, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual.GetEndPosition(ctx, extra)
}

func (c *reconfigurableClient) MoveToPosition(
	ctx context.Context,
	pose *commonpb.Pose,
	worldState *commonpb.WorldState,
	extra map[string]interface{},
) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual.MoveToPosition(ctx, pose, worldState, extra)
}

func (c *reconfigurableClient) MoveToJointPositions(ctx context.Context, positions *pb.JointPositions, extra map[string]interface{}) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual.MoveToJointPositions(ctx, positions, extra)
}

func (c *reconfigurableClient) GetJointPositions(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual.GetJointPositions(ctx, extra)
}

func (c *reconfigurableClient) Stop(ctx context.Context, extra map[string]interface{}) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual.Stop(ctx, extra)
}

func (c *reconfigurableClient) ModelFrame() referenceframe.Model {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual.ModelFrame()
}

func (c *reconfigurableClient) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual.CurrentInputs(ctx)
}

func (c *reconfigurableClient) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual.GoToInputs(ctx, goal)
}

func (c *reconfigurableClient) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual.Do(ctx, cmd)
}

func (c *client) GetEndPosition(ctx context.Context, extra map[string]interface{}) (*commonpb.Pose, error) {
	ext, err := structpb.NewStruct(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetEndPosition(ctx, &pb.GetEndPositionRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		fmt.Println("erroring in GetEndPosition")
		return nil, err
	}
	return resp.Pose, nil
}

func (c *client) MoveToPosition(
	ctx context.Context,
	pose *commonpb.Pose,
	worldState *commonpb.WorldState,
	extra map[string]interface{},
) error {
	ext, err := structpb.NewStruct(extra)
	if err != nil {
		return err
	}
	_, err = c.client.MoveToPosition(ctx, &pb.MoveToPositionRequest{
		Name:       c.name,
		To:         pose,
		WorldState: worldState,
		Extra:      ext,
	})
	return err
}

func (c *client) MoveToJointPositions(ctx context.Context, positions *pb.JointPositions, extra map[string]interface{}) error {
	ext, err := structpb.NewStruct(extra)
	if err != nil {
		return err
	}
	_, err = c.client.MoveToJointPositions(ctx, &pb.MoveToJointPositionsRequest{
		Name:      c.name,
		Positions: positions,
		Extra:     ext,
	})
	return err
}

func (c *client) GetJointPositions(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
	ext, err := structpb.NewStruct(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetJointPositions(ctx, &pb.GetJointPositionsRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return nil, err
	}
	return resp.Positions, nil
}

func (c *client) Stop(ctx context.Context, extra map[string]interface{}) error {
	ext, err := structpb.NewStruct(extra)
	if err != nil {
		return err
	}
	_, err = c.client.Stop(ctx, &pb.StopRequest{
		Name:  c.name,
		Extra: ext,
	})
	return err
}

func (c *client) ModelFrame() referenceframe.Model {
	// TODO(erh): this feels wrong
	return nil
}

func (c *client) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	return nil, errArmClientInputsNotSupport
}

func (c *client) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	return errArmClientInputsNotSupport
}

func (c *client) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return generic.DoFromConnection(ctx, c.conn, c.name, cmd)
}
