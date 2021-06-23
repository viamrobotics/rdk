package client_test

import (
	"context"
	"net"
	"testing"

	"go.viam.com/utils"

	"go.viam.com/core/config"
	"go.viam.com/core/grpc/server"
	"go.viam.com/core/lidar/client"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/registry"
	"go.viam.com/core/sensor"
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

	f := registry.SensorLookup(compass.Type, client.ModelNameClient)
	test.That(t, f, test.ShouldNotBeNil)
	_, err = f(context.Background(), nil, config.Component{
		Host: listener1.Addr().(*net.TCPAddr).IP.String(),
		Port: listener1.Addr().(*net.TCPAddr).Port,
	}, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no sensor")

	injectDev := &inject.Compass{}
	injectDev.HeadingFunc = func(ctx context.Context) (float64, error) {
		return 5.2, nil
	}
	injectRobot2.SensorByNameFunc = func(name string) sensor.Sensor {
		return injectDev
	}

	dev, err := f(context.Background(), nil, config.Component{
		Host: listener2.Addr().(*net.TCPAddr).IP.String(),
		Port: listener2.Addr().(*net.TCPAddr).Port,
	}, logger)
	test.That(t, err, test.ShouldBeNil)
	compassDev := dev.(compass.Compass)
	test.That(t, compassDev, test.ShouldNotImplement, (*compass.RelativeCompass)(nil))

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
	injectRobot3.SensorByNameFunc = func(name string) sensor.Sensor {
		return injectRelDev
	}

	dev, err = f(context.Background(), nil, config.Component{
		Host: listener3.Addr().(*net.TCPAddr).IP.String(),
		Port: listener3.Addr().(*net.TCPAddr).Port,
	}, logger)
	test.That(t, err, test.ShouldBeNil)
	compassDev = dev.(compass.Compass)
	test.That(t, compassDev, test.ShouldImplement, (*compass.RelativeCompass)(nil))
	compassRelDev := compassDev.(compass.RelativeCompass)

	heading, err = compassRelDev.Heading(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, heading, test.ShouldEqual, 5.2)

	test.That(t, compassRelDev.Mark(context.Background()), test.ShouldBeNil)

	test.That(t, utils.TryClose(compassRelDev), test.ShouldBeNil)
}
