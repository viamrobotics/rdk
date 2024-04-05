package modmanager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap/zaptest/observer"
	v1 "go.viam.com/api/module/v1"
	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	modlib "go.viam.com/rdk/module"
	modmanageroptions "go.viam.com/rdk/module/modmanager/options"
	"go.viam.com/rdk/resource"
	rtestutils "go.viam.com/rdk/testutils"
	rutils "go.viam.com/rdk/utils"
)

func TestModManagerFunctions(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	// Precompile module copies to avoid timeout issues when building takes too long.
	modPath := rtestutils.BuildTempModule(t, "examples/customresources/demos/simplemodule")
	modPath2 := rtestutils.BuildTempModule(t, "examples/customresources/demos/simplemodule")

	myCounterModel := resource.NewModel("acme", "demo", "mycounter")
	rNameCounter1 := resource.NewName(generic.API, "counter1")
	cfgCounter1 := resource.Config{
		Name:  "counter1",
		API:   generic.API,
		Model: myCounterModel,
	}
	_, err := cfgCounter1.Validate("test", resource.APITypeComponentName)
	test.That(t, err, test.ShouldBeNil)

	parentAddr, err := modlib.CreateSocketAddress(t.TempDir(), "parent")
	test.That(t, err, test.ShouldBeNil)
	fakeRobot := rtestutils.MakeRobotForModuleLogging(t, parentAddr)
	defer func() {
		test.That(t, fakeRobot.Stop(), test.ShouldBeNil)
	}()

	t.Log("test Helpers")
	viamHomeTemp := t.TempDir()
	mgr := NewManager(ctx, parentAddr, logger, modmanageroptions.Options{UntrustedEnv: false, ViamHomeDir: viamHomeTemp})

	mod := &module{
		cfg: config.Module{
			Name:        "test",
			ExePath:     modPath,
			Type:        config.ModuleTypeRegistry,
			ModuleID:    "new:york",
			Environment: map[string]string{"SMART": "MACHINES"},
		},
		dataDir: "module-data-dir",
	}

	err = mod.startProcess(ctx, parentAddr, nil, logger, viamHomeTemp)
	test.That(t, err, test.ShouldBeNil)

	err = mod.dial()
	test.That(t, err, test.ShouldBeNil)

	err = mod.checkReady(ctx, parentAddr, logger)
	test.That(t, err, test.ShouldBeNil)

	mod.registerResources(mgr, logger)
	reg, ok := resource.LookupRegistration(generic.API, myCounterModel)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, reg, test.ShouldNotBeNil)
	test.That(t, reg.Constructor, test.ShouldNotBeNil)

	mod.deregisterResources()
	_, ok = resource.LookupRegistration(generic.API, myCounterModel)
	test.That(t, ok, test.ShouldBeFalse)

	test.That(t, mod.process.Stop(), test.ShouldBeNil)

	modEnv := mod.getFullEnvironment(viamHomeTemp)
	test.That(t, modEnv["VIAM_HOME"], test.ShouldEqual, viamHomeTemp)
	test.That(t, modEnv["VIAM_MODULE_DATA"], test.ShouldEqual, "module-data-dir")
	test.That(t, modEnv["VIAM_MODULE_ID"], test.ShouldEqual, "new:york")
	test.That(t, modEnv["SMART"], test.ShouldEqual, "MACHINES")

	// Test that VIAM_MODULE_ID is unset for local modules
	mod.cfg.Type = config.ModuleTypeLocal
	modEnv = mod.getFullEnvironment(viamHomeTemp)
	_, ok = modEnv["VIAM_MODULE_ID"]
	test.That(t, ok, test.ShouldBeFalse)

	// Make a copy of addr and client to test that connections are properly remade
	oldAddr := mod.addr
	oldClient := mod.client

	mod.startProcess(ctx, parentAddr, nil, logger, viamHomeTemp)
	err = mod.dial()
	test.That(t, err, test.ShouldBeNil)

	// make sure mod.addr has changed
	test.That(t, mod.addr, test.ShouldNotEqual, oldAddr)
	// check that we're still able to use the old client
	_, err = oldClient.Ready(ctx, &v1.ReadyRequest{ParentAddress: parentAddr})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, mod.process.Stop(), test.ShouldBeNil)
	test.That(t, mgr.Close(ctx), test.ShouldBeNil)

	t.Log("test AddModule")
	mgr = NewManager(ctx, parentAddr, logger, modmanageroptions.Options{UntrustedEnv: false})
	test.That(t, err, test.ShouldBeNil)

	modCfg := config.Module{
		Name:    "simple-module",
		ExePath: modPath,
	}
	err = mgr.Add(ctx, modCfg)
	test.That(t, err, test.ShouldBeNil)

	reg, ok = resource.LookupRegistration(generic.API, myCounterModel)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, reg.Constructor, test.ShouldNotBeNil)

	t.Log("test Provides")
	ok = mgr.Provides(cfgCounter1)
	test.That(t, ok, test.ShouldBeTrue)

	cfg2 := resource.Config{
		API:   motor.API,
		Model: resource.DefaultModelFamily.WithModel("fake"),
	}
	ok = mgr.Provides(cfg2)
	test.That(t, ok, test.ShouldBeFalse)

	t.Log("test AddResource")
	counter, err := mgr.AddResource(ctx, cfgCounter1, nil)
	test.That(t, err, test.ShouldBeNil)

	ret, err := counter.DoCommand(ctx, map[string]interface{}{"command": "get"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ret["total"], test.ShouldEqual, 0)

	t.Log("test IsModularResource")
	ok = mgr.IsModularResource(rNameCounter1)
	test.That(t, ok, test.ShouldBeTrue)

	ok = mgr.IsModularResource(resource.NewName(generic.API, "missing"))
	test.That(t, ok, test.ShouldBeFalse)

	t.Log("test ValidateConfig")
	// ValidateConfig for cfgCounter1 will not actually call any Validate functionality,
	// as the mycounter model does not have a configuration object with Validate.
	// Assert that ValidateConfig does not fail in this case (allows unimplemented
	// validation).
	deps, err := mgr.ValidateConfig(ctx, cfgCounter1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, deps, test.ShouldBeNil)

	t.Log("test ReconfigureResource")
	// Reconfigure should replace the proxied object, resetting the counter
	ret, err = counter.DoCommand(ctx, map[string]interface{}{"command": "add", "value": 73})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ret["total"], test.ShouldEqual, 73)

	err = mgr.ReconfigureResource(ctx, cfgCounter1, nil)
	test.That(t, err, test.ShouldBeNil)

	ret, err = counter.DoCommand(ctx, map[string]interface{}{"command": "get"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ret["total"], test.ShouldEqual, 0)

	t.Log("test RemoveResource")
	err = mgr.RemoveResource(ctx, rNameCounter1)
	test.That(t, err, test.ShouldBeNil)

	ok = mgr.IsModularResource(rNameCounter1)
	test.That(t, ok, test.ShouldBeFalse)

	err = mgr.RemoveResource(ctx, rNameCounter1)
	test.That(t, err, test.ShouldNotBeNil)

	_, err = counter.DoCommand(ctx, map[string]interface{}{"command": "get"})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

	t.Log("test ReconfigureModule")
	// Re-add counter1.
	_, err = mgr.AddResource(ctx, cfgCounter1, nil)
	test.That(t, err, test.ShouldBeNil)
	// Add 24 to counter and ensure 'total' gets reset after reconfiguration.
	ret, err = counter.DoCommand(ctx, map[string]interface{}{"command": "add", "value": 24})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ret["total"], test.ShouldEqual, 24)

	// Change underlying binary path of module to be a different copy of the same module
	modCfg.ExePath = modPath2

	// Reconfigure module with new ExePath.
	orphanedResourceNames, err := mgr.Reconfigure(ctx, modCfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, orphanedResourceNames, test.ShouldResemble, []resource.Name{rNameCounter1})

	t.Log("test RemoveModule")
	orphanedResourceNames, err = mgr.Remove("simple-module")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, orphanedResourceNames, test.ShouldBeNil)

	ok = mgr.IsModularResource(rNameCounter1)
	test.That(t, ok, test.ShouldBeFalse)
	_, err = counter.DoCommand(ctx, map[string]interface{}{"command": "get"})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not connected")

	err = counter.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	err = mgr.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	t.Log("test UntrustedEnv")
	mgr = NewManager(ctx, parentAddr, logger, modmanageroptions.Options{UntrustedEnv: true})

	modCfg = config.Module{
		Name:    "simple-module",
		ExePath: modPath,
	}
	err = mgr.Add(ctx, modCfg)
	test.That(t, err, test.ShouldEqual, errModularResourcesDisabled)

	t.Log("test empty dir for CleanModuleDataDirectory")
	mgr = NewManager(ctx, parentAddr, logger, modmanageroptions.Options{UntrustedEnv: false, ViamHomeDir: ""})
	err = mgr.CleanModuleDataDirectory()
	test.That(t, fmt.Sprint(err), test.ShouldContainSubstring, "cannot clean a root level module data directory")

	t.Log("test CleanModuleDataDirectory")
	viamHomeTemp = t.TempDir()
	robotCloudID := "a-b-c-d"
	expectedDataDir := filepath.Join(viamHomeTemp, parentModuleDataFolderName, robotCloudID)
	mgr = NewManager(
		ctx,
		parentAddr,
		logger,
		modmanageroptions.Options{UntrustedEnv: false, ViamHomeDir: viamHomeTemp, RobotCloudID: robotCloudID},
	)
	// check that premature clean is okay
	err = mgr.CleanModuleDataDirectory()
	test.That(t, err, test.ShouldBeNil)
	// create a module and add it to the modmanager
	modCfg = config.Module{
		Name:    "simple-module",
		ExePath: modPath,
	}
	err = mgr.Add(ctx, modCfg)
	test.That(t, err, test.ShouldBeNil)
	// check that we created the expected directory
	moduleDataDir := filepath.Join(expectedDataDir, modCfg.Name)
	_, err = os.Stat(moduleDataDir)
	test.That(t, err, test.ShouldBeNil)
	// make unwanted / unexpected directory
	litterDataDir := filepath.Join(expectedDataDir, "litter")
	err = os.MkdirAll(litterDataDir, os.ModePerm)
	test.That(t, err, test.ShouldBeNil)
	// clean
	err = mgr.CleanModuleDataDirectory()
	test.That(t, err, test.ShouldBeNil)
	// check that the module directory still exists
	_, err = os.Stat(moduleDataDir)
	test.That(t, err, test.ShouldBeNil)
	// check that the litter directory is removed
	_, err = os.Stat(litterDataDir)
	test.That(t, err, test.ShouldBeError)
	// remove the module and verify that the entire directory is removed
	_, err = mgr.Remove("simple-module")
	test.That(t, err, test.ShouldBeNil)
	// clean
	err = mgr.CleanModuleDataDirectory()
	test.That(t, err, test.ShouldBeNil)
	_, err = os.Stat(expectedDataDir)
	test.That(t, err, test.ShouldBeError)
}

func TestModManagerValidation(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	// Precompile module to avoid timeout issues when building takes too long.
	modPath := rtestutils.BuildTempModule(t, "examples/customresources/demos/complexmodule")

	myBaseModel := resource.NewModel("acme", "demo", "mybase")
	cfgMyBase1 := resource.Config{
		Name:  "mybase1",
		API:   base.API,
		Model: myBaseModel,
		Attributes: map[string]interface{}{
			"motorL": "motor1",
			"motorR": "motor2",
		},
	}
	_, err := cfgMyBase1.Validate("test", resource.APITypeComponentName)
	test.That(t, err, test.ShouldBeNil)
	// cfgMyBase2 is missing required attributes "motorL" and "motorR" and should
	// cause module Validation error.
	cfgMyBase2 := resource.Config{
		Name:  "mybase2",
		API:   base.API,
		Model: myBaseModel,
	}
	_, err = cfgMyBase2.Validate("test", resource.APITypeComponentName)
	test.That(t, err, test.ShouldBeNil)

	parentAddr, err := modlib.CreateSocketAddress(t.TempDir(), "parent")
	test.That(t, err, test.ShouldBeNil)
	fakeRobot := rtestutils.MakeRobotForModuleLogging(t, parentAddr)
	defer func() {
		test.That(t, fakeRobot.Stop(), test.ShouldBeNil)
	}()

	t.Log("adding complex module")
	mgr := NewManager(ctx, parentAddr, logger, modmanageroptions.Options{UntrustedEnv: false})

	modCfg := config.Module{
		Name:    "complex-module",
		ExePath: modPath,
	}
	err = mgr.Add(ctx, modCfg)
	test.That(t, err, test.ShouldBeNil)

	reg, ok := resource.LookupRegistration(base.API, myBaseModel)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, reg.Constructor, test.ShouldNotBeNil)

	t.Log("test ValidateConfig")
	deps, err := mgr.ValidateConfig(ctx, cfgMyBase1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, deps, test.ShouldNotBeNil)
	test.That(t, deps[0], test.ShouldResemble, "motor1")
	test.That(t, deps[1], test.ShouldResemble, "motor2")

	_, err = mgr.ValidateConfig(ctx, cfgMyBase2)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldResemble,
		`rpc error: code = Unknown desc = error validating resource: expected "motorL" attribute for mybase "mybase2"`)

	// Test that ValidateConfig respects validateConfigTimeout by artificially
	// lowering it to an impossibly small duration.
	validateConfigTimeout = 1 * time.Nanosecond
	_, err = mgr.ValidateConfig(ctx, cfgMyBase1)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldResemble,
		"rpc error: code = DeadlineExceeded desc = context deadline exceeded")

	err = mgr.Close(ctx)
	test.That(t, err, test.ShouldBeNil)
}

func TestModuleReloading(t *testing.T) {
	ctx := context.Background()

	// Lower global timeout early to avoid race with actual restart code.
	defer func(oriOrigVal time.Duration) {
		oueRestartInterval = oriOrigVal
	}(oueRestartInterval)
	oueRestartInterval = 10 * time.Millisecond

	myHelperModel := resource.NewModel("rdk", "test", "helper")
	rNameMyHelper := generic.Named("myhelper")
	cfgMyHelper := resource.Config{
		Name:  "myhelper",
		API:   generic.API,
		Model: myHelperModel,
	}
	_, err := cfgMyHelper.Validate("test", resource.APITypeComponentName)
	test.That(t, err, test.ShouldBeNil)

	parentAddr, err := modlib.CreateSocketAddress(t.TempDir(), "parent")
	test.That(t, err, test.ShouldBeNil)
	fakeRobot := rtestutils.MakeRobotForModuleLogging(t, parentAddr)
	defer func() {
		test.That(t, fakeRobot.Stop(), test.ShouldBeNil)
	}()

	modCfg := config.Module{Name: "test-module"}

	t.Run("successful restart", func(t *testing.T) {
		logger, logs := logging.NewObservedTestLogger(t)

		// Precompile module to avoid timeout issues when building takes too long.
		modCfg.ExePath = rtestutils.BuildTempModule(t, "module/testmodule")

		// This test neither uses a resource manager nor asserts anything about
		// the existence of resources in the graph. Use a dummy
		// RemoveOrphanedResources function so orphaned resource logic does not
		// panic.
		var dummyRemoveOrphanedResourcesCallCount atomic.Uint64
		dummyRemoveOrphanedResources := func(context.Context, []resource.Name) {
			dummyRemoveOrphanedResourcesCallCount.Add(1)
		}
		mgr := NewManager(ctx, parentAddr, logger, modmanageroptions.Options{
			UntrustedEnv:            false,
			RemoveOrphanedResources: dummyRemoveOrphanedResources,
		})
		err = mgr.Add(ctx, modCfg)
		test.That(t, err, test.ShouldBeNil)

		// Add helper resource and ensure "echo" works correctly.
		h, err := mgr.AddResource(ctx, cfgMyHelper, nil)
		test.That(t, err, test.ShouldBeNil)
		ok := mgr.IsModularResource(rNameMyHelper)
		test.That(t, ok, test.ShouldBeTrue)

		resp, err := h.DoCommand(ctx, map[string]interface{}{"command": "echo"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldNotBeNil)
		test.That(t, resp["command"], test.ShouldEqual, "echo")

		// Run 'kill_module' command through helper resource to cause module to exit
		// with error. Assert that after module is restarted, helper is modularly
		// managed again and remains functional.
		_, err = h.DoCommand(ctx, map[string]interface{}{"command": "kill_module"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "rpc error")

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, logs.FilterMessageSnippet("Module successfully restarted").Len(),
				test.ShouldEqual, 1)
		})

		ok = mgr.IsModularResource(rNameMyHelper)
		test.That(t, ok, test.ShouldBeTrue)
		resp, err = h.DoCommand(ctx, map[string]interface{}{"command": "echo"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldNotBeNil)
		test.That(t, resp["command"], test.ShouldEqual, "echo")

		err = mgr.Close(ctx)
		test.That(t, err, test.ShouldBeNil)

		// Assert that logs reflect that test-module crashed and there were no
		// errors during restart.
		test.That(t, logs.FilterMessageSnippet("Module has unexpectedly exited").Len(),
			test.ShouldEqual, 1)
		test.That(t, logs.FilterMessageSnippet("Error while restarting crashed module").Len(),
			test.ShouldEqual, 0)

		// Assert that RemoveOrphanedResources was called once.
		test.That(t, dummyRemoveOrphanedResourcesCallCount.Load(), test.ShouldEqual, 1)
	})
	t.Run("unsuccessful restart", func(t *testing.T) {
		logger, logs := logging.NewObservedTestLogger(t)

		// Precompile module to avoid timeout issues when building takes too long.
		modCfg.ExePath = rtestutils.BuildTempModule(t, "module/testmodule")

		// This test neither uses a resource manager nor asserts anything about
		// the existence of resources in the graph. Use a dummy
		// RemoveOrphanedResources function so orphaned resource logic does not
		// panic.
		var dummyRemoveOrphanedResourcesCallCount atomic.Uint64
		dummyRemoveOrphanedResources := func(context.Context, []resource.Name) {
			dummyRemoveOrphanedResourcesCallCount.Add(1)
		}
		mgr := NewManager(ctx, parentAddr, logger, modmanageroptions.Options{
			UntrustedEnv:            false,
			RemoveOrphanedResources: dummyRemoveOrphanedResources,
		})
		err = mgr.Add(ctx, modCfg)
		test.That(t, err, test.ShouldBeNil)

		// Add helper resource and ensure "echo" works correctly.
		h, err := mgr.AddResource(ctx, cfgMyHelper, nil)
		test.That(t, err, test.ShouldBeNil)
		ok := mgr.IsModularResource(rNameMyHelper)
		test.That(t, ok, test.ShouldBeTrue)

		resp, err := h.DoCommand(ctx, map[string]interface{}{"command": "echo"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldNotBeNil)
		test.That(t, resp["command"], test.ShouldEqual, "echo")

		// Remove testmodule binary, so process cannot be successfully restarted
		// after crash.
		err = os.Remove(modCfg.ExePath)
		test.That(t, err, test.ShouldBeNil)

		// Run 'kill_module' command through helper resource to cause module to
		// exit with error. Assert that after three restart errors occur, helper is
		// not modularly managed and commands return error.
		_, err = h.DoCommand(ctx, map[string]interface{}{"command": "kill_module"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "rpc error")

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, logs.FilterMessageSnippet("Error while restarting crashed module").Len(),
				test.ShouldEqual, 3)
		})

		ok = mgr.IsModularResource(rNameMyHelper)
		test.That(t, ok, test.ShouldBeFalse)
		_, err = h.DoCommand(ctx, map[string]interface{}{"command": "echo"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not connected")

		err = mgr.Close(ctx)
		test.That(t, err, test.ShouldBeNil)

		// Assert that logs reflect that test-module crashed and was not
		// successfully restarted.
		test.That(t, logs.FilterMessageSnippet("Module has unexpectedly exited").Len(),
			test.ShouldEqual, 1)
		test.That(t, logs.FilterMessageSnippet("Module successfully restarted").Len(),
			test.ShouldEqual, 0)

		// Assert that RemoveOrphanedResources was called once.
		test.That(t, dummyRemoveOrphanedResourcesCallCount.Load(), test.ShouldEqual, 1)
	})
	t.Run("do not restart if context canceled", func(t *testing.T) {
		logger, logs := logging.NewObservedTestLogger(t)

		// Precompile module to avoid timeout issues when building takes too long.
		modCfg.ExePath = rtestutils.BuildTempModule(t, "module/testmodule")

		// This test neither uses a resource manager nor asserts anything about
		// the existence of resources in the graph. Use a dummy
		// RemoveOrphanedResources function so orphaned resource logic does not
		// panic.
		var dummyRemoveOrphanedResourcesCallCount atomic.Uint64
		dummyRemoveOrphanedResources := func(context.Context, []resource.Name) {
			dummyRemoveOrphanedResourcesCallCount.Add(1)
		}
		mgr := NewManager(ctx, parentAddr, logger, modmanageroptions.Options{
			UntrustedEnv:            false,
			RemoveOrphanedResources: dummyRemoveOrphanedResources,
		})
		err = mgr.Add(ctx, modCfg)
		test.That(t, err, test.ShouldBeNil)

		// Add helper resource and ensure "echo" works correctly.
		h, err := mgr.AddResource(ctx, cfgMyHelper, nil)
		test.That(t, err, test.ShouldBeNil)
		ok := mgr.IsModularResource(rNameMyHelper)
		test.That(t, ok, test.ShouldBeTrue)

		mgr.(*Manager).restartCtxCancel()

		// Run 'kill_module' command through helper resource to cause module to
		// exit with error. Assert that we do not restart the module if context is cancelled.
		_, err = h.DoCommand(ctx, map[string]interface{}{"command": "kill_module"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "rpc error")

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, logs.FilterMessageSnippet("Will not attempt to restart crashed module").Len(),
				test.ShouldEqual, 1)
		})

		ok = mgr.IsModularResource(rNameMyHelper)
		test.That(t, ok, test.ShouldBeFalse)
		_, err = h.DoCommand(ctx, map[string]interface{}{"command": "echo"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not connected")

		err = mgr.Close(ctx)
		test.That(t, err, test.ShouldBeNil)

		// Assert that logs reflect that test-module crashed and was not
		// successfully restarted.
		test.That(t, logs.FilterMessageSnippet("Module has unexpectedly exited").Len(),
			test.ShouldEqual, 1)
		test.That(t, logs.FilterMessageSnippet("Module successfully restarted").Len(),
			test.ShouldEqual, 0)

		// Assert that RemoveOrphanedResources was called once.
		test.That(t, dummyRemoveOrphanedResourcesCallCount.Load(), test.ShouldEqual, 1)
	})
	t.Run("timed out module process is stopped", func(t *testing.T) {
		logger, logs := logging.NewObservedTestLogger(t)

		modCfg.ExePath = rutils.ResolveFile("module/testmodule/fakemodule.sh")

		// Lower module startup timeout to avoid waiting for 5 mins.
		t.Setenv(rutils.ModuleStartupTimeoutEnvVar, "10ms")

		// This test neither uses a resource manager nor asserts anything about
		// the existence of resources in the graph. Use a dummy
		// RemoveOrphanedResources function so orphaned resource logic does not
		// panic.
		dummyRemoveOrphanedResources := func(context.Context, []resource.Name) {}
		mgr := NewManager(ctx, parentAddr, logger, modmanageroptions.Options{
			UntrustedEnv:            false,
			RemoveOrphanedResources: dummyRemoveOrphanedResources,
		})
		err = mgr.Add(ctx, modCfg)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring,
			"module test-module timed out after 10ms during startup")

		// Assert that number of "fakemodule is running" messages does not increase
		// over time (the process was stopped).
		msgNum := logs.FilterMessageSnippet("fakemodule is running").Len()
		time.Sleep(100 * time.Millisecond)
		test.That(t, logs.FilterMessageSnippet("fakemodule is running").Len(), test.ShouldEqual, msgNum)

		// Assert that manager removes module.
		test.That(t, len(mgr.Configs()), test.ShouldEqual, 0)

		err = mgr.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("cancelled module process is stopped", func(t *testing.T) {
		logger, logs := logging.NewObservedTestLogger(t)

		modCfg.ExePath = rutils.ResolveFile("module/testmodule/fakemodule.sh")

		// Lower module startup timeout to avoid waiting for 5 mins.
		t.Setenv(rutils.ModuleStartupTimeoutEnvVar, "30s")

		// This test neither uses a resource manager nor asserts anything about
		// the existence of resources in the graph. Use a dummy
		// RemoveOrphanedResources function so orphaned resource logic does not
		// panic.
		ctx, cancel := context.WithCancel(ctx)
		cancel()
		dummyRemoveOrphanedResources := func(context.Context, []resource.Name) {}
		mgr := NewManager(ctx, parentAddr, logger, modmanageroptions.Options{
			UntrustedEnv:            false,
			RemoveOrphanedResources: dummyRemoveOrphanedResources,
		})
		err = mgr.Add(ctx, modCfg)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "context canceled")

		// Assert that number of "fakemodule is running" messages does not increase
		// over time (the process was stopped).
		msgNum := logs.FilterMessageSnippet("fakemodule is running").Len()
		time.Sleep(100 * time.Millisecond)
		test.That(t, logs.FilterMessageSnippet("fakemodule is running").Len(), test.ShouldEqual, msgNum)

		// Assert that manager removes module.
		test.That(t, len(mgr.Configs()), test.ShouldEqual, 0)

		err = mgr.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
	})
}

func TestDebugModule(t *testing.T) {
	ctx := context.Background()

	// Precompile module to avoid timeout issues when building takes too long.
	modPath := rtestutils.BuildTempModule(t, "module/testmodule")

	parentAddr, err := modlib.CreateSocketAddress(t.TempDir(), "parent")
	test.That(t, err, test.ShouldBeNil)
	fakeRobot := rtestutils.MakeRobotForModuleLogging(t, parentAddr)
	defer func() {
		test.That(t, fakeRobot.Stop(), test.ShouldBeNil)
	}()

	testCases := []struct {
		name                   string
		managerDebugEnabled    bool
		moduleLogLevel         string
		debugStatementExpected bool
	}{
		{
			"manager false debug/module empty log",
			false,
			"",
			false,
		},
		{
			"manager false debug/module info log",
			false,
			"info",
			false,
		},
		{
			"manager false debug/module debug log",
			false,
			"debug",
			true,
		},
		{
			"manager true debug/module empty log",
			true,
			"",
			true,
		},
		{
			"manager true debug/module info log",
			true,
			"info",
			false,
		},
		{
			"manager true debug/module debug log",
			true,
			"debug",
			true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var logger logging.Logger
			var logs *observer.ObservedLogs
			if tc.managerDebugEnabled {
				logger, logs = logging.NewObservedTestLogger(t)
			} else {
				logger, logs = rtestutils.NewInfoObservedTestLogger(t)
			}
			mgr := NewManager(ctx, parentAddr, logger, modmanageroptions.Options{UntrustedEnv: false})
			defer mgr.Close(ctx)

			modCfg := config.Module{
				Name:     "test-module",
				ExePath:  modPath,
				LogLevel: tc.moduleLogLevel,
			}

			err = mgr.Add(ctx, modCfg)
			test.That(t, err, test.ShouldBeNil)

			if tc.debugStatementExpected {
				testutils.WaitForAssertion(t, func(tb testing.TB) {
					test.That(tb, logs.FilterMessageSnippet("debug mode enabled").Len(),
						test.ShouldEqual, 1)
				})
				return
			}

			// Assert that after two seconds, "debug mode enabled" debug log is not
			// printed by testmodule
			time.Sleep(2 * time.Second)
			test.That(t, logs.FilterMessageSnippet("debug mode enabled").Len(),
				test.ShouldEqual, 0)
		})
	}
}

func TestModuleMisc(t *testing.T) {
	ctx := context.Background()

	parentAddr, err := modlib.CreateSocketAddress(t.TempDir(), "parent")
	test.That(t, err, test.ShouldBeNil)
	fakeRobot := rtestutils.MakeRobotForModuleLogging(t, parentAddr)
	defer func() {
		test.That(t, fakeRobot.Stop(), test.ShouldBeNil)
	}()

	// Build the testmodule
	modPath := rtestutils.BuildTempModule(t, "module/testmodule")
	modCfg := config.Module{
		Name:    "test-module",
		ExePath: modPath,
		Type:    config.ModuleTypeLocal,
	}

	testViamHomeDir := t.TempDir()
	// Add a component that uses the module
	myHelperModel := resource.NewModel("rdk", "test", "helper")
	rNameMyHelper := generic.Named("myhelper")
	cfgMyHelper := resource.Config{
		Name:  "myhelper",
		API:   generic.API,
		Model: myHelperModel,
	}

	t.Run("data directory fullstack", func(t *testing.T) {
		logger, logs := logging.NewObservedTestLogger(t)
		mgr := NewManager(ctx, parentAddr, logger, modmanageroptions.Options{
			UntrustedEnv: false,
			ViamHomeDir:  testViamHomeDir,
		})
		// Test that cleaning the data directory before it has been created does not produce log messages
		err = mgr.CleanModuleDataDirectory()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, logs.FilterMessageSnippet("Removing module data").Len(), test.ShouldEqual, 0)

		// Add the module
		err = mgr.Add(ctx, modCfg)
		test.That(t, err, test.ShouldBeNil)

		_, err = cfgMyHelper.Validate("test", resource.APITypeComponentName)
		test.That(t, err, test.ShouldBeNil)

		h, err := mgr.AddResource(ctx, cfgMyHelper, nil)
		test.That(t, err, test.ShouldBeNil)
		ok := mgr.IsModularResource(rNameMyHelper)
		test.That(t, ok, test.ShouldBeTrue)

		// Create a file in the modules data directory and then verify that it was written
		resp, err := h.DoCommand(ctx, map[string]interface{}{
			"command":  "write_data_file",
			"filename": "data.txt",
			"contents": "hello, world!",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldNotBeNil)
		dataFullPath, ok := resp["fullpath"].(string)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, dataFullPath, test.ShouldEqual, filepath.Join(testViamHomeDir, "module-data", "local", "test-module", "data.txt"))
		dataFileContents, err := os.ReadFile(dataFullPath)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(dataFileContents), test.ShouldEqual, "hello, world!")

		err = mgr.RemoveResource(ctx, rNameMyHelper)
		test.That(t, err, test.ShouldBeNil)
		_, err = mgr.Remove("test-module")
		test.That(t, err, test.ShouldBeNil)
		// test that the data directory is cleaned up
		err = mgr.CleanModuleDataDirectory()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, logs.FilterMessageSnippet("Removing module data").Len(), test.ShouldEqual, 1)
		_, err = os.Stat(filepath.Join(testViamHomeDir, "module-data", "local"))
		test.That(t, fmt.Sprint(err), test.ShouldContainSubstring, "no such file or directory")

		err = mgr.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
	})
	t.Run("module working directory", func(t *testing.T) {
		logger := logging.NewTestLogger(t)
		mgr := NewManager(ctx, parentAddr, logger, modmanageroptions.Options{
			UntrustedEnv: false,
			ViamHomeDir:  testViamHomeDir,
		})

		// Add the module with a user-specified VIAM_MODULE_ROOT
		modCfg := config.Module{
			Name:        "test-module",
			ExePath:     modPath,
			Environment: map[string]string{"VIAM_MODULE_ROOT": "/"},
			Type:        config.ModuleTypeLocal,
		}
		err = mgr.Add(ctx, modCfg)
		test.That(t, err, test.ShouldBeNil)

		_, err = cfgMyHelper.Validate("test", resource.APITypeComponentName)
		test.That(t, err, test.ShouldBeNil)

		h, err := mgr.AddResource(ctx, cfgMyHelper, nil)
		test.That(t, err, test.ShouldBeNil)
		ok := mgr.IsModularResource(rNameMyHelper)
		test.That(t, ok, test.ShouldBeTrue)

		// Create a file in the modules data directory and then verify that it was written
		resp, err := h.DoCommand(ctx, map[string]interface{}{
			"command": "get_working_directory",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldNotBeNil)
		modWorkingDirectory, ok := resp["path"].(string)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, modWorkingDirectory, test.ShouldEqual, "/")

		err = mgr.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
	})
	t.Run("module working directory fallback", func(t *testing.T) {
		logger := logging.NewTestLogger(t)
		mgr := NewManager(ctx, parentAddr, logger, modmanageroptions.Options{
			UntrustedEnv: false,
			ViamHomeDir:  testViamHomeDir,
		})
		// Add the module
		err = mgr.Add(ctx, modCfg)
		test.That(t, err, test.ShouldBeNil)

		_, err = cfgMyHelper.Validate("test", resource.APITypeComponentName)
		test.That(t, err, test.ShouldBeNil)

		h, err := mgr.AddResource(ctx, cfgMyHelper, nil)
		test.That(t, err, test.ShouldBeNil)
		ok := mgr.IsModularResource(rNameMyHelper)
		test.That(t, ok, test.ShouldBeTrue)

		// Create a file in the modules data directory and then verify that it was written
		resp, err := h.DoCommand(ctx, map[string]interface{}{
			"command": "get_working_directory",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldNotBeNil)
		modWorkingDirectory, ok := resp["path"].(string)
		test.That(t, ok, test.ShouldBeTrue)
		// MacOS prepends "/private/" to the filepath so we check the end of the path to account for this
		// i.e.  '/private/var/folders/p1/nl3sq7jn5nx8tfkdwpz2_g7r0000gn/T/TestModuleMisc1764175663/002'
		test.That(t, modWorkingDirectory, test.ShouldEndWith, filepath.Dir(modPath))
		err = mgr.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
	})
}

func TestTwoModulesRestart(t *testing.T) {
	ctx := context.Background()
	logger, logs := logging.NewObservedTestLogger(t)

	modCfgs := []config.Module{
		{
			Name:    "test-module",
			ExePath: rtestutils.BuildTempModule(t, "module/testmodule"),
			Type:    config.ModuleTypeLocal,
		},
		{
			Name:    "test-module2",
			ExePath: rtestutils.BuildTempModule(t, "module/testmodule2"),
			Type:    config.ModuleTypeLocal,
		},
	}

	// Lower global timeout early to avoid race with actual restart code.
	defer func(oriOrigVal time.Duration) {
		oueRestartInterval = oriOrigVal
	}(oueRestartInterval)
	oueRestartInterval = 10 * time.Millisecond

	parentAddr, err := modlib.CreateSocketAddress(t.TempDir(), "parent")
	test.That(t, err, test.ShouldBeNil)
	fakeRobot := rtestutils.MakeRobotForModuleLogging(t, parentAddr)
	defer func() {
		test.That(t, fakeRobot.Stop(), test.ShouldBeNil)
	}()

	var dummyRemoveOrphanedResourcesCallCount atomic.Uint64
	dummyRemoveOrphanedResources := func(context.Context, []resource.Name) {
		dummyRemoveOrphanedResourcesCallCount.Add(1)
	}
	mgr := NewManager(ctx, parentAddr, logger, modmanageroptions.Options{
		UntrustedEnv:            false,
		RemoveOrphanedResources: dummyRemoveOrphanedResources,
	})
	err = mgr.Add(ctx, modCfgs...)
	test.That(t, err, test.ShouldBeNil)

	// Add resources and ensure "echo" works correctly.
	models := map[string]resource.Model{
		"myhelper":  resource.NewModel("rdk", "test", "helper"),
		"myhelper2": resource.NewModel("rdk", "test", "helper2"),
	}
	for name, model := range models {
		resName := generic.Named(name)
		resCfg := resource.Config{
			Name:  name,
			API:   generic.API,
			Model: model,
		}
		_, err = resCfg.Validate("test", resource.APITypeComponentName)
		test.That(t, err, test.ShouldBeNil)

		res, err := mgr.AddResource(ctx, resCfg, nil)
		test.That(t, err, test.ShouldBeNil)
		ok := mgr.IsModularResource(resName)
		test.That(t, ok, test.ShouldBeTrue)

		resp, err := res.DoCommand(ctx, map[string]interface{}{"command": "echo"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldNotBeNil)
		test.That(t, resp["command"], test.ShouldEqual, "echo")

		// Run 'kill_module' command through helper resource to cause module to exit
		// with error. Assert that after module is restarted, helper is modularly
		// managed again and remains functional.
		_, err = res.DoCommand(ctx, map[string]interface{}{"command": "kill_module"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "rpc error")
	}

	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		test.That(tb, logs.FilterMessageSnippet("Module successfully restarted").Len(),
			test.ShouldEqual, 2)
	})

	err = mgr.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	// Assert that logs reflect that test-module crashed and there were no
	// errors during restart.
	test.That(t, logs.FilterMessageSnippet("Module has unexpectedly exited").Len(),
		test.ShouldEqual, 2)
	test.That(t, logs.FilterMessageSnippet("Error while restarting crashed module").Len(),
		test.ShouldEqual, 0)

	// Assert that RemoveOrphanedResources was called once for each module.
	test.That(t, dummyRemoveOrphanedResourcesCallCount.Load(), test.ShouldEqual, 2)
}
