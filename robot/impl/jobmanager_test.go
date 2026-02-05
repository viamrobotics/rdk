package robotimpl

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap/zapcore"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/audioin"
	"go.viam.com/rdk/components/audioout"
	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/button"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/components/gantry"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/components/input"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/components/posetracker"
	"go.viam.com/rdk/components/powersensor"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/components/servo"
	sw "go.viam.com/rdk/components/switch"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/ml"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	rclient "go.viam.com/rdk/robot/client"
	weboptions "go.viam.com/rdk/robot/web/options"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/services/discovery"
	genSvc "go.viam.com/rdk/services/generic"
	"go.viam.com/rdk/services/mlmodel"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/navigation"
	_ "go.viam.com/rdk/services/register"
	"go.viam.com/rdk/services/shell"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	injectmotion "go.viam.com/rdk/testutils/inject/motion"
	"go.viam.com/rdk/testutils/robottestutils"
	rutils "go.viam.com/rdk/utils"
)

func TestJobManagerDurationAndCronFromJson(t *testing.T) {
	t.Parallel()
	logger := logging.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake_jobs.json", logger, nil)
	test.That(t, err, test.ShouldBeNil)
	logger, logs := logging.NewObservedTestLogger(t)
	setupLocalRobot(t, context.Background(), cfg, logger)

	testutils.WaitForAssertionWithSleep(t, time.Second, 7, func(tb testing.TB) {
		tb.Helper()
		// the jobs in the config are on 4-5s schedules so after 6-7 seconds, all of them
		// should run at least once
		test.That(tb, logs.FilterMessage("Job triggered").Len(),
			test.ShouldBeGreaterThanOrEqualTo, 3)
		test.That(tb, logs.FilterMessage("Job succeeded").Len(),
			test.ShouldBeGreaterThanOrEqualTo, 3)
	})
}

func TestLogLevelChange(t *testing.T) {
	t.Parallel()
	// This is created at debug level
	logger, logs := logging.NewObservedTestLogger(t)

	fakeSensorComponent := []resource.Config{
		{
			Model: resource.DefaultModelFamily.WithModel("fake"),
			Name:  "sensor",
			API:   sensor.API,
		},
	}

	cfg := &config.Config{
		Components: fakeSensorComponent,
		Jobs: []config.JobConfig{
			{
				config.JobConfigData{
					Name:     "fake sensor",
					Schedule: "1s",
					Resource: "sensor",
					Method:   "GetReadings",
				},
			},
		},
	}
	cfgWarn := &config.Config{
		Components: fakeSensorComponent,
		Jobs: []config.JobConfig{
			{
				config.JobConfigData{
					Name:     "fake sensor",
					Schedule: "1s",
					Resource: "sensor",
					Method:   "GetReadings",
					LogConfiguration: &resource.LogConfig{
						Level: logging.WARN,
					},
				},
			},
		},
	}
	cfgDebug := &config.Config{
		Components: fakeSensorComponent,
		Jobs: []config.JobConfig{
			{
				config.JobConfigData{
					Name:     "fake sensor",
					Schedule: "1s",
					Resource: "sensor",
					Method:   "GetReadings",
					LogConfiguration: &resource.LogConfig{
						Level: logging.DEBUG,
					},
				},
			},
		},
	}
	ctx := context.Background()
	lr := setupLocalRobot(t, ctx, cfg, logger)

	time.Sleep(7 * time.Second)
	test.That(t, logs.FilterMessage("Job triggered").FilterLevelExact(zapcore.DebugLevel).Len(),
		test.ShouldBeGreaterThan, 2)
	test.That(t, logs.FilterMessage("Job succeeded").FilterLevelExact(zapcore.DebugLevel).Len(),
		test.ShouldBeGreaterThan, 2)

	lr.Reconfigure(ctx, cfgWarn)
	logs.TakeAll()
	time.Sleep(7 * time.Second)
	// update will let the previous job iteration complete first, so may be 1 or 0.
	test.That(t, logs.FilterMessage("Job triggered").FilterLevelExact(zapcore.DebugLevel).Len(),
		test.ShouldBeLessThanOrEqualTo, 1)
	test.That(t, logs.FilterMessage("Job succeeded").FilterLevelExact(zapcore.DebugLevel).Len(),
		test.ShouldBeLessThanOrEqualTo, 1)

	lr.Reconfigure(ctx, cfgDebug)
	logs.TakeAll()
	time.Sleep(7 * time.Second)
	test.That(t, logs.FilterMessage("Job triggered").FilterLevelExact(zapcore.DebugLevel).Len(),
		test.ShouldBeGreaterThan, 2)
	test.That(t, logs.FilterMessage("Job succeeded").FilterLevelExact(zapcore.DebugLevel).Len(),
		test.ShouldBeGreaterThan, 2)
}

func TestJobManagerHistory(t *testing.T) {
	logger := logging.NewTestLogger(t)

	fakeSensorModel := resource.DefaultModelFamily.WithModel("fakesensor")
	injectSensor := inject.NewSensor("fakesensor")
	injectSensor.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
		return map[string]interface{}{"a": 1, "b": 2, "c": 3}, nil
	}
	injectSensor.DoFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
		if cmd["command"] == "pass" {
			return nil, nil
		}
		return nil, errors.New("fail")
	}
	fakeSensorModelPanic := resource.DefaultModelFamily.WithModel("fakesensorPanic")
	injectSensorPanic := inject.NewSensor("fakesensorPanic")
	injectSensorPanic.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
		panic("panic")
	}
	injectSensorPanic.DoFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
		return nil, nil
	}
	resource.RegisterComponent(
		sensor.API,
		fakeSensorModel,
		resource.Registration[sensor.Sensor, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (sensor.Sensor, error) {
			return injectSensor, nil
		}})
	resource.RegisterComponent(
		sensor.API,
		fakeSensorModelPanic,
		resource.Registration[sensor.Sensor, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (sensor.Sensor, error) {
			return injectSensorPanic, nil
		}})

	// test GetReadings success, DoCommand fail
	cfg := &config.Config{
		Components: []resource.Config{
			{
				Model: fakeSensorModel,
				Name:  "sensor",
				API:   sensor.API,
			},
		},
		Jobs: []config.JobConfig{
			{
				config.JobConfigData{
					Name:     "fake sensor",
					Schedule: "200ms",
					Resource: "sensor",
					Method:   "GetReadings",
				},
			},
			{
				config.JobConfigData{
					Name:     "test DoCommand",
					Schedule: "200ms",
					Resource: "sensor",
					Method:   "DoCommand",
					Command: map[string]any{
						"command": "fail",
					},
				},
			},
		},
	}
	// switch to test GetReadings Panic, DoCommand success
	cfgSwitch := &config.Config{
		Components: []resource.Config{
			{
				Model: fakeSensorModelPanic,
				Name:  "sensor",
				API:   sensor.API,
			},
		},
		Jobs: []config.JobConfig{
			{
				config.JobConfigData{
					Name:     "fake sensor",
					Schedule: "200ms",
					Resource: "sensor",
					Method:   "GetReadings",
				},
			},
			{
				config.JobConfigData{
					Name:     "test DoCommand",
					Schedule: "200ms",
					Resource: "sensor",
					Method:   "DoCommand",
					Command: map[string]any{
						"command": "pass",
					},
				},
			},
		},
	}

	ctx := context.Background()
	lr := setupLocalRobot(t, ctx, cfg, logger)
	o, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	err := lr.StartWeb(ctx, o)
	test.That(t, err, test.ShouldBeNil)
	robotClient, err := rclient.New(ctx, addr, logger)
	test.That(t, err, test.ShouldBeNil)
	defer robotClient.Close(ctx)

	//nolint:dupl
	testutils.WaitForAssertionWithSleep(t, time.Second, 5, func(tb testing.TB) {
		tb.Helper()
		ms, err := robotClient.MachineStatus(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, len(ms.JobStatuses), test.ShouldEqual, 2)

		successJob, ok := ms.JobStatuses["fake sensor"]
		test.That(tb, ok, test.ShouldBeTrue)
		test.That(tb, len(successJob.RecentSuccessfulRuns), test.ShouldBeGreaterThan, 0)
		test.That(tb, len(successJob.RecentFailedRuns), test.ShouldEqual, 0)

		failJob, ok := ms.JobStatuses["test DoCommand"]
		test.That(tb, ok, test.ShouldBeTrue)
		test.That(tb, len(failJob.RecentSuccessfulRuns), test.ShouldEqual, 0)
		test.That(tb, len(failJob.RecentFailedRuns), test.ShouldBeGreaterThan, 0)
	})

	lr.Reconfigure(ctx, cfgSwitch)
	//nolint:dupl
	testutils.WaitForAssertionWithSleep(t, time.Second, 5, func(tb testing.TB) {
		tb.Helper()
		ms, err := robotClient.MachineStatus(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, len(ms.JobStatuses), test.ShouldEqual, 2)

		// both ShouldBeGreaterThan because history from previous run remains
		panicJob, ok := ms.JobStatuses["fake sensor"]
		test.That(tb, ok, test.ShouldBeTrue)
		test.That(tb, len(panicJob.RecentSuccessfulRuns), test.ShouldBeGreaterThan, 0)
		test.That(tb, len(panicJob.RecentFailedRuns), test.ShouldBeGreaterThan, 0)

		successJob, ok := ms.JobStatuses["test DoCommand"]
		test.That(tb, ok, test.ShouldBeTrue)
		test.That(tb, len(successJob.RecentSuccessfulRuns), test.ShouldBeGreaterThan, 0)
		test.That(tb, len(successJob.RecentFailedRuns), test.ShouldBeGreaterThan, 0)
	})
}

// Test continuous mode, include switching to and from.
func TestJobContinuousSchedule(t *testing.T) {
	t.Parallel()
	logger := logging.NewTestLogger(t)

	fakeSensorComponent := []resource.Config{
		{
			Model: resource.DefaultModelFamily.WithModel("fake"),
			Name:  "sensor",
			API:   sensor.API,
		},
	}

	cfg := &config.Config{
		Components: fakeSensorComponent,
		Jobs: []config.JobConfig{
			{
				config.JobConfigData{
					Name:     "fake sensor",
					Schedule: "1s",
					Resource: "sensor",
					Method:   "GetReadings",
				},
			},
		},
	}
	cfgCron := &config.Config{
		Components: fakeSensorComponent,
		Jobs: []config.JobConfig{
			{
				config.JobConfigData{
					Name:     "fake sensor",
					Schedule: "*/1 * * * * *",
					Resource: "sensor",
					Method:   "GetReadings",
				},
			},
		},
	}
	cfgContinuous := &config.Config{
		Components: fakeSensorComponent,
		Jobs: []config.JobConfig{
			{
				config.JobConfigData{
					Name:     "fake sensor",
					Schedule: "continuous",
					Resource: "sensor",
					Method:   "GetReadings",
				},
			},
		},
	}
	ctx := context.Background()
	lr := setupLocalRobot(t, ctx, cfg, logger)
	o, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	err := lr.StartWeb(ctx, o)
	test.That(t, err, test.ShouldBeNil)
	robotClient, err := rclient.New(ctx, addr, logger)
	test.That(t, err, test.ShouldBeNil)
	defer robotClient.Close(ctx)

	// Start running in 1s duration mode. Expect last 2 latest success timestamps differ by > 900ms
	testutils.WaitForAssertionWithSleep(t, time.Second, 10, func(tb testing.TB) {
		tb.Helper()
		ms, err := robotClient.MachineStatus(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, len(ms.JobStatuses), test.ShouldEqual, 1)
		jh, ok := ms.JobStatuses["fake sensor"]
		test.That(tb, ok, test.ShouldBeTrue)
		successes := jh.RecentSuccessfulRuns
		test.That(tb, len(successes), test.ShouldBeGreaterThanOrEqualTo, 2)
		if len(successes) >= 2 {
			test.That(tb, successes[len(successes)-1].Sub(successes[len(successes)-2]), test.ShouldBeGreaterThan, 900*time.Millisecond)
		}
	})

	// Switch from duration to continuous. Should run more than 10x. Expect latest success timestamp - earliest < 900ms
	lr.Reconfigure(ctx, cfgContinuous)
	testutils.WaitForAssertionWithSleep(t, time.Second, 10, func(tb testing.TB) {
		tb.Helper()
		ms, err := robotClient.MachineStatus(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, len(ms.JobStatuses), test.ShouldEqual, 1)
		jh, ok := ms.JobStatuses["fake sensor"]
		test.That(tb, ok, test.ShouldBeTrue)
		successes := jh.RecentSuccessfulRuns
		test.That(tb, ok, test.ShouldBeTrue)
		// increase this if bumping history size
		test.That(tb, len(successes), test.ShouldBeGreaterThanOrEqualTo, 10)
		if len(successes) >= 2 {
			test.That(tb, successes[len(successes)-1].Sub(successes[0]), test.ShouldBeLessThan, 900*time.Millisecond)
		}
	})

	// Switch from continuous to 1s cron. Expect last 2 latest success timestamps differ by > 900ms
	lr.Reconfigure(ctx, cfgCron)
	testutils.WaitForAssertionWithSleep(t, time.Second, 10, func(tb testing.TB) {
		tb.Helper()
		ms, err := robotClient.MachineStatus(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, len(ms.JobStatuses), test.ShouldEqual, 1)
		jh, ok := ms.JobStatuses["fake sensor"]
		test.That(tb, ok, test.ShouldBeTrue)
		successes := jh.RecentSuccessfulRuns
		test.That(tb, ok, test.ShouldBeTrue)
		// History still contains runs from prev. If stored size is 10, we still expect 10 here.
		test.That(tb, len(successes), test.ShouldBeGreaterThanOrEqualTo, 10)
		if len(successes) >= 2 {
			test.That(tb, successes[len(successes)-1].Sub(successes[len(successes)-2]), test.ShouldBeGreaterThan, 900*time.Millisecond)
		}
	})
}

func TestJobManagerConfigChanges(t *testing.T) {
	t.Parallel()
	logger := logging.NewTestLogger(t)
	model := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(8))

	var (
		doCommandFirstCount1 atomic.Int64
		doCommandFirstCount2 atomic.Int64
		doCommandSecondCount atomic.Int64
		doCommandThirdCount  atomic.Int64
		doCommandIntCheck    atomic.Int64
		doCommandBoolCheck   atomic.Bool
	)
	dummyArm1 := &inject.Arm{
		DoFunc: func(ctx context.Context, cmd map[string]any) (map[string]any, error) {
			myCommand, exists := cmd["command"]
			if !exists {
				return nil, errors.New("command not in the map")
			}
			if myCommand == "first 1" {
				doCommandFirstCount1.Add(1)
			} else {
				doCommandFirstCount2.Add(1)
			}
			return map[string]any{
				"count": "done",
			}, nil
		},
	}
	dummyArm2 := &inject.Arm{
		DoFunc: func(ctx context.Context, cmd map[string]any) (map[string]any, error) {
			doCommandSecondCount.Add(1)
			return map[string]any{
				"count2": "done",
			}, nil
		},
	}
	dummyArm3 := &inject.Arm{
		DoFunc: func(ctx context.Context, cmd map[string]any) (map[string]any, error) {
			doCommandThirdCount.Add(1)
			boolVal, ok := cmd["bool"].(bool)
			if !ok {
				return nil, errors.New("bool argument must be a boolean")
			}
			doCommandBoolCheck.Store(boolVal)
			logger.Info(cmd["int"])
			intVal, ok := cmd["int"].(int)
			logger.Info(intVal)
			if !ok {
				return nil, errors.New("int argument must be an integer")
			}
			doCommandIntCheck.Store(int64(intVal))
			return map[string]any{
				"count3": "done",
			}, nil
		},
	}
	resource.RegisterComponent(
		arm.API,
		model,
		resource.Registration[arm.Arm, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (arm.Arm, error) {
			if conf.Name == "arm1" {
				return dummyArm1, nil
			} else if conf.Name == "arm2" {
				return dummyArm2, nil
			}
			return dummyArm3, nil
		}})

	defer func() {
		resource.Deregister(arm.API, model)
	}()

	cfg := &config.Config{
		Components: []resource.Config{
			{
				Model: model,
				Name:  "arm1",
				API:   arm.API,
			},
			{
				Model: model,
				Name:  "arm2",
				API:   arm.API,
			},
			{
				Model: model,
				Name:  "arm3",
				API:   arm.API,
			},
		},
		Jobs: []config.JobConfig{
			{
				config.JobConfigData{
					Name:     "arm1 job",
					Schedule: "3s",
					Resource: "arm1",
					Method:   "DoCommand",
					Command: map[string]any{
						"command": "first 1",
					},
				},
			},
			{
				config.JobConfigData{
					Name:     "arm2 job",
					Schedule: "3s",
					Resource: "arm2",
					Method:   "DoCommand",
					Command: map[string]any{
						"command": "second",
					},
				},
			},
		},
	}

	ctx := context.Background()
	r := setupLocalRobot(t, ctx, cfg, logger)

	testutils.WaitForAssertionWithSleep(t, time.Second, 5, func(tb testing.TB) {
		tb.Helper()
		// after 3 seconds, FirstCount1 and SecondCount should fire at least once
		test.That(tb, doCommandFirstCount1.Load(), test.ShouldEqual, 1)
		test.That(tb, doCommandSecondCount.Load(), test.ShouldEqual, 1)
	})

	// then, we reconfigure to change the first job, remove second, add third
	newJobs := []config.JobConfig{
		{
			config.JobConfigData{
				Name:     "arm1 job",
				Schedule: "3s",
				Resource: "arm1",
				Method:   "DoCommand",
				Command: map[string]any{
					"command": "first 2",
				},
			},
		},
		{
			config.JobConfigData{
				Name:     "arm3 job",
				Schedule: "3s",
				Resource: "arm3",
				Method:   "DoCommand",
				Command: map[string]any{
					"command": "third",
					"int":     10,
					"bool":    true,
				},
			},
		},
	}

	cfg.Jobs = newJobs

	r.Reconfigure(context.Background(), cfg)

	// NOTE: test could flake because of precies "ShouldEqual"
	testutils.WaitForAssertionWithSleep(t, time.Second, 8, func(tb testing.TB) {
		tb.Helper()
		// after two rounds of 3 second jobs, the FirstCount2 and ThirdCount should
		// have happened twice, while the other two jobs only once.
		test.That(tb, doCommandFirstCount1.Load(), test.ShouldBeGreaterThanOrEqualTo, 1)
		test.That(tb, doCommandSecondCount.Load(), test.ShouldBeGreaterThanOrEqualTo, 1)
		test.That(tb, doCommandFirstCount2.Load(), test.ShouldBeGreaterThanOrEqualTo, 2)
		test.That(tb, doCommandThirdCount.Load(), test.ShouldBeGreaterThanOrEqualTo, 2)
		test.That(tb, doCommandIntCheck.Load(), test.ShouldEqual, 10)
		test.That(tb, doCommandBoolCheck.Load(), test.ShouldEqual, true)
	})
}

func TestJobManagerRemote(t *testing.T) {
	t.Parallel()
	logger, logs := logging.NewObservedTestLogger(t)
	model := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(8))

	// sensor
	dummySensor := inject.NewSensor("sensor")
	dummySensor.ReadingsFunc = func(ctx context.Context, extra map[string]any) (map[string]any, error) {
		output := make(map[string]any)
		output["test"] = "sensor"
		return output, nil
	}
	resource.RegisterComponent(
		sensor.API,
		model,
		resource.Registration[sensor.Sensor, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (sensor.Sensor, error) {
			return dummySensor, nil
		}})

	defer func() {
		resource.Deregister(sensor.API, model)
	}()

	ctx := context.Background()
	startWeb := func(r robot.LocalRobot) string {
		var boundAddress string
		for range 10 {
			port, err := utils.TryReserveRandomPort()
			test.That(t, err, test.ShouldBeNil)

			options := weboptions.New()
			boundAddress = fmt.Sprintf("localhost:%v", port)
			options.Network.BindAddress = boundAddress
			if err := r.StartWeb(ctx, options); err != nil {
				r.StopWeb()
				if strings.Contains(err.Error(), "address already in use") {
					logger.Infow("port in use; restarting on new port", "port", port, "err", err)
					continue
				}
				t.Fatalf("StartWeb error: %v", err)
			}
			break
		}
		return boundAddress
	}

	remCfg := &config.Config{
		Components: []resource.Config{
			{
				Model: model,
				Name:  "sensor",
				API:   sensor.API,
			},
		},
	}
	remoteRobot := setupLocalRobot(t, ctx, remCfg, logger.Sublogger("remoteRobot"))
	addr := startWeb(remoteRobot)
	test.That(t, addr, test.ShouldNotBeBlank)

	cfg := &config.Config{
		Remotes: []config.Remote{
			{
				Name:    "remote",
				Address: addr,
			},
		},
		Jobs: []config.JobConfig{
			{
				config.JobConfigData{
					Name:     "sensor job",
					Schedule: "3s",
					Resource: "sensor",
					Method:   "GetReadings",
				},
			},
		},
	}
	setupLocalRobot(t, ctx, cfg, logger.Sublogger("robot"))

	testutils.WaitForAssertionWithSleep(t, time.Second, 5, func(tb testing.TB) {
		tb.Helper()
		// we will test for succeeded jobs to be the amount we started,
		// and that there are no failed jobs
		test.That(tb, logs.FilterMessage("Job triggered").Len(),
			test.ShouldBeGreaterThanOrEqualTo, 1)
		test.That(tb, logs.FilterMessage("Job succeeded").Len(),
			test.ShouldBeGreaterThanOrEqualTo, 1)
		test.That(tb, logs.FilterMessage("Job failed").Len(),
			test.ShouldBeLessThanOrEqualTo, 0)
	})
}

func TestJobManagerRemoteWithPrefix(t *testing.T) {
	t.Parallel()
	logger, logs := logging.NewObservedTestLogger(t)
	model := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(8))

	// sensor
	dummySensor := inject.NewSensor("sensor")
	dummySensor.ReadingsFunc = func(ctx context.Context, extra map[string]any) (map[string]any, error) {
		output := make(map[string]any)
		output["test"] = "sensor"
		return output, nil
	}
	resource.RegisterComponent(
		sensor.API,
		model,
		resource.Registration[sensor.Sensor, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (sensor.Sensor, error) {
			return dummySensor, nil
		}})

	defer func() {
		resource.Deregister(sensor.API, model)
	}()

	ctx := context.Background()
	startWeb := func(r robot.LocalRobot) string {
		var boundAddress string
		for range 10 {
			port, err := utils.TryReserveRandomPort()
			test.That(t, err, test.ShouldBeNil)

			options := weboptions.New()
			boundAddress = fmt.Sprintf("localhost:%v", port)
			options.Network.BindAddress = boundAddress
			if err := r.StartWeb(ctx, options); err != nil {
				r.StopWeb()
				if strings.Contains(err.Error(), "address already in use") {
					logger.Infow("port in use; restarting on new port", "port", port, "err", err)
					continue
				}
				t.Fatalf("StartWeb error: %v", err)
			}
			break
		}
		return boundAddress
	}

	remCfg := &config.Config{
		Components: []resource.Config{
			{
				Model: model,
				Name:  "sensor",
				API:   sensor.API,
			},
		},
	}
	remoteRobot := setupLocalRobot(t, ctx, remCfg, logger.Sublogger("remoteRobot"))
	addr := startWeb(remoteRobot)
	test.That(t, addr, test.ShouldNotBeBlank)

	cfg := &config.Config{
		Remotes: []config.Remote{
			{
				Name:    "remote",
				Address: addr,
				Prefix:  "remote-",
			},
		},
		Jobs: []config.JobConfig{
			{
				config.JobConfigData{
					Name:     "sensor job",
					Schedule: "3s",
					Resource: "remote-sensor",
					Method:   "GetReadings",
				},
			},
		},
	}
	setupLocalRobot(t, ctx, cfg, logger.Sublogger("robot"))

	testutils.WaitForAssertionWithSleep(t, time.Second, 5, func(tb testing.TB) {
		tb.Helper()
		// we will test for succeeded jobs to be the amount we started,
		// and that there are no failed jobs
		test.That(tb, logs.FilterMessage("Job triggered").Len(),
			test.ShouldBeGreaterThanOrEqualTo, 1)
		test.That(tb, logs.FilterMessage("Job succeeded").Len(),
			test.ShouldBeGreaterThanOrEqualTo, 1)
		test.That(tb, logs.FilterMessage("Job failed").Len(),
			test.ShouldBeLessThanOrEqualTo, 0)
	})
}

func TestJobManagerComponents(t *testing.T) {
	t.Parallel()
	logger, logs := logging.NewObservedTestLogger(t)
	model := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(8))

	// arm
	dummyArm := inject.NewArm("arm")
	dummyArm.GeometriesFunc = func(ctx context.Context) ([]spatialmath.Geometry, error) {
		return make([]spatialmath.Geometry, 0), nil
	}
	resource.RegisterComponent(
		arm.API,
		model,
		resource.Registration[arm.Arm, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (arm.Arm, error) {
			return dummyArm, nil
		}})

	// audioin
	dummyAudioIn := inject.NewAudioIn("audioin")
	dummyAudioIn.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (rutils.Properties, error) {
		return rutils.Properties{}, nil
	}
	resource.RegisterComponent(
		audioin.API,
		model,
		resource.Registration[audioin.AudioIn, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (audioin.AudioIn, error) {
			return dummyAudioIn, nil
		}})

	// audioout
	dummyAudioOut := inject.NewAudioOut("audioout")
	dummyAudioOut.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (rutils.Properties, error) {
		return rutils.Properties{}, nil
	}
	resource.RegisterComponent(
		audioout.API,
		model,
		resource.Registration[audioout.AudioOut, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (audioout.AudioOut, error) {
			return dummyAudioOut, nil
		}})

	// base
	dummyBase := inject.NewBase("base")
	dummyBase.IsMovingFunc = func(ctx context.Context) (bool, error) {
		return true, nil
	}
	resource.RegisterComponent(
		base.API,
		model,
		resource.Registration[base.Base, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (base.Base, error) {
			return dummyBase, nil
		}})

	// board
	dummyBoard := inject.NewBoard("board")
	dummyBoard.GPIOPinByNameFunc = func(name string) (board.GPIOPin, error) {
		return &inject.GPIOPin{
			GetFunc: func(ctx context.Context, extra map[string]any) (bool, error) {
				return true, nil
			},
		}, nil
	}
	resource.RegisterComponent(
		board.API,
		model,
		resource.Registration[board.Board, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (board.Board, error) {
			return dummyBoard, nil
		}})

	// button
	dummyButton := inject.NewButton("button")
	dummyButton.PushFunc = func(ctx context.Context, extra map[string]any) error {
		return nil
	}
	dummyButton.CloseFunc = func(ctx context.Context) error {
		return nil
	}
	resource.RegisterComponent(
		button.API,
		model,
		resource.Registration[button.Button, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (button.Button, error) {
			return dummyButton, nil
		}})
	// camera
	dummyCamera := inject.NewCamera("camera")
	dummyCamera.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
		return camera.Properties{SupportsPCD: true}, nil
	}
	resource.RegisterComponent(
		camera.API,
		model,
		resource.Registration[camera.Camera, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (camera.Camera, error) {
			return dummyCamera, nil
		}})
	// encoder
	dummyEncoder := inject.NewEncoder("encoder")
	dummyEncoder.PropertiesFunc = func(ctx context.Context,
		extra map[string]any,
	) (encoder.Properties, error) {
		return encoder.Properties{TicksCountSupported: true}, nil
	}
	resource.RegisterComponent(
		encoder.API,
		model,
		resource.Registration[encoder.Encoder, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (encoder.Encoder, error) {
			return dummyEncoder, nil
		}})
	// gantry
	dummyGantry := inject.NewGantry("gantry")
	dummyGantry.HomeFunc = func(ctx context.Context, extra map[string]any) (bool, error) {
		return true, nil
	}
	resource.RegisterComponent(
		gantry.API,
		model,
		resource.Registration[gantry.Gantry, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (gantry.Gantry, error) {
			return dummyGantry, nil
		}})
	// generic
	genericCounter := 0
	dummyGeneric := inject.NewGenericComponent("generic")
	dummyGeneric.DoFunc = func(ctx context.Context, cmd map[string]any) (map[string]any, error) {
		genericCounter++
		return map[string]any{
			"generic_count": genericCounter,
		}, nil
	}
	resource.RegisterComponent(
		generic.API,
		model,
		resource.Registration[generic.Resource, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (generic.Resource, error) {
			return dummyGeneric, nil
		}})
	// gripper
	dummyGripper := inject.NewGripper("gripper")
	dummyGripper.GrabFunc = func(ctx context.Context, extra map[string]any) (bool, error) {
		return true, nil
	}
	resource.RegisterComponent(
		gripper.API,
		model,
		resource.Registration[gripper.Gripper, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (gripper.Gripper, error) {
			return dummyGripper, nil
		}})
	// inputcontroller
	dummyInputController := inject.NewInputController("my_input")
	dummyInputController.EventsFunc = func(ctx context.Context,
		extra map[string]any,
	) (map[input.Control]input.Event, error) {
		control := make(map[input.Control]input.Event)
		control[input.AbsoluteHat0X] = input.Event{}
		return control, nil
	}
	resource.RegisterComponent(
		input.API,
		model,
		resource.Registration[input.Controller, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (input.Controller, error) {
			return dummyInputController, nil
		}})
	// motor
	dummyMotor := inject.NewMotor("motor")
	dummyMotor.SetPowerFunc = func(ctx context.Context, powerPct float64, extra map[string]any) error {
		return nil
	}
	resource.RegisterComponent(
		motor.API,
		model,
		resource.Registration[motor.Motor, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (motor.Motor, error) {
			return dummyMotor, nil
		}})
	// movementsensor
	dummyMovementSensor := inject.NewMovementSensor("move_sensor")
	dummyMovementSensor.OrientationFunc = func(ctx context.Context, extra map[string]any) (spatialmath.Orientation, error) {
		return nil, nil
	}
	resource.RegisterComponent(
		movementsensor.API,
		model,
		resource.Registration[movementsensor.MovementSensor, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (movementsensor.MovementSensor, error) {
			return dummyMovementSensor, nil
		}})
	// posetracker
	dummyPoseTracker := inject.NewPoseTracker("pose")
	dummyPoseTracker.PosesFunc = func(ctx context.Context,
		bodyNames []string,
		extra map[string]any,
	) (referenceframe.FrameSystemPoses, error) {
		return make(referenceframe.FrameSystemPoses), nil
	}
	resource.RegisterComponent(
		posetracker.API,
		model,
		resource.Registration[posetracker.PoseTracker, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (posetracker.PoseTracker, error) {
			return dummyPoseTracker, nil
		}})
	// powersensor
	dummyPowerSensor := inject.NewPowerSensor("power_sensor")
	dummyPowerSensor.VoltageFunc = func(ctx context.Context, extra map[string]any) (float64, bool, error) {
		return 14.3, false, nil
	}
	resource.RegisterComponent(
		powersensor.API,
		model,
		resource.Registration[powersensor.PowerSensor, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (powersensor.PowerSensor, error) {
			return dummyPowerSensor, nil
		}})
	// sensor
	dummySensor := inject.NewSensor("sensor")
	dummySensor.ReadingsFunc = func(ctx context.Context, extra map[string]any) (map[string]any, error) {
		output := make(map[string]any)
		output["test"] = "sensor"
		return output, nil
	}
	resource.RegisterComponent(
		sensor.API,
		model,
		resource.Registration[sensor.Sensor, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (sensor.Sensor, error) {
			return dummySensor, nil
		}})
	// servo
	dummyServo := inject.NewServo("servo")
	dummyServo.PositionFunc = func(ctx context.Context, extra map[string]any) (uint32, error) {
		return 43, nil
	}
	resource.RegisterComponent(
		servo.API,
		model,
		resource.Registration[servo.Servo, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (servo.Servo, error) {
			return dummyServo, nil
		}})
	// switch
	dummySwitch := inject.NewSwitch("switch")
	dummySwitch.GetNumberOfPositionsFunc = func(ctx context.Context, extra map[string]interface{}) (uint32, []string, error) {
		return 1, []string{"test"}, nil
	}
	dummySwitch.CloseFunc = func(ctx context.Context) error {
		return nil
	}
	resource.RegisterComponent(
		sw.API,
		model,
		resource.Registration[sw.Switch, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (sw.Switch, error) {
			return dummySwitch, nil
		}})

	// create the actual config and fill it
	cfg := &config.Config{
		Components: []resource.Config{
			{
				Model: model,
				Name:  "arm",
				API:   arm.API,
			},
			{
				Model: model,
				Name:  "audioin",
				API:   audioin.API,
			},
			{
				Model: model,
				Name:  "audioout",
				API:   audioout.API,
			},
			{
				Model: model,
				Name:  "base",
				API:   base.API,
			},
			{
				Model: model,
				Name:  "board",
				API:   board.API,
			},
			{
				Model: model,
				Name:  "button",
				API:   button.API,
			},
			{
				Model: model,
				Name:  "camera",
				API:   camera.API,
			},
			{
				Model: model,
				Name:  "encoder",
				API:   encoder.API,
			},
			{
				Model: model,
				Name:  "gantry",
				API:   gantry.API,
			},
			{
				Model: model,
				Name:  "generic",
				API:   generic.API,
			},
			{
				Model: model,
				Name:  "gripper",
				API:   gripper.API,
			},
			{
				Model: model,
				Name:  "my_input",
				API:   input.API,
			},
			{
				Model: model,
				Name:  "motor",
				API:   motor.API,
			},
			{
				Model: model,
				Name:  "mov_sensor",
				API:   movementsensor.API,
			},
			{
				Model: model,
				Name:  "pose",
				API:   posetracker.API,
			},
			{
				Model: model,
				Name:  "power_sensor",
				API:   powersensor.API,
			},
			{
				Model: model,
				Name:  "sensor",
				API:   sensor.API,
			},
			{
				Model: model,
				Name:  "servo",
				API:   servo.API,
			},
			{
				Model: model,
				Name:  "switch",
				API:   sw.API,
			},
		},
		Jobs: []config.JobConfig{
			{
				config.JobConfigData{
					Name:     "arm job",
					Schedule: "3s",
					Resource: "arm",
					Method:   "GetGeometries",
				},
			},
			{
				config.JobConfigData{
					Name:     "audioin job",
					Schedule: "3s",
					Resource: "audioin",
					Method:   "GetProperties",
				},
			},
			{
				config.JobConfigData{
					Name:     "audioout job",
					Schedule: "3s",
					Resource: "audioout",
					Method:   "GetProperties",
				},
			},
			{
				config.JobConfigData{
					Name:     "base job",
					Schedule: "3s",
					Resource: "base",
					Method:   "IsMoving",
				},
			},
			{
				config.JobConfigData{
					Name:     "board job",
					Schedule: "3s",
					Resource: "board",
					Method:   "GetGPIO",
				},
			},
			{
				config.JobConfigData{
					Name:     "button job",
					Schedule: "3s",
					Resource: "button",
					Method:   "Push",
				},
			},
			{
				config.JobConfigData{
					Name:     "camera job",
					Schedule: "3s",
					Resource: "camera",
					Method:   "GetProperties",
				},
			},
			{
				config.JobConfigData{
					Name:     "encoder job",
					Schedule: "3s",
					Resource: "encoder",
					Method:   "GetProperties",
				},
			},
			{
				config.JobConfigData{
					Name:     "gantry job",
					Schedule: "3s",
					Resource: "gantry",
					Method:   "Home",
				},
			},
			{
				config.JobConfigData{
					Name:     "generic job",
					Schedule: "3s",
					Resource: "generic",
					Method:   "DoCommand",
					Command: map[string]any{
						"any": "any",
					},
				},
			},
			{
				config.JobConfigData{
					Name:     "gripper job",
					Schedule: "3s",
					Resource: "gripper",
					Method:   "Grab",
				},
			},
			{
				config.JobConfigData{
					Name:     "input job",
					Schedule: "3s",
					Resource: "my_input",
					Method:   "GetEvents",
				},
			},
			{
				config.JobConfigData{
					Name:     "motor job",
					Schedule: "3s",
					Resource: "motor",
					Method:   "SetPower",
				},
			},
			{
				config.JobConfigData{
					Name:     "movement sensor job",
					Schedule: "3s",
					Resource: "mov_sensor",
					Method:   "GetOrientation",
				},
			},
			{
				config.JobConfigData{
					Name:     "pose job",
					Schedule: "3s",
					Resource: "pose",
					Method:   "GetPoses",
				},
			},
			{
				config.JobConfigData{
					Name:     "power sensor job",
					Schedule: "3s",
					Resource: "power_sensor",
					Method:   "GetVoltage",
				},
			},
			{
				config.JobConfigData{
					Name:     "sensor job",
					Schedule: "3s",
					Resource: "sensor",
					Method:   "GetReadings",
				},
			},
			{
				config.JobConfigData{
					Name:     "servo job",
					Schedule: "3s",
					Resource: "servo",
					Method:   "GetPosition",
				},
			},
			{
				config.JobConfigData{
					Name:     "switch job",
					Schedule: "3s",
					Resource: "switch",
					Method:   "GetNumberOfPositions",
				},
			},
		},
	}
	defer func() {
		resource.Deregister(arm.API, model)
		resource.Deregister(audioin.API, model)
		resource.Deregister(audioout.API, model)
		resource.Deregister(base.API, model)
		resource.Deregister(board.API, model)
		resource.Deregister(button.API, model)
		resource.Deregister(camera.API, model)
		resource.Deregister(encoder.API, model)
		resource.Deregister(gantry.API, model)
		resource.Deregister(generic.API, model)
		resource.Deregister(gripper.API, model)
		resource.Deregister(input.API, model)
		resource.Deregister(motor.API, model)
		resource.Deregister(movementsensor.API, model)
		resource.Deregister(posetracker.API, model)
		resource.Deregister(powersensor.API, model)
		resource.Deregister(sensor.API, model)
		resource.Deregister(servo.API, model)
		resource.Deregister(sw.API, model)
	}()

	ctx := context.Background()
	setupLocalRobot(t, ctx, cfg, logger)

	testutils.WaitForAssertionWithSleep(t, time.Second, 5, func(tb testing.TB) {
		tb.Helper()
		// we will test for succeeded jobs to be the amount we started,
		// and that there are no failed jobs
		test.That(tb, logs.FilterMessage("Job triggered").Len(),
			test.ShouldBeGreaterThanOrEqualTo, 19)
		test.That(tb, logs.FilterMessage("Job succeeded").Len(),
			test.ShouldBeGreaterThanOrEqualTo, 19)
		test.That(tb, logs.FilterMessage("Job failed").Len(),
			test.ShouldBeLessThanOrEqualTo, 0)
	})
}

func TestJobManagerServices(t *testing.T) {
	t.Parallel()
	logger, logs := logging.NewObservedTestLogger(t)
	model := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(8))

	// datamanager
	dummyDataManager := inject.NewDataManagerService("data_manager")
	dummyDataManager.SyncFunc = func(ctx context.Context, extra map[string]any) error {
		return nil
	}
	resource.RegisterService(
		datamanager.API,
		model,
		resource.Registration[datamanager.Service, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (datamanager.Service, error) {
			return dummyDataManager, nil
		}})

	// discovery
	dummyDiscovery := inject.NewDiscoveryService("discovery")
	dummyDiscovery.DiscoverResourcesFunc = func(ctx context.Context,
		extra map[string]any,
	) ([]resource.Config, error) {
		return make([]resource.Config, 0), nil
	}
	dummyDiscovery.DoFunc = func(ctx context.Context,
		cmd map[string]interface{},
	) (map[string]interface{}, error) {
		return nil, nil
	}
	resource.RegisterService(
		discovery.API,
		model,
		resource.Registration[discovery.Service, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (discovery.Service, error) {
			return dummyDiscovery, nil
		}})

	// generic
	var genericCounter int
	dummyGeneric := inject.NewGenericService("generic")
	dummyGeneric.DoFunc = func(ctx context.Context, cmd map[string]any) (map[string]any, error) {
		genericCounter++
		return map[string]any{
			"generic_count": genericCounter,
		}, nil
	}
	resource.RegisterService(
		genSvc.API,
		model,
		resource.Registration[genSvc.Service, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (genSvc.Service, error) {
			return dummyGeneric, nil
		}})
	// ml_model
	dummyML := inject.NewMLModelService("ml_model")
	dummyML.InferFunc = func(ctx context.Context, tensors ml.Tensors) (ml.Tensors, error) {
		return make(ml.Tensors), nil
	}
	resource.RegisterService(
		mlmodel.API,
		model,
		resource.Registration[mlmodel.Service, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (mlmodel.Service, error) {
			return dummyML, nil
		}})
	// motion
	dummyMotion := injectmotion.NewMotionService("motion")
	dummyMotion.ListPlanStatusesFunc = func(ctx context.Context, req motion.ListPlanStatusesReq) ([]motion.PlanStatusWithID, error) {
		return make([]motion.PlanStatusWithID, 0), nil
	}
	resource.RegisterService(
		motion.API,
		model,
		resource.Registration[motion.Service, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (motion.Service, error) {
			return dummyMotion, nil
		}})
	// navigation
	dummyNav := inject.NewNavigationService("navigation")
	dummyNav.ModeFunc = func(ctx context.Context, extra map[string]any) (navigation.Mode, error) {
		return navigation.ModeExplore, nil
	}
	resource.RegisterService(
		navigation.API,
		model,
		resource.Registration[navigation.Service, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (navigation.Service, error) {
			return dummyNav, nil
		}})
	// shell
	var shellCounter int
	dummyShell := inject.NewShellService("shell")
	dummyShell.DoCommandFunc = func(ctx context.Context, cmd map[string]any) (map[string]any, error) {
		shellCounter++
		return map[string]any{
			"shell_count": shellCounter,
		}, nil
	}
	resource.RegisterService(
		shell.API,
		model,
		resource.Registration[shell.Service, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (shell.Service, error) {
			return dummyShell, nil
		}})
	// slam
	dummySlam := inject.NewSLAMService("slam")
	dummySlam.PropertiesFunc = func(ctx context.Context) (slam.Properties, error) {
		return slam.Properties{
			CloudSlam: true,
		}, nil
	}
	resource.RegisterService(
		slam.API,
		model,
		resource.Registration[slam.Service, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (slam.Service, error) {
			return dummySlam, nil
		}})
	// vision
	dummyVision := inject.NewVisionService("vision")
	dummyVision.GetPropertiesFunc = func(ctx context.Context, extra map[string]any) (*vision.Properties, error) {
		return &vision.Properties{
			ClassificationSupported: true,
			ObjectPCDsSupported:     false,
			DetectionSupported:      true,
		}, nil
	}
	resource.RegisterService(
		vision.API,
		model,
		resource.Registration[vision.Service, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (vision.Service, error) {
			return dummyVision, nil
		}})
	cfg := &config.Config{
		Services: []resource.Config{
			{
				Model: model,
				Name:  "data_manager",
				API:   datamanager.API,
			},
			{
				Model: model,
				Name:  "discovery",
				API:   discovery.API,
			},
			{
				Model: model,
				Name:  "generic",
				API:   genSvc.API,
			},
			{
				Model: model,
				Name:  "ml_model",
				API:   mlmodel.API,
			},
			{
				Model: model,
				Name:  "navigation",
				API:   navigation.API,
			},
			{
				Model: model,
				Name:  "shell",
				API:   shell.API,
			},
			{
				Model: model,
				Name:  "slam",
				API:   slam.API,
			},
			{
				Model: model,
				Name:  "vision",
				API:   vision.API,
			},
			{
				Model: model,
				Name:  "motion",
				API:   motion.API,
			},
		},
		Jobs: []config.JobConfig{
			// {
			// TODO(RSDK-9718)
			// Discovery Service is currently excluded from the list of services; it will be
			// available after a change in the API repo.
			// config.JobConfigData{
			// Name:     "discovery job",
			// Schedule: "3s",
			// Resource: "discovery",
			// Method:   "DiscoverResources",
			//  },
			// },
			{
				config.JobConfigData{
					Name:     "data manager job",
					Schedule: "3s",
					Resource: "data_manager",
					Method:   "Sync",
				},
			},
			{
				config.JobConfigData{
					Name:     "generic job",
					Schedule: "3s",
					Resource: "generic",
					Method:   "DoCommand",
					Command: map[string]any{
						"command": "test",
					},
				},
			},
			{
				config.JobConfigData{
					Name:     "ml job",
					Schedule: "3s",
					Resource: "ml_model",
					Method:   "Infer",
				},
			},
			{
				config.JobConfigData{
					Name:     "nav job",
					Schedule: "3s",
					Resource: "navigation",
					Method:   "GetMode",
				},
			},
			{
				config.JobConfigData{
					Name:     "shell job",
					Schedule: "3s",
					Resource: "shell",
					Method:   "DoCommand",
					Command: map[string]any{
						"command": "test",
					},
				},
			},
			{
				config.JobConfigData{
					Name:     "slam job",
					Schedule: "3s",
					Resource: "slam",
					Method:   "GetProperties",
				},
			},
			{
				config.JobConfigData{
					Name:     "vision job",
					Schedule: "3s",
					Resource: "vision",
					Method:   "GetProperties",
				},
			},
			{
				config.JobConfigData{
					Name:     "motion job",
					Schedule: "3s",
					Resource: "motion",
					Method:   "ListPlanStatuses",
				},
			},
		},
	}
	defer func() {
		resource.Deregister(datamanager.API, model)
		resource.Deregister(discovery.API, model)
		resource.Deregister(genSvc.API, model)
		resource.Deregister(mlmodel.API, model)
		resource.Deregister(navigation.API, model)
		resource.Deregister(shell.API, model)
		resource.Deregister(slam.API, model)
		resource.Deregister(vision.API, model)
		resource.Deregister(motion.API, model)
	}()

	ctx := context.Background()
	setupLocalRobot(t, ctx, cfg, logger)

	testutils.WaitForAssertionWithSleep(t, time.Second, 5, func(tb testing.TB) {
		tb.Helper()
		test.That(tb, logs.FilterMessage("Job triggered").Len(),
			test.ShouldBeGreaterThanOrEqualTo, 8)
		test.That(tb, logs.FilterMessage("Job succeeded").Len(),
			test.ShouldBeGreaterThanOrEqualTo, 8)
		test.That(tb, logs.FilterMessage("Job failed").Len(),
			test.ShouldBeLessThanOrEqualTo, 0)
	})
}

func TestJobManagerErrors(t *testing.T) {
	t.Parallel()
	logger := logging.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake_jobs.json", logger, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(cfg.Jobs), test.ShouldBeGreaterThanOrEqualTo, 1)
	// components in the fake config do not implement do command; we can alter the config of
	// of the first job and check the error message
	unimplementedDoCommandCfg := config.JobConfig{
		config.JobConfigData{
			Name:     "test unimplemented",
			Schedule: "3s",
			Resource: cfg.Jobs[0].Resource,
			Method:   "DoCommand",
			Command: map[string]any{
				"command": "test unimplemented",
			},
		},
	}
	cfg.Jobs = []config.JobConfig{
		unimplementedDoCommandCfg,
	}
	logger, logs := logging.NewObservedTestLogger(t)
	r := setupLocalRobot(t, context.Background(), cfg, logger)

	testutils.WaitForAssertionWithSleep(t, time.Second, 5, func(tb testing.TB) {
		tb.Helper()
		test.That(tb, logs.FilterMessage("Job triggered").Len(),
			test.ShouldBeGreaterThanOrEqualTo, 1)
		test.That(tb, logs.FilterMessage("Job failed").Len(),
			test.ShouldBeGreaterThanOrEqualTo, 1)
	})

	// get the error log and check the actual message
	foundErrorLogs := logs.FilterFieldKey("error").All()

	test.That(t, len(foundErrorLogs), test.ShouldBeGreaterThanOrEqualTo, 1)
	test.That(t, foundErrorLogs[0].ContextMap()["error"], test.ShouldEqual, "DoCommand unimplemented")
	badResourceJobCfg := config.JobConfig{
		config.JobConfigData{
			Name:     "resource not in graph",
			Schedule: "1s",
			Resource: "unexpected",
			Method:   "method",
		},
	}

	cfg.Jobs = append(cfg.Jobs, badResourceJobCfg)
	r.Reconfigure(context.Background(), cfg)

	testutils.WaitForAssertionWithSleep(t, time.Second, 3, func(tb testing.TB) {
		tb.Helper()
		test.That(tb, logs.FilterMessageSnippet("Could not get resource").Len(),
			test.ShouldBeGreaterThanOrEqualTo, 1)
	})

	// reset logs
	logs.TakeAll()

	// create functions that return errors
	model := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(8))
	dummyArm := inject.NewArm("arm")
	dummyArm.GeometriesFunc = func(ctx context.Context) ([]spatialmath.Geometry, error) {
		return nil, errors.New("test error api function")
	}
	dummyArm.DoFunc = func(ctx context.Context, cmd map[string]any) (map[string]any, error) {
		return nil, errors.New("test error do command")
	}
	resource.RegisterComponent(
		arm.API,
		model,
		resource.Registration[arm.Arm, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (arm.Arm, error) {
			return dummyArm, nil
		}})

	newConf := config.Config{
		Components: []resource.Config{
			{
				Model: model,
				Name:  "arm",
				API:   arm.API,
			},
		},
		Jobs: []config.JobConfig{
			{
				config.JobConfigData{
					Name:     "arm job",
					Schedule: "3s",
					Resource: "arm",
					Method:   "GetGeometries",
				},
			},
			{
				config.JobConfigData{
					Name:     "arm job do command",
					Schedule: "3s",
					Resource: "arm",
					Method:   "DoCommand",
					Command: map[string]any{
						"command": "test",
					},
				},
			},
		},
	}
	r.Reconfigure(context.Background(), &newConf)

	testutils.WaitForAssertionWithSleep(t, time.Second, 5, func(tb testing.TB) {
		tb.Helper()
		// check that we have two errors, one from doCommand and one from an rpc call
		test.That(tb, logs.FilterMessageSnippet("Job failed").Len(),
			test.ShouldBeGreaterThanOrEqualTo, 2)
		errorLogs := logs.FilterFieldKey("error").All()
		errorMessages := []any{}
		for _, log := range errorLogs {
			errorMessages = append(errorMessages, log.ContextMap()["error"])
		}
		test.That(tb, slices.Contains(errorMessages, "test error do command"), test.ShouldBeTrue)
		test.That(tb, slices.Contains(errorMessages, "rpc error: code = Unknown desc = test error api function"), test.ShouldBeTrue)
	})
}
