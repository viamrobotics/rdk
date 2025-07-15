package robotimpl

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/pion/mediadevices/pkg/prop"
	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/audioinput"
	//"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"

	//weboptions "go.viam.com/rdk/robot/web/options"

	//"google.golang.org/grpc/codes"
	//"google.golang.org/grpc/status"
	//"go.viam.com/rdk/components/arm/fake"
	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/button"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/encoder"

	//fakeencoder "go.viam.com/rdk/components/encoder/fake"
	"go.viam.com/rdk/components/gantry"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/components/input"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/components/posetracker"
	"go.viam.com/rdk/components/powersensor"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/components/servo"

	//fakemotor "go.viam.com/rdk/components/motor/fake"
	"go.viam.com/rdk/components/movementsensor"
	_ "go.viam.com/rdk/components/register"
	sw "go.viam.com/rdk/components/switch"
	"go.viam.com/test"

	_ "go.viam.com/rdk/components/register"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	_ "go.viam.com/rdk/services/register"
	"go.viam.com/utils"

	//"go.viam.com/utils/rpc"
	"go.viam.com/rdk/testutils/inject"
	//robottestutils "go.viam.com/rdk/testutils/robottestutils"
	"go.viam.com/utils/testutils"
)

// tests to do:
// test that checks every service (one method each)
// test that checks DoCommand unimplemented
// test that checks that jobs can be modified/added/removed
// tcp test
// test singleton mode for duration and for cron

func TestDurationsAndCronJobManager(t *testing.T) {
	logger := logging.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake_jobs.json", logger, nil)
	test.That(t, err, test.ShouldBeNil)
	logger, logs := logging.NewObservedTestLogger(t)
	robotContext := context.Background()
	setupLocalRobot(t, robotContext, cfg, logger)

	testutils.WaitForAssertionWithSleep(t, time.Second, 22, func(tb testing.TB) {
		tb.Helper()
		// this config has a 10s job, a 20s jobs, and a 5s cron job. After (around) 20 seconds,
		// it should have about 7 instances of finished jobs (since cron depends on the clock,
		// not on program start time)
		test.That(tb, logs.FilterMessage("Job triggered").Len(),
			test.ShouldBeGreaterThanOrEqualTo, 7)
		test.That(tb, logs.FilterMessage("Job succeeded").Len(),
			test.ShouldBeGreaterThanOrEqualTo, 7)
	})
}

func TestJobManagerDoCommand(t *testing.T) {
	logger := logging.NewTestLogger(t)
	//channel := make(chan struct{})
	model := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(8))

	//logger, logs := logging.NewObservedTestLogger(t)
	var (
		doCommandCount1 int
		doCommandCount2 int
	)
	dummyArm1 := &inject.Arm{
		DoFunc: func(ctx context.Context, cmd map[string]any) (map[string]any, error) {
			doCommandCount1++
			return map[string]any{
				"count1": doCommandCount1,
			}, nil
		},
	}
	dummyArm2 := &inject.Arm{
		DoFunc: func(ctx context.Context, cmd map[string]any) (map[string]any, error) {
			doCommandCount2++
			return map[string]any{
				"count2": doCommandCount2,
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
			}
			return dummyArm2, nil
		}})

	armConfig := fmt.Sprintf(`{
		"components": [
			{
				"model": "%[1]s",
				"name": "arm1",
				"type": "arm"
			},
			{
				"model": "%[1]s",
				"name": "arm2",
				"type": "arm"
			}
		],
	"jobs" : [
	{
		"name" : "arm1 job",
		"schedule" : "*/3 * * * * *",
		"resource" : "arm1",
		"method" : "DoCommand",
		"command" : {
			"command" : "test"
		}
	},
	{
		"name" : "arm2 job",
		"schedule" : "5s",
		"resource" : "arm2",
		"method" : "DoCommand",
		"command" : {
			"command" : "test"
		}
	}
	]
	} 
	`, model.String())
	defer func() {
		resource.Deregister(arm.API, model)
	}()

	cfg, err := config.FromReader(context.Background(), "", strings.NewReader(armConfig), logger, nil)
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()
	setupLocalRobot(t, ctx, cfg, logger)

	testutils.WaitForAssertionWithSleep(t, time.Second, 20, func(tb testing.TB) {
		tb.Helper()
		//test.That(tb, logs.FilterMessage(fmt.Sprintf("{%q:{%q:5}}", "response", "count1")).Len(),
		//test.ShouldBeGreaterThanOrEqualTo, 1)
		//test.That(tb, logs.FilterMessage(fmt.Sprintf("{%q:{%q:3}}", "response", "count2")).Len(),
		//test.ShouldBeGreaterThanOrEqualTo, 1)

		test.That(tb, doCommandCount1, test.ShouldEqual, 5)
		test.That(tb, doCommandCount2, test.ShouldEqual, 3)
	})

	// Test OPID cancellation
	//options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	//err = r.StartWeb(ctx, options)
	//test.That(t, err, test.ShouldBeNil)

	//conn, err := rgrpc.Dial(ctx, addr, logger)
	//test.That(t, err, test.ShouldBeNil)
	//defer utils.UncheckedErrorFunc(conn.Close)
	//arm1, err := arm.NewClientFromConn(ctx, conn, "somerem", arm.Named("arm1"), logger)
	//test.That(t, err, test.ShouldBeNil)

	//foundOPID := false
	//stopAllErrCh := make(chan error, 1)
	//go func() {
	//<-channel
	//for _, opid := range r.OperationManager().All() {
	//if opid.Method == "/viam.component.arm.v1.ArmService/DoCommand" {
	//foundOPID = true
	//stopAllErrCh <- r.StopAll(ctx, nil)
	//}
	//}
	//}()
	//_, err = arm1.DoCommand(ctx, map[string]interface{}{})
	//s, isGRPCErr := status.FromError(err)
	//test.That(t, isGRPCErr, test.ShouldBeTrue)
	//test.That(t, s.Code(), test.ShouldEqual, codes.Canceled)

	//stopAllErr := <-stopAllErrCh
	//test.That(t, foundOPID, test.ShouldBeTrue)
	//test.That(t, stopAllErr, test.ShouldBeNil)
}

func TestEveryComponentJobManager(t *testing.T) {
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

	// audioinput
	dummyAudioInput := inject.NewAudioInput("audio")
	dummyAudioInput.MediaPropertiesFunc = func(ctx context.Context) (prop.Audio, error) {
		audio := prop.Audio{
			ChannelCount: 10,
			Latency:      3 * time.Second,
			SampleRate:   128,
		}
		return audio, nil
	}
	//dummyAudioInput.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.AudioStream, error) {
	//return nil, nil
	//}
	resource.RegisterComponent(
		audioinput.API,
		model,
		resource.Registration[audioinput.AudioInput, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (audioinput.AudioInput, error) {
			return dummyAudioInput, nil
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
		extra map[string]any) (encoder.Properties, error) {
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
	var genericCounter = 0
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
		extra map[string]any) (map[input.Control]input.Event, error) {
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
		extra map[string]any) (referenceframe.FrameSystemPoses, error) {
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
				Name:  "audio",
				API:   audioinput.API,
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
					Schedule: "2s",
					Resource: "arm",
					Method:   "GetGeometries",
				},
			},
			{
				config.JobConfigData{
					Name:     "audio input job",
					Schedule: "2s",
					Resource: "audio",
					Method:   "Properties",
				},
			},
			{
				config.JobConfigData{
					Name:     "base job",
					Schedule: "2s",
					Resource: "base",
					Method:   "IsMoving",
				},
			},
			{
				config.JobConfigData{
					Name:     "board job",
					Schedule: "2s",
					Resource: "board",
					Method:   "GetGPIO",
				},
			},
			{
				config.JobConfigData{
					Name:     "button job",
					Schedule: "2s",
					Resource: "button",
					Method:   "Push",
				},
			},
			{
				config.JobConfigData{
					Name:     "camera job",
					Schedule: "2s",
					Resource: "camera",
					Method:   "GetProperties",
				},
			},
			{
				config.JobConfigData{
					Name:     "encoder job",
					Schedule: "2s",
					Resource: "encoder",
					Method:   "GetProperties",
				},
			},
			{
				config.JobConfigData{
					Name:     "gantry job",
					Schedule: "2s",
					Resource: "gantry",
					Method:   "Home",
				},
			},
			{
				config.JobConfigData{
					Name:     "generic job",
					Schedule: "2s",
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
					Schedule: "2s",
					Resource: "gripper",
					Method:   "Grab",
				},
			},
			{
				config.JobConfigData{
					Name:     "input job",
					Schedule: "2s",
					Resource: "my_input",
					Method:   "GetEvents",
				},
			},
			{
				config.JobConfigData{
					Name:     "motor job",
					Schedule: "2s",
					Resource: "motor",
					Method:   "SetPower",
				},
			},
			{
				config.JobConfigData{
					Name:     "movement sensor job",
					Schedule: "2s",
					Resource: "mov_sensor",
					Method:   "GetOrientation",
				},
			},
			{
				config.JobConfigData{
					Name:     "pose job",
					Schedule: "2s",
					Resource: "pose",
					Method:   "GetPoses",
				},
			},
			{
				config.JobConfigData{
					Name:     "power sensor job",
					Schedule: "2s",
					Resource: "power_sensor",
					Method:   "GetVoltage",
				},
			},
			{
				config.JobConfigData{
					Name:     "sensor job",
					Schedule: "2s",
					Resource: "sensor",
					Method:   "GetReadings",
				},
			},
			{
				config.JobConfigData{
					Name:     "servo job",
					Schedule: "2s",
					Resource: "servo",
					Method:   "GetPosition",
				},
			},
			{
				config.JobConfigData{
					Name:     "switch job",
					Schedule: "2s",
					Resource: "switch",
					Method:   "GetNumberOfPositions",
				},
			},
		},
	}
	defer func() {
		resource.Deregister(arm.API, model)
		resource.Deregister(audioinput.API, model)
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

	testutils.WaitForAssertionWithSleep(t, time.Second, 3, func(tb testing.TB) {
		tb.Helper()
		// we will test for succeeded jobs to be the amount we started,
		// and that there are no failed jobs
		test.That(tb, logs.FilterMessage("Job triggered").Len(),
			test.ShouldBeGreaterThanOrEqualTo, 18)
		test.That(tb, logs.FilterMessage("Job succeeded").Len(),
			test.ShouldBeGreaterThanOrEqualTo, 18)
		test.That(tb, logs.FilterMessage("Job failed").Len(),
			test.ShouldBeLessThanOrEqualTo, 0)
	})

}

func TestEveryServiceJobManager(t *testing.T) {

	// services:
	// datamanager
	// discovery
	// generic
	// ml_model
	// motion
	// navigation
	// sensors
	// shell
	// slam
	// vision
}
