package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"math"
	"net"
	"testing"

	"go.uber.org/zap/zaptest/observer"
	"go.viam.com/robotcore/api/server"
	"go.viam.com/robotcore/lidar"
	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/robots/fake"
	"go.viam.com/robotcore/testutils"
	"go.viam.com/robotcore/testutils/inject"

	// register
	_ "go.viam.com/robotcore/lidar/client"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
	"google.golang.org/grpc"
)

func TestMain(t *testing.T) {
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectRobot := &inject.Robot{}
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))

	injectRobot.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
		return &pb.Status{
			LidarDevices: map[string]bool{
				"lidar1": true,
			},
		}, nil
	}

	injectDev := &inject.LidarDevice{}
	injectRobot.LidarDeviceByNameFunc = func(name string) lidar.Device {
		return injectDev
	}

	go gServer.Serve(listener)
	defer gServer.Stop()

	injectDev.InfoFunc = func(ctx context.Context) (map[string]interface{}, error) {
		return map[string]interface{}{"hello": "world"}, nil
	}
	injectDev.StartFunc = func(ctx context.Context) error {
		return nil
	}
	injectDev.StopFunc = func(ctx context.Context) error {
		return nil
	}
	injectDev.CloseFunc = func(ctx context.Context) error {
		return nil
	}
	injectDev.ScanFunc = func(ctx context.Context, opts lidar.ScanOptions) (lidar.Measurements, error) {
		return lidar.Measurements{lidar.NewMeasurement(0, 5)}, nil
	}
	injectDev.RangeFunc = func(ctx context.Context) (int, error) {
		return 10, nil
	}
	injectDev.BoundsFunc = func(ctx context.Context) (image.Point, error) {
		return image.Point{4, 5}, nil
	}
	injectDev.AngularResolutionFunc = func(ctx context.Context) (float64, error) {
		return 1, nil
	}

	go gServer.Serve(listener)
	defer gServer.Stop()

	assignLogger := func(t *testing.T, tLogger golog.Logger, exec *testutils.ContextualMainExecution) {
		logger = tLogger
	}

	lidar.RegisterDeviceType("fail_info", lidar.DeviceTypeRegistration{
		New: func(ctx context.Context, desc lidar.DeviceDescription, logger golog.Logger) (lidar.Device, error) {
			dev := &inject.LidarDevice{Device: &fake.Lidar{}}
			dev.InfoFunc = func(ctx context.Context) (map[string]interface{}, error) {
				return nil, errors.New("whoops")
			}
			return dev, nil
		},
	})
	lidar.RegisterDeviceType("fail_ang", lidar.DeviceTypeRegistration{
		New: func(ctx context.Context, desc lidar.DeviceDescription, logger golog.Logger) (lidar.Device, error) {
			dev := &inject.LidarDevice{Device: &fake.Lidar{}}
			dev.AngularResolutionFunc = func(ctx context.Context) (float64, error) {
				return math.NaN(), errors.New("whoops")
			}
			return dev, nil
		},
	})
	lidar.RegisterDeviceType("fail_stop", lidar.DeviceTypeRegistration{
		New: func(ctx context.Context, desc lidar.DeviceDescription, logger golog.Logger) (lidar.Device, error) {
			dev := &inject.LidarDevice{Device: &fake.Lidar{}}
			dev.StopFunc = func(ctx context.Context) error {
				return errors.New("whoops")
			}
			return dev, nil
		},
	})
	lidar.RegisterDeviceType("fail_scan", lidar.DeviceTypeRegistration{
		New: func(ctx context.Context, desc lidar.DeviceDescription, logger golog.Logger) (lidar.Device, error) {
			dev := &inject.LidarDevice{Device: &fake.Lidar{}}
			var once bool
			dev.ScanFunc = func(ctx context.Context, options lidar.ScanOptions) (lidar.Measurements, error) {
				if once {
					return nil, errors.New("whoops")
				}
				once = true
				return lidar.Measurements{}, nil
			}
			return dev, nil
		},
	})
	testutils.TestMain(t, mainWithArgs, []testutils.MainTestCase{
		// parsing
		{"no args", nil, "no suitable", assignLogger, nil, nil},
		{"unknown named arg", []string{"--unknown"}, "not defined", assignLogger, nil, nil},
		{"bad device", []string{"--device=foo"}, "format", assignLogger, nil, nil},

		// reading
		{"bad device type", []string{"--device=foo,blah"}, "do not know how", assignLogger, nil, nil},
		{"bad device info", []string{"--device=fail_info,zero"}, "whoops", assignLogger, nil, nil},
		{"bad device ang res", []string{"--device=fail_ang,zero"}, "whoops", assignLogger, nil, nil},
		{"bad device stop", []string{"--device=fail_stop,zero"}, "whoops", assignLogger, nil, nil},
		{"normal", []string{fmt.Sprintf("--device=grpc,%s", listener.Addr())}, "", func(t *testing.T, tLogger golog.Logger, exec *testutils.ContextualMainExecution) {
			assignLogger(t, tLogger, exec)
			exec.ExpectIters(t, 2)
		}, func(ctx context.Context, t *testing.T, exec *testutils.ContextualMainExecution) {
			exec.QuitSignal(t)
			exec.WaitIters(t)
			exec.ExpectIters(t, 12)
			exec.QuitSignal(t)
			exec.WaitIters(t)
		}, func(t *testing.T, logs *observer.ObservedLogs) {
			test.That(t, logs.FilterMessageSnippet("marking").All(), test.ShouldHaveLength, 2)
			test.That(t, logs.FilterMessageSnippet("marked").All(), test.ShouldHaveLength, 2)
			test.That(t, len(logs.FilterMessageSnippet("heading").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
			test.That(t, len(logs.FilterMessageSnippet("median").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
		}},
	})
}
