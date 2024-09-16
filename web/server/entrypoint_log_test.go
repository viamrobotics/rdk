package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"testing"

	"go.viam.com/test"
	"go.viam.com/utils"
	gtestutils "go.viam.com/utils/testutils"

	componentgeneric "go.viam.com/rdk/components/generic"
	_ "go.viam.com/rdk/components/register"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/client"
	servicegeneric "go.viam.com/rdk/services/generic"
	_ "go.viam.com/rdk/services/register"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/robottestutils"
)

func TestLogConfiguration(t *testing.T) {
	fakeModel := resource.DefaultModelFamily.WithModel("fake")
	helperModel := resource.NewModel("rdk", "test", "helper")
	otherModel := resource.NewModel("rdk", "test", "other")
	testPath := testutils.BuildTempModule(t, "module/testmodule")

	cfg := &config.Config{
		Modules: []config.Module{
			{
				Name:    "mod",
				ExePath: testPath,
			},
		},
		Components: []resource.Config{
			{
				Name:  "foo",
				API:   componentgeneric.API,
				Model: fakeModel,
			},
			{
				Name:  "helper",
				API:   componentgeneric.API,
				Model: helperModel,
			},
		},
		Services: []resource.Config{
			{
				Name:  "bar",
				API:   servicegeneric.API,
				Model: fakeModel,
			},
			{
				Name:  "other",
				API:   servicegeneric.API,
				Model: otherModel,
			},
		},
		LogConfig: []logging.LoggerPatternConfig{
			{
				Pattern: "rdk.resource_manager.rdk:component:generic/foo",
				Level:   "info",
			},
			{
				Pattern: "rdk.resource_manager.rdk:component:generic/helper",
				Level:   "info",
			},
			{
				Pattern: "rdk.resource_manager.rdk:service:generic/bar",
				Level:   "info",
			},
			{
				Pattern: "rdk.resource_manager.rdk:service:generic/other",
				Level:   "info",
			},
		},
	}

	logger, logObserver, registry := logging.NewObservedTestLoggerWithRegistry(t)

	var port int
	var success bool
	var cfgFilename string
	for portTryNum := 0; portTryNum < 10; portTryNum++ {
		p, err := utils.TryReserveRandomPort()
		port = p
		test.That(t, err, test.ShouldBeNil)

		cfg.Network.BindAddress = fmt.Sprintf(":%d", port)

		cfgFilename, err = robottestutils.MakeTempConfig(t, cfg, logger)
		test.That(t, err, test.ShouldBeNil)
		args := Arguments{ConfigFile: cfgFilename}
		rs := &robotServer{args, logger, registry}
		robotCtx, cancel := context.WithCancel(context.Background())
		rs.runServer(robotCtx)

		if success = robottestutils.WaitForServing(logObserver, port); success {
			defer func() {
				cancel()
			}()
			break
		}
		logger.Infow("Port in use. Restarting on new port.", "port", port, "err", err)
		cancel()
		continue
	}
	test.That(t, success, test.ShouldBeTrue)

	// Assert that config and logger map eventually look as expected.
	registryConfig := registry.GetCurrentConfig()
	registryLoggers := registry.GetLoggers()

	gtestutils.WaitForAssertion(t, func(tb testing.TB) {
		test.That(tb, registryConfig, test.ShouldResemble, cfg.LogConfig)

		fooLogger, ok := registryLoggers["rdk.resource_manager.rdk:component:generic/foo"]
		test.That(tb, ok, test.ShouldBeTrue)
		test.That(tb, fooLogger.GetLevel(), test.ShouldEqual, logging.INFO)
		helperLogger, ok := registryLoggers["rdk.resource_manager.rdk:component:generic/helper"]
		test.That(tb, ok, test.ShouldBeTrue)
		test.That(tb, helperLogger.GetLevel(), test.ShouldEqual, logging.DEBUG)
		barLogger, ok := registryLoggers["rdk.resource_manager.rdk:service:generic/bar"]
		test.That(tb, ok, test.ShouldBeTrue)
		test.That(tb, barLogger.GetLevel(), test.ShouldEqual, logging.INFO)
		otherLogger, ok := registryLoggers["rdk.resource_manager.rdk:service:generic/other"]
		test.That(tb, ok, test.ShouldBeTrue)
		test.That(tb, otherLogger.GetLevel(), test.ShouldEqual, logging.INFO)
	})

	// Assert that modular resources are logging at the correct level (INFO) by
	// creating a client to the robot and running `DoCommand`.

	//clientLogger := logging.NewTestLogger(t)
	//rc, err := client.New(clientLogger, fmt.Sprintf("localhost:%d", port))
	//h, err := rc.ResourceByName(componentgeneric.Named("helper"))
	//test.That(t, err, test.ShouldBeNil)
	//_, err = h.DoCommand(context.Background(), map[string]any{"command": "log", "level": "debug"})
	//test.That(t, err, test.ShouldBeNil)

	// Mutate config to assert that `LogConfiguration` fields on resources take higher
	// priority than `LogConfig` fields.
	cfg1 := &config.Config{
		Modules: []config.Module{
			{
				Name:    "mod",
				ExePath: testPath,
			},
		},
		Components: []resource.Config{
			{
				Name:             "foo",
				API:              componentgeneric.API,
				Model:            fakeModel,
				LogConfiguration: &resource.LogConfig{Level: logging.DEBUG},
			},
			{
				Name:             "helper",
				API:              componentgeneric.API,
				Model:            helperModel,
				LogConfiguration: &resource.LogConfig{Level: logging.DEBUG},
			},
		},
		Services: []resource.Config{
			{
				Name:             "bar",
				API:              servicegeneric.API,
				Model:            fakeModel,
				LogConfiguration: &resource.LogConfig{Level: logging.DEBUG},
			},
			{
				Name:             "other",
				API:              servicegeneric.API,
				Model:            otherModel,
				LogConfiguration: &resource.LogConfig{Level: logging.DEBUG},
			},
		},
		LogConfig: []logging.LoggerPatternConfig{
			{
				Pattern: "rdk.resource_manager.rdk:component:generic/foo",
				Level:   "info",
			},
			{
				Pattern: "rdk.resource_manager.rdk:component:generic/helper",
				Level:   "info",
			},
			{
				Pattern: "rdk.resource_manager.rdk:service:generic/bar",
				Level:   "info",
			},
			{
				Pattern: "rdk.resource_manager.rdk:service:generic/other",
				Level:   "info",
			},
		},
	}

	// Overwrite file with JSON contents of cfg. Rely on entrypoint code's
	// file watcher to pick up the change.
	overwriteConfigFile(t, cfg1, cfgFilename)

	// Assert that config and logger map eventually look as expected (all loggers now DEBUG).
	registryConfig = registry.GetCurrentConfig()
	registryLoggers = registry.GetLoggers()

	gtestutils.WaitForAssertion(t, func(tb testing.TB) {
		test.That(tb, registryConfig, test.ShouldResemble, cfg.LogConfig)

		fooLogger, ok := registryLoggers["rdk.resource_manager.rdk:component:generic/foo"]
		test.That(tb, ok, test.ShouldBeTrue)
		test.That(tb, fooLogger.GetLevel(), test.ShouldEqual, logging.DEBUG)
		helperLogger, ok := registryLoggers["rdk.resource_manager.rdk:component:generic/helper"]
		test.That(tb, ok, test.ShouldBeTrue)
		test.That(tb, helperLogger.GetLevel(), test.ShouldEqual, logging.DEBUG)
		barLogger, ok := registryLoggers["rdk.resource_manager.rdk:service:generic/bar"]
		test.That(tb, ok, test.ShouldBeTrue)
		test.That(tb, barLogger.GetLevel(), test.ShouldEqual, logging.DEBUG)
		otherLogger, ok := registryLoggers["rdk.resource_manager.rdk:service:generic/other"]
		test.That(tb, ok, test.ShouldBeTrue)
		test.That(tb, otherLogger.GetLevel(), test.ShouldEqual, logging.DEBUG)
	})

	// TODO(benji): Mutate again to remove all logger patterns. Assert everything
	// returns to DEBUG level.
}

func overwriteConfigFile(t *testing.T, cfg *config.Config, filename string) {
	t.Helper()

	output, err := json.Marshal(cfg)
	test.That(t, err, test.ShouldBeNil)
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_TRUNC, 0644)
	test.That(t, err, test.ShouldBeNil)
	_, writeErr := file.Write(output)
	// Make sure to close file even if writing failed.
	closeErr := file.Close()
	test.That(t, writeErr, test.ShouldBeNil)
	test.That(t, closeErr, test.ShouldBeNil)
}
