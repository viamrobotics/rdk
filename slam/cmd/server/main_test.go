package main

import (
	"context"
	"errors"
	"image"
	"math"
	"net"
	"testing"

	"go.viam.com/robotcore/lidar"
	pb "go.viam.com/robotcore/proto/sensor/compass/v1"
	"go.viam.com/robotcore/robots/fake"
	"go.viam.com/robotcore/sensor/compass"
	"go.viam.com/robotcore/testutils"
	"go.viam.com/robotcore/testutils/inject"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc"
)

func TestMain(t *testing.T) {
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectDev := &inject.Compass{}
	pb.RegisterCompassServiceServer(gServer, compass.NewServer(injectDev))

	injectDev.HeadingFunc = func(ctx context.Context) (float64, error) {
		return 23.45, nil
	}

	go gServer.Serve(listener)
	defer gServer.Stop()

	lidar.RegisterDeviceType("fail_info", lidar.DeviceTypeRegistration{
		New: func(ctx context.Context, desc lidar.DeviceDescription, logger golog.Logger) (lidar.Device, error) {
			dev := &inject.LidarDevice{Device: &fake.Lidar{}}
			dev.InfoFunc = func(ctx context.Context) (map[string]interface{}, error) {
				return nil, errors.New("whoops")
			}
			return dev, nil
		},
	})
	lidar.RegisterDeviceType("fail_width", lidar.DeviceTypeRegistration{
		New: func(ctx context.Context, desc lidar.DeviceDescription, logger golog.Logger) (lidar.Device, error) {
			dev := &inject.LidarDevice{Device: &fake.Lidar{}}
			dev.BoundsFunc = func(ctx context.Context) (image.Point, error) {
				return image.Point{}, errors.New("whoops")
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

	before := func(t *testing.T, tLogger golog.Logger, exec *testutils.ContextualMainExecution) {
		logger = tLogger
		randomPort, err := utils.TryReserveRandomPort()
		test.That(t, err, test.ShouldBeNil)
		defaultPort = randomPort
	}
	testutils.TestMain(t, mainWithArgs, []testutils.MainTestCase{
		// parsing
		{"no args", nil, "", before, nil, func(t *testing.T, logs *observer.ObservedLogs) {
			test.That(t, logs.FilterMessageSnippet(`lidar "0"`).All(), test.ShouldHaveLength, 1)
		}},
		{"bad port", []string{"ten"}, "invalid syntax", before, nil, nil},
		{"too big port", []string{"65536"}, "out of range", before, nil, nil},
		{"unknown named arg", []string{"--unknown"}, "not defined", before, nil, nil},
		{"bad lidar", []string{"--lidar=foo"}, "format", before, nil, nil},
		{"bad offset", []string{"--lidar-offset=foo"}, "format", before, nil, nil},
		{"lidar and offset mismatch", []string{"--lidar=fake,one", "--lidar=fake,two", "--lidar-offset=1,2,3", "--lidar-offset=1,2,4"}, "have up to", before, nil, nil},

		// running
		{"bad lidar device type", []string{"--lidar=foo,blah"}, "do not know how", before, nil, nil},
		{"bad lidar device info", []string{"--lidar=fail_info,zero"}, "whoops", before, nil, nil},
		{"bad lidar device width", []string{"--lidar=fail_width,zero"}, "whoops", before, nil, nil},
		{"bad lidar device ang res", []string{"--lidar=fail_ang,zero"}, "whoops", before, nil, nil},
		{"bad lidar device stop", []string{"--lidar=fail_stop,zero"}, "whoops", before, nil, nil},
		{"normal", []string{"--lidar=fake,1"}, "", before, nil, nil},
		{"normal with compass", []string{"--compass=" + listener.Addr().String()}, "", before, nil, nil},
	})
}
