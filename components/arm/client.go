// Package arm contains a gRPC based arm client.
package arm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/arm/v1"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/logging"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// client implements ArmServiceClient.
type client struct {
	resource.Named
	resource.TriviallyCloseable
	name   string
	client pb.ArmServiceClient
	logger logging.Logger

	mu    sync.Mutex
	model referenceframe.Model
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger logging.Logger,
) (Arm, error) {
	pbClient := pb.NewArmServiceClient(conn)
	return &client{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		name:   name.Name,
		client: pbClient,
		logger: logger,
	}, nil
}

// Reconfigure invalidates the cached `model` value. It's expected to be invoked when this `client`
// represents an rdk <-> modular arm connection and the arm rebuilds.
func (c *client) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	c.mu.Lock()
	c.model = nil
	c.mu.Unlock()

	return nil
}

func (c *client) EndPosition(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetEndPosition(ctx, &pb.GetEndPositionRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return nil, err
	}
	return spatialmath.NewPoseFromProtobuf(resp.Pose), nil
}

func (c *client) MoveToPosition(ctx context.Context, pose spatialmath.Pose, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	if pose == nil {
		c.logger.Warnf("%s MoveToPosition: pose parameter is nil", c.name)
	}
	_, err = c.client.MoveToPosition(ctx, &pb.MoveToPositionRequest{
		Name:  c.name,
		To:    spatialmath.PoseToProtobuf(pose),
		Extra: ext,
	})
	return err
}

func (c *client) MoveToJointPositions(ctx context.Context, positions []referenceframe.Input, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	m, err := c.Kinematics(ctx)
	if err != nil {
		warnKinematicsUnsafe(ctx, c.logger, err)
	} else if err := CheckDesiredJointPositions(ctx, c, positions); err != nil {
		return err
	}

	jp, err := referenceframe.JointPositionsFromInputs(m, positions)
	if err != nil {
		return err
	}
	_, err = c.client.MoveToJointPositions(ctx, &pb.MoveToJointPositionsRequest{
		Name:      c.name,
		Positions: jp,
		Extra:     ext,
	})
	return err
}

func (c *client) MoveThroughJointPositions(
	ctx context.Context,
	positions [][]referenceframe.Input,
	options *MoveOptions,
	extra map[string]interface{},
) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	if positions == nil {
		c.logger.Warnf("%s MoveThroughJointPositions: position argument is nil", c.name)
	}
	allJPs := make([]*pb.JointPositions, 0, len(positions))

	var limits []referenceframe.Limit
	var prevPosition []referenceframe.Input
	m, err := c.Kinematics(ctx)
	if err != nil {
		warnKinematicsUnsafe(ctx, c.logger, err)
	} else {
		limits = m.DoF()
		prevPosition, err = c.JointPositions(ctx, nil)
		if err != nil {
			return fmt.Errorf("cannot get JointPositions: %w", err)
		}
	}

	for _, position := range positions {
		if len(limits) > 0 {
			if err := checkDesiredJointPositions(limits, prevPosition, position); err != nil {
				return err
			}
			prevPosition = position
		}
		jp, err := referenceframe.JointPositionsFromInputs(m, position)
		if err != nil {
			return err
		}
		allJPs = append(allJPs, jp)
	}
	req := &pb.MoveThroughJointPositionsRequest{
		Name:      c.name,
		Positions: allJPs,
		Extra:     ext,
	}
	if options != nil {
		req.Options = options.toProtobuf()
	}
	_, err = c.client.MoveThroughJointPositions(ctx, req)
	return err
}

func (c *client) MoveThroughJointPositionsStreamed(
	ctx context.Context,
	batches <-chan []TrajectoryPoint,
	responses chan<- Response,
	extra map[string]interface{},
) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}

	// Kinematics is used only to encode JointPositions on the wire. Tolerate failure here so we
	// stay usable on arms whose kinematics haven't been registered yet (matching the unary path).
	model, err := c.Kinematics(ctx)
	if err != nil {
		warnKinematicsUnsafe(ctx, c.logger, err)
	}

	// We open the stream under a context we can cancel, so one cancel() both tears the gRPC stream
	// down and tells the send goroutine to quit. We lean on that when the recv loop finishes:
	// without it, the send goroutine could sit forever waiting on a caller who never closes batches.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stream, err := c.client.MoveThroughJointPositionsStreamed(ctx)
	if err != nil {
		return err
	}

	if err := stream.Send(&pb.MoveThroughJointPositionsStreamedRequest{
		Name: c.name,
		Message: &pb.MoveThroughJointPositionsStreamedRequest_Init_{
			Init: &pb.MoveThroughJointPositionsStreamedRequest_Init{
				Extra: ext,
			},
		},
	}); err != nil {
		return err
	}

	// Feed the caller's batches onto the wire, one TrajectoryBatch per slice they hand us. That is
	// how the caller sets the pace on the wire: they choose how much to put in each slice. When they
	// close the channel we CloseSend, which lets the server's recv loop finish.
	var sendErr error
	var sendOnce sync.Once
	setSendErr := func(e error) { sendOnce.Do(func() { sendErr = e }) }
	sendDone := make(chan struct{})
	goutils.PanicCapturingGo(func() {
		defer close(sendDone)
		for {
			select {
			case <-ctx.Done():
				setSendErr(ctx.Err())
				return
			case batch, ok := <-batches:
				if !ok {
					if err := stream.CloseSend(); err != nil {
						setSendErr(err)
					}
					return
				}
				pbPoints := make([]*pb.TrajectoryPoint, 0, len(batch))
				for _, p := range batch {
					pp, err := trajectoryPointToProto(model, p)
					if err != nil {
						setSendErr(err)
						return
					}
					pbPoints = append(pbPoints, pp)
				}
				if err := stream.Send(&pb.MoveThroughJointPositionsStreamedRequest{
					Message: &pb.MoveThroughJointPositionsStreamedRequest_Batch{
						Batch: &pb.MoveThroughJointPositionsStreamedRequest_TrajectoryBatch{
							Points: pbPoints,
						},
					},
				}); err != nil {
					setSendErr(err)
					return
				}
			}
		}
	})

	// Back on the calling goroutine, read responses off the wire and hand them to the caller's
	// channel. We do not close that channel: to the caller we are just another Arm impl, and by the
	// same ownership rule the impl follows, the caller closes responses once we have returned.
	var recvErr error
recvLoop:
	for {
		resp, err := stream.Recv()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				recvErr = err
			}
			break
		}
		_ = resp // Response carries no fields yet; future per-batch status will be read from resp here.
		select {
		case responses <- Response{}:
		case <-ctx.Done():
			recvErr = ctx.Err()
			break recvLoop
		}
	}
	// Tear the stream down and wake the send goroutine, which may still be parked waiting on batches.
	cancel()
	<-sendDone

	if recvErr != nil {
		return recvErr
	}
	return sendErr
}

func (c *client) JointPositions(ctx context.Context, extra map[string]interface{}) ([]referenceframe.Input, error) {
	ext, err := protoutils.StructToStructPb(extra)
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
	m, err := c.Kinematics(ctx)
	if err != nil {
		warnKinematicsUnsafe(ctx, c.logger, err)
	}
	return referenceframe.InputsFromJointPositions(m, resp.Positions)
}

func (c *client) Stop(ctx context.Context, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.Stop(ctx, &pb.StopRequest{
		Name:  c.name,
		Extra: ext,
	})
	return err
}

func (c *client) Kinematics(ctx context.Context) (referenceframe.Model, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// for performance we cache the model after building it once, and can quickly return if its already been created.
	if c.model == nil {
		resp, err := c.client.GetKinematics(ctx, &commonpb.GetKinematicsRequest{Name: c.name})
		if err != nil {
			return nil, err
		}
		model, err := referenceframe.KinematicModelFromProtobuf(c.name, resp)
		if err != nil {
			return nil, err
		}
		c.model = model
	}
	return c.model, nil
}

func (c *client) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	return c.JointPositions(ctx, nil)
}

func (c *client) GoToInputs(ctx context.Context, inputSteps ...[]referenceframe.Input) error {
	return c.MoveThroughJointPositions(ctx, inputSteps, nil, nil)
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return rprotoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}

func (c *client) Status(ctx context.Context) (map[string]interface{}, error) {
	return rprotoutils.GetStatusFromResourceClient(ctx, c.client, c.name)
}

func (c *client) IsMoving(ctx context.Context) (bool, error) {
	resp, err := c.client.IsMoving(ctx, &pb.IsMovingRequest{Name: c.name})
	if err != nil {
		return false, err
	}
	return resp.IsMoving, nil
}

func (c *client) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetGeometries(ctx, &commonpb.GetGeometriesRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return nil, err
	}
	return referenceframe.NewGeometriesFromProto(resp.GetGeometries())
}

func (c *client) Get3DModels(ctx context.Context, extra map[string]interface{}) (map[string]*commonpb.Mesh, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Get3DModels(ctx, &commonpb.Get3DModelsRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return nil, err
	}
	return resp.Models, nil
}

// warnKinematicsUnsafe is a helper function to warn the user that no kinematics have been supplied for the conversion between
// joints space and Inputs. The assumption we are making here is safe for any arm that has only revolute joints (true for most
// commercially available arms) and will only come into play if the kinematics for the arm have not been cached successfully yet.
// The other assumption being made here is that it will be annoying for new users implementing an arm module to not be able to move their
// arm until the kinematics have been supplied.  This log message will be very noisy as it will be logged whenever kinematics are not found
// so we are hoping that they will want to do things the correct way and supply kinematics to quiet it.
func warnKinematicsUnsafe(ctx context.Context, logger logging.Logger, err error) {
	logger.CWarnw(
		ctx,
		"error getting model for arm; making the assumption that joints are revolute and that their positions are specified in degrees",
		"err",
		err,
	)
}
