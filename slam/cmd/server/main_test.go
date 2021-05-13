package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"testing"

	"go.viam.com/core/config"
	"go.viam.com/core/grpc/server"
	"go.viam.com/core/lidar"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/robots/fake"
	"go.viam.com/core/sensor"
	"go.viam.com/core/sensor/compass"
	"go.viam.com/core/testutils"
	"go.viam.com/core/testutils/inject"
	"go.viam.com/core/utils"

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
			Sensors: map[string]*pb.SensorStatus{
				"compass1": {
					Type: compass.RelativeCompassType,
				},
			},
		}, nil
	}

	go gServer.Serve(listener)
	defer gServer.Stop()

	injectDev := &inject.Compass{}
	injectDev.HeadingFunc = func(ctx context.Context) (float64, error) {
		return 23.45, nil
	}
	injectRobot.SensorByNameFunc = func(name string) sensor.Sensor {
		return injectDev
	}

	go gServer.Serve(listener)
	defer gServer.Stop()

	registry.RegisterLidar("fail_info", func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (lidar.Lidar, error) {
		dev := &inject.Lidar{Lidar: &fake.Lidar{}}
		dev.InfoFunc = func(ctx context.Context) (map[string]interface{}, error) {
			return nil, errors.New("whoops")
		}
		return dev, nil
	})
	registry.RegisterLidar("fail_width", func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (lidar.Lidar, error) {
		dev := &inject.Lidar{Lidar: &fake.Lidar{}}
		dev.BoundsFunc = func(ctx context.Context) (r2.Point, error) {
			return r2.Point{}, errors.New("whoops")
		}
		return dev, nil
	})
	registry.RegisterLidar("fail_ang", func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (lidar.Lidar, error) {
		dev := &inject.Lidar{Lidar: &fake.Lidar{}}
		dev.AngularResolutionFunc = func(ctx context.Context) (float64, error) {
			return math.NaN(), errors.New("whoops")
		}
		return dev, nil
	})
	registry.RegisterLidar("fail_stop", func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (lidar.Lidar, error) {
		dev := &inject.Lidar{Lidar: &fake.Lidar{}}
		dev.StopFunc = func(ctx context.Context) error {
			return errors.New("whoops")
		}
		return dev, nil
	})

	before := func(t *testing.T, tLogger golog.Logger, exec *testutils.ContextualMainExecution) {
		logger = tLogger
		randomPort, err := utils.TryReserveRandomPort()
		test.That(t, err, test.ShouldBeNil)
		defaultPort = randomPort
	}
	host := listener.Addr().(*net.TCPAddr).IP.String()
	port := listener.Addr().(*net.TCPAddr).Port
	testutils.TestMain(t, mainWithArgs, []testutils.MainTestCase{
		// parsing
		{"no args", nil, "", before, nil, func(t *testing.T, logs *observer.ObservedLogs) {
			test.That(t, logs.FilterMessageSnippet(`fake`).All(), test.ShouldHaveLength, 1)
			test.That(t, logs.FilterMessageSnippet(`relative compass`).All(), test.ShouldHaveLength, 1)
		}},
		{"bad port", []string{"ten"}, "invalid syntax", before, nil, nil},
		{"too big port", []string{"65536"}, "out of range", before, nil, nil},
		{"unknown named arg", []string{"--unknown"}, "not defined", before, nil, nil},
		{"bad lidar", []string{"--lidar=foo"}, "format", before, nil, nil},
		{"bad offset", []string{"--lidar-offset=foo"}, "format", before, nil, nil},
		{"lidar and offset mismatch", []string{"--lidar=type=lidar,model=fake,host=one", "--lidar=type=lidar,model=fake,host=two", "--lidar-offset=1,2,3", "--lidar-offset=1,2,4"}, "have up to", before, nil, nil},

		// running
		{"bad lidar type", []string{"--lidar=type=lidar,model=foo,host=blah"}, "unknown lidar model", before, nil, nil},
		{"bad lidar info", []string{"--lidar=type=lidar,model=fail_info,host=zero"}, "whoops", before, nil, nil},
		{"bad lidar width", []string{"--lidar=type=lidar,model=fail_width,host=zero"}, "whoops", before, nil, nil},
		{"bad lidar ang res", []string{"--lidar=type=lidar,model=fail_ang,host=zero"}, "whoops", before, nil, nil},
		{"bad lidar stop", []string{"--lidar=type=lidar,model=fail_stop,host=zero"}, "whoops", before, nil, nil},
		{"normal", []string{"--lidar=type=lidar,model=fake,host=1"}, "", before, nil, nil},
		{"normal with compass", []string{fmt.Sprintf("--compass=type=sensor,subtype=compass,model=grpc,host=%s,port=%d", host, port)}, "", before, nil, nil},
	})
}
