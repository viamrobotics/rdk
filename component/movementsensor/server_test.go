package movementsensor_test

import (
	"context"
	"errors"
	"testing"

	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"

	"go.viam.com/rdk/component/gps"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/gps/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
)

func newServer() (pb.GPSServiceServer, *inject.GPS, *inject.GPS, error) {
	injectGPS := &inject.GPS{}
	injectGPS2 := &inject.GPS{}
	gpss := map[resource.Name]interface{}{
		gps.Named(testGPSName): injectGPS,
		gps.Named(failGPSName): injectGPS2,
		gps.Named(fakeGPSName): "notGPS",
	}
	gpsSvc, err := subtype.New(gpss)
	if err != nil {
		return nil, nil, nil, err
	}
	return gps.NewServer(gpsSvc), injectGPS, injectGPS2, nil
}

func TestServer(t *testing.T) {
	gpsServer, injectGPS, injectGPS2, err := newServer()
	test.That(t, err, test.ShouldBeNil)

	loc := geo.NewPoint(90, 1)
	alt := 50.5
	speed := 5.4
	hAcc := 0.7
	vAcc := 0.8

	injectGPS.ReadLocationFunc = func(ctx context.Context) (*geo.Point, error) { return loc, nil }
	injectGPS.ReadAltitudeFunc = func(ctx context.Context) (float64, error) { return alt, nil }
	injectGPS.ReadSpeedFunc = func(ctx context.Context) (float64, error) { return speed, nil }
	injectGPS.ReadAccuracyFunc = func(ctx context.Context) (float64, float64, error) { return hAcc, vAcc, nil }

	injectGPS2.ReadLocationFunc = func(ctx context.Context) (*geo.Point, error) { return nil, errors.New("can't get location") }
	injectGPS2.ReadAltitudeFunc = func(ctx context.Context) (float64, error) { return 0, errors.New("can't get altitude") }
	injectGPS2.ReadSpeedFunc = func(ctx context.Context) (float64, error) { return 0, errors.New("can't get speed") }
	injectGPS2.ReadAccuracyFunc = func(ctx context.Context) (float64, float64, error) { return 0, 0, errors.New("can't get accuracy") }

	t.Run("ReadLocation", func(t *testing.T) {
		resp, err := gpsServer.ReadLocation(context.Background(), &pb.ReadLocationRequest{Name: testGPSName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Coordinate, test.ShouldResemble, &commonpb.GeoPoint{Latitude: loc.Lat(), Longitude: loc.Lng()})

		_, err = gpsServer.ReadLocation(context.Background(), &pb.ReadLocationRequest{Name: failGPSName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get location")

		_, err = gpsServer.ReadLocation(context.Background(), &pb.ReadLocationRequest{Name: fakeGPSName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not a GPS")

		_, err = gpsServer.ReadLocation(context.Background(), &pb.ReadLocationRequest{Name: missingGPSName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no GPS")
	})

	//nolint:dupl
	t.Run("ReadAltitude", func(t *testing.T) {
		resp, err := gpsServer.ReadAltitude(context.Background(), &pb.ReadAltitudeRequest{Name: testGPSName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.AltitudeMeters, test.ShouldAlmostEqual, alt)

		_, err = gpsServer.ReadAltitude(context.Background(), &pb.ReadAltitudeRequest{Name: failGPSName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get altitude")

		_, err = gpsServer.ReadAltitude(context.Background(), &pb.ReadAltitudeRequest{Name: fakeGPSName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not a GPS")

		_, err = gpsServer.ReadAltitude(context.Background(), &pb.ReadAltitudeRequest{Name: missingGPSName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no GPS")
	})

	//nolint:dupl
	t.Run("ReadSpeed", func(t *testing.T) {
		resp, err := gpsServer.ReadSpeed(context.Background(), &pb.ReadSpeedRequest{Name: testGPSName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.SpeedMmPerSec, test.ShouldResemble, speed)

		_, err = gpsServer.ReadSpeed(context.Background(), &pb.ReadSpeedRequest{Name: failGPSName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get speed")

		_, err = gpsServer.ReadSpeed(context.Background(), &pb.ReadSpeedRequest{Name: fakeGPSName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not a GPS")

		_, err = gpsServer.ReadSpeed(context.Background(), &pb.ReadSpeedRequest{Name: missingGPSName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no GPS")
	})
}
