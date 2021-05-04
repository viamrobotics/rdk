package client_test

import (
	"context"
	"net"
	"testing"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/api/server"
	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/lidar/client"
	pb "go.viam.com/robotcore/proto/api/v1"
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
	defer gServer1.Stop()
	go gServer2.Serve(listener2)
	defer gServer2.Stop()

	f := api.LidarDeviceLookup(client.ModelNameClient)
	test.That(t, f, test.ShouldNotBeNil)
	_, err = f(context.Background(), nil, api.ComponentConfig{
		Host: listener1.Addr().(*net.TCPAddr).IP.String(),
		Port: listener1.Addr().(*net.TCPAddr).Port,
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

	dev, err := f(context.Background(), nil, api.ComponentConfig{
		Host: listener2.Addr().(*net.TCPAddr).IP.String(),
		Port: listener2.Addr().(*net.TCPAddr).Port,
	}, logger)
	test.That(t, err, test.ShouldBeNil)

	info, err := dev.Info(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, info, test.ShouldResemble, infoM)

	test.That(t, utils.TryClose(dev), test.ShouldBeNil)
}
