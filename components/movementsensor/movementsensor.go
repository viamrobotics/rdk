// Package movementsensor defines the interfaces of a MovementSensor
package movementsensor

import (
	"context"
	"strings"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	pb "go.viam.com/api/component/movementsensor/v1"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/spatialmath"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[MovementSensor]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterMovementSensorServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.MovementSensorService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: position.String(),
	}, newPositionCollector)
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: linearVelocity.String(),
	}, newLinearVelocityCollector)
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: angularVelocity.String(),
	}, newAngularVelocityCollector)
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: compassHeading.String(),
	}, newCompassHeadingCollector)
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: linearAcceleration.String(),
	}, newLinearAccelerationCollector)
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: orientation.String(),
	}, newOrientationCollector)
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: readings.String(),
	}, newReadingsCollector)
}

// SubtypeName is a constant that identifies the component resource API string "movement_sensor".
const SubtypeName = "movement_sensor"

// API is a variable that identifies the component resource API.
var API = resource.APINamespaceRDK.WithComponentType(SubtypeName)

// Named is a helper for getting the named MovementSensor's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// NmeaGGAFixType defines an integer type for representing various
// GPS fix types as defined in the NMEA standard: https://docs.novatel.com/OEM7/Content/Logs/GPGGA.htm#GPSQualityIndicators
type NmeaGGAFixType int

// A MovementSensor reports information about the robot's direction, position and speed.
type MovementSensor interface {
	resource.Sensor
	resource.Resource
	Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error)                // (lat, long), altitude (m)
	LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error)                    // m / sec
	AngularVelocity(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) // deg / sec
	LinearAcceleration(ctx context.Context, extra map[string]interface{}) (r3.Vector, error)
	CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) // [0->360)
	Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error)
	Properties(ctx context.Context, extra map[string]interface{}) (*Properties, error)
	Accuracy(ctx context.Context, extra map[string]interface{}) (map[string]float32, float32, float32, NmeaGGAFixType, float32, error)
}

// FromDependencies is a helper for getting the named movementsensor from a collection of
// dependencies.
func FromDependencies(deps resource.Dependencies, name string) (MovementSensor, error) {
	return resource.FromDependencies[MovementSensor](deps, Named(name))
}

// FromRobot is a helper for getting the named MovementSensor from the given Robot.
func FromRobot(r robot.Robot, name string) (MovementSensor, error) {
	return robot.ResourceFromRobot[MovementSensor](r, Named(name))
}

// NamesFromRobot is a helper for getting all MovementSensor names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesByAPI(r, API)
}

// DefaultAPIReadings is a helper for getting all readings from a MovementSensor.
func DefaultAPIReadings(ctx context.Context, g MovementSensor, extra map[string]interface{}) (map[string]interface{}, error) {
	readings := map[string]interface{}{}

	pos, altitude, err := g.Position(ctx, extra)
	if err != nil {
		if !strings.Contains(err.Error(), ErrMethodUnimplementedPosition.Error()) {
			return nil, err
		}
	} else {
		readings["position"] = pos
		readings["altitude"] = altitude
	}

	vel, err := g.LinearVelocity(ctx, extra)
	if err != nil {
		if !strings.Contains(err.Error(), ErrMethodUnimplementedLinearVelocity.Error()) {
			return nil, err
		}
	} else {
		readings["linear_velocity"] = vel
	}

	la, err := g.LinearAcceleration(ctx, extra)
	if err != nil {
		if !strings.Contains(err.Error(), ErrMethodUnimplementedLinearAcceleration.Error()) {
			return nil, err
		}
	} else {
		readings["linear_acceleration"] = la
	}

	avel, err := g.AngularVelocity(ctx, extra)
	if err != nil {
		if !strings.Contains(err.Error(), ErrMethodUnimplementedAngularVelocity.Error()) {
			return nil, err
		}
	} else {
		readings["angular_velocity"] = avel
	}

	compass, err := g.CompassHeading(ctx, extra)
	if err != nil {
		if !strings.Contains(err.Error(), ErrMethodUnimplementedCompassHeading.Error()) {
			return nil, err
		}
	} else {
		readings["compass"] = compass
	}

	ori, err := g.Orientation(ctx, extra)
	if err != nil {
		if !strings.Contains(err.Error(), ErrMethodUnimplementedOrientation.Error()) {
			return nil, err
		}
	} else {
		readings["orientation"] = ori
	}

	return readings, nil
}

// ToNmeaGGAFixType converts a pb.NmeaGGAFix enumeration to its corresponding NmeaGGAFixType.
// This function serves as a translator between the protobuf representation of GPS fix types.
func ToNmeaGGAFixType(fixType *pb.NmeaGGAFix) NmeaGGAFixType {
	switch fixType {
	case nil:
		return 0
	case pb.NmeaGGAFix_NMEA_GGA_FIX_GNSS.Enum():
		return 1
	case pb.NmeaGGAFix_NMEA_GGA_FIX_DGPS.Enum():
		return 2
	case pb.NmeaGGAFix_NMEA_GGA_FIX_PPS.Enum():
		return 3
	case pb.NmeaGGAFix_NMEA_GGA_FIX_RTK_FIXED.Enum():
		return 4
	case pb.NmeaGGAFix_NMEA_GGA_FIX_RTK_FLOAT.Enum():
		return 5
	case pb.NmeaGGAFix_NMEA_GGA_FIX_DEAD_RECKONING.Enum():
		return 6
	case pb.NmeaGGAFix_NMEA_GGA_FIX_MANUAL.Enum():
		return 7
	case pb.NmeaGGAFix_NMEA_GGA_FIX_SIMULATION.Enum():
		return 8
	default:
		return -1
	}
}

// ToProtoNmeaGGAFixType converts a NmeaGGAFixType to its corresponding pb.NmeaGGAFix enum.
// This function takes an NmeaGGAFixType as input and returns the equivalent protobuf NmeaGGAFix enumeration.
// The conversion is based on predefined cases where each case corresponds to a specific type of GPS fix.
func ToProtoNmeaGGAFixType(fixType NmeaGGAFixType) pb.NmeaGGAFix {
	switch fixType {
	case 0:
		return pb.NmeaGGAFix_NMEA_GGA_FIX_INVALID_UNSPECIFIED
	case 1:
		return pb.NmeaGGAFix_NMEA_GGA_FIX_GNSS
	case 2:
		return pb.NmeaGGAFix_NMEA_GGA_FIX_DGPS
	case 3:
		return pb.NmeaGGAFix_NMEA_GGA_FIX_PPS
	case 4:
		return pb.NmeaGGAFix_NMEA_GGA_FIX_RTK_FIXED
	case 5:
		return pb.NmeaGGAFix_NMEA_GGA_FIX_RTK_FLOAT
	case 6:
		return pb.NmeaGGAFix_NMEA_GGA_FIX_DEAD_RECKONING
	case 7:
		return pb.NmeaGGAFix_NMEA_GGA_FIX_MANUAL
	case 8:
		return pb.NmeaGGAFix_NMEA_GGA_FIX_SIMULATION
	default:
		return -1
	}
}
