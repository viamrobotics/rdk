package gps_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/component/sensor"
	viamgrpc "go.viam.com/rdk/grpc"
	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()

	loc := geo.NewPoint(90, 1)
	alt := 50.5
	speed := 5.4
	hAcc := 0.7
	vAcc := 0.8
	rs := []interface{}{loc.Lat(), loc.Lng(), alt, speed, hAcc, vAcc}
	desc := sensor.Description{sensor.Type("gps"), ""}

	gps1 := "gps1"
	injectGPS := &inject.GPS{}
	injectGPS.LocationFunc = func(ctx context.Context) (*geo.Point, error) { return loc, nil }
	injectGPS.AltitudeFunc = func(ctx context.Context) (float64, error) { return alt, nil }
	injectGPS.SpeedFunc = func(ctx context.Context) (float64, error) { return speed, nil }
	injectGPS.AccuracyFunc = func(ctx context.Context) (float64, float64, error) { return hAcc, vAcc, nil }
	injectGPS.DescFunc = func() sensor.Description { return desc }

	gps2 := "gps2"
	injectGPS2 := &inject.GPS{}
	injectGPS2.LocationFunc = func(ctx context.Context) (*geo.Point, error) { return nil, errors.New("can't get location") }
	injectGPS2.AltitudeFunc = func(ctx context.Context) (float64, error) { return 0, errors.New("can't get altitude") }
	injectGPS2.SpeedFunc = func(ctx context.Context) (float64, error) { return 0, errors.New("can't get speed") }
	injectGPS2.AccuracyFunc = func(ctx context.Context) (float64, float64, error) { return 0, 0, errors.New("can't get accuracy") }
	injectGPS2.DescFunc = func() sensor.Description { return desc }

	gpsSvc, err := subtype.New((map[resource.Name]interface{}{gps.Named(gps1): injectGPS, gps.Named(gps2): injectGPS2}))
	test.That(t, err, test.ShouldBeNil)
	pb.RegisterGPSServiceServer(gServer, gps.NewServer(gpsSvc))

	go gServer.Serve(listener1)
	defer gServer.Stop()

	// failing
	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = gps.NewClient(cancelCtx, gps1, listener1.Addr().String(), logger, rpc.WithInsecure())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	t.Run("GPS client 1", func(t *testing.T) {
		// working
		gps1Client, err := gps.NewClient(context.Background(), gps1, listener1.Addr().String(), logger, rpc.WithInsecure())
		test.That(t, err, test.ShouldBeNil)

		loc1, err := gps1Client.Location(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, loc1, test.ShouldResemble, loc)

		alt1, err := gps1Client.Altitude(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, alt1, test.ShouldAlmostEqual, alt)

		speed1, err := gps1Client.Speed(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, speed1, test.ShouldAlmostEqual, speed)

		hAcc1, vAcc1, err := gps1Client.Accuracy(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, hAcc1, test.ShouldAlmostEqual, hAcc)
		test.That(t, vAcc1, test.ShouldAlmostEqual, vAcc)

		rs1, err := gps1Client.Readings(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, rs1, test.ShouldResemble, rs)

		desc1 := gps1Client.Desc()
		test.That(t, desc1, test.ShouldResemble, desc)

		test.That(t, utils.TryClose(context.Background(), gps1Client), test.ShouldBeNil)
	})

	t.Run("GPS client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger, rpc.WithInsecure())
		test.That(t, err, test.ShouldBeNil)
		gps2Client := gps.NewClientFromConn(context.Background(), conn, gps2, logger)

		_, err = gps2Client.Location(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get location")

		_, err = gps2Client.Altitude(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get altitude")

		_, err = gps2Client.Speed(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get speed")

		_, _, err = gps2Client.Accuracy(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get accuracy")

		_, err = gps2Client.Readings(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get location")

		desc2 := gps2Client.Desc()
		test.That(t, desc2, test.ShouldResemble, desc)

		test.That(t, utils.TryClose(context.Background(), gps2Client), test.ShouldBeNil)
	})
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectGPS := &inject.GPS{}
	gps1 := "gps1"

	gpsSvc, err := subtype.New((map[resource.Name]interface{}{gps.Named(gps1): injectGPS}))
	test.That(t, err, test.ShouldBeNil)
	pb.RegisterGPSServiceServer(gServer, gps.NewServer(gpsSvc))

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	client1, err := gps.NewClient(ctx, gps1, listener.Addr().String(), logger, rpc.WithInsecure())
	test.That(t, err, test.ShouldBeNil)
	client2, err := gps.NewClient(ctx, gps1, listener.Addr().String(), logger, rpc.WithInsecure())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.DialCalled, test.ShouldEqual, 2)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)
}
