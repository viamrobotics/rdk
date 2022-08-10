package movementsensor_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"

	"go.viam.com/rdk/component/movementsensor"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/movementsensor/v1"
	"go.viam.com/rdk/resource"
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

	loc := geo.NewPoint(90, 1)
	alt := 50.5
	speed := 5.4

	injectMovementSensor.GetPositionFunc = func(ctx context.Context) (*geo.Point, float64, error) { return loc, alt, nil }
	injectMovementSensor.GetLinearVelocityFunc = func(ctx context.Context) (r3.Vector, error) { return r3.Vector{0, speed, 0}, nil }

	injectMovementSensor2.GetPositionFunc = func(ctx context.Context) (*geo.Point, float64, error) {
		return nil, 0, errors.New("can't get location")
	}
	injectMovementSensor2.GetLinearVelocityFunc = func(ctx context.Context) (r3.Vector, error) {
		return r3.Vector{}, errors.New("can't get speed")
	}

	t.Run("GetPosition", func(t *testing.T) {
		resp, err := gpsServer.GetPosition(context.Background(), &pb.GetPositionRequest{Name: testMovementSensorName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Coordinate, test.ShouldResemble, &commonpb.GeoPoint{Latitude: loc.Lat(), Longitude: loc.Lng()})
		test.That(t, resp.AltitudeMm, test.ShouldEqual, alt)

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
		resp, err := gpsServer.GetLinearVelocity(context.Background(), &pb.GetLinearVelocityRequest{Name: testMovementSensorName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.LinearVelocity.Y, test.ShouldResemble, speed)

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
}
