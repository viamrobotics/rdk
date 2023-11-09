// Package movementsensor defines the interfaces of a MovementSensor
package movementsensor

import (
	"context"
	"strings"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	pb "go.viam.com/api/component/movementsensor/v1"

	"go.viam.com/rdk/components/sensor"
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
	}, NewPositionCollector)
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: linearVelocity.String(),
	}, NewLinearVelocityCollector)
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: angularVelocity.String(),
	}, NewAngularVelocityCollector)
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: compassHeading.String(),
	}, NewCompassHeadingCollector)
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: linearAcceleration.String(),
	}, NewLinearAccelerationCollector)
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: orientation.String(),
	}, NewOrientationCollector)
}

// SubtypeName is a constant that identifies the component resource API string "movement_sensor".
const SubtypeName = "movement_sensor"

// API is a variable that identifies the component resource API.
var API = resource.APINamespaceRDK.WithComponentType(SubtypeName)

// Named is a helper for getting the named MovementSensor's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

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
	Accuracy(ctx context.Context, extra map[string]interface{}) (map[string]float32, error)
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
