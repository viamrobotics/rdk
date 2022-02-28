// Package posetracker contains the interface and gRPC infrastructure
// for a pose tracker component
package posetracker

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	pb "go.viam.com/rdk/proto/api/component/posetracker/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		Reconfigurable: WrapWithReconfigurable,
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.PoseTrackerService_ServiceDesc,
				NewServer(subtypeSvc),
				pb.RegisterPoseTrackerServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})
}

// SubtypeName is a constant that identifies the component resource subtype string "posetracker".
const SubtypeName = resource.SubtypeName("posetracker")

// Subtype is a constant that identifies the component resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// Named is a helper for getting the named ForceMatrix's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

var (
	_ = PoseTracker(&reconfigurablePoseTracker{})
	_ = resource.Reconfigurable(&reconfigurablePoseTracker{})
)

// BodyToPoseInFrame represents a map of body names to PoseInFrames.
type BodyToPoseInFrame map[string]*referenceframe.PoseInFrame

// A PoseTracker represents a robot component that can observe bodies in an
// environment and provide their respective poses in space. These poses are
// given in the context of the PoseTracker's frame of reference.
type PoseTracker interface {
	GetPoses(ctx context.Context, bodyNames []string) (BodyToPoseInFrame, error)
}

// FromRobot is a helper for getting the named force matrix sensor from the given Robot.
func FromRobot(r robot.Robot, name string) (PoseTracker, error) {
	res, ok := r.ResourceByName(Named(name))
	if !ok {
		return nil, utils.NewResourceNotFoundError(Named(name))
	}
	part, ok := res.(PoseTracker)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("PoseTracker", res)
	}
	return part, nil
}

type reconfigurablePoseTracker struct {
	mu     sync.RWMutex
	actual PoseTracker
}

func (r *reconfigurablePoseTracker) ProxyFor() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual
}

func (r *reconfigurablePoseTracker) GetPoses(
	ctx context.Context, bodyNames []string,
) (BodyToPoseInFrame, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GetPoses(ctx, bodyNames)
}

func (r *reconfigurablePoseTracker) Close(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return viamutils.TryClose(ctx, r.actual)
}

func (r *reconfigurablePoseTracker) Reconfigure(
	ctx context.Context, newPoseTracker resource.Reconfigurable,
) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	actual, ok := newPoseTracker.(*reconfigurablePoseTracker)
	if !ok {
		return utils.NewUnexpectedTypeError(r, newPoseTracker)
	}
	if err := viamutils.TryClose(ctx, r.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
	return nil
}

// WrapWithReconfigurable converts a regular PoseTracker implementation to a reconfigurablePoseTracker.
// If pose tracker is already a reconfigurablePoseTracker, then nothing is done.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	poseTracker, ok := r.(PoseTracker)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("PoseTracker", r)
	}
	if reconfigurable, ok := poseTracker.(*reconfigurablePoseTracker); ok {
		return reconfigurable, nil
	}
	return &reconfigurablePoseTracker{actual: poseTracker}, nil
}
