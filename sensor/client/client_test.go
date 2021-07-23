package client_test

import (
	"context"
	"net"
	"testing"

	"go.viam.com/utils"

	"go.viam.com/core/grpc/server"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/sensor"
	"go.viam.com/core/sensor/client"
	"go.viam.com/core/sensor/compass"
	"go.viam.com/core/testutils/inject"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"google.golang.org/grpc"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	listener2, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	listener3, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer1 := grpc.NewServer()
	injectRobot1 := &inject.Robot{}
	gServer2 := grpc.NewServer()
	injectRobot2 := &inject.Robot{}
	gServer3 := grpc.NewServer()
	injectRobot3 := &inject.Robot{}
	pb.RegisterRobotServiceServer(gServer1, server.New(injectRobot1))
	pb.RegisterRobotServiceServer(gServer2, server.New(injectRobot2))
	pb.RegisterRobotServiceServer(gServer3, server.New(injectRobot3))

	injectRobot1.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
		return &pb.Status{}, nil
	}
	injectRobot2.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
		return &pb.Status{
			Sensors: map[string]*pb.SensorStatus{
				"sensor1": {
					Type: compass.Type,
				},
			},
		}, nil
	}
	injectRobot3.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
		return &pb.Status{
			Sensors: map[string]*pb.SensorStatus{
				"sensor1": {
					Type: compass.RelativeType,
				},
			},
		}, nil
	}

	go gServer1.Serve(listener1)
	defer gServer1.Stop()
	go gServer2.Serve(listener2)
	defer gServer2.Stop()
	go gServer3.Serve(listener3)
	defer gServer3.Stop()

	_, err = client.NewClient(context.Background(), listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no sensor")

	injectDev := &inject.Compass{}
	injectDev.HeadingFunc = func(ctx context.Context) (float64, error) {
		return 5.2, nil
	}
	injectRobot2.SensorByNameFunc = func(name string) (sensor.Sensor, bool) {
		return injectDev, true
	}

	dev, err := client.NewClient(context.Background(), listener2.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	compassDev := dev.Wrapped().(compass.Compass)
	test.That(t, compassDev, test.ShouldNotImplement, (*compass.RelativeCompass)(nil))

	readings, err := dev.Readings(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, readings, test.ShouldResemble, []interface{}{5.2})

	heading, err := compassDev.Heading(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, heading, test.ShouldEqual, 5.2)

	test.That(t, utils.TryClose(compassDev), test.ShouldBeNil)
	test.That(t, dev.Close(), test.ShouldBeNil)

	injectRelDev := &inject.RelativeCompass{}
	injectRelDev.HeadingFunc = func(ctx context.Context) (float64, error) {
		return 5.2, nil
	}
	injectRelDev.MarkFunc = func(ctx context.Context) error {
		return nil
	}
	injectRobot3.SensorByNameFunc = func(name string) (sensor.Sensor, bool) {
		return injectRelDev, true
	}

	dev, err = client.NewClient(context.Background(), listener3.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	compassDev = dev.Wrapped().(compass.Compass)
	test.That(t, compassDev, test.ShouldImplement, (*compass.RelativeCompass)(nil))

	readings, err = dev.Readings(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, readings, test.ShouldResemble, []interface{}{5.2})

	compassRelDev := compassDev.(compass.RelativeCompass)

	heading, err = compassRelDev.Heading(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, heading, test.ShouldEqual, 5.2)

	test.That(t, compassRelDev.Mark(context.Background()), test.ShouldBeNil)

	test.That(t, utils.TryClose(compassRelDev), test.ShouldBeNil)
	test.That(t, dev.Close(), test.ShouldBeNil)
}
