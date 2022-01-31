// Package imu defines the interface of an IMU providing angular velocity, roll,
// pitch, and yaw measurements.
package imu

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/sensor"
	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		Reconfigurable: WrapWithReconfigurable,
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.IMUService_ServiceDesc,
				NewServer(subtypeSvc),
				pb.RegisterIMUServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})
}

// SubtypeName is a constant that identifies the component resource subtype string "imu".
const SubtypeName = resource.SubtypeName("imu")

// Subtype is a constant that identifies the component resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// Named is a helper for getting the named IMU's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// An IMU represents a sensor that can report ReadAngularVelocity and Orientation measurements.
type IMU interface {
	sensor.Sensor
	ReadAngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error)
	ReadOrientation(ctx context.Context) (spatialmath.Orientation, error)
}

var (
	_ = IMU(&reconfigurableIMU{})
	_ = resource.Reconfigurable(&reconfigurableIMU{})
)

// FromRobot is a helper for getting the named IMU from the given Robot.
func FromRobot(r robot.Robot, name string) (IMU, bool) {
	res, ok := r.ResourceByName(Named(name))
	if ok {
		part, ok := res.(IMU)
		if ok {
			return part, true
		}
	}
	return nil, false
}

// NamesFromRobot is a helper for getting all IMU names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesBySubtype(r, Subtype)
}

type reconfigurableIMU struct {
	mu     sync.RWMutex
	actual IMU
}

func (r *reconfigurableIMU) Close(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return viamutils.TryClose(ctx, r.actual)
}

func (r *reconfigurableIMU) ProxyFor() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual
}

func (r *reconfigurableIMU) ReadAngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.ReadAngularVelocity(ctx)
}

func (r *reconfigurableIMU) ReadOrientation(ctx context.Context) (spatialmath.Orientation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.ReadOrientation(ctx)
}

func (r *reconfigurableIMU) GetReadings(ctx context.Context) ([]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GetReadings(ctx)
}

func (r *reconfigurableIMU) Reconfigure(ctx context.Context, newIMU resource.Reconfigurable) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	actual, ok := newIMU.(*reconfigurableIMU)
	if !ok {
		return utils.NewUnexpectedTypeError(r, newIMU)
	}
	if err := viamutils.TryClose(ctx, r.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
	return nil
}

// WrapWithReconfigurable converts a regular IMU implementation to a reconfigurableIMU.
// If imu is already a reconfigurableIMU, then nothing is done.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	imu, ok := r.(IMU)
	if !ok {
		return nil, errors.Errorf("expected resource to be IMU but got %T", r)
	}
	if reconfigurable, ok := imu.(*reconfigurableIMU); ok {
		return reconfigurable, nil
	}
	return &reconfigurableIMU{actual: imu}, nil
}
