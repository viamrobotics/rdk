package main

import (
	"context"
	"fmt"
	"math"
	"net"
	"testing"

	"github.com/go-errors/errors"

	"go.viam.com/core/grpc/server"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/sensor"
	"go.viam.com/core/sensor/compass"
	"go.viam.com/core/testutils"
	"go.viam.com/core/testutils/inject"

	"github.com/edaniels/golog"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"go.viam.com/test"
	"google.golang.org/grpc"
)

func TestMainMain(t *testing.T) {
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
		return &pb.Status{
			Sensors: map[string]*pb.SensorStatus{
				"sensor1": {
					Type: compass.Type,
				},
			},
		}, nil
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

	injectDev1 := &inject.Compass{}
	injectDev2 := &inject.Compass{}
	injectRobot1.SensorByNameFunc = func(name string) sensor.Sensor {
		return injectDev1
	}
	injectRobot2.SensorByNameFunc = func(name string) sensor.Sensor {
		return injectDev2
	}

	go gServer1.Serve(listener1)
	defer gServer1.Stop()
	go gServer2.Serve(listener2)
	defer gServer2.Stop()

	injectDev1.HeadingFunc = func(ctx context.Context) (float64, error) {
		return math.NaN(), errors.New("whoops")
	}
	injectDev2.HeadingFunc = func(ctx context.Context) (float64, error) {
		return 23.45, nil
	}

	go gServer1.Serve(listener1)
	defer gServer1.Stop()
	go gServer2.Serve(listener2)
	defer gServer2.Stop()

	assignLogger := func(t *testing.T, tLogger golog.Logger, exec *testutils.ContextualMainExecution) {
		logger = tLogger
	}
	testutils.TestMain(t, mainWithArgs, []testutils.MainTestCase{
		// parsing
		{"no args", nil, "Usage of", assignLogger, nil, nil},
		{"unknown named arg", []string{"--unknown"}, "not defined", assignLogger, nil, nil},

		// reading
		{"bad heading", []string{fmt.Sprintf("--device=%s", listener1.Addr().String())}, "", assignLogger, nil, func(t *testing.T, logs *observer.ObservedLogs) {
			test.That(t, len(logs.FilterMessageSnippet("failed").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
			test.That(t, len(logs.FilterMessageSnippet("stats").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
		}},
		{"normal heading", []string{fmt.Sprintf("--device=%s", listener2.Addr().String())}, "", assignLogger, nil, func(t *testing.T, logs *observer.ObservedLogs) {
			test.That(t, len(logs.FilterMessageSnippet("heading").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
			test.That(t, len(logs.FilterField(zap.Float64("data", 23.45)).All()), test.ShouldBeGreaterThanOrEqualTo, 1)
			test.That(t, len(logs.FilterMessageSnippet("stats").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
		}},
	})
}
