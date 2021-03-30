package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"testing"

	pb "go.viam.com/robotcore/proto/sensor/compass/v1"
	"go.viam.com/robotcore/sensor/compass"
	"go.viam.com/robotcore/testutils"
	"go.viam.com/robotcore/testutils/inject"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc"
)

func TestMain(t *testing.T) {
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	listener2, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer1 := grpc.NewServer()
	gServer2 := grpc.NewServer()
	injectDev1 := &inject.Compass{}
	injectDev2 := &inject.Compass{}
	pb.RegisterCompassServiceServer(gServer1, compass.NewServer(injectDev1))
	pb.RegisterCompassServiceServer(gServer2, compass.NewServer(injectDev2))

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
