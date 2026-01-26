// Package main is a module for testing, with an inline generic component to return internal data and perform other test functions.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/utils"
	"go.viam.com/utils/trace"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	genericservice "go.viam.com/rdk/services/generic"
)

var (
	helperModel              = resource.NewModel("rdk", "test", "helper")
	otherModel               = resource.NewModel("rdk", "test", "other")
	testMotorModel           = resource.NewModel("rdk", "test", "motor")
	testSlowModel            = resource.NewModel("rdk", "test", "slow")
	testFSDependentModel     = resource.NewModel("rdk", "test", "fsdep")
	testSensorDependentModel = resource.NewModel("rdk", "test", "sensordep")
	myMod                    *module.Module
)

func main() {
	utils.ContextualMainWithSIGPIPE(mainWithArgs, module.NewLoggerFromArgs("TestModule"))
}

func mainWithArgs(ctx context.Context, args []string, logger logging.Logger) error {
	if os.Getenv("VIAM_TESTMODULE_PANIC") != "" {
		panic("there is no flavor town")
	}
	logger.Debug("debug mode enabled")

	var err error
	myMod, err = module.NewModuleFromArgs(ctx)
	if err != nil {
		return err
	}

	resource.RegisterComponent(
		generic.API,
		helperModel,
		resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: newHelper,
		})
	err = myMod.AddModelFromRegistry(ctx, generic.API, helperModel)
	if err != nil {
		return err
	}

	resource.RegisterService(
		genericservice.API,
		otherModel,
		resource.Registration[resource.Resource, resource.NoNativeConfig]{Constructor: newOther})
	err = myMod.AddModelFromRegistry(ctx, genericservice.API, otherModel)
	if err != nil {
		return err
	}

	resource.RegisterComponent(
		motor.API,
		testMotorModel,
		resource.Registration[resource.Resource, resource.NoNativeConfig]{Constructor: newTestMotor})
	err = myMod.AddModelFromRegistry(ctx, motor.API, testMotorModel)
	if err != nil {
		return err
	}

	resource.RegisterComponent(
		generic.API,
		testSlowModel,
		resource.Registration[resource.Resource, resource.NoNativeConfig]{Constructor: newSlow})
	err = myMod.AddModelFromRegistry(ctx, generic.API, testSlowModel)
	if err != nil {
		return err
	}

	resource.RegisterComponent(
		generic.API,
		testFSDependentModel,
		resource.Registration[resource.Resource, *fsDepConfig]{Constructor: newFSDependent})
	err = myMod.AddModelFromRegistry(ctx, generic.API, testFSDependentModel)
	if err != nil {
		return err
	}

	resource.RegisterComponent(
		sensor.API,
		testSensorDependentModel,
		resource.Registration[resource.Resource, *sensorDepConfig]{Constructor: newSensorDependent})
	err = myMod.AddModelFromRegistry(ctx, sensor.API, testSensorDependentModel)
	if err != nil {
		return err
	}

	// Set up SIGUSR1 handler to log a distinctive message for testing stack dump functionality
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGUSR1)
	go func() {
		for range sigChan {
			logger.Info("Received SIGUSR1 signal in testmodule.")
		}
	}()
	defer signal.Stop(sigChan)

	err = myMod.Start(ctx)
	defer myMod.Close(ctx)
	if err != nil {
		return err
	}
	<-ctx.Done()

	// If this is set, sleep to catch module slow shutdown logs.
	sleepTime := os.Getenv("VIAM_TESTMODULE_SLOW_CLOSE")

	if sleepTime != "" {
		sleepDuration, err := time.ParseDuration(sleepTime)
		if err != nil {
			return err
		}
		time.Sleep(sleepDuration)
	}

	return nil
}

var attemptedConstruction bool

func newHelper(
	ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger,
) (resource.Resource, error) {
	// VIAM_TESTMODULE_FAIL_ON_FIRST will always fail the first attempt at construction
	if os.Getenv("VIAM_TESTMODULE_FAIL_ON_FIRST") != "" {
		if !attemptedConstruction {
			attemptedConstruction = true
			logger.Warn("VIAM_TESTMODULE_FAIL_ON_FIRST causing intentional failure")
			return nil, errors.New("gotta fail fast")
		}
		logger.Info("VIAM_TESTMODULE_FAIL_ON_FIRST not failing this time")
	}
	var dependsOnSensor sensor.Sensor
	var err error
	if len(conf.DependsOn) > 0 {
		dependsOnSensor, err = sensor.FromProvider(deps, conf.DependsOn[0])
		if err != nil {
			return nil, err
		}
	}

	// deps contains at least $framesystem
	if len(deps) > 1 && dependsOnSensor == nil {
		return nil, fmt.Errorf("sensor not found in deps: %v", deps)
	}

	return &helper{
		Named:           conf.ResourceName().AsNamed(),
		logger:          logger,
		dependsOnSensor: dependsOnSensor,
	}, nil
}

type helper struct {
	resource.Named
	resource.TriviallyCloseable
	logger              logging.Logger
	numReconfigurations int
	dependsOnSensor     sensor.Sensor
}

// DoCommand looks up the "real" command from the map it's passed.
func (h *helper) DoCommand(ctx context.Context, req map[string]interface{}) (map[string]interface{}, error) {
	cmd, ok := req["command"]
	if !ok {
		return nil, errors.New("missing 'command' string")
	}

	switch req["command"] {
	case "sleep":
		time.Sleep(time.Second * 1)
		//nolint:nilnil
		return nil, nil
	case "get_ops":
		// For testing the module's operation manager
		ops := myMod.OperationManager().All()
		var opsOut []string
		for _, op := range ops {
			opsOut = append(opsOut, op.ID.String())
		}
		return map[string]interface{}{"ops": opsOut}, nil
	case "echo":
		// For testing module liveliness
		return req, nil
	case "kill_module":
		// For testing module reloading & unexpected exists
		os.Exit(1)
		// unreachable return statement needed for compilation
		return nil, errors.New("unreachable error")
	case "write_data_file":
		// For testing that the module's data directory has been created and that the VIAM_MODULE_DATA env var exists
		filename, ok := req["filename"].(string)
		if !ok {
			return nil, errors.New("missing 'filename' string")
		}
		contents, ok := req["contents"].(string)
		if !ok {
			return nil, errors.New("missing 'contents' string")
		}
		dataFilePath := filepath.Join(os.Getenv("VIAM_MODULE_DATA"), filename)
		err := os.WriteFile(dataFilePath, []byte(contents), 0o600)
		if err != nil {
			return map[string]interface{}{}, err
		}
		return map[string]interface{}{"fullpath": dataFilePath}, nil
	case "get_working_directory":
		// For testing that modules are started with the correct working directory
		workingDir, err := os.Getwd()
		if err != nil {
			return map[string]interface{}{}, err
		}
		return map[string]interface{}{"path": workingDir}, nil
	case "log":
		level, err := logging.LevelFromString(req["level"].(string))
		if err != nil {
			return nil, err
		}

		msg := req["msg"].(string)
		switch level {
		case logging.DEBUG:
			h.logger.CDebugw(ctx, msg, "foo", "bar", "err", errors.New("crash me"))
		case logging.INFO:
			h.logger.CInfow(ctx, msg, "foo", "bar")
		case logging.WARN:
			h.logger.CWarnw(ctx, msg, "foo", "bar")
		case logging.ERROR:
			h.logger.CErrorw(ctx, msg, "foo", "bar")
		}

		// Beyond just logging at the specified level, also report the current log level back
		// in the DoCommand response.
		return map[string]any{"level": h.logger.GetLevel().String()}, nil
	case "get_num_reconfigurations":
		return map[string]any{"num_reconfigurations": h.numReconfigurations}, nil
	case "do_readings_on_dep":
		_, err := h.dependsOnSensor.Readings(ctx, nil)
		return nil, err
	case "get_trace_id":
		traceID := trace.FromContext(ctx).SpanContext().TraceID().String()
		return map[string]any{"trace_id": traceID}, nil
	default:
		return nil, fmt.Errorf("unknown command string %s", cmd)
	}
}

// Reconfigure increments numReconfigurations.
func (h *helper) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	h.numReconfigurations++
	return nil
}

func newOther(
	ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger,
) (resource.Resource, error) {
	return &other{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
	}, nil
}

type other struct {
	resource.Named
	resource.TriviallyCloseable
	logger              logging.Logger
	numReconfigurations int
}

// DoCommand looks up the "real" command from the map it's passed.
func (o *other) DoCommand(ctx context.Context, req map[string]interface{}) (map[string]interface{}, error) {
	cmd, ok := req["command"]
	if !ok {
		return nil, errors.New("missing 'command' string")
	}

	switch req["command"] {
	case "log":
		level, err := logging.LevelFromString(req["level"].(string))
		if err != nil {
			return nil, err
		}

		msg := req["msg"].(string)
		switch level {
		case logging.DEBUG:
			o.logger.CDebugw(ctx, msg, "foo", "bar")
		case logging.INFO:
			o.logger.CInfow(ctx, msg, "foo", "bar")
		case logging.WARN:
			o.logger.CWarnw(ctx, msg, "foo", "bar")
		case logging.ERROR:
			o.logger.CErrorw(ctx, msg, "foo", "bar")
		}
		return map[string]any{}, nil
	case "get_num_reconfigurations":
		return map[string]any{"num_reconfigurations": o.numReconfigurations}, nil
	default:
		return nil, fmt.Errorf("unknown command string %s", cmd)
	}
}

// Reconfigure increments numReconfigurations.
func (o *other) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	o.numReconfigurations++
	return nil
}

func newTestMotor(
	ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger,
) (resource.Resource, error) {
	return &testMotor{
		Named: conf.ResourceName().AsNamed(),
	}, nil
}

type testMotor struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
}

var _ motor.Motor = &testMotor{}

// SetPower trivially implements motor.Motor.
func (tm *testMotor) SetPower(_ context.Context, _ float64, _ map[string]interface{}) error {
	return nil
}

// GoFor trivially implements motor.Motor.
func (tm *testMotor) GoFor(_ context.Context, _, _ float64, _ map[string]interface{}) error {
	return nil
}

// GoTo trivially implements motor.Motor.
func (tm *testMotor) GoTo(_ context.Context, _, _ float64, _ map[string]interface{}) error {
	return nil
}

// SetRPM trivially implements motor.Motor.
func (tm *testMotor) SetRPM(_ context.Context, _ float64, _ map[string]interface{}) error {
	return nil
}

// ResetZeroPosition trivially implements motor.Motor.
func (tm *testMotor) ResetZeroPosition(_ context.Context, _ float64, _ map[string]interface{}) error {
	return nil
}

// Position trivially implements motor.Motor.
func (tm *testMotor) Position(_ context.Context, _ map[string]interface{}) (float64, error) {
	return 0.0, nil
}

// Properties trivially implements motor.Motor.
func (tm *testMotor) Properties(_ context.Context, _ map[string]interface{}) (motor.Properties, error) {
	return motor.Properties{}, nil
}

// Stop trivially implements motor.Motor.
func (tm *testMotor) Stop(_ context.Context, _ map[string]interface{}) error {
	return nil
}

// IsPowered trivally implements motor.Motor.
func (tm *testMotor) IsPowered(_ context.Context, _ map[string]interface{}) (bool, float64, error) {
	return false, 0.0, nil
}

// DoCommand trivially implements motor.Motor.
func (tm *testMotor) DoCommand(_ context.Context, _ map[string]interface{}) (map[string]interface{}, error) {
	//nolint:nilnil
	return nil, nil
}

// IsMoving trivially implements motor.Motor.
func (tm *testMotor) IsMoving(context.Context) (bool, error) {
	return false, nil
}

func newSlow(
	ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger,
) (resource.Resource, error) {
	configDuration, err := time.ParseDuration(conf.Attributes.String("config_duration"))
	if err != nil {
		return nil, err
	}
	time.Sleep(configDuration)
	return &slow{
		Named:          conf.ResourceName().AsNamed(),
		configDuration: configDuration,
	}, nil
}

type slow struct {
	resource.Named
	configDuration time.Duration
}

// Reconfigure does nothing but is slow.
func (s *slow) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	time.Sleep(s.configDuration)
	return nil
}

// Close does nothing but is slow.
func (s *slow) Close(ctx context.Context) error {
	time.Sleep(s.configDuration)
	return nil
}

func newFSDependent(
	ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger,
) (resource.Resource, error) {
	fs, err := framesystem.FromDependencies(deps)
	if err != nil {
		return nil, err
	}
	return &fsDependent{
		Named: conf.ResourceName().AsNamed(),
		fs:    fs,
	}, nil
}

type fsDepConfig struct{}

// Validate INCORRECTLY returns $framesystem, framesystem, and
// framesystem.PublicServiceName.String() as implicit dependencies (both required and
// optional). Validate methods do NOT need to do this, as the framesystem is always
// available in constructors and Reconfigure methods through framesystem.FromDependencies.
// The incorrect Validate method here is only used for testing that these specific string
// values are ignored in dependency calculation (resource can construct and reconfigure).
func (fsc *fsDepConfig) Validate(_ string) ([]string, []string, error) {
	return []string{"$framesystem", "framesystem"}, []string{framesystem.PublicServiceName.String()}, nil
}

type fsDependent struct {
	resource.Named
	resource.TriviallyCloseable
	resource.TriviallyReconfigurable
	fs framesystem.Service
}

// Reconfigure ensures that the framesystem is available in the dependencies passed to
// reconfigure (not just the constructor).
func (fd *fsDependent) Reconfigure(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
) error {
	fs, err := framesystem.FromProvider(deps)
	if err != nil {
		return err
	}
	fsCfg, err := fs.FrameSystemConfig(ctx)
	if err != nil {
		return err
	}
	if fsCfg == nil {
		return errors.New("received an empty framesystem config in Reconfigure")
	}
	return nil
}

// DoCommand always returns a stringified version of the frame system config as "fsCfg".
func (fd *fsDependent) DoCommand(ctx context.Context, _ map[string]interface{}) (map[string]interface{}, error) {
	fsCfg, err := fd.fs.FrameSystemConfig(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"fsCfg": fsCfg.String()}, nil
}

func newSensorDependent(
	ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger,
) (resource.Resource, error) {
	config, err := resource.NativeConfig[*sensorDepConfig](conf)
	if err != nil {
		return nil, err
	}
	s, err := sensor.FromProvider(deps, config.Sensor)
	if err != nil {
		return nil, err
	}
	return &sensorDependent{
		Named:  conf.ResourceName().AsNamed(),
		sensor: s,
	}, nil
}

type sensorDepConfig struct {
	Sensor string `json:"sensor"`
}

// Validate will ensure that sensor
func (sc *sensorDepConfig) Validate(_ string) ([]string, []string, error) {
	if sc.Sensor == "" {
		return nil, nil, errors.New("empty sensor")
	}
	return []string{sc.Sensor}, []string{}, nil
}

type sensorDependent struct {
	resource.Named
	resource.TriviallyCloseable
	resource.AlwaysRebuild
	sensor resource.Sensor
}

// Readings always returns Readings from the sensor held inside the struct.
func (sd *sensorDependent) Readings(ctx context.Context, _ map[string]interface{}) (map[string]interface{}, error) {
	return sd.sensor.Readings(ctx, map[string]interface{}{})
}
