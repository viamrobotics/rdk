// Package posetracker contains the interface and gRPC infrastructure
// for a pose tracker component
package posetracker

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/sensor"
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
	res, err := r.ResourceByName(Named(name))
	if err != nil {
		return nil, err
	}
	part, ok := res.(PoseTracker)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("PoseTracker", res)
	}
	return part, nil
}

// GetReadings is a helper for getting all readings from a PoseTracker.
func GetReadings(ctx context.Context, poseTracker PoseTracker) ([]interface{}, error) {
	poseLookup, err := poseTracker.GetPoses(ctx, []string{})
	if err != nil {
		return nil, err
	}
	result := make([]interface{}, 0)
	for bodyName, poseInFrame := range poseLookup {
		pose := poseInFrame.Pose()
		orientationVec := pose.Orientation().OrientationVectorRadians()
		poseInfo := []interface{}{
			bodyName, poseInFrame.FrameName(),
			pose.Point().X, pose.Point().Y, pose.Point().Z,
			orientationVec.OX, orientationVec.OY, orientationVec.OZ, orientationVec.Theta,
		}
		result = append(result, poseInfo)
	}
	return result, nil
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

// Do will try to Do() and error if not implemented.
func (r *reconfigurablePoseTracker) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if dev, ok := r.actual.(generic.Generic); ok {
		return dev.Do(ctx, cmd)
	}
	return nil, utils.NewUnimplementedInterfaceError("Generic", r)
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

// GetReadings will use the default PoseTracker GetReadings if not provided.
func (r *reconfigurablePoseTracker) GetReadings(ctx context.Context) ([]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if sensor, ok := r.actual.(sensor.Sensor); ok {
		return sensor.GetReadings(ctx)
	}
	return GetReadings(ctx, r.actual)
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
