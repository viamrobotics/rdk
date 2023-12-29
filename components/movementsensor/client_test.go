package movementsensor_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/components/sensor"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

var (
	testMovementSensorName    = "ms1"
	failMovementSensorName    = "ms2"
	missingMovementSensorName = "ms4"
	errLocation               = errors.New("can't get location")
	errLinearVelocity         = errors.New("can't get linear velocity")
	errLinearAcceleration     = errors.New("can't get linear acceleration")
	errAngularVelocity        = errors.New("can't get angular velocity")
	errOrientation            = errors.New("can't get orientation")
	errCompassHeading         = errors.New("can't get compass heading")
	errProperties             = errors.New("can't get properties")
	errAccuracy               = errors.New("can't get accuracy")
)

func TestClient(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger.AsZap(), rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	loc := geo.NewPoint(90, 1)
	alt := 50.5
	speed := 5.4
	ang := 1.1
	ori := spatialmath.NewEulerAngles()
	ori.Roll = 1.1
	heading := 202.
	props := &movementsensor.Properties{LinearVelocitySupported: true}
	aclZ := 1.0
	acy := map[string]float32{"x": 1.1}
	rs := map[string]interface{}{
		"position":            loc,
		"altitude":            alt,
		"linear_velocity":     r3.Vector{X: 0, Y: speed, Z: 0},
		"linear_acceleration": r3.Vector{X: 0, Y: 0, Z: aclZ},
		"angular_velocity":    spatialmath.AngularVelocity{X: 0, Y: 0, Z: ang},
		"compass":             heading,
		"orientation":         ori.OrientationVectorDegrees(),
	}

	injectMovementSensor := &inject.MovementSensor{}
	injectMovementSensor.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
		return loc, alt, nil
	}
	injectMovementSensor.LinearVelocityFunc = func(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
		return r3.Vector{X: 0, Y: speed, Z: 0}, nil
	}
	injectMovementSensor.LinearAccelerationFunc = func(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
		return r3.Vector{X: 0, Y: 0, Z: aclZ}, nil
	}
	injectMovementSensor.AngularVelocityFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
		return spatialmath.AngularVelocity{X: 0, Y: 0, Z: ang}, nil
	}
	injectMovementSensor.OrientationFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
		return ori, nil
	}
	injectMovementSensor.CompassHeadingFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) { return heading, nil }
	injectMovementSensor.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
		return props, nil
	}
	injectMovementSensor.AccuracyFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]float32,
		float32, float32, movementsensor.NmeaGGAFixType, float32, error) {
		return acy, 0, 0, -1, 0, nil
	}
	injectMovementSensor.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
		return rs, nil
	}

	injectMovementSensor2 := &inject.MovementSensor{}
	injectMovementSensor2.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
		return nil, 0, errLocation
	}
	injectMovementSensor2.LinearVelocityFunc = func(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
		return r3.Vector{}, errLinearVelocity
	}
	injectMovementSensor2.LinearAccelerationFunc = func(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
		return r3.Vector{}, errLinearAcceleration
	}
	injectMovementSensor2.AngularVelocityFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
		return spatialmath.AngularVelocity{}, errAngularVelocity
	}
	injectMovementSensor2.OrientationFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
		return nil, errOrientation
	}
	injectMovementSensor2.CompassHeadingFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		return 0, errCompassHeading
	}
	injectMovementSensor2.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
		return nil, errReadingsFailed
	}

	gpsSvc, err := resource.NewAPIResourceCollection(movementsensor.API, map[resource.Name]movementsensor.MovementSensor{
		movementsensor.Named(testMovementSensorName): injectMovementSensor,
		movementsensor.Named(failMovementSensorName): injectMovementSensor2,
	})
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[movementsensor.MovementSensor](movementsensor.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, gpsSvc), test.ShouldBeNil)

	injectMovementSensor.DoFunc = testutils.EchoFunc

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	// failing
	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := viamgrpc.Dial(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeError, context.Canceled)
	})

	t.Run("MovementSensor client 1", func(t *testing.T) {
		// working
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		gps1Client, err := movementsensor.NewClientFromConn(context.Background(), conn, "", movementsensor.Named(testMovementSensorName), logger)
		test.That(t, err, test.ShouldBeNil)

		// DoCommand
		resp, err := gps1Client.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, testutils.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])

		loc1, alt1, err := gps1Client.Position(context.Background(), map[string]interface{}{"foo": "bar"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, loc1, test.ShouldResemble, loc)
		test.That(t, alt1, test.ShouldAlmostEqual, alt)
		test.That(t, injectMovementSensor.PositionFuncExtraCap, test.ShouldResemble, map[string]interface{}{"foo": "bar"})

		vel1, err := gps1Client.LinearVelocity(context.Background(), map[string]interface{}{"foo": "bar"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, vel1.Y, test.ShouldAlmostEqual, speed)
		test.That(t, injectMovementSensor.LinearVelocityFuncExtraCap, test.ShouldResemble, map[string]interface{}{"foo": "bar"})

		av1, err := gps1Client.AngularVelocity(context.Background(), map[string]interface{}{"foo": "bar"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, av1.Z, test.ShouldAlmostEqual, ang)
		test.That(t, injectMovementSensor.AngularVelocityFuncExtraCap, test.ShouldResemble, map[string]interface{}{"foo": "bar"})

		o1, err := gps1Client.Orientation(context.Background(), map[string]interface{}{"foo": "bar"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, o1.OrientationVectorDegrees(), test.ShouldResemble, ori.OrientationVectorDegrees())
		test.That(t, injectMovementSensor.OrientationFuncExtraCap, test.ShouldResemble, map[string]interface{}{"foo": "bar"})

		ch, err := gps1Client.CompassHeading(context.Background(), map[string]interface{}{"foo": "bar"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ch, test.ShouldResemble, heading)
		test.That(t, injectMovementSensor.CompassHeadingFuncExtraCap, test.ShouldResemble, map[string]interface{}{"foo": "bar"})

		props1, err := gps1Client.Properties(context.Background(), map[string]interface{}{"foo": "bar"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, props1.LinearVelocitySupported, test.ShouldResemble, props.LinearVelocitySupported)
		test.That(t, injectMovementSensor.PropertiesFuncExtraCap, test.ShouldResemble, map[string]interface{}{"foo": "bar"})

		acc1, _, _, _, _, err := gps1Client.Accuracy(context.Background(), map[string]interface{}{"foo": "bar"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, acc1, test.ShouldResemble, acy)
		test.That(t, injectMovementSensor.AccuracyFuncExtraCap, test.ShouldResemble, map[string]interface{}{"foo": "bar"})

		la1, err := gps1Client.LinearAcceleration(context.Background(), map[string]interface{}{"foo": "bar"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, la1.Z, test.ShouldResemble, aclZ)
		test.That(t, injectMovementSensor.LinearAccelerationExtraCap, test.ShouldResemble, map[string]interface{}{"foo": "bar"})

		rs1, err := gps1Client.Readings(context.Background(), map[string]interface{}{"foo": "bar"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, rs1["position"], test.ShouldResemble, rs["position"])
		test.That(t, rs1["altitude"], test.ShouldResemble, rs["altitude"])
		test.That(t, rs1["linear_velocity"], test.ShouldResemble, rs["linear_velocity"])
		test.That(t, rs1["linear_acceleration"], test.ShouldResemble, rs["linear_acceleration"])
		test.That(t, rs1["angular_velocity"], test.ShouldResemble, rs["angular_velocity"])
		test.That(t, rs1["compass"], test.ShouldResemble, rs["compass"])
		test.That(t, rs1["orientation"], test.ShouldResemble, rs["orientation"])

		test.That(t, gps1Client.Close(context.Background()), test.ShouldBeNil)

		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("MovementSensor client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client2, err := resourceAPI.RPCClient(context.Background(), conn, "", movementsensor.Named(failMovementSensorName), logger)
		test.That(t, err, test.ShouldBeNil)

		_, _, err = client2.Position(context.Background(), make(map[string]interface{}))
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errLocation.Error())

		_, err = client2.LinearVelocity(context.Background(), make(map[string]interface{}))
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errLinearVelocity.Error())

		_, err = client2.LinearAcceleration(context.Background(), make(map[string]interface{}))
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errLinearAcceleration.Error())

		_, err = client2.AngularVelocity(context.Background(), make(map[string]interface{}))
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errAngularVelocity.Error())

		_, err = client2.(sensor.Sensor).Readings(context.Background(), make(map[string]interface{}))
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errReadingsFailed.Error())

		test.That(t, client2.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}
