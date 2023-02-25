// Package posetracker contains the interface and gRPC infrastructure
// for a pose tracker component
package posetracker

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	pb "go.viam.com/api/component/posetracker/v1"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
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
		RPCServiceDesc: &pb.PoseTrackerService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})
}

// SubtypeName is a constant that identifies the component resource subtype string "posetracker".
const SubtypeName = resource.SubtypeName("pose_tracker")

// Subtype is a constant that identifies the component resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// Named is a helper for getting the named PoseTracker's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

var (
	_ = PoseTracker(&reconfigurablePoseTracker{})
	_ = sensor.Sensor(&reconfigurablePoseTracker{})
	_ = resource.Reconfigurable(&reconfigurablePoseTracker{})
	_ = viamutils.ContextCloser(&reconfigurablePoseTracker{})
)

// BodyToPoseInFrame represents a map of body names to PoseInFrames.
type BodyToPoseInFrame map[string]*referenceframe.PoseInFrame

// A PoseTracker represents a robot component that can observe bodies in an
// environment and provide their respective poses in space. These poses are
// given in the context of the PoseTracker's frame of reference.
type PoseTracker interface {
	Poses(ctx context.Context, bodyNames []string, extra map[string]interface{}) (BodyToPoseInFrame, error)

	sensor.Sensor
	generic.Generic
}

// NewUnimplementedInterfaceError is used when there is a failed interface check.
func NewUnimplementedInterfaceError(actual interface{}) error {
	return utils.NewUnimplementedInterfaceError((*PoseTracker)(nil), actual)
}

// FromRobot is a helper for getting the named force matrix sensor from the given Robot.
func FromRobot(r robot.Robot, name string) (PoseTracker, error) {
	return robot.ResourceFromRobot[PoseTracker](r, Named(name))
}

// Readings is a helper for getting all readings from a PoseTracker.
func Readings(ctx context.Context, poseTracker PoseTracker) (map[string]interface{}, error) {
	poseLookup, err := poseTracker.Poses(ctx, []string{}, map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	result := map[string]interface{}{}
	for bodyName, poseInFrame := range poseLookup {
		result[bodyName] = poseInFrame
	}
	return result, nil
}

type reconfigurablePoseTracker struct {
	mu     sync.RWMutex
	name   resource.Name
	actual PoseTracker
}

func (r *reconfigurablePoseTracker) Name() resource.Name {
	return r.name
}

func (r *reconfigurablePoseTracker) ProxyFor() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual
}

// DoCommand passes generic commands/data.
func (r *reconfigurablePoseTracker) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.DoCommand(ctx, cmd)
}

func (r *reconfigurablePoseTracker) Poses(
	ctx context.Context, bodyNames []string, extra map[string]interface{},
) (BodyToPoseInFrame, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Poses(ctx, bodyNames, extra)
}

// Readings returns the PoseTrack readings.
func (r *reconfigurablePoseTracker) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Readings(ctx, extra)
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
		golog.Global().Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
	return nil
}

// WrapWithReconfigurable converts a regular PoseTracker implementation to a reconfigurablePoseTracker.
// If pose tracker is already a reconfigurablePoseTracker, then nothing is done.
func WrapWithReconfigurable(r interface{}, name resource.Name) (resource.Reconfigurable, error) {
	poseTracker, ok := r.(PoseTracker)
	if !ok {
		return nil, NewUnimplementedInterfaceError(r)
	}
	if reconfigurable, ok := poseTracker.(*reconfigurablePoseTracker); ok {
		return reconfigurable, nil
	}
	return &reconfigurablePoseTracker{name: name, actual: poseTracker}, nil
}
