package client_test

import (
	"context"
	"net"
	"testing"

	"go.viam.com/robotcore/api/server"
	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/lidar/client"
	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/testutils/inject"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
	"google.golang.org/grpc"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	listener2, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer1 := grpc.NewServer()
	injectRobot1 := &inject.Robot{}
	gServer2 := grpc.NewServer()
	injectRobot2 := &inject.Robot{}
	pb.RegisterRobotServiceServer(gServer1, server.New(injectRobot1))
	pb.RegisterRobotServiceServer(gServer2, server.New(injectRobot2))

	injectRobot1.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
		return &pb.Status{}, nil
	}
	injectRobot2.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
		return &pb.Status{
			LidarDevices: map[string]bool{
				"lidar1": true,
			},
		}, nil
	}

	go gServer1.Serve(listener1)
	defer gServer2.Stop()
	go gServer2.Serve(listener2)
	defer gServer2.Stop()

	_, err = lidar.CreateDevice(context.Background(), lidar.DeviceDescription{
		Type: client.DeviceTypeClient,
		Path: listener1.Addr().String(),
	}, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no lidar")

	injectDev := &inject.LidarDevice{}
	infoM := map[string]interface{}{"hello": "world"}
	injectDev.InfoFunc = func(ctx context.Context) (map[string]interface{}, error) {
		return infoM, nil
	}
	injectRobot2.LidarDeviceByNameFunc = func(name string) lidar.Device {
		return injectDev
	}

	dev, err := lidar.CreateDevice(context.Background(), lidar.DeviceDescription{
		Type: client.DeviceTypeClient,
		Path: listener2.Addr().String(),
	}, logger)
	test.That(t, err, test.ShouldBeNil)

	info, err := dev.Info(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, info, test.ShouldResemble, infoM)
}
