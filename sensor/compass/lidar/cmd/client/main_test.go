package main

import (
	"context"
	"fmt"
	"math"
	"net"
	"testing"

	"github.com/go-errors/errors"

	"go.viam.com/core/config"
	"go.viam.com/core/grpc/server"
	"go.viam.com/core/lidar"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/robots/fake"
	"go.viam.com/core/testutils"
	"go.viam.com/core/testutils/inject"

	// register
	_ "go.viam.com/core/lidar/client"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r2"
	"go.uber.org/zap/zaptest/observer"
	"go.viam.com/test"
	"google.golang.org/grpc"
)

func TestMainMain(t *testing.T) {
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectRobot := &inject.Robot{}
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))

	injectRobot.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
		return &pb.Status{
			Lidars: map[string]bool{
				"lidar1": true,
			},
		}, nil
	}

	injectDev := &inject.Lidar{}
	injectRobot.LidarByNameFunc = func(name string) lidar.Lidar {
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
	injectDev.CloseFunc = func() error {
		return nil
	}
	injectDev.ScanFunc = func(ctx context.Context, opts lidar.ScanOptions) (lidar.Measurements, error) {
		return lidar.Measurements{lidar.NewMeasurement(0, 5)}, nil
	}
	injectDev.RangeFunc = func(ctx context.Context) (float64, error) {
		return 10, nil
	}
	injectDev.BoundsFunc = func(ctx context.Context) (r2.Point, error) {
		return r2.Point{4, 5}, nil
	}
	injectDev.AngularResolutionFunc = func(ctx context.Context) (float64, error) {
		return 1, nil
	}

	go gServer.Serve(listener)
	defer gServer.Stop()

	assignLogger := func(t *testing.T, tLogger golog.Logger, exec *testutils.ContextualMainExecution) {
		logger = tLogger
	}

	registry.RegisterLidar("fail_info", func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (lidar.Lidar, error) {
		dev := &inject.Lidar{Lidar: fake.NewLidar("")}
		dev.InfoFunc = func(ctx context.Context) (map[string]interface{}, error) {
			return nil, errors.New("whoops")
		}
		return dev, nil
	})
	registry.RegisterLidar("fail_ang", func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (lidar.Lidar, error) {
		dev := &inject.Lidar{Lidar: fake.NewLidar("")}
		dev.AngularResolutionFunc = func(ctx context.Context) (float64, error) {
			return math.NaN(), errors.New("whoops")
		}
		return dev, nil
	})
	registry.RegisterLidar("fail_stop", func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (lidar.Lidar, error) {
		dev := &inject.Lidar{Lidar: fake.NewLidar("")}
		dev.StopFunc = func(ctx context.Context) error {
			return errors.New("whoops")
		}
		return dev, nil
	})
	registry.RegisterLidar("fail_scan", func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (lidar.Lidar, error) {
		dev := &inject.Lidar{Lidar: fake.NewLidar("")}
		var once bool
		dev.ScanFunc = func(ctx context.Context, options lidar.ScanOptions) (lidar.Measurements, error) {
			if once {
				return nil, errors.New("whoops")
			}
			once = true
			return lidar.Measurements{}, nil
		}
		return dev, nil
	})
	host := listener.Addr().(*net.TCPAddr).IP.String()
	port := listener.Addr().(*net.TCPAddr).Port
	testutils.TestMain(t, mainWithArgs, []testutils.MainTestCase{
		// parsing
		{"no args", nil, "no suitable", assignLogger, nil, nil},
		{"unknown named arg", []string{"--unknown"}, "not defined", assignLogger, nil, nil},
		{"bad device", []string{"--device=foo"}, "format", assignLogger, nil, nil},

		// reading
		{"bad device type", []string{"--device=type=lidar,model=foo,host=blah"}, "unknown lidar model", assignLogger, nil, nil},
		{"bad device info", []string{"--device=type=lidar,model=fail_info,host=zero"}, "whoops", assignLogger, nil, nil},
		{"bad device ang res", []string{"--device=type=lidar,model=fail_ang,host=zero"}, "whoops", assignLogger, nil, nil},
		{"bad device stop", []string{"--device=type=lidar,model=fail_stop,host=zero"}, "whoops", assignLogger, nil, nil},
		{"normal", []string{fmt.Sprintf("--device=type=lidar,model=grpc,host=%s,port=%d", host, port)}, "", func(t *testing.T, tLogger golog.Logger, exec *testutils.ContextualMainExecution) {
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
