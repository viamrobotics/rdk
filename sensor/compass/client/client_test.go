package client_test

import (
	"context"
	"net"
	"testing"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/api/server"
	"go.viam.com/robotcore/lidar/client"
	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/sensor"
	"go.viam.com/robotcore/sensor/compass"
	"go.viam.com/robotcore/testutils/inject"
	"go.viam.com/robotcore/utils"

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
					Type: compass.DeviceType,
				},
			},
		}, nil
	}
	injectRobot3.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
		return &pb.Status{
			Sensors: map[string]*pb.SensorStatus{
				"sensor1": {
					Type: compass.RelativeDeviceType,
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

	f := api.SensorLookup(compass.DeviceType, client.ModelNameClient)
	test.That(t, f, test.ShouldNotBeNil)
	_, err = f(context.Background(), nil, api.ComponentConfig{
		Host: listener1.Addr().(*net.TCPAddr).IP.String(),
		Port: listener1.Addr().(*net.TCPAddr).Port,
	}, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no sensor")

	injectDev := &inject.Compass{}
	injectDev.HeadingFunc = func(ctx context.Context) (float64, error) {
		return 5.2, nil
	}
	injectRobot2.SensorByNameFunc = func(name string) sensor.Device {
		return injectDev
	}

	dev, err := f(context.Background(), nil, api.ComponentConfig{
		Host: listener2.Addr().(*net.TCPAddr).IP.String(),
		Port: listener2.Addr().(*net.TCPAddr).Port,
	}, logger)
	test.That(t, err, test.ShouldBeNil)
	compassDev := dev.(compass.Device)
	test.That(t, compassDev, test.ShouldNotImplement, (*compass.RelativeDevice)(nil))

	heading, err := compassDev.Heading(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, heading, test.ShouldEqual, 5.2)

	test.That(t, utils.TryClose(compassDev), test.ShouldBeNil)

	injectRelDev := &inject.RelativeCompass{}
	injectRelDev.HeadingFunc = func(ctx context.Context) (float64, error) {
		return 5.2, nil
	}
	injectRelDev.MarkFunc = func(ctx context.Context) error {
		return nil
	}
	injectRobot3.SensorByNameFunc = func(name string) sensor.Device {
		return injectRelDev
	}

	dev, err = f(context.Background(), nil, api.ComponentConfig{
		Host: listener3.Addr().(*net.TCPAddr).IP.String(),
		Port: listener3.Addr().(*net.TCPAddr).Port,
	}, logger)
	test.That(t, err, test.ShouldBeNil)
	compassDev = dev.(compass.Device)
	test.That(t, compassDev, test.ShouldImplement, (*compass.RelativeDevice)(nil))
	compassRelDev := compassDev.(compass.RelativeDevice)

	heading, err = compassRelDev.Heading(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, heading, test.ShouldEqual, 5.2)

	test.That(t, compassRelDev.Mark(context.Background()), test.ShouldBeNil)

	test.That(t, utils.TryClose(compassRelDev), test.ShouldBeNil)
}
