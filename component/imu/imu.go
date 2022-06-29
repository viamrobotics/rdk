// Package imu defines the interface of an IMU providing angular velocity, roll,
// pitch, and yaw measurements.
package imu

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/sensor"
	"go.viam.com/rdk/data"
	pb "go.viam.com/rdk/proto/api/component/imu/v1"
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
		RPCServiceDesc: &pb.IMUService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})

	data.RegisterCollector(data.MethodMetadata{
		Subtype:    SubtypeName,
		MethodName: readAngularVelocity.String(),
	}, newReadAngularVelocityCollector)
	data.RegisterCollector(data.MethodMetadata{
		Subtype:    SubtypeName,
		MethodName: readOrientation.String(),
	}, newReadOrientationCollector)
	data.RegisterCollector(data.MethodMetadata{
		Subtype:    SubtypeName,
		MethodName: readAcceleration.String(),
	}, newReadAccelerationCollector)
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

// An IMU represents a sensor that can report AngularVelocity, Orientation, Acceleration and Magnetometer
// measurements.
type IMU interface {
	ReadAngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error)
	ReadOrientation(ctx context.Context) (spatialmath.Orientation, error)
	ReadAcceleration(ctx context.Context) (r3.Vector, error)
	ReadMagnetometer(ctx context.Context) (r3.Vector, error)
	generic.Generic
}

var (
	_ = IMU(&reconfigurableIMU{})
	_ = sensor.Sensor(&reconfigurableIMU{})
	_ = resource.Reconfigurable(&reconfigurableIMU{})
)

// FromDependencies is a helper for getting the named imu from a collection of
// dependencies.
func FromDependencies(deps registry.Dependencies, name string) (IMU, error) {
	res, ok := deps[Named(name)]
	if !ok {
		return nil, utils.DependencyNotFoundError(name)
	}
	part, ok := res.(IMU)
	if !ok {
		return nil, utils.DependencyTypeError(name, "IMU", res)
	}
	return part, nil
}

// FromRobot is a helper for getting the named IMU from the given Robot.
func FromRobot(r robot.Robot, name string) (IMU, error) {
	res, err := r.ResourceByName(Named(name))
	if err != nil {
		return nil, err
	}
	part, ok := res.(IMU)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("IMU", res)
	}
	return part, nil
}

// NamesFromRobot is a helper for getting all IMU names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesBySubtype(r, Subtype)
}

// GetReadings is a helper for getting all readings from an IMU.
func GetReadings(ctx context.Context, i IMU) ([]interface{}, error) {
	vel, err := i.ReadAngularVelocity(ctx)
	if err != nil {
		return nil, err
	}
	orientation, err := i.ReadOrientation(ctx)
	if err != nil {
		return nil, err
	}
	ea := orientation.EulerAngles()
	ac, err := i.ReadAcceleration(ctx)
	if err != nil {
		return nil, err
	}
	mg, err := i.ReadMagnetometer(ctx)
	if err != nil {
		return nil, err
	}

	return []interface{}{
		vel.X, vel.Y, vel.Z,
		ea.Roll, ea.Pitch, ea.Yaw,
		ac.X, ac.Y, ac.Z,
		mg.X, mg.Y, mg.Z,
	}, nil
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

// Do passes generic commands/data.
func (r *reconfigurableIMU) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Do(ctx, cmd)
}

// ReadAngularVelocity returns angular velocity from the gyroscope deg_per_sec.
func (r *reconfigurableIMU) ReadAngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.ReadAngularVelocity(ctx)
}

// ReadOrientation returns gyroscope orientation in degrees.
func (r *reconfigurableIMU) ReadOrientation(ctx context.Context) (spatialmath.Orientation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.ReadOrientation(ctx)
}

// ReadAcceleration returns accelerometer reading in mm_per_sec_per_sec.
func (r *reconfigurableIMU) ReadAcceleration(ctx context.Context) (r3.Vector, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.ReadAcceleration(ctx)
}

// ReadMagnetometer returns megnetif field data in gauss.
func (r *reconfigurableIMU) ReadMagnetometer(ctx context.Context) (r3.Vector, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.ReadMagnetometer(ctx)
}

// GetReadings will use the default IMU GetReadings if not provided.
func (r *reconfigurableIMU) GetReadings(ctx context.Context) ([]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if sensor, ok := r.actual.(sensor.Sensor); ok {
		return sensor.GetReadings(ctx)
	}
	return GetReadings(ctx, r.actual)
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
		return nil, utils.NewUnimplementedInterfaceError("IMU", r)
	}
	if reconfigurable, ok := imu.(*reconfigurableIMU); ok {
		return reconfigurable, nil
	}
	return &reconfigurableIMU{actual: imu}, nil
}
