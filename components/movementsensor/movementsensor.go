// Package movementsensor defines the interfaces of a MovementSensor.
// For more information, see the [movement sensor component docs].
//
// [movement sensor component docs]: https://docs.viam.com/components/movement-sensor/
package movementsensor

import (
	"context"
	"math"
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

// A MovementSensor reports information about the robot's direction, position and speed.
// For more information, see the [movement sensor component docs].
//
// Position example:
//
//	// Get the current position of the movement sensor above sea level in meters.
//	position, altitude, err := myMovementSensor.Position(context.Background(), nil)
//
// For more information, see the [Position method docs].
//
// LinearVelocity example:
//
//	// Get the current linear velocity of the movement sensor.
//	linVel, err := myMovementSensor.LinearVelocity(context.Background(), nil)
//
// For more information, see the [LinearVelocity method docs].
//
// AngularVelocity example:
//
//	// Get the current angular velocity of the movement sensor.
//	angVel, err := myMovementSensor.AngularVelocity(context.Background(), nil)
//
//	// Get the y component of angular velocity.
//	yAngVel := angVel.Y
//
// For more information, see the [AngularVelocity method docs].
//
// LinearAcceleration example:
//
//	// Get the current linear acceleration of the movement sensor.
//	linAcc, err := myMovementSensor.LinearAcceleration(context.Background(), nil)
//
// For more information, see the [LinearAcceleration method docs].
//
// CompassHeading example:
//
//	// Get the current compass heading of the movement sensor.
//	heading, err := myMovementSensor.CompassHeading(context.Background(), nil)
//
// For more information, see the [CompassHeading method docs].
//
// Orientation example:
//
//	// Get the current orientation of the movement sensor.
//	sensorOrientation, err := myMovementSensor.Orientation(context.Background(), nil)
//
//	// Get the orientation vector.
//	orientation := sensorOrientation.OrientationVectorDegrees()
//
//	// Print out the orientation vector.
//	logger.Info("The x component of the orientation vector: ", orientation.OX)
//	logger.Info("The y component of the orientation vector: ", orientation.OY)
//	logger.Info("The z component of the orientation vector: ", orientation.OZ)
//	logger.Info("The number of degrees that the movement sensor is rotated about the vector: ", orientation.Theta)
//
// For more information, see the [Orientation method docs].
//
// Properties example:
//
//	// Get the supported properties of the movement sensor.
//	properties, err := myMovementSensor.Properties(context.Background(), nil)
//
// For more information, see the [Properties method docs].
//
// Accuracy example:
//
//	// Get the accuracy of the movement sensor.
//	accuracy, err := myMovementSensor.Accuracy(context.Background(), nil)
//
// For more information, see the [Accuracy method docs].
//
// [movement sensor component docs]: https://docs.viam.com/dev/reference/apis/components/movement-sensor/
// [Position method docs]: https://docs.viam.com/dev/reference/apis/components/movement-sensor/#getposition
// [LinearVelocity method docs]: https://docs.viam.com/dev/reference/apis/components/movement-sensor/#getlinearvelocity
// [AngularVelocity method docs]: https://docs.viam.com/dev/reference/apis/components/movement-sensor/#getangularvelocity
// [LinearAcceleration method docs]: https://docs.viam.com/dev/reference/apis/components/movement-sensor/#getlinearacceleration
// [CompassHeading method docs]: https://docs.viam.com/dev/reference/apis/components/movement-sensor/#getcompassheading
// [Orientation method docs]: https://docs.viam.com/dev/reference/apis/components/movement-sensor/#getorientation
// [Properties method docs]: https://docs.viam.com/dev/reference/apis/components/movement-sensor/#getproperties
// [Accuracy method docs]: https://docs.viam.com/dev/reference/apis/components/movement-sensor/#getaccuracy
type MovementSensor interface {
	resource.Sensor
	resource.Resource
	// Position returns the current GeoPoint (latitude, longitude) and altitude of the movement sensor above sea level in meters.
	// Supported by GPS models.
	Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) // (lat, long), altitude (m)

	// LinearVelocity returns the current linear velocity as a 3D vector in meters per second.
	LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) // m / sec

	// AngularVelcoity returns the current angular velocity as a 3D vector in degrees per second.
	AngularVelocity(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) // deg / sec

	// LinearAcceleration returns the current linear acceleration as a 3D vector in meters per second per second.
	LinearAcceleration(ctx context.Context, extra map[string]interface{}) (r3.Vector, error)

	// CompassHeading returns the current compass heading in degrees.
	CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) // [0->360)

	// Orientation returns the current orientation of the movement sensor.
	Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error)

	// Properties returns the supported properties of the movement sensor.
	Properties(ctx context.Context, extra map[string]interface{}) (*Properties, error)

	// Accuracy returns the reliability metrics of the movement sensor,
	// including various parameters to access the sensor's accuracy and precision in different dimensions.
	Accuracy(ctx context.Context, extra map[string]interface{}) (*Accuracy, error)
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

// UnimplementedOptionalAccuracies returns accuracy values that will not show up on movement sensor's RC card
// or be useable for a caller of the GetAccuracies method. The RC card currently continuously polls accuracies,
// so a nil error must be rturned from the GetAccuracies call.
// It contains NaN definitiions for accuracies returned in floats, an invalid integer value for the NMEAFix of a gps
// and an empty map of other accuracies.
func UnimplementedOptionalAccuracies() *Accuracy {
	nan32Bit := float32(math.NaN())
	nmeaInvalid := int32(-1)

	return &Accuracy{
		Hdop:               nan32Bit,
		Vdop:               nan32Bit,
		NmeaFix:            nmeaInvalid,
		CompassDegreeError: nan32Bit,
	}
}
