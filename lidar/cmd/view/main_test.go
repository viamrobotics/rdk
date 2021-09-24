package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"testing"

	"github.com/go-errors/errors"

	"go.uber.org/zap/zaptest/observer"

	"go.viam.com/utils"
	"go.viam.com/utils/testutils"

	"go.viam.com/core/config"
	"go.viam.com/core/lidar"
	"go.viam.com/core/pointcloud"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/robots/fake"
	"go.viam.com/core/testutils/inject"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r2"
	"go.viam.com/test"
)

func TestMainMain(t *testing.T) {
	registry.RegisterLidar("fail_info", registry.Lidar{Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (lidar.Lidar, error) {
		dev := &inject.Lidar{Lidar: fake.NewLidar("")}
		dev.InfoFunc = func(ctx context.Context) (map[string]interface{}, error) {
			return nil, errors.New("whoops")
		}
		return dev, nil
	}})
	registry.RegisterLidar("fail_width", registry.Lidar{Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (lidar.Lidar, error) {
		dev := &inject.Lidar{Lidar: fake.NewLidar("")}
		dev.BoundsFunc = func(ctx context.Context) (r2.Point, error) {
			return r2.Point{}, errors.New("whoops")
		}
		return dev, nil
	}})
	registry.RegisterLidar("fail_ang", registry.Lidar{Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (lidar.Lidar, error) {
		dev := &inject.Lidar{Lidar: fake.NewLidar("")}
		dev.AngularResolutionFunc = func(ctx context.Context) (float64, error) {
			return math.NaN(), errors.New("whoops")
		}
		return dev, nil
	}})
	registry.RegisterLidar("fail_stop", registry.Lidar{Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (lidar.Lidar, error) {
		dev := &inject.Lidar{Lidar: fake.NewLidar("")}
		dev.StopFunc = func(ctx context.Context) error {
			return errors.New("whoops")
		}
		return dev, nil
	}})
	registry.RegisterLidar("fail_scan", registry.Lidar{Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (lidar.Lidar, error) {
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
	}})

	temp, err := ioutil.TempFile("", "*.las")
	test.That(t, err, test.ShouldBeNil)
	defer os.Remove(temp.Name())

	prevPort := defaultPort
	defer func() {
		defaultPort = prevPort
	}()

	before := func(t *testing.T, tLogger golog.Logger, exec *testutils.ContextualMainExecution) {
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
		test.That(t, resp.Body.Close(), test.ShouldBeNil)
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
		{"bad device type", []string{"--device=type=lidar,model=foo,host=blah"}, "unknown lidar model", before, nil, nil},
		{"bad device info", []string{"--device=type=lidar,model=fail_info,host=zero"}, "whoops", before, nil, nil},
		{"bad device width", []string{"--device=type=lidar,model=fail_width,host=zero", "--save=somewhere"}, "whoops", before, nil, nil},
		{"bad device ang res", []string{"--device=type=lidar,model=fail_ang,host=zero"}, "whoops", before, nil, nil},
		{"bad device stop", []string{"--device=type=lidar,model=fail_stop,host=zero"}, "whoops", before, nil, nil},
		{"bad save path", []string{"--save=/"}, "is a directory", before, nil, nil},
		{"heading", nil, "", func(t *testing.T, tLogger golog.Logger, exec *testutils.ContextualMainExecution) {
			before(t, tLogger, exec)
			exec.ExpectIters(t, 2)
		}, func(ctx context.Context, t *testing.T, exec *testutils.ContextualMainExecution) {
			exec.QuitSignal(t)
			exec.WaitIters(t)
			exec.QuitSignal(t)
			testPort(t)
		}, func(t *testing.T, logs *observer.ObservedLogs) {
			test.That(t, logs.FilterMessageSnippet("marking").All(), test.ShouldHaveLength, 2)
			test.That(t, logs.FilterMessageSnippet("marked").All(), test.ShouldHaveLength, 2)
			test.That(t, len(logs.FilterMessageSnippet("heading").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
		}},
		{"heading fail", []string{"--device=type=lidar,model=fail_scan,host=zero"}, "", func(t *testing.T, tLogger golog.Logger, exec *testutils.ContextualMainExecution) {
			before(t, tLogger, exec)
			exec.ExpectIters(t, 2)
		}, func(ctx context.Context, t *testing.T, exec *testutils.ContextualMainExecution) {
			exec.QuitSignal(t)
			exec.WaitIters(t)
			exec.QuitSignal(t)
			testPort(t)
		}, func(t *testing.T, logs *observer.ObservedLogs) {
			test.That(t, len(logs.FilterMessageSnippet("failed").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
			test.That(t, len(logs.FilterMessageSnippet("error marking").All()), test.ShouldBeGreaterThanOrEqualTo, 1)
		}},
		{"saving", []string{"--save=" + temp.Name()}, "", func(t *testing.T, tLogger golog.Logger, exec *testutils.ContextualMainExecution) {
			before(t, tLogger, exec)
			exec.ExpectIters(t, 2)
		}, func(ctx context.Context, t *testing.T, exec *testutils.ContextualMainExecution) {
			exec.WaitIters(t)
			testPort(t)
		}, func(t *testing.T, logs *observer.ObservedLogs) {
			logger := golog.NewTestLogger(t)
			pc, err := pointcloud.NewFromFile(temp.Name(), logger)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, pc.Size(), test.ShouldNotBeZeroValue)
		}},
	})
}
