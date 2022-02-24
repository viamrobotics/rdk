package sensors_test

import (
	"context"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/component/imu"
	viamgrpc "go.viam.com/rdk/grpc"
	pb "go.viam.com/rdk/proto/api/service/sensors/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/sensors"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	injectSensors := &inject.SensorsService{}
	omMap := map[resource.Name]interface{}{
		sensors.Name: injectSensors,
	}
	svc, err := subtype.New(omMap)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype := registry.ResourceSubtypeLookup(sensors.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, svc)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	t.Run("failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = sensors.NewClient(cancelCtx, "", listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	// working client
	t.Run("sensors client 1", func(t *testing.T) {
		client, err := sensors.NewClient(context.Background(), "", listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		names := []resource.Name{gps.Named("gps"), imu.Named("imu")}
		injectSensors.GetSensorsFunc = func(ctx context.Context) ([]resource.Name, error) {
			return names, nil
		}
		sensorNames, err := client.GetSensors(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, sensorNames, test.ShouldResemble, names)

		iReading := sensors.Readings{Name: imu.Named("imu"), Readings: []interface{}{1.2, 2.3, 3.4}}
		gReading := sensors.Readings{Name: gps.Named("gps"), Readings: []interface{}{4.5, 5.6, 6.7}}
		readings := []sensors.Readings{iReading, gReading}
		expected := map[resource.Name]interface{}{
			iReading.Name: iReading.Readings,
			gReading.Name: gReading.Readings,
		}

		injectSensors.GetReadingsFunc = func(ctx context.Context, sensors []resource.Name) ([]sensors.Readings, error) {
			return readings, nil
		}

		readings, err = client.GetReadings(context.Background(), []resource.Name{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(readings), test.ShouldEqual, 2)
		observed := map[resource.Name]interface{}{
			readings[0].Name: readings[0].Readings,
			readings[1].Name: readings[1].Readings,
		}
		test.That(t, observed, test.ShouldResemble, expected)

		test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
	})

	// broken client
	t.Run("sensors client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client := resourceSubtype.RPCClient(context.Background(), conn, "", logger)
		client2, ok := client.(sensors.Service)
		test.That(t, ok, test.ShouldBeTrue)

		passedErr := errors.New("can't get sensors")
		injectSensors.GetSensorsFunc = func(ctx context.Context) ([]resource.Name, error) {
			return nil, passedErr
		}

		_, err = client2.GetSensors(context.Background())
		test.That(t, err.Error(), test.ShouldContainSubstring, passedErr.Error())

		passedErr = errors.New("can't get readings")
		injectSensors.GetReadingsFunc = func(ctx context.Context, sensors []resource.Name) ([]sensors.Readings, error) {
			return nil, passedErr
		}
		_, err = client2.GetReadings(context.Background(), []resource.Name{})
		test.That(t, err.Error(), test.ShouldContainSubstring, passedErr.Error())

		test.That(t, utils.TryClose(context.Background(), client2), test.ShouldBeNil)
	})
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()

	injectSensors := &inject.SensorsService{}
	omMap := map[resource.Name]interface{}{
		sensors.Name: injectSensors,
	}
	server, err := newServer(omMap)
	test.That(t, err, test.ShouldBeNil)
	pb.RegisterSensorsServiceServer(gServer, server)

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	client1, err := sensors.NewClient(ctx, "", listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)
	client2, err := sensors.NewClient(ctx, "", listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)
}
