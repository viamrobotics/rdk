package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"testing"
	"time"

	"go.uber.org/zap/zaptest/observer"
	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/pointcloud"
	"go.viam.com/robotcore/robots/fake"
	"go.viam.com/robotcore/testutils"
	"go.viam.com/robotcore/testutils/inject"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
)

func TestMain(t *testing.T) {
	lidar.RegisterDeviceType("fail_info", lidar.DeviceTypeRegistration{
		New: func(ctx context.Context, desc lidar.DeviceDescription) (lidar.Device, error) {
			dev := &inject.LidarDevice{Device: &fake.Lidar{}}
			dev.InfoFunc = func(ctx context.Context) (map[string]interface{}, error) {
				return nil, errors.New("whoops")
			}
			return dev, nil
		},
	})
	lidar.RegisterDeviceType("fail_width", lidar.DeviceTypeRegistration{
		New: func(ctx context.Context, desc lidar.DeviceDescription) (lidar.Device, error) {
			dev := &inject.LidarDevice{Device: &fake.Lidar{}}
			dev.BoundsFunc = func(ctx context.Context) (image.Point, error) {
				return image.Point{}, errors.New("whoops")
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

	temp, err := ioutil.TempFile("", "*.las")
	test.That(t, err, test.ShouldBeNil)
	defer os.Remove(temp.Name())

	before := func(tLogger golog.Logger) {
		logger = tLogger
		randomPort, err := utils.TryReserveRandomPort()
		test.That(t, err, test.ShouldBeNil)
		defaultPort = randomPort
	}
	testPort := func(t *testing.T) {
		hostPort := fmt.Sprintf("localhost:%d", defaultPort)
		test.That(t, testutils.WaitSuccessfulDial(hostPort), test.ShouldBeNil)
		req, err := http.NewRequest("GET", "http://"+hostPort, nil)
		test.That(t, err, test.ShouldBeNil)
		resp, err := http.DefaultClient.Do(req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.StatusCode, test.ShouldEqual, http.StatusOK)
	}
	testutils.TestMain(t, mainWithArgs, []testutils.MainTestCase{
		// parsing
		{"no args", nil, "", before, nil, nil},
		{"bad port", []string{"ten"}, "invalid syntax", before, nil, nil},
		{"too big port", []string{"65536"}, "out of range", before, nil, nil},
		{"unknown named arg", []string{"--unknown"}, "not defined", before, nil, nil},
		{"bad device", []string{"--device=foo"}, "format", before, nil, nil},

		// viewing
		{"bad device type", []string{"--device=foo,blah"}, "do not know how", before, nil, nil},
		{"bad device info", []string{"--device=fail_info,zero"}, "whoops", before, nil, nil},
		{"bad device width", []string{"--device=fail_width,zero", "--save=somewhere"}, "whoops", before, nil, nil},
		{"bad device ang res", []string{"--device=fail_ang,zero"}, "whoops", before, nil, nil},
		{"bad device stop", []string{"--device=fail_stop,zero"}, "whoops", before, nil, nil},
		{"bad save path", []string{"--save=/"}, "is a directory", before, nil, nil},
		{"heading", nil, "", before, func(exec *testutils.ContextualMainExecution) {
			exec.QuitSignal()
			time.Sleep(2 * time.Second)
			exec.QuitSignal()
			testPort(t)
		}, func(t *testing.T, logs *observer.ObservedLogs) {
			test.That(t, logs.FilterMessageSnippet("marking").All(), test.ShouldHaveLength, 2)
			test.That(t, logs.FilterMessageSnippet("marked").All(), test.ShouldHaveLength, 2)
			test.That(t, len(logs.FilterMessageSnippet("heading").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
		}},
		{"heading fail", []string{"--device=fail_scan,zero"}, "", before, func(exec *testutils.ContextualMainExecution) {
			exec.QuitSignal()
			time.Sleep(2 * time.Second)
			exec.QuitSignal()
			testPort(t)
		}, func(t *testing.T, logs *observer.ObservedLogs) {
			test.That(t, len(logs.FilterMessageSnippet("failed").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
			test.That(t, len(logs.FilterMessageSnippet("error marking").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
		}},
		{"saving", []string{"--save=" + temp.Name()}, "", before, func(exec *testutils.ContextualMainExecution) {
			testPort(t)
		}, func(t *testing.T, logs *observer.ObservedLogs) {
			pc, err := pointcloud.NewFromFile(temp.Name())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, pc.Size(), test.ShouldNotBeZeroValue)
		}},
	})
}
