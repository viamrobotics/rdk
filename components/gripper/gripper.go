// Package gripper defines a robotic gripper.
package gripper

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/gripper/v1"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		Status: func(ctx context.Context, res resource.Resource) (interface{}, error) {
			return CreateStatus(ctx, res)
		},
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.GripperService_ServiceDesc,
				NewServer(subtypeSvc),
				pb.RegisterGripperServiceHandlerFromEndpoint,
			)
		},
		RPCServiceDesc: &pb.GripperService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name resource.Name, logger golog.Logger) (resource.Resource, error) {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})
}

// SubtypeName is a constant that identifies the component resource subtype string.
const SubtypeName = resource.SubtypeName("gripper")

// Subtype is a constant that identifies the component resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// Named is a helper for getting the named grippers's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// A Gripper represents a physical robotic gripper.
type Gripper interface {
	resource.Resource

	// Open opens the gripper.
	// This will block until done or a new operation cancels this one
	Open(ctx context.Context, extra map[string]interface{}) error

	// Grab makes the gripper grab.
	// returns true if we grabbed something.
	// This will block until done or a new operation cancels this one
	Grab(ctx context.Context, extra map[string]interface{}) (bool, error)

	// Stop stops the gripper. It is assumed the gripper stops immediately.
	Stop(ctx context.Context, extra map[string]interface{}) error

	referenceframe.ModelFramer
	resource.MovingCheckable
}

// ErrStopUnimplemented is used for when Stop is unimplemented.
var ErrStopUnimplemented = errors.New("Stop unimplemented")

// FromRobot is a helper for getting the named Gripper from the given Robot.
func FromRobot(r robot.Robot, name string) (Gripper, error) {
	return robot.ResourceFromRobot[Gripper](r, Named(name))
}

// NamesFromRobot is a helper for getting all gripper names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesBySubtype(r, Subtype)
}

// CreateStatus creates a status from the gripper.
func CreateStatus(ctx context.Context, res resource.Resource) (*commonpb.ActuatorStatus, error) {
	g, err := resource.AsType[Gripper](res)
	if err != nil {
		return nil, err
	}
	isMoving, err := g.IsMoving(ctx)
	if err != nil {
		return nil, err
	}
	return &commonpb.ActuatorStatus{IsMoving: isMoving}, nil
}
