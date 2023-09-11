package modmanager

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.uber.org/zap/zaptest/observer"
	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/config"
	modmanageroptions "go.viam.com/rdk/module/modmanager/options"
	"go.viam.com/rdk/resource"
	rtestutils "go.viam.com/rdk/testutils"
	rutils "go.viam.com/rdk/utils"
)

func TestModManagerFunctions(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	// Precompile module copies to avoid timeout issues when building takes too long.
	modPath, err := rtestutils.BuildTempModule(t, "examples/customresources/demos/simplemodule")
	test.That(t, err, test.ShouldBeNil)
	modPath2, err := rtestutils.BuildTempModule(t, "examples/customresources/demos/simplemodule")
	test.That(t, err, test.ShouldBeNil)

	myCounterModel := resource.NewModel("acme", "demo", "mycounter")
	rNameCounter1 := resource.NewName(generic.API, "counter1")
	cfgCounter1 := resource.Config{
		Name:  "counter1",
		API:   generic.API,
		Model: myCounterModel,
	}
	_, err = cfgCounter1.Validate("test", resource.APITypeComponentName)
	test.That(t, err, test.ShouldBeNil)

	// This cannot use t.TempDir() as the path it gives on MacOS exceeds module.MaxSocketAddressLength.
	parentAddr, err := os.MkdirTemp("", "viam-test-*")
	test.That(t, err, test.ShouldBeNil)
	defer os.RemoveAll(parentAddr)
	parentAddr += "/parent.sock"

	t.Log("test Helpers")
	mgr := NewManager(parentAddr, logger, modmanageroptions.Options{UntrustedEnv: false})

	mod := &module{name: "test", exe: modPath}

	err = mod.startProcess(ctx, parentAddr, nil, logger)
	test.That(t, err, test.ShouldBeNil)

	err = mod.dial()
	test.That(t, err, test.ShouldBeNil)

	// check that dial can re-use connections.
	oldConn := mod.conn
	err = mod.dial()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, mod.conn, test.ShouldEqual, oldConn)

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

	test.That(t, mgr.Close(ctx), test.ShouldBeNil)
	test.That(t, mod.process.Stop(), test.ShouldBeNil)

	t.Log("test AddModule")
	mgr = NewManager(parentAddr, logger, modmanageroptions.Options{UntrustedEnv: false})
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
	test.That(t, orphanedResourceNames, test.ShouldBeNil)

	// counter1 should still be provided by reconfigured module.
	ok = mgr.IsModularResource(rNameCounter1)
	test.That(t, ok, test.ShouldBeTrue)
	ret, err = counter.DoCommand(ctx, map[string]interface{}{"command": "get"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ret["total"], test.ShouldEqual, 0)

	t.Log("test RemoveModule")
	orphanedResourceNames, err = mgr.Remove("simple-module")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, orphanedResourceNames, test.ShouldResemble, []resource.Name{rNameCounter1})

	// module will only really go away after resources within it are removed/closed
	ok = mgr.IsModularResource(rNameCounter1)
	test.That(t, ok, test.ShouldBeTrue)

	err = mgr.RemoveResource(ctx, rNameCounter1)
	test.That(t, err, test.ShouldBeNil)

	ok = mgr.IsModularResource(rNameCounter1)
	test.That(t, ok, test.ShouldBeFalse)
	_, err = counter.DoCommand(ctx, map[string]interface{}{"command": "get"})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "the client connection is closing")

	err = counter.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	err = mgr.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	t.Log("test UntrustedEnv")
	mgr = NewManager(parentAddr, logger, modmanageroptions.Options{UntrustedEnv: true})

	modCfg = config.Module{
		Name:    "simple-module",
		ExePath: modPath,
	}
	err = mgr.Add(ctx, modCfg)
	test.That(t, err, test.ShouldEqual, errModularResourcesDisabled)
}

func TestModManagerValidation(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	// Precompile module to avoid timeout issues when building takes too long.
	modPath, err := rtestutils.BuildTempModule(t, "examples/customresources/demos/complexmodule")
	test.That(t, err, test.ShouldBeNil)

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
	_, err = cfgMyBase1.Validate("test", resource.APITypeComponentName)
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

	// This cannot use t.TempDir() as the path it gives on MacOS exceeds module.MaxSocketAddressLength.
	parentAddr, err := os.MkdirTemp("", "viam-test-*")
	test.That(t, err, test.ShouldBeNil)
	defer os.RemoveAll(parentAddr)
	parentAddr += "/parent.sock"

	t.Log("adding complex module")
	mgr := NewManager(parentAddr, logger, modmanageroptions.Options{UntrustedEnv: false})

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

	myHelperModel := resource.NewModel("rdk", "test", "helper")
	rNameMyHelper := generic.Named("myhelper")
	cfgMyHelper := resource.Config{
		Name:  "myhelper",
		API:   generic.API,
		Model: myHelperModel,
	}
	_, err := cfgMyHelper.Validate("test", resource.APITypeComponentName)
	test.That(t, err, test.ShouldBeNil)

	// This cannot use t.TempDir() as the path it gives on MacOS exceeds module.MaxSocketAddressLength.
	parentAddr, err := os.MkdirTemp("", "viam-test-*")
	test.That(t, err, test.ShouldBeNil)
	defer os.RemoveAll(parentAddr)
	parentAddr += "/parent.sock"

	modCfg := config.Module{Name: "test-module"}

	t.Run("successful restart", func(t *testing.T) {
		logger, logs := golog.NewObservedTestLogger(t)

		// Precompile module to avoid timeout issues when building takes too long.
		modPath, err := rtestutils.BuildTempModule(t, "module/testmodule")
		test.That(t, err, test.ShouldBeNil)
		modCfg.ExePath = modPath

		// This test neither uses a resource manager nor asserts anything about
		// the existence of resources in the graph. Use a dummy
		// RemoveOrphanedResources function so orphaned resource logic does not
		// panic.
		var dummyRemoveOrphanedResourcesCallCount atomic.Uint64
		dummyRemoveOrphanedResources := func(context.Context, []resource.Name) {
			dummyRemoveOrphanedResourcesCallCount.Add(1)
		}
		mgr := NewManager(parentAddr, logger, modmanageroptions.Options{
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
		test.That(t, err.Error(), test.ShouldContainSubstring,
			"error reading from server")

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, logs.FilterMessageSnippet("module successfully restarted").Len(),
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
		test.That(t, logs.FilterMessageSnippet("module has unexpectedly exited").Len(),
			test.ShouldEqual, 1)
		test.That(t, logs.FilterMessageSnippet("error while restarting crashed module").Len(),
			test.ShouldEqual, 0)

		// Assert that RemoveOrphanedResources was called once.
		test.That(t, dummyRemoveOrphanedResourcesCallCount.Load(), test.ShouldEqual, 1)
	})
	t.Run("unsuccessful restart", func(t *testing.T) {
		logger, logs := golog.NewObservedTestLogger(t)

		// Precompile module to avoid timeout issues when building takes too long.
		modPath, err := rtestutils.BuildTempModule(t, "module/testmodule")
		test.That(t, err, test.ShouldBeNil)
		modCfg.ExePath = modPath

		// lower global timeout early to avoid race with actual restart code
		defer func(origVal time.Duration) {
			oueRestartInterval = origVal
		}(oueRestartInterval)
		oueRestartInterval = 10 * time.Millisecond

		// This test neither uses a resource manager nor asserts anything about
		// the existence of resources in the graph. Use a dummy
		// RemoveOrphanedResources function so orphaned resource logic does not
		// panic.
		var dummyRemoveOrphanedResourcesCallCount atomic.Uint64
		dummyRemoveOrphanedResources := func(context.Context, []resource.Name) {
			dummyRemoveOrphanedResourcesCallCount.Add(1)
		}
		mgr := NewManager(parentAddr, logger, modmanageroptions.Options{
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
		// after crash. Also lower oueRestartInterval so attempted restarts happen
		// at faster rate.
		err = os.Remove(modPath)
		test.That(t, err, test.ShouldBeNil)

		// Run 'kill_module' command through helper resource to cause module to
		// exit with error. Assert that after three restart errors occur, helper is
		// not modularly managed and commands return error.
		_, err = h.DoCommand(ctx, map[string]interface{}{"command": "kill_module"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring,
			"error reading from server")

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, logs.FilterMessageSnippet("error while restarting crashed module").Len(),
				test.ShouldEqual, 3)
		})

		ok = mgr.IsModularResource(rNameMyHelper)
		test.That(t, ok, test.ShouldBeFalse)
		_, err = h.DoCommand(ctx, map[string]interface{}{"command": "echo"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring,
			"connection is closing")

		err = mgr.Close(ctx)
		test.That(t, err, test.ShouldBeNil)

		// Assert that logs reflect that test-module crashed and was not
		// successfully restarted.
		test.That(t, logs.FilterMessageSnippet("module has unexpectedly exited").Len(),
			test.ShouldEqual, 1)
		test.That(t, logs.FilterMessageSnippet("module successfully restarted").Len(),
			test.ShouldEqual, 0)

		// Assert that RemoveOrphanedResources was called once.
		test.That(t, dummyRemoveOrphanedResourcesCallCount.Load(), test.ShouldEqual, 1)
	})
	t.Run("timed out module process is stopped", func(t *testing.T) {
		logger, logs := golog.NewObservedTestLogger(t)

		modCfg.ExePath = rutils.ResolveFile("module/testmodule/fakemodule.sh")

		// Lower global timeout early to avoid race with actual restart code.
		defer func(oriOrigVal time.Duration) {
			oueRestartInterval = oriOrigVal
		}(oueRestartInterval)
		oueRestartInterval = 10 * time.Millisecond

		// Lower resource configuration timeout to avoid waiting for 60 seconds
		// for manager.Add to time out waiting for module to start listening.
		defer func() {
			test.That(t, os.Unsetenv(rutils.ResourceConfigurationTimeoutEnvVar),
				test.ShouldBeNil)
		}()
		test.That(t, os.Setenv(rutils.ResourceConfigurationTimeoutEnvVar, "10ms"),
			test.ShouldBeNil)

		// This test neither uses a resource manager nor asserts anything about
		// the existence of resources in the graph. Use a dummy
		// RemoveOrphanedResources function so orphaned resource logic does not
		// panic.
		dummyRemoveOrphanedResources := func(context.Context, []resource.Name) {}
		mgr := NewManager(parentAddr, logger, modmanageroptions.Options{
			UntrustedEnv:            false,
			RemoveOrphanedResources: dummyRemoveOrphanedResources,
		})
		err = mgr.Add(ctx, modCfg)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring,
			"timed out waiting for module test-module to start listening")

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
	modPath, err := rtestutils.BuildTempModule(t, "module/testmodule")
	test.That(t, err, test.ShouldBeNil)

	// This cannot use t.TempDir() as the path it gives on MacOS exceeds module.MaxSocketAddressLength.
	parentAddr, err := os.MkdirTemp("", "viam-test-*")
	test.That(t, err, test.ShouldBeNil)
	defer os.RemoveAll(parentAddr)
	parentAddr += "/parent.sock"

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
			var logger golog.Logger
			var logs *observer.ObservedLogs
			if tc.managerDebugEnabled {
				logger, logs = golog.NewObservedTestLogger(t)
			} else {
				logger, logs = rtestutils.NewInfoObservedTestLogger(t)
			}
			mgr := NewManager(parentAddr, logger, modmanageroptions.Options{UntrustedEnv: false})
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

func TestGracefulShutdownWithMalformedModule(t *testing.T) {
	// This test ensures that module manager's `Add` can be interrupted by a `Close`
	// call correctly, and no OUE restart goroutines will continue beyond their `inStartup`
	// check. With our current design, `local_robot.Reconfigure` blocks the main thread,
	// so the manager will not be `Closed` while a module is being `Add`ed. Future work
	// (RSDK-4854) may change that though.
	logger, logs := golog.NewObservedTestLogger(t)
	// Precompile module to avoid timeout issues when building takes too long.
	modPath, err := rtestutils.BuildTempModule(t, "module/testmodule")
	test.That(t, err, test.ShouldBeNil)

	modCfg := config.Module{
		Name:     "test-module",
		ExePath:  modPath,
		LogLevel: "info",
	}

	// This cannot use t.TempDir() as the path it gives on MacOS exceeds module.MaxSocketAddressLength.
	parentAddr, err := os.MkdirTemp("", "viam-test-*")
	test.That(t, err, test.ShouldBeNil)
	defer os.RemoveAll(parentAddr)
	parentAddr += "/parent.sock"

	mgr := NewManager(parentAddr, logger, modmanageroptions.Options{UntrustedEnv: false})

	channel := make(chan struct{})
	go func() {
		err = mgr.Add(context.Background(), modCfg)
		channel <- struct{}{}
	}()
	// close the mgr so we can confirm that `Add` still finishes, despite manager being closed
	err = mgr.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)

	// Confirm that the call to `Add` has completed and `err` has been set to its return value
	<-channel
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "error while starting module test-module")

	// this message comes from the OUE handler, which shouldn't have been called at this point
	// (we closed the mgr before `Add` hits its normal timeout). At any rate, the OUE handler
	// will always exit quickly without doing anything so long as `Add` is mid-call.
	test.That(t, logs.FilterMessageSnippet("module has unexpectedly exited").Len(), test.ShouldEqual, 0)
}
