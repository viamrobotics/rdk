// Package movementsensor defines the interfaces of a MovementSensor
package movementsensor

import (
	"context"
	"errors"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	pb "go.viam.com/api/component/movementsensor/v1"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/spatialmath"
)

// Properties tells you what a MovementSensor supports.
type Properties pb.GetPropertiesResponse

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[MovementSensor]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterMovementSensorServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.MovementSensorService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})

	registerCollector("Position", func(ctx context.Context, ms MovementSensor) (interface{}, error) {
		type Position struct {
			Lat float64
			Lng float64
		}
		p, _, err := ms.Position(ctx, make(map[string]interface{}))
		return Position{Lat: p.Lat(), Lng: p.Lng()}, err
	})
	registerCollector("LinearVelocity", func(ctx context.Context, ms MovementSensor) (interface{}, error) {
		v, err := ms.LinearVelocity(ctx, make(map[string]interface{}))
		return v, err
	})
	registerCollector("AngularVelocity", func(ctx context.Context, ms MovementSensor) (interface{}, error) {
		v, err := ms.AngularVelocity(ctx, make(map[string]interface{}))
		return v, err
	})
	registerCollector("CompassHeading", func(ctx context.Context, ms MovementSensor) (interface{}, error) {
		type Heading struct {
			Heading float64
		}
		h, err := ms.CompassHeading(ctx, make(map[string]interface{}))
		return Heading{Heading: h}, err
	})
	registerCollector("LinearAcceleration", func(ctx context.Context, ms MovementSensor) (interface{}, error) {
		v, err := ms.LinearAcceleration(ctx, make(map[string]interface{}))
		return v, err
	})
	registerCollector("Orientation", func(ctx context.Context, ms MovementSensor) (interface{}, error) {
		v, err := ms.Orientation(ctx, make(map[string]interface{}))
		return v, err
	})
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
	sensor.Sensor
	Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error)                // (lat, long), altitude (mm)
	LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error)                    // mm / sec
	AngularVelocity(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) // radians / sec
	LinearAcceleration(ctx context.Context, extra map[string]interface{}) (r3.Vector, error)
	CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) // [0->360)
	Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error)
	Properties(ctx context.Context, extra map[string]interface{}) (*Properties, error)
	Accuracy(ctx context.Context, extra map[string]interface{}) (map[string]float32, error) // in mm
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

// Readings is a helper for getting all readings from a MovementSensor.
func Readings(ctx context.Context, g MovementSensor, extra map[string]interface{}) (map[string]interface{}, error) {
	readings := map[string]interface{}{}

	pos, altitude, err := g.Position(ctx, extra)
	if err != nil {
		if !errors.Is(err, ErrMethodUnimplementedPosition) {
			return nil, err
		}
	} else {
		readings["position"] = pos
		readings["altitude"] = altitude
	}

	vel, err := g.LinearVelocity(ctx, extra)
	if err != nil {
		if !errors.Is(err, ErrMethodUnimplementedLinearVelocity) {
			return nil, err
		}
	} else {
		readings["linear_velocity"] = vel
	}

	la, err := g.LinearAcceleration(ctx, extra)
	if err != nil {
		if !errors.Is(err, ErrMethodUnimplementedLinearAcceleration) {
			return nil, err
		}
	} else {
		readings["linear_acceleration"] = la
	}

	avel, err := g.AngularVelocity(ctx, extra)
	if err != nil {
		if !errors.Is(err, ErrMethodUnimplementedAngularVelocity) {
			return nil, err
		}
	} else {
		readings["angular_velocity"] = avel
	}

	compass, err := g.CompassHeading(ctx, extra)
	if err != nil {
		if !errors.Is(err, ErrMethodUnimplementedCompassHeading) {
			return nil, err
		}
	} else {
		readings["compass"] = compass
	}

	ori, err := g.Orientation(ctx, extra)
	if err != nil {
		if !errors.Is(err, ErrMethodUnimplementedOrientation) {
			return nil, err
		}
	} else {
		readings["orientation"] = ori
	}

	return readings, nil
}
