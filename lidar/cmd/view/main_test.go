package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"math"
	"net/http"
	"testing"

	"go.uber.org/zap/zaptest/observer"
	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/robots/fake"
	"go.viam.com/robotcore/testutils"
	"go.viam.com/robotcore/testutils/inject"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
)

func TestMain(t *testing.T) {
	randomPort, err := utils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	defaultPort = randomPort

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

	for _, tc := range []struct {
		Name         string
		Args         []string
		ExpectedPort int
		Err          string
		During       func(exec *testutils.ContextualMainExecution)
		After        func(t *testing.T, logs *observer.ObservedLogs)
	}{
		// parsing
		// {"no args", nil, defaultPort, "", nil, nil},
		{"bad port", []string{"ten"}, 0, "invalid syntax", nil, nil},
		{"too big port", []string{"65536"}, 0, "out of range", nil, nil},
		{"unknown named arg", []string{"--unknown"}, 0, "not defined", nil, nil},
		{"bad device", []string{"--device=foo"}, 0, "format", nil, nil},

		// viewing
		{"bad device", []string{"--device=foo,blah"}, 0, "do not know how", nil, nil},
		{"bad device info", []string{"--device=fail_info,zero"}, 0, "whoops", nil, nil},
		{"bad device width", []string{"--device=fail_width,zero", "--save=somewhere"}, 0, "whoops", nil, nil},
		{"bad device ang res", []string{"--device=fail_ang,zero"}, 0, "whoops", nil, nil},
		{"bad device stop", []string{"--device=fail_stop,zero"}, 0, "whoops", nil, nil},
		// {"bad save path", []string{"--save=/"}, 0, "is a directory", nil, nil},
		// {"heading", nil, defaultPort, "", func(exec *testutils.ContextualMainExecution) {
		// 	exec.QuitSignal()
		// 	exec.QuitSignal()
		// }, func(t *testing.T, logs *observer.ObservedLogs) {
		// 	fmt.Println(logs.All())
		// }},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			var logs *observer.ObservedLogs
			logger, logs = golog.NewObservedTestLogger(t)
			exec := testutils.ContextualMain(mainWithArgs, tc.Args)
			<-exec.Ready

			if tc.During != nil {
				tc.During(&exec)
			}
			if tc.Err == "" {
				req, err := http.NewRequest("GET", fmt.Sprintf("http://localhost:%d", tc.ExpectedPort), nil)
				test.That(t, err, test.ShouldBeNil)
				resp, err := http.DefaultClient.Do(req)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, resp.StatusCode, test.ShouldEqual, http.StatusOK)
			}
			exec.Stop()
			err = <-exec.Done
			if tc.Err == "" {
				test.That(t, err, test.ShouldBeNil)
				return
			}
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, tc.Err)
			if tc.After != nil {
				tc.After(t, logs)
			}
		})
	}
}
