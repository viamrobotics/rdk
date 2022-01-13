package gps_test

import (
	"context"
	"errors"
	"testing"

	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"

	"go.viam.com/rdk/component/gps"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/sensor"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testGPSName    = "gps1"
	failGPSName    = "gps2"
	fakeGPSName    = "gps3"
	missingGPSName = "gps4"
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
	desc := sensor.Description{sensor.Type("gps"), ""}

	injectGPS.LocationFunc = func(ctx context.Context) (*geo.Point, error) { return loc, nil }
	injectGPS.AltitudeFunc = func(ctx context.Context) (float64, error) { return alt, nil }
	injectGPS.SpeedFunc = func(ctx context.Context) (float64, error) { return speed, nil }
	injectGPS.AccuracyFunc = func(ctx context.Context) (float64, float64, error) { return hAcc, vAcc, nil }
	injectGPS.DescFunc = func() sensor.Description { return desc }

	injectGPS2.LocationFunc = func(ctx context.Context) (*geo.Point, error) { return nil, errors.New("can't get location") }
	injectGPS2.AltitudeFunc = func(ctx context.Context) (float64, error) { return 0, errors.New("can't get altitude") }
	injectGPS2.SpeedFunc = func(ctx context.Context) (float64, error) { return 0, errors.New("can't get speed") }
	injectGPS2.AccuracyFunc = func(ctx context.Context) (float64, float64, error) { return 0, 0, errors.New("can't get accuracy") }
	injectGPS2.DescFunc = func() sensor.Description { return desc }

	t.Run("Location", func(t *testing.T) {
		resp, err := gpsServer.Location(context.Background(), &pb.GPSServiceLocationRequest{Name: testGPSName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Coordinate, test.ShouldResemble, &commonpb.GeoPoint{Latitude: loc.Lat(), Longitude: loc.Lng()})

		_, err = gpsServer.Location(context.Background(), &pb.GPSServiceLocationRequest{Name: failGPSName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get location")

		_, err = gpsServer.Location(context.Background(), &pb.GPSServiceLocationRequest{Name: fakeGPSName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not a GPS")

		_, err = gpsServer.Location(context.Background(), &pb.GPSServiceLocationRequest{Name: missingGPSName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no GPS")
	})

	//nolint:dupl
	t.Run("Altitude", func(t *testing.T) {
		resp, err := gpsServer.Altitude(context.Background(), &pb.GPSServiceAltitudeRequest{Name: testGPSName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Altitude, test.ShouldAlmostEqual, alt)

		_, err = gpsServer.Altitude(context.Background(), &pb.GPSServiceAltitudeRequest{Name: failGPSName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get altitude")

		_, err = gpsServer.Altitude(context.Background(), &pb.GPSServiceAltitudeRequest{Name: fakeGPSName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not a GPS")

		_, err = gpsServer.Altitude(context.Background(), &pb.GPSServiceAltitudeRequest{Name: missingGPSName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no GPS")
	})

	//nolint:dupl
	t.Run("Speed", func(t *testing.T) {
		resp, err := gpsServer.Speed(context.Background(), &pb.GPSServiceSpeedRequest{Name: testGPSName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.SpeedKph, test.ShouldResemble, speed)

		_, err = gpsServer.Speed(context.Background(), &pb.GPSServiceSpeedRequest{Name: failGPSName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get speed")

		_, err = gpsServer.Speed(context.Background(), &pb.GPSServiceSpeedRequest{Name: fakeGPSName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not a GPS")

		_, err = gpsServer.Speed(context.Background(), &pb.GPSServiceSpeedRequest{Name: missingGPSName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no GPS")
	})

	t.Run("Accuracy", func(t *testing.T) {
		resp, err := gpsServer.Accuracy(context.Background(), &pb.GPSServiceAccuracyRequest{Name: testGPSName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.HorizontalAccuracy, test.ShouldResemble, hAcc)
		test.That(t, resp.VerticalAccuracy, test.ShouldResemble, vAcc)

		_, err = gpsServer.Accuracy(context.Background(), &pb.GPSServiceAccuracyRequest{Name: failGPSName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get accuracy")

		_, err = gpsServer.Accuracy(context.Background(), &pb.GPSServiceAccuracyRequest{Name: fakeGPSName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not a GPS")

		_, err = gpsServer.Accuracy(context.Background(), &pb.GPSServiceAccuracyRequest{Name: missingGPSName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no GPS")
	})
}
