package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"testing"
	"time"

	"go.uber.org/zap/zaptest/observer"
	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/robots/fake"
	"go.viam.com/robotcore/testutils"
	"go.viam.com/robotcore/testutils/inject"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
	"github.com/edaniels/wsapi"
)

func TestMain(t *testing.T) {
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	port := listener.Addr().(*net.TCPAddr).Port
	httpServer := &http.Server{
		Addr:           listener.Addr().String(),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	wsServer := wsapi.NewServer()
	wsServer.RegisterCommand(lidar.WSCommandStart, wsapi.CommandHandlerFunc(func(ctx context.Context, cmd *wsapi.Command) (interface{}, error) {
		return nil, nil
	}))
	wsServer.RegisterCommand(lidar.WSCommandInfo, wsapi.CommandHandlerFunc(func(ctx context.Context, cmd *wsapi.Command) (interface{}, error) {
		return map[string]interface{}{"hello": "world"}, nil
	}))
	wsServer.RegisterCommand(lidar.WSCommandAngularResolution, wsapi.CommandHandlerFunc(func(ctx context.Context, cmd *wsapi.Command) (interface{}, error) {
		return 1, nil
	}))
	wsServer.RegisterCommand(lidar.WSCommandScan, wsapi.CommandHandlerFunc(func(ctx context.Context, cmd *wsapi.Command) (interface{}, error) {
		return lidar.Measurements{lidar.NewMeasurement(0, 5)}, nil
	}))
	wsServer.RegisterCommand(lidar.WSCommandStop, wsapi.CommandHandlerFunc(func(ctx context.Context, cmd *wsapi.Command) (interface{}, error) {
		return nil, nil
	}))
	httpServer.Handler = wsServer.HTTPHandler()
	go func() {
		httpServer.Serve(listener)
	}()
	defer httpServer.Close()

	assignLogger := func(t *testing.T, tLogger golog.Logger, exec *testutils.ContextualMainExecution) {
		logger = tLogger
		wsServer.SetLogger(logger)
	}

	lidar.RegisterDeviceType("fail_info", lidar.DeviceTypeRegistration{
		New: func(ctx context.Context, desc lidar.DeviceDescription) (lidar.Device, error) {
			dev := &inject.LidarDevice{Device: &fake.Lidar{}}
			dev.InfoFunc = func(ctx context.Context) (map[string]interface{}, error) {
				return nil, errors.New("whoops")
			}
			return dev, nil
		},
	})
	lidar.RegisterDeviceType("fail_ang", lidar.DeviceTypeRegistration{
		New: func(ctx context.Context, desc lidar.DeviceDescription) (lidar.Device, error) {
			dev := &inject.LidarDevice{Device: &fake.Lidar{}}
			dev.AngularResolutionFunc = func(ctx context.Context) (float64, error) {
				return math.NaN(), errors.New("whoops")
			}
			return dev, nil
		},
	})
	lidar.RegisterDeviceType("fail_stop", lidar.DeviceTypeRegistration{
		New: func(ctx context.Context, desc lidar.DeviceDescription) (lidar.Device, error) {
			dev := &inject.LidarDevice{Device: &fake.Lidar{}}
			dev.StopFunc = func(ctx context.Context) error {
				return errors.New("whoops")
			}
			return dev, nil
		},
	})
	lidar.RegisterDeviceType("fail_scan", lidar.DeviceTypeRegistration{
		New: func(ctx context.Context, desc lidar.DeviceDescription) (lidar.Device, error) {
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
		{"normal", []string{fmt.Sprintf("--device=lidarws,ws://127.0.0.1:%d", port)}, "", func(t *testing.T, tLogger golog.Logger, exec *testutils.ContextualMainExecution) {
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
