package movementsensor_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/movementsensor/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
)

func newServer() (pb.MovementSensorServiceServer, *inject.MovementSensor, *inject.MovementSensor, error) {
	injectMovementSensor := &inject.MovementSensor{}
	injectMovementSensor2 := &inject.MovementSensor{}
	gpss := map[resource.Name]interface{}{
		movementsensor.Named(testMovementSensorName): injectMovementSensor,
		movementsensor.Named(failMovementSensorName): injectMovementSensor2,
		movementsensor.Named(fakeMovementSensorName): "notMovementSensor",
	}
	gpsSvc, err := subtype.New(gpss)
	if err != nil {
		return nil, nil, nil, err
	}
	return movementsensor.NewServer(gpsSvc), injectMovementSensor, injectMovementSensor2, nil
}

func TestServer(t *testing.T) {
	gpsServer, injectMovementSensor, injectMovementSensor2, err := newServer()
	test.That(t, err, test.ShouldBeNil)

	t.Run("GetPosition", func(t *testing.T) {
		loc := geo.NewPoint(90, 1)
		alt := 50.5
		injectMovementSensor.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
			return loc, alt, nil
		}
		injectMovementSensor2.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
			return nil, 0, errors.New("can't get location")
		}

		ext, err := protoutils.StructToStructPb(map[string]interface{}{"foo": "bar"})
		test.That(t, err, test.ShouldBeNil)
		resp, err := gpsServer.GetPosition(context.Background(), &pb.GetPositionRequest{Name: testMovementSensorName, Extra: ext})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Coordinate, test.ShouldResemble, &commonpb.GeoPoint{Latitude: loc.Lat(), Longitude: loc.Lng()})
		test.That(t, resp.AltitudeMm, test.ShouldEqual, alt)
		test.That(t, injectMovementSensor.PositionFuncExtraCap, test.ShouldResemble, map[string]interface{}{"foo": "bar"})

		_, err = gpsServer.GetPosition(context.Background(), &pb.GetPositionRequest{Name: failMovementSensorName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get location")

		_, err = gpsServer.GetPosition(context.Background(), &pb.GetPositionRequest{Name: fakeMovementSensorName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not a MovementSensor")

		_, err = gpsServer.GetPosition(context.Background(), &pb.GetPositionRequest{Name: missingMovementSensorName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no MovementSensor")
	})

	t.Run("GetLinearVelocity", func(t *testing.T) {
		speed := 5.4
		injectMovementSensor.LinearVelocityFunc = func(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
			return r3.Vector{0, speed, 0}, nil
		}

		injectMovementSensor2.LinearVelocityFunc = func(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
			return r3.Vector{}, errors.New("can't get speed")
		}

		ext, err := protoutils.StructToStructPb(map[string]interface{}{"foo": "bar"})
		test.That(t, err, test.ShouldBeNil)
		resp, err := gpsServer.GetLinearVelocity(context.Background(), &pb.GetLinearVelocityRequest{Name: testMovementSensorName, Extra: ext})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.LinearVelocity.Y, test.ShouldResemble, speed)
		test.That(t, injectMovementSensor.LinearVelocityFuncExtraCap, test.ShouldResemble, map[string]interface{}{"foo": "bar"})

		_, err = gpsServer.GetLinearVelocity(context.Background(), &pb.GetLinearVelocityRequest{Name: failMovementSensorName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get speed")

		_, err = gpsServer.GetLinearVelocity(context.Background(), &pb.GetLinearVelocityRequest{Name: fakeMovementSensorName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not a MovementSensor")

		_, err = gpsServer.GetLinearVelocity(context.Background(), &pb.GetLinearVelocityRequest{Name: missingMovementSensorName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no MovementSensor")
	})

	t.Run("GetAngularVelocity", func(t *testing.T) {
		angZ := 1.1
		injectMovementSensor.AngularVelocityFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
			return spatialmath.AngularVelocity{0, 0, angZ}, nil
		}
		injectMovementSensor2.AngularVelocityFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
			return spatialmath.AngularVelocity{}, errors.New("can't get angular velocity")
		}

		ext, err := protoutils.StructToStructPb(map[string]interface{}{"foo": "bar"})
		test.That(t, err, test.ShouldBeNil)
		resp, err := gpsServer.GetAngularVelocity(context.Background(), &pb.GetAngularVelocityRequest{Name: testMovementSensorName, Extra: ext})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.AngularVelocity.Z, test.ShouldResemble, angZ)
		test.That(t, injectMovementSensor.AngularVelocityFuncExtraCap, test.ShouldResemble, map[string]interface{}{"foo": "bar"})

		_, err = gpsServer.GetAngularVelocity(context.Background(), &pb.GetAngularVelocityRequest{Name: failMovementSensorName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get angular velocity")

		_, err = gpsServer.GetAngularVelocity(context.Background(), &pb.GetAngularVelocityRequest{Name: fakeMovementSensorName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not a MovementSensor")

		_, err = gpsServer.GetAngularVelocity(context.Background(), &pb.GetAngularVelocityRequest{Name: missingMovementSensorName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no MovementSensor")
	})

	t.Run("GetOrientation", func(t *testing.T) {
		ori := spatialmath.NewEulerAngles()
		ori.Roll = 1.1
		injectMovementSensor.OrientationFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
			return ori, nil
		}
		injectMovementSensor2.OrientationFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
			return nil, errors.New("can't get orientation")
		}

		ext, err := protoutils.StructToStructPb(map[string]interface{}{"foo": "bar"})
		test.That(t, err, test.ShouldBeNil)
		resp, err := gpsServer.GetOrientation(context.Background(), &pb.GetOrientationRequest{Name: testMovementSensorName, Extra: ext})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Orientation.OZ, test.ShouldResemble, ori.OrientationVectorDegrees().OZ)
		test.That(t, injectMovementSensor.OrientationFuncExtraCap, test.ShouldResemble, map[string]interface{}{"foo": "bar"})

		_, err = gpsServer.GetOrientation(context.Background(), &pb.GetOrientationRequest{Name: failMovementSensorName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get orientation")

		_, err = gpsServer.GetOrientation(context.Background(), &pb.GetOrientationRequest{Name: fakeMovementSensorName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not a MovementSensor")

		_, err = gpsServer.GetOrientation(context.Background(), &pb.GetOrientationRequest{Name: missingMovementSensorName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no MovementSensor")
	})

	t.Run("GetCompassHeading", func(t *testing.T) {
		heading := 202.
		injectMovementSensor.CompassHeadingFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) { return heading, nil }
		injectMovementSensor2.CompassHeadingFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
			return 0.0, errors.New("can't get compass heading")
		}

		ext, err := protoutils.StructToStructPb(map[string]interface{}{"foo": "bar"})
		test.That(t, err, test.ShouldBeNil)
		resp, err := gpsServer.GetCompassHeading(context.Background(), &pb.GetCompassHeadingRequest{Name: testMovementSensorName, Extra: ext})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Value, test.ShouldResemble, heading)
		test.That(t, injectMovementSensor.CompassHeadingFuncExtraCap, test.ShouldResemble, map[string]interface{}{"foo": "bar"})

		_, err = gpsServer.GetCompassHeading(context.Background(), &pb.GetCompassHeadingRequest{Name: failMovementSensorName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get compass heading")

		_, err = gpsServer.GetCompassHeading(context.Background(), &pb.GetCompassHeadingRequest{Name: fakeMovementSensorName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not a MovementSensor")

		_, err = gpsServer.GetCompassHeading(context.Background(), &pb.GetCompassHeadingRequest{Name: missingMovementSensorName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no MovementSensor")
	})

	t.Run("GetProperties", func(t *testing.T) {
		props := &movementsensor.Properties{LinearVelocitySupported: true}
		injectMovementSensor.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
			return props, nil
		}
		injectMovementSensor2.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
			return nil, errors.New("can't get properties")
		}

		ext, err := protoutils.StructToStructPb(map[string]interface{}{"foo": "bar"})
		test.That(t, err, test.ShouldBeNil)
		resp, err := gpsServer.GetProperties(context.Background(), &pb.GetPropertiesRequest{Name: testMovementSensorName, Extra: ext})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.LinearVelocitySupported, test.ShouldResemble, props.LinearVelocitySupported)
		test.That(t, injectMovementSensor.PropertiesFuncExtraCap, test.ShouldResemble, map[string]interface{}{"foo": "bar"})

		_, err = gpsServer.GetProperties(context.Background(), &pb.GetPropertiesRequest{Name: failMovementSensorName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get properties")

		_, err = gpsServer.GetProperties(context.Background(), &pb.GetPropertiesRequest{Name: fakeMovementSensorName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not a MovementSensor")

		_, err = gpsServer.GetProperties(context.Background(), &pb.GetPropertiesRequest{Name: missingMovementSensorName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no MovementSensor")
	})

	t.Run("GetAccuracy", func(t *testing.T) {
		acc := map[string]float32{"x": 1.1}
		injectMovementSensor.AccuracyFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]float32, error) {
			return acc, nil
		}
		injectMovementSensor2.AccuracyFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]float32, error) {
			return nil, errors.New("can't get accuracy")
		}

		ext, err := protoutils.StructToStructPb(map[string]interface{}{"foo": "bar"})
		test.That(t, err, test.ShouldBeNil)
		resp, err := gpsServer.GetAccuracy(context.Background(), &pb.GetAccuracyRequest{Name: testMovementSensorName, Extra: ext})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.AccuracyMm, test.ShouldResemble, acc)
		test.That(t, injectMovementSensor.AccuracyFuncExtraCap, test.ShouldResemble, map[string]interface{}{"foo": "bar"})

		_, err = gpsServer.GetAccuracy(context.Background(), &pb.GetAccuracyRequest{Name: failMovementSensorName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get accuracy")

		_, err = gpsServer.GetAccuracy(context.Background(), &pb.GetAccuracyRequest{Name: fakeMovementSensorName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not a MovementSensor")

		_, err = gpsServer.GetAccuracy(context.Background(), &pb.GetAccuracyRequest{Name: missingMovementSensorName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no MovementSensor")
	})
}
