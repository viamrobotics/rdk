// Package gps defines the interfaces of a MovementSensor 
package movementsensor

import (
	"context"
	"math"
	"sync"

	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/sensor"
	"go.viam.com/rdk/data"
	pb "go.viam.com/rdk/proto/api/component/gps/v1"
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
				&pb.MovementSensorService_ServiceDesc,
				NewServer(subtypeSvc),
				pb.RegisterMovementSensorServiceHandlerFromEndpoint,
			)
		},
		RPCServiceDesc: &pb.MovementSensorService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})

	data.RegisterCollector(data.MethodMetadata{
		Subtype:    SubtypeName,
		MethodName: readLocation.String(),
	}, newReadLocationCollector)
	data.RegisterCollector(data.MethodMetadata{
		Subtype:    SubtypeName,
		MethodName: readAltitude.String(),
	}, newReadAltitudeCollector)
	data.RegisterCollector(data.MethodMetadata{
		Subtype:    SubtypeName,
		MethodName: readSpeed.String(),
	}, newReadSpeedCollector)
}

// SubtypeName is a constant that identifies the component resource subtype string "movement_sensor".
const SubtypeName = resource.SubtypeName("movement_sensor")

// Subtype is a constant that identifies the component resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// Named is a helper for getting the named MovementSensor's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// A MovementSensor represents a MovementSensor that can report lat/long measurements.
type MovementSensor interface {
	GetLinearVelocity(ctx context.Context) (r3.Vector3, error)
	GetAngularVelocity(ctx context.Context) (r3.Vector3, error)
	GetCompassHeading(ctx context.Context) (float64, error)
	GetOrientation(ctx context.Context)  (r3.Vector3, error)
	GetPosition(ctx context.Context) (r3.Vector3, float64, error)
	
	generic.Generic
	sensor.Sensor
}

var (
	_ = MovementSensor(&reconfigurableMovementSensor{})
	_ = sensor.Sensor(&reconfigurableMovementSensor{})
	_ = sensor.Sensor(&reconfigurableLocalMovementSensor{})
	_ = resource.Reconfigurable(&reconfigurableMovementSensor{})
	_ = resource.Reconfigurable(&reconfigurableLocalMovementSensor{})
	_ = viamutils.ContextCloser(&reconfigurableLocalMovementSensor{})
)

// FromDependencies is a helper for getting the named gps from a collection of
// dependencies.
func FromDependencies(deps registry.Dependencies, name string) (MovementSensor, error) {
	res, ok := deps[Named(name)]
	if !ok {
		return nil, utils.DependencyNotFoundError(name)
	}
	part, ok := res.(MovementSensor)
	if !ok {
		return nil, utils.DependencyTypeError(name, "MovementSensor", res)
	}
	return part, nil
}

// FromRobot is a helper for getting the named MovementSensor from the given Robot.
func FromRobot(r robot.Robot, name string) (MovementSensor, error) {
	res, err := r.ResourceByName(Named(name))
	if err != nil {
		return nil, err
	}
	part, ok := res.(MovementSensor)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("MovementSensor", res)
	}
	return part, nil
}

// NamesFromRobot is a helper for getting all MovementSensor names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesBySubtype(r, Subtype)
}

// GetReadings is a helper for getting all readings from a MovementSensor.
func GetReadings(ctx context.Context, g MovementSensor) ([]interface{}, error) {
	panic(1)
}

// GetHeading calculates bearing and absolute heading angles given 2 MovementSensor coordinates
// 0 degrees indicate North, 90 degrees indicate East and so on.
func GetHeading(gps1 *geo.Point, gps2 *geo.Point, yawOffset float64) (float64, float64, float64) {
	// convert latitude and longitude readings from degrees to radians
	gps1Lat := utils.DegToRad(gps1.Lat())
	gps1Long := utils.DegToRad(gps1.Lng())
	gps2Lat := utils.DegToRad(gps2.Lat())
	gps2Long := utils.DegToRad(gps2.Lng())

	// calculate bearing from gps1 to gps 2
	dLon := gps2Long - gps1Long
	y := math.Sin(dLon) * math.Cos(gps2Lat)
	x := math.Cos(gps1Lat)*math.Sin(gps2Lat) - math.Sin(gps1Lat)*math.Cos(gps2Lat)*math.Cos(dLon)
	brng := utils.RadToDeg(math.Atan2(y, x))

	// maps bearing to 0-360 degrees
	if brng < 0 {
		brng += 360
	}

	// calculate absolute heading from bearing, accounting for yaw offset
	// e.g if the MovementSensor antennas are mounted on the left and right sides of the robot,
	// the yaw offset would be roughly 90 degrees
	var standardBearing float64
	if brng > 180 {
		standardBearing = -(360 - brng)
	} else {
		standardBearing = brng
	}
	heading := brng - yawOffset

	// make heading positive again
	if heading < 0 {
		diff := math.Abs(heading)
		heading = 360 - diff
	}

	return brng, heading, standardBearing
}

type reconfigurableMovementSensor struct {
	mu     sync.RWMutex
	actual MovementSensor
}

func (r *reconfigurableMovementSensor) Close(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return viamutils.TryClose(ctx, r.actual)
}

func (r *reconfigurableMovementSensor) ProxyFor() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual
}

func (r *reconfigurableMovementSensor) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Do(ctx, cmd)
}

func (r *reconfigurableMovementSensor) ReadLocation(ctx context.Context) (*geo.Point, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.ReadLocation(ctx)
}

func (r *reconfigurableMovementSensor) ReadAltitude(ctx context.Context) (float64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.ReadAltitude(ctx)
}

func (r *reconfigurableMovementSensor) ReadSpeed(ctx context.Context) (float64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.ReadSpeed(ctx)
}

// GetReadings will use the default MovementSensor GetReadings if not provided.
func (r *reconfigurableMovementSensor) GetReadings(ctx context.Context) ([]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if sensor, ok := r.actual.(sensor.Sensor); ok {
		return sensor.GetReadings(ctx)
	}
	return GetReadings(ctx, r.actual)
}

func (r *reconfigurableMovementSensor) Reconfigure(ctx context.Context, newMovementSensor resource.Reconfigurable) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.reconfigure(ctx, newMovementSensor)
}

func (r *reconfigurableMovementSensor) reconfigure(ctx context.Context, newMovementSensor resource.Reconfigurable) error {
	actual, ok := newMovementSensor.(*reconfigurableMovementSensor)
	if !ok {
		return utils.NewUnexpectedTypeError(r, newMovementSensor)
	}
	if err := viamutils.TryClose(ctx, r.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
	return nil
}

type reconfigurableLocalMovementSensor struct {
	*reconfigurableMovementSensor
	actual LocalMovementSensor
}

func (r *reconfigurableLocalMovementSensor) Reconfigure(ctx context.Context, newMovementSensor resource.Reconfigurable) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	gps, ok := newMovementSensor.(*reconfigurableLocalMovementSensor)
	if !ok {
		return utils.NewUnexpectedTypeError(r, newMovementSensor)
	}
	if err := viamutils.TryClose(ctx, r.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}

	r.actual = gps.actual
	return r.reconfigurableMovementSensor.reconfigure(ctx, gps.reconfigurableMovementSensor)
}

func (r *reconfigurableLocalMovementSensor) ReadSatellites(ctx context.Context) (int, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.ReadSatellites(ctx)
}

func (r *reconfigurableLocalMovementSensor) ReadAccuracy(ctx context.Context) (float64, float64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.ReadAccuracy(ctx)
}

func (r *reconfigurableLocalMovementSensor) ReadValid(ctx context.Context) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.ReadValid(ctx)
}

// WrapWithReconfigurable converts a MovementSensor to a reconfigurableMovementSensor
// and a LocalMovementSensor implementation to a reconfigurableLocalMovementSensor.
// If MovementSensor or LocalMovementSensor is already a reconfigurableMovementSensor, then nothing is done.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	gps, ok := r.(MovementSensor)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("MovementSensor", r)
	}
	if reconfigurable, ok := gps.(*reconfigurableMovementSensor); ok {
		return reconfigurable, nil
	}
	rMovementSensor := &reconfigurableMovementSensor{actual: gps}
	gpsLocal, ok := r.(LocalMovementSensor)
	if !ok {
		return rMovementSensor, nil
	}
	if reconfigurable, ok := gps.(*reconfigurableLocalMovementSensor); ok {
		return reconfigurable, nil
	}
	return &reconfigurableLocalMovementSensor{actual: gpsLocal, reconfigurableMovementSensor: rMovementSensor}, nil
}
