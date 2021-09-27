package client_test

import (
	"context"
	"net"
	"testing"

	"go.viam.com/utils"

	"go.viam.com/core/config"
	"go.viam.com/core/grpc/server"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/registry"
	"go.viam.com/core/sensor"
	"go.viam.com/core/sensor/imu"
	"go.viam.com/core/sensor/imu/client"
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
			Sensors: map[string]*pb.SensorStatus{
				"sensor1": {
					Type: imu.Type,
				},
			},
		}, nil
	}

	go gServer1.Serve(listener1)
	defer gServer1.Stop()
	go gServer2.Serve(listener2)
	defer gServer2.Stop()

	f := registry.SensorLookup(imu.Type, client.ModelNameClient)
	test.That(t, f, test.ShouldNotBeNil)
	_, err = f.Constructor(context.Background(), nil, config.Component{
		Host: listener1.Addr().(*net.TCPAddr).IP.String(),
		Port: listener1.Addr().(*net.TCPAddr).Port,
	}, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no sensor")

	injectDev := &inject.IMU{}
	rotationVector := []float64{5.2, 1.0, -0.9}
	orientationVector := []float64{5.2, 1.0, -0.9}
	injectDev.AngularVelocitiesFunc = func(ctx context.Context) ([]float64, error) {
		return rotationVector, nil
	}
	injectDev.OrientationFunc = func(ctx context.Context) ([]float64, error) {
		return orientationVector, nil
	}
	injectRobot2.SensorByNameFunc = func(name string) (sensor.Sensor, bool) {
		return injectDev, true
	}

	dev, err := f.Constructor(context.Background(), nil, config.Component{
		Host: listener2.Addr().(*net.TCPAddr).IP.String(),
		Port: listener2.Addr().(*net.TCPAddr).Port,
	}, logger)
	test.That(t, err, test.ShouldBeNil)
	imuDev := dev.(imu.IMU)

	// test angular velocity properly set
	angularVelocities, err := imuDev.AngularVelocity(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, angularVelocities, test.ShouldResemble, rotationVector)

	// test orientation properly set
	orientation, err := imuDev.Orientation(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, orientation, test.ShouldResemble, orientationVector)

	// test close
	test.That(t, utils.TryClose(imuDev), test.ShouldBeNil)

}
