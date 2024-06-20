// Package gantry contains a gRPC based gantry client.
package gantry

import (
	"context"

	pb "go.viam.com/api/component/gantry/v1"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/logging"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
)

// client implements GantryServiceClient.
type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	name   string
	client pb.GantryServiceClient
	logger logging.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger logging.Logger,
) (Gantry, error) {
	c := pb.NewGantryServiceClient(conn)
	return &client{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		name:   name.ShortName(),
		client: c,
		logger: logger,
	}, nil
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

func (c *client) Home(ctx context.Context, extra map[string]interface{}) (bool, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return false, err
	}
	homed, err := c.client.Home(ctx, &pb.HomeRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return false, err
	}
	return homed.Homed, nil
}

func (c *client) MoveToPosition(ctx context.Context, positionsMm, speedsMmPerSec []float64, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}

	if speedsMmPerSec == nil {
		c.logger.Warnf("%s MoveToPosition: speedMmPerSec is nil", c.name)
	}

	if positionsMm == nil {
		c.logger.Warnf("%s MoveToPosition: positionsMm is nil", c.name)
	}
	_, err = c.client.MoveToPosition(ctx, &pb.MoveToPositionRequest{
		Name:           c.name,
		PositionsMm:    positionsMm,
		SpeedsMmPerSec: speedsMmPerSec,
		Extra:          ext,
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

func (c *client) GoToInputs(ctx context.Context, inputSteps ...[]referenceframe.Input) error {
	for _, goal := range inputSteps {
		speeds := []float64{}
		err := c.MoveToPosition(ctx, referenceframe.InputsToFloats(goal), speeds, nil)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return rprotoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}

func (c *client) IsMoving(ctx context.Context) (bool, error) {
	resp, err := c.client.IsMoving(ctx, &pb.IsMovingRequest{Name: c.name})
	if err != nil {
		return false, err
	}
	return resp.IsMoving, nil
}
