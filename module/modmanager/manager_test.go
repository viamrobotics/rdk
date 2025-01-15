package modmanager

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/pion/rtp"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	v1 "go.viam.com/api/module/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/camera"
	fakeCamera "go.viam.com/rdk/components/camera/fake"
	"go.viam.com/rdk/components/camera/rtppassthrough"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/ftdc"
	"go.viam.com/rdk/logging"
	modlib "go.viam.com/rdk/module"
	modmanageroptions "go.viam.com/rdk/module/modmanager/options"
	"go.viam.com/rdk/module/modmaninterface"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/web"
	rtestutils "go.viam.com/rdk/testutils"
	rutils "go.viam.com/rdk/utils"
)

type testDiscoveryResult map[string]interface{}

func setupSocketWithRobot(t *testing.T) string {
	t.Helper()

	var socketAddress string
	var err error
	if rutils.ViamTCPSockets() {
		socketAddress = "127.0.0.1:" + strconv.Itoa(web.TestTCPParentPort)
	} else {
		socketAddress, err = modlib.CreateSocketAddress(t.TempDir(), "parent")
		test.That(t, err, test.ShouldBeNil)
	}

	rtestutils.MakeRobotForModuleLogging(t, socketAddress)
	return socketAddress
}

func setupModManager(
	t *testing.T,
	ctx context.Context,
	parentAddr string,
	logger logging.Logger,
	options modmanageroptions.Options,
) modmaninterface.ModuleManager {
	t.Helper()
	mgr := NewManager(ctx, parentAddr, logger, options)
	t.Cleanup(func() {
		// Wait for module recovery processes here because modmanager.Close does not.
		// Do so by grabbing a copy of the modules and then waiting after
		// mgr.Close() completes, which cancels all contexts relating to module
		// recovery.
		mMgr, ok := mgr.(*Manager)
		test.That(t, ok, test.ShouldBeTrue)
		modules := []*module{}
		mMgr.modules.Range(func(_ string, mod *module) bool {
			modules = append(modules, mod)
			return true
		})
		test.That(t, mgr.Close(ctx), test.ShouldBeNil)
		for _, mod := range modules {
			if mod != nil {
				func() {
					// Wait for module recovery processes to complete.
					mod.inRecoveryLock.Lock()
					defer mod.inRecoveryLock.Unlock()
				}()
			}
		}
	})
	return mgr
}

func TestModManagerFunctions(t *testing.T) {
	// Precompile module copies to avoid timeout issues when building takes too long.
	modPath := rtestutils.BuildTempModule(t, "examples/customresources/demos/simplemodule")
	modPath2 := rtestutils.BuildTempModule(t, "examples/customresources/demos/simplemodule")

	for _, mode := range []string{"tcp", "unix"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			logger := logging.NewTestLogger(t)

			if mode == "tcp" {
				os.Setenv("VIAM_TCP_SOCKETS", "yes")
				t.Cleanup(func() { os.Unsetenv("VIAM_TCP_SOCKETS") })
			}

			myCounterModel := resource.NewModel("acme", "demo", "mycounter")
			rNameCounter1 := resource.NewName(generic.API, "counter1")
			cfgCounter1 := resource.Config{
				Name:  "counter1",
				API:   generic.API,
				Model: myCounterModel,
			}
			_, err := cfgCounter1.Validate("test", resource.APITypeComponentName)
			test.That(t, err, test.ShouldBeNil)

			parentAddr := setupSocketWithRobot(t)

			t.Log("test Helpers")
			viamHomeTemp := t.TempDir()
			mgr := setupModManager(t, ctx, parentAddr, logger, modmanageroptions.Options{UntrustedEnv: false, ViamHomeDir: viamHomeTemp})

			mod := &module{
				cfg: config.Module{
					Name:        "test",
					ExePath:     modPath,
					Type:        config.ModuleTypeRegistry,
					ModuleID:    "new:york",
					Environment: map[string]string{"SMART": "MACHINES"},
				},
				dataDir: "module-data-dir",
				logger:  logger,
				port:    tcpPortRange,
			}

			err = mod.startProcess(ctx, parentAddr, nil, viamHomeTemp, filepath.Join(viamHomeTemp, "packages"))
			test.That(t, err, test.ShouldBeNil)

			err = mod.dial()
			test.That(t, err, test.ShouldBeNil)

			err = mod.checkReady(ctx, parentAddr)
			test.That(t, err, test.ShouldBeNil)

			mod.registerResources(mgr)
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

			utils.UncheckedError(mod.startProcess(ctx, parentAddr, nil, viamHomeTemp, filepath.Join(viamHomeTemp, "packages")))
			err = mod.dial()
			test.That(t, err, test.ShouldBeNil)

			if mode == "unix" {
				// make sure mod.addr has changed
				test.That(t, mod.addr, test.ShouldNotEqual, oldAddr)

				// check that we're still able to use the old client
				_, err = oldClient.Ready(ctx, &v1.ReadyRequest{ParentAddress: parentAddr})
				test.That(t, err, test.ShouldBeNil)
			}

			test.That(t, mod.process.Stop(), test.ShouldBeNil)

			t.Log("test AddModule")
			mgr = setupModManager(t, ctx, parentAddr, logger, modmanageroptions.Options{UntrustedEnv: false})
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

			t.Log("test UntrustedEnv")
			mgr = setupModManager(t, ctx, parentAddr, logger, modmanageroptions.Options{UntrustedEnv: true})

			modCfg = config.Module{
				Name:    "simple-module",
				ExePath: modPath,
			}
			err = mgr.Add(ctx, modCfg)
			test.That(t, err, test.ShouldEqual, errModularResourcesDisabled)

			t.Log("test empty dir for CleanModuleDataDirectory")
			mgr = setupModManager(t, ctx, parentAddr, logger, modmanageroptions.Options{UntrustedEnv: false, ViamHomeDir: ""})
			err = mgr.CleanModuleDataDirectory()
			test.That(t, fmt.Sprint(err), test.ShouldContainSubstring, "cannot clean a root level module data directory")

			t.Log("test CleanModuleDataDirectory")
			viamHomeTemp = t.TempDir()
			robotCloudID := "a-b-c-d"
			expectedDataDir := filepath.Join(viamHomeTemp, parentModuleDataFolderName, robotCloudID)
			mgr = setupModManager(
				t,
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
		})
	}
}

func TestModManagerKill(t *testing.T) {
	// this test will not pass on windows as it relies on the UnixPid of the managed process
	if runtime.GOOS == "windows" {
		t.Skip()
	}
	modPath := rtestutils.BuildTempModule(t, "examples/customresources/demos/simplemodule")
	logger, logs := logging.NewObservedTestLogger(t)
	parentAddr := setupSocketWithRobot(t)

	ctx := context.Background()
	mgr := setupModManager(t, ctx, parentAddr, logger, modmanageroptions.Options{})
	modCfg := config.Module{
		Name:    "simple-module",
		ExePath: modPath,
	}
	err := mgr.Add(ctx, modCfg)
	test.That(t, err, test.ShouldBeNil)

	// get the module from the module map
	mMgr, ok := mgr.(*Manager)
	test.That(t, ok, test.ShouldBeTrue)

	mod, ok := mMgr.modules.Load(modCfg.Name)
	test.That(t, ok, test.ShouldBeTrue)

	mgr.Kill()

	testutils.WaitForAssertion(t, func(tb testing.TB) {
		test.That(tb, logs.FilterMessageSnippet("Killing module").Len(),
			test.ShouldEqual, 1)
	})

	// in CI, we have to send another signal to make sure the cmd.Wait() in
	// the manage goroutine actually returns.
	// We do not care about the error if it is expected.
	// maybe related to https://github.com/golang/go/issues/18874
	pid, err := mod.process.UnixPid()
	test.That(t, err, test.ShouldBeNil)
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		test.That(t, errors.Is(err, os.ErrProcessDone), test.ShouldBeFalse)
	}
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

	parentAddr := setupSocketWithRobot(t)

	t.Log("adding complex module")
	mgr := setupModManager(t, ctx, parentAddr, logger, modmanageroptions.Options{UntrustedEnv: false})

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

	parentAddr := setupSocketWithRobot(t)

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
		mgr := setupModManager(t, ctx, parentAddr, logger, modmanageroptions.Options{
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
			test.That(tb, logs.FilterMessageSnippet("Module resources successfully re-added after module restart").Len(),
				test.ShouldEqual, 1)
		})

		ok = mgr.IsModularResource(rNameMyHelper)
		test.That(t, ok, test.ShouldBeTrue)
		resp, err = h.DoCommand(ctx, map[string]interface{}{"command": "echo"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldNotBeNil)
		test.That(t, resp["command"], test.ShouldEqual, "echo")

		// Assert that logs reflect that test-module crashed and there were no
		// errors during restart.
		test.That(t, logs.FilterMessageSnippet("Module has unexpectedly exited").Len(),
			test.ShouldEqual, 1)
		test.That(t, logs.FilterMessageSnippet("Error while restarting crashed module").Len(),
			test.ShouldEqual, 0)

		// Assert that RemoveOrphanedResources was not called (successful restart and re-addition of
		// modular resources should not require removal of any orphans).
		test.That(t, dummyRemoveOrphanedResourcesCallCount.Load(), test.ShouldEqual, 0)
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
		mgr := setupModManager(t, ctx, parentAddr, logger, modmanageroptions.Options{
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
			test.That(tb, logs.FilterMessageSnippet("Removed resources after failed module restart").Len(),
				test.ShouldEqual, 1)
		})

		ok = mgr.IsModularResource(rNameMyHelper)
		test.That(t, ok, test.ShouldBeFalse)
		_, err = h.DoCommand(ctx, map[string]interface{}{"command": "echo"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not connected")

		// Assert that logs reflect that test-module crashed and was not
		// successfully restarted.
		test.That(t, logs.FilterMessageSnippet("Module has unexpectedly exited").Len(),
			test.ShouldEqual, 1)
		test.That(t, logs.FilterMessageSnippet("Module successfully restarted").Len(),
			test.ShouldEqual, 0)
		test.That(t, logs.FilterMessageSnippet("Error while restarting crashed module").Len(),
			test.ShouldEqual, 3)

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
		mgr := setupModManager(t, ctx, parentAddr, logger, modmanageroptions.Options{
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
			test.That(tb, logs.FilterMessageSnippet("Removed resources after failed module restart").Len(),
				test.ShouldEqual, 1)
		})

		ok = mgr.IsModularResource(rNameMyHelper)
		test.That(t, ok, test.ShouldBeFalse)
		_, err = h.DoCommand(ctx, map[string]interface{}{"command": "echo"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not connected")

		// Assert that logs reflect that test-module crashed and was not
		// restarted.
		test.That(t, logs.FilterMessageSnippet("Module has unexpectedly exited").Len(),
			test.ShouldEqual, 1)
		test.That(t, logs.FilterMessageSnippet("Module successfully restarted").Len(),
			test.ShouldEqual, 0)
		test.That(t, logs.FilterMessageSnippet("Will not attempt to restart crashed module").Len(),
			test.ShouldEqual, 1)

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
		mgr := setupModManager(t, ctx, parentAddr, logger, modmanageroptions.Options{
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
		mgr := setupModManager(t, ctx, parentAddr, logger, modmanageroptions.Options{
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
	})
}

func TestDebugModule(t *testing.T) {
	ctx := context.Background()

	// Precompile module to avoid timeout issues when building takes too long.
	modPath := rtestutils.BuildTempModule(t, "module/testmodule")

	parentAddr := setupSocketWithRobot(t)

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
			mgr := setupModManager(t, ctx, parentAddr, logger, modmanageroptions.Options{UntrustedEnv: false})

			modCfg := config.Module{
				Name:     "test-module",
				ExePath:  modPath,
				LogLevel: tc.moduleLogLevel,
			}

			err := mgr.Add(ctx, modCfg)
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

	parentAddr := setupSocketWithRobot(t)

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
		mgr := setupModManager(t, ctx, parentAddr, logger, modmanageroptions.Options{
			UntrustedEnv: false,
			ViamHomeDir:  testViamHomeDir,
		})
		// Test that cleaning the data directory before it has been created does not produce log messages
		err := mgr.CleanModuleDataDirectory()
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
	})
	t.Run("module working directory", func(t *testing.T) {
		logger := logging.NewTestLogger(t)
		mgr := setupModManager(t, ctx, parentAddr, logger, modmanageroptions.Options{
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
		err := mgr.Add(ctx, modCfg)
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
	})
	t.Run("module working directory fallback", func(t *testing.T) {
		logger := logging.NewTestLogger(t)
		mgr := setupModManager(t, ctx, parentAddr, logger, modmanageroptions.Options{
			UntrustedEnv: false,
			ViamHomeDir:  testViamHomeDir,
		})
		// Add the module
		err := mgr.Add(ctx, modCfg)
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
	})

	t.Run("allowed viam modules only in untrusted environment", func(t *testing.T) {
		logger := logging.NewTestLogger(t)
		mgr := setupModManager(t, ctx, parentAddr, logger, modmanageroptions.Options{
			UntrustedEnv: true,
			ViamHomeDir:  testViamHomeDir,
		})
		// confirm that nothing is added when all modules are not in the allowedList
		err := mgr.Add(ctx, modCfg)
		test.That(t, err, test.ShouldBeError, errModularResourcesDisabled)

		allowedCfg := config.Module{
			Name:     "test-module",
			ExePath:  modPath,
			Type:     config.ModuleTypeLocal,
			ModuleID: "viam:raspberry-pi",
		}

		// this currently logs and does not return an error
		err = mgr.Add(ctx, allowedCfg, modCfg)
		test.That(t, err, test.ShouldBeNil)

		// confirm only the raspberry-pi module was added
		test.That(t, len(mgr.Configs()), test.ShouldEqual, 1)
		for _, conf := range mgr.Configs() {
			test.That(t, conf.ModuleID, test.ShouldContainSubstring, "viam")
		}
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

	parentAddr := setupSocketWithRobot(t)

	var dummyRemoveOrphanedResourcesCallCount atomic.Uint64
	dummyRemoveOrphanedResources := func(context.Context, []resource.Name) {
		dummyRemoveOrphanedResourcesCallCount.Add(1)
	}
	mgr := setupModManager(t, ctx, parentAddr, logger, modmanageroptions.Options{
		UntrustedEnv:            false,
		RemoveOrphanedResources: dummyRemoveOrphanedResources,
	})
	err := mgr.Add(ctx, modCfgs...)
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
		test.That(tb, logs.FilterMessageSnippet("Module resources successfully re-added after module restart").Len(),
			test.ShouldEqual, 2)
	})

	// Assert that logs reflect that test-module crashed and there were no
	// errors during restart.
	test.That(t, logs.FilterMessageSnippet("Module has unexpectedly exited").Len(),
		test.ShouldEqual, 2)
	test.That(t, logs.FilterMessageSnippet("Error while restarting crashed module").Len(),
		test.ShouldEqual, 0)

	// Assert that RemoveOrphanedResources was not called for either module
	// (successful restart and re-addition of modular resources should not
	// require removal of any orphans).
	test.That(t, dummyRemoveOrphanedResourcesCallCount.Load(), test.ShouldEqual, 0)
}

var (
	Green = "\033[32m"
	Reset = "\033[0m"
)

// this helps make the test case much easier to read.
func greenLog(t *testing.T, msg string) {
	t.Log(Green + msg + Reset)
}

func TestRTPPassthrough(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewInMemoryLogger(t)

	// Precompile module copies to avoid timeout issues when building takes too long.
	modPath := rtestutils.BuildTempModule(t, "examples/customresources/demos/rtppassthrough")
	modPath2 := rtestutils.BuildTempModule(t, "examples/customresources/demos/rtppassthrough")

	// configs
	noPassConf := resource.Config{
		Name:  "no_rtp_passthrough_camera",
		API:   camera.API,
		Model: resource.NewModel("acme", "camera", "fake"),
	}
	_, err := noPassConf.Validate("test", resource.APITypeComponentName)
	test.That(t, err, test.ShouldBeNil)

	passConf := resource.Config{
		Name:       "rtp_passthrough_camera",
		API:        camera.API,
		Model:      resource.NewModel("acme", "camera", "fake"),
		Attributes: map[string]interface{}{"rtp_passthrough": true},
	}
	_, err = passConf.Validate("test", resource.APITypeComponentName)
	test.That(t, err, test.ShouldBeNil)

	// robot config
	parentAddr := setupSocketWithRobot(t)

	greenLog(t, "test AddModule")
	mgr := NewManager(ctx, parentAddr, logger, modmanageroptions.Options{UntrustedEnv: false})
	test.That(t, err, test.ShouldBeNil)

	// add module executable
	modCfg := config.Module{
		Name:    "rtp-passthrough-module",
		ExePath: modPath,
	}
	err = mgr.Add(ctx, modCfg)
	test.That(t, err, test.ShouldBeNil)

	// confirm registered
	reg, ok := resource.LookupRegistration(camera.API, passConf.Model)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, reg.Constructor, test.ShouldNotBeNil)

	greenLog(t, "Camera that does not support rtp_passthrough returns errors from rtppassthrough.Source methods")
	noPassCam, err := mgr.AddResource(ctx, noPassConf, nil)
	test.That(t, err, test.ShouldBeNil)

	noPassSource, ok := noPassCam.(rtppassthrough.Source)
	test.That(t, ok, test.ShouldBeTrue)

	subCtx := context.Background()
	sub, err := noPassSource.SubscribeRTP(subCtx, 512, func(pkts []*rtp.Packet) {
		t.Log("should not happen")
		t.FailNow()
	})
	test.That(t, err, test.ShouldBeError)
	test.That(t, err.Error(), test.ShouldContainSubstring, fakeCamera.ErrRTPPassthroughNotEnabled.Error())
	test.That(t, sub, test.ShouldResemble, rtppassthrough.NilSubscription)

	err = noPassSource.Unsubscribe(context.Background(), sub.ID)
	test.That(t, err, test.ShouldBeError)
	test.That(t, err, test.ShouldBeError, camera.ErrUnknownSubscriptionID)

	greenLog(t, "Camera that supports rtp_passthrough")
	passCam, err := mgr.AddResource(ctx, passConf, nil)
	test.That(t, err, test.ShouldBeNil)

	passSource, ok := passCam.(rtppassthrough.Source)
	test.That(t, ok, test.ShouldBeTrue)

	// SubscribeRTP succeeds
	calledCtx, calledFn := context.WithCancel(context.Background())
	sub, err = passSource.SubscribeRTP(subCtx, 512, func(pkts []*rtp.Packet) {
		test.That(t, len(pkts), test.ShouldBeGreaterThan, 0)
		calledFn()
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sub.ID, test.ShouldNotResemble, rtppassthrough.NilSubscription)
	test.That(t, sub.Terminated.Err(), test.ShouldBeNil)
	<-calledCtx.Done()

	// Unsubscribe succeeds and terminates the subscription
	greenLog(t, "Unsubscribe immediately terminates the relevant subscription")
	err = passSource.Unsubscribe(context.Background(), sub.ID)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sub.Terminated.Err(), test.ShouldBeError, context.Canceled)

	greenLog(t, "The first SubscribeRTP call receives rtp packets")
	calledCtx1, calledFn1 := context.WithCancel(context.Background())
	sub1, err := passSource.SubscribeRTP(subCtx, 512, func(pkts []*rtp.Packet) {
		test.That(t, len(pkts), test.ShouldBeGreaterThan, 0)
		calledFn1()
	})
	test.That(t, err, test.ShouldBeNil)

	greenLog(t, "The second SubscribeRTP call receives rtp packets concurrently")
	calledCtx2, calledFn2 := context.WithCancel(context.Background())
	sub2, err := passSource.SubscribeRTP(subCtx, 512, func(pkts []*rtp.Packet) {
		test.That(t, len(pkts), test.ShouldBeGreaterThan, 0)
		calledFn2()
	})
	test.That(t, err, test.ShouldBeNil)
	<-calledCtx1.Done()
	<-calledCtx2.Done()
	test.That(t, sub1.Terminated.Err(), test.ShouldBeNil)
	test.That(t, sub2.Terminated.Err(), test.ShouldBeNil)

	greenLog(t, "camera.Close immediately terminates all subscriptions")
	err = passCam.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, sub1.Terminated.Err(), test.ShouldBeError, context.Canceled)
	test.That(t, sub2.Terminated.Err(), test.ShouldBeError, context.Canceled)

	// reset passthrough
	err = mgr.RemoveResource(ctx, passConf.ResourceName())
	test.That(t, err, test.ShouldBeNil)

	passCam, err = mgr.AddResource(ctx, passConf, nil)
	test.That(t, err, test.ShouldBeNil)

	passSource, ok = passCam.(rtppassthrough.Source)
	test.That(t, ok, test.ShouldBeTrue)

	greenLog(t, "RemoveResource eventually terminates all subscriptions")
	// create 2 sub
	sub1, err = passSource.SubscribeRTP(subCtx, 512, func(pkts []*rtp.Packet) {})
	test.That(t, err, test.ShouldBeNil)

	sub2, err = passSource.SubscribeRTP(subCtx, 512, func(pkts []*rtp.Packet) {})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, sub1.Terminated.Err(), test.ShouldBeNil)
	test.That(t, sub2.Terminated.Err(), test.ShouldBeNil)

	// remove resource
	err = mgr.RemoveResource(ctx, passConf.ResourceName())
	test.That(t, err, test.ShouldBeNil)

	// subs are canceled
	test.That(t, utils.SelectContextOrWait(sub1.Terminated, time.Second), test.ShouldBeFalse)
	test.That(t, utils.SelectContextOrWait(sub2.Terminated, time.Second), test.ShouldBeFalse)
	test.That(t, sub1.Terminated.Err(), test.ShouldBeError, context.Canceled)
	test.That(t, sub2.Terminated.Err(), test.ShouldBeError, context.Canceled)

	// reset passthrough
	passCam, err = mgr.AddResource(ctx, passConf, nil)
	test.That(t, err, test.ShouldBeNil)

	passSource, ok = passCam.(rtppassthrough.Source)
	test.That(t, ok, test.ShouldBeTrue)

	// NOTE: This test relies on the model's Close() method hanlding terminating all subscriptions
	greenLog(t, "ReconfigureResource eventually terminates all subscriptions when the new model doesn't impelement Reconfigure")
	// create a sub
	sub, err = passSource.SubscribeRTP(subCtx, 512, func(pkts []*rtp.Packet) {})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, sub.Terminated.Err(), test.ShouldBeNil)

	// reconfigure
	err = mgr.ReconfigureResource(ctx, passConf, nil)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, utils.SelectContextOrWait(sub.Terminated, time.Second), test.ShouldBeFalse)
	test.That(t, sub.Terminated.Err(), test.ShouldBeError, context.Canceled)

	greenLog(t, "replacing a module binary eventually cancels subscriptions")
	// add a subscription
	sub, err = passSource.SubscribeRTP(subCtx, 512, func(pkts []*rtp.Packet) {})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sub.Terminated.Err(), test.ShouldBeNil)

	// Change underlying binary path of module to be a different copy of the same module
	modCfg.ExePath = modPath2

	// Reconfigure module with new ExePath.
	orphanedResourceNames, err := mgr.Reconfigure(ctx, modCfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(orphanedResourceNames), test.ShouldEqual, 2)
	test.That(t, orphanedResourceNames, test.ShouldContain, noPassConf.ResourceName())
	test.That(t, orphanedResourceNames, test.ShouldContain, passConf.ResourceName())
	// the subscription from the previous module instance should be terminated
	test.That(t, utils.SelectContextOrWait(sub.Terminated, time.Second), test.ShouldBeFalse)
	test.That(t, sub.Terminated.Err(), test.ShouldBeError, context.Canceled)

	greenLog(t, "modmanager Close eventually cancels subscriptions")
	// reset passthrough
	passCam, err = mgr.AddResource(ctx, passConf, nil)
	test.That(t, err, test.ShouldBeNil)
	passSource, ok = passCam.(rtppassthrough.Source)
	test.That(t, ok, test.ShouldBeTrue)

	// add a subscription
	sub, err = passSource.SubscribeRTP(subCtx, 512, func(pkts []*rtp.Packet) {})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sub.Terminated.Err(), test.ShouldBeNil)

	// close the mod manager
	err = mgr.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	// the subscription should be terminated
	test.That(t, utils.SelectContextOrWait(sub.Terminated, time.Second), test.ShouldBeFalse)
	test.That(t, sub.Terminated.Err(), test.ShouldBeError, context.Canceled)
}

func TestAddStreamMaxTrackErr(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewInMemoryLogger(t)

	// Precompile module copies to avoid timeout issues when building takes too long.
	modPath := rtestutils.BuildTempModule(t, "examples/customresources/demos/rtppassthrough")
	logger.Info(modPath)

	api := camera.API
	model := resource.NewModel("acme", "camera", "fake")
	confs := []resource.Config{}
	for i := 0; i < 10; i++ {
		conf := resource.Config{
			Name:       fmt.Sprintf("fake-%d", i),
			API:        api,
			Model:      model,
			Attributes: map[string]interface{}{"rtp_passthrough": true},
		}
		_, err := conf.Validate("test", resource.APITypeComponentName)
		confs = append(confs, conf)
		test.That(t, err, test.ShouldBeNil)
	}

	parentAddr := setupSocketWithRobot(t)

	greenLog(t, "test AddModule")
	mgr := NewManager(ctx, parentAddr, logger, modmanageroptions.Options{UntrustedEnv: false})
	defer func() {
		test.That(t, mgr.Close(ctx), test.ShouldBeNil)
	}()
	// close the mod manager

	modCfg := config.Module{
		Name:    "rtp-passthrough-module",
		ExePath: modPath,
	}
	err := mgr.Add(ctx, modCfg)
	test.That(t, err, test.ShouldBeNil)

	reg, ok := resource.LookupRegistration(camera.API, model)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, reg.Constructor, test.ShouldNotBeNil)

	greenLog(t, "add 10 cameras")
	cams := []resource.Resource{}
	for _, conf := range confs {
		cam, err := mgr.AddResource(ctx, conf, nil)
		test.That(t, err, test.ShouldBeNil)
		cams = append(cams, cam)
	}

	sources := []rtppassthrough.Source{}
	for _, cam := range cams {
		source, ok := cam.(rtppassthrough.Source)
		test.That(t, ok, test.ShouldBeTrue)
		sources = append(sources, source)
	}

	first9Sources := sources[1:]

	test.That(t, len(first9Sources), test.ShouldEqual, 9)

	greenLog(t, "the first 9's SubscribeRTP calls succeed")
	subCtx := context.Background()
	for _, source := range first9Sources {
		calledCtx, calledFn := context.WithCancel(context.Background())
		// SubscribeRTP succeeds
		sub, err := source.SubscribeRTP(subCtx, 512, func(pkts []*rtp.Packet) {
			test.That(t, len(pkts), test.ShouldBeGreaterThan, 0)
			calledFn()
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, sub.ID, test.ShouldNotResemble, rtppassthrough.NilSubscription)
		test.That(t, sub.Terminated.Err(), test.ShouldBeNil)
		<-calledCtx.Done()
	}

	greenLog(t, "the 10th returns an error")
	sub, err := sources[0].SubscribeRTP(subCtx, 512, func(pkts []*rtp.Packet) {
		t.Log("should not happen")
		t.FailNow()
	})
	test.That(t, err, test.ShouldBeError)
	test.That(t, err.Error(), test.ShouldContainSubstring, "only 9 WebRTC tracks are supported per peer connection")
	test.That(t, sub, test.ShouldResemble, rtppassthrough.NilSubscription)
}

func TestBadModuleFailsFast(t *testing.T) {
	t.Setenv("VIAM_TESTMODULE_PANIC", "1")
	logger := logging.NewTestLogger(t)

	modCfgs := []config.Module{
		{
			Name:    "test-module",
			ExePath: rtestutils.BuildTempModule(t, "module/testmodule"),
			Type:    config.ModuleTypeLocal,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	parentAddr := setupSocketWithRobot(t)
	opts := modmanageroptions.Options{UntrustedEnv: false}
	mgr := setupModManager(t, ctx, parentAddr, logger, opts)

	err := mgr.Add(ctx, modCfgs...)

	test.That(t, err.Error(), test.ShouldContainSubstring, "module test-module exited too quickly after attempted startup")
}

// TestFTDCAfterModuleCrash is to give confidence that the FTDC sections devoted to tracking module
// process information (e.g: CPU usage) is in sync with the Process IDs (PIDs) that are actually
// running.
func TestFTDCAfterModuleCrash(t *testing.T) {
	logger := logging.NewTestLogger(t)
	modCfgs := []config.Module{
		{
			Name: "test-module",
			// testmodule2 has a `kill_module` DoCommand to force a module/process crash.
			ExePath: rtestutils.BuildTempModule(t, "module/testmodule2"),
			Type:    config.ModuleTypeLocal,
		},
	}

	ctx := context.Background()
	parentAddr := setupSocketWithRobot(t)
	opts := modmanageroptions.Options{UntrustedEnv: false}

	// Start up a mod manager with FTDC enabled. We will inspect the FTDC output for the
	// `ElapsedTimeSecs` to assert the "pid tracking code" is working correctly.
	ftdcData := bytes.NewBuffer(nil)
	opts.FTDC = ftdc.NewWithWriter(ftdcData, logger)
	// Normally a test would explicitly call `constructDatum` to control/guarantee FTDC gets
	// data. But as a short-cut to avoid exposing methods that are currently private, we just run
	// FTDC in the background. And sleep long enough between testing events (killing the module) to
	// assert the right behavior.
	opts.FTDC.Start()

	// Set up a mod manager. Currently there are zero modules running.
	mgr := setupModManager(t, ctx, parentAddr, logger, opts)

	// Add a module, this will register an FTDC "section" for that module process.
	err := mgr.Add(ctx, modCfgs...)
	test.That(t, err, test.ShouldBeNil)

	// Add a resource -- this is simply to invoke the `kill_module` command.
	res, err := mgr.AddResource(ctx, resource.Config{
		Name:  "foo",
		API:   generic.API,
		Model: resource.NewModel("rdk", "test", "helper2"),
	}, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, mgr.IsModularResource(generic.Named("foo")), test.ShouldBeTrue)

	// Kill the module a few times for good measure.
	for idx := 0; idx < 3; idx++ {
		_, _ = res.DoCommand(ctx, map[string]interface{}{"command": "kill_module"})

		// FTDC is running in the background with a one second interval. So we sleep for two seconds
		// and cross our fingers we don't get a poor scheduler execution. The assertions are
		// intentionally weak to minimize the risk of false positives (a test failure with correct
		// production code).
		time.Sleep(2 * time.Second)
	}

	mgr.Close(ctx)
	opts.FTDC.StopAndJoin(ctx)

	datums, err := ftdc.Parse(ftdcData)
	test.That(t, err, test.ShouldBeNil)
	logger.Info("Num ftdc datums: ", len(datums))

	// Keep count of the number of `ElapsedTimeSecs` readings we encounter. It is a testing bug if
	// we don't see any process FTDC metrics for the module.
	numModuleElapsedTimeMetricsSeen := 0
	for _, datum := range datums {
		for _, reading := range datum.Readings {
			if reading.MetricName == "proc.modules.test-module.ElapsedTimeSecs" {
				logger.Infow("Reading", "timestamp", datum.Time, "elapsedTimeSecs", reading.Value)
				numModuleElapsedTimeMetricsSeen++
				// Dan: I don't have a good reason to believe that we can't (legitimately) observe
				// an `ElapsedTimeSecs` of 0 here. It's more likely we'd see a 0 because we queried
				// a bad PID.
				//
				// If my assumption is wrong and we get a false positive here, we can reevaluate the
				// options for making a more robust test.
				test.That(t, reading.Value, test.ShouldBeGreaterThan, 0)
			}
		}
	}

	// Assert that we saw at least one datapoint before considering the test a success.
	test.That(t, numModuleElapsedTimeMetricsSeen, test.ShouldBeGreaterThan, 0)
}

func TestModularDiscoverFunc(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	modPath := rtestutils.BuildTempModule(t, "module/testmodule")

	modCfg := config.Module{
		Name:    "test-module",
		ExePath: modPath,
	}

	parentAddr := setupSocketWithRobot(t)

	mgr := setupModManager(t, ctx, parentAddr, logger, modmanageroptions.Options{UntrustedEnv: false})

	err := mgr.Add(ctx, modCfg)
	test.That(t, err, test.ShouldBeNil)

	// The "helper" model implements actual (foobar) discovery
	reg, ok := resource.LookupRegistration(generic.API, resource.NewModel("rdk", "test", "helper"))
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, reg, test.ShouldNotBeNil)
	test.That(t, reg.Discover, test.ShouldNotBeNil)

	testCases := []struct {
		name          string
		params        map[string]interface{}
		expectedExtra string
	}{
		{
			name:          "Without extra set",
			params:        map[string]interface{}{},
			expectedExtra: "default",
		},
		{
			name:          "With extra set",
			params:        map[string]interface{}{"extra": "not the default"},
			expectedExtra: "not the default",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := reg.Discover(ctx, logger, tc.params)
			test.That(t, err, test.ShouldBeNil)
			t.Log("Discovery result: ", result)

			jsonData, err := json.Marshal(result)
			test.That(t, err, test.ShouldBeNil)
			t.Logf("Raw JSON: %s", string(jsonData))

			var discoveryResult testDiscoveryResult
			err = json.Unmarshal(jsonData, &discoveryResult)
			test.That(t, err, test.ShouldBeNil)
			t.Logf("Casted struct: %+v", discoveryResult)

			test.That(t, len(discoveryResult), test.ShouldEqual, 1)
			extraStr, ok := discoveryResult["extra"].(string)
			test.That(t, ok, test.ShouldBeTrue)
			test.That(t, extraStr, test.ShouldEqual, tc.expectedExtra)
		})
	}
}

func TestFirstRun(t *testing.T) {
	t.Run("fails", func(t *testing.T) {
		ctx := context.Background()
		logger, logs := logging.NewObservedTestLogger(t)

		exePath := rtestutils.BuildTempModuleWithFirstRun(t, "module/testmodule")
		modCfg := config.Module{
			Name:    "test-module",
			ExePath: exePath,
		}
		parentAddr := setupSocketWithRobot(t)
		opts := modmanageroptions.Options{
			UntrustedEnv: false,
		}
		mgr := setupModManager(t, ctx, parentAddr, logger, opts)

		t.Setenv("VIAM_TEST_FAIL_RUN_FIRST", "1")

		err := mgr.FirstRun(ctx, modCfg)
		test.That(t, err, test.ShouldNotBeNil)

		test.That(t, logs.FilterMessage("executing first run script").Len(), test.ShouldEqual, 1)

		stdio := logs.FilterMessage("got stdio").FilterLevelExact(zapcore.InfoLevel)
		test.That(t, stdio.Len(), test.ShouldEqual, 1)
		expectedStdio := map[string]struct{}{
			"failed!": {},
		}
		for _, msg := range stdio.All() {
			line := msg.ContextMap()["output"].(string)
			delete(expectedStdio, line)
		}
		test.That(t, expectedStdio, test.ShouldBeEmpty)

		stderr := logs.FilterMessage("got stderr").FilterLevelExact(zapcore.WarnLevel)
		test.That(t, stderr.Len(), test.ShouldEqual, 2)
		expectedStderr := map[string]struct{}{
			"erroring... 1": {},
			"erroring... 2": {},
		}
		for _, msg := range stderr.All() {
			line := msg.ContextMap()["output"].(string)
			delete(expectedStderr, line)
		}
		test.That(t, expectedStderr, test.ShouldBeEmpty)

		test.That(t, logs.FilterMessage("first run script failed").Len(), test.ShouldEqual, 1)
	})

	t.Run("succeeds once", func(t *testing.T) {
		ctx := context.Background()
		logger, logs := logging.NewObservedTestLogger(t)

		exePath := rtestutils.BuildTempModuleWithFirstRun(t, "module/testmodule")
		modCfg := config.Module{
			Name:    "test-module",
			ExePath: exePath,
		}
		parentAddr := setupSocketWithRobot(t)
		opts := modmanageroptions.Options{
			UntrustedEnv: false,
		}
		mgr := setupModManager(t, ctx, parentAddr, logger, opts)

		t.Log("=== FIRST RUN SUCCEEDS ===")

		err := mgr.FirstRun(ctx, modCfg)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, logs.FilterMessage("executing first run script").Len(), test.ShouldEqual, 1)

		stdio := logs.FilterMessage("got stdio").FilterLevelExact(zapcore.InfoLevel)
		test.That(t, stdio.Len(), test.ShouldEqual, 4)
		expectedStdio := map[string]struct{}{
			"running... 1": {},
			"running... 2": {},
			"running... 3": {},
			"done!":        {},
		}
		for _, msg := range stdio.All() {
			line := msg.ContextMap()["output"].(string)
			delete(expectedStdio, line)
		}
		test.That(t, expectedStdio, test.ShouldBeEmpty)

		stderr := logs.FilterMessage("got stderr").FilterLevelExact(zapcore.WarnLevel)
		test.That(t, stderr.Len(), test.ShouldEqual, 2)
		expectedStderr := map[string]struct{}{
			"hiccup 1": {},
			"hiccup 2": {},
		}
		for _, msg := range stderr.All() {
			line := msg.ContextMap()["output"].(string)
			delete(expectedStderr, line)
		}
		test.That(t, expectedStderr, test.ShouldBeEmpty)

		test.That(t, logs.FilterMessage("first run script succeeded").Len(), test.ShouldEqual, 1)

		t.Log("=== FIRST RUN SKIPPED AFTER SUCCESS ===")

		logs.TakeAll() // remove logs observed up to this point

		err = mgr.FirstRun(ctx, modCfg)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, logs.FilterMessage("first run already ran").Len(), test.ShouldEqual, 1)

		t.Log("FIRST RUN SKIPPED AFTER SUCCESS AND MODULE MANAGER RESTART")

		logs.TakeAll() // remove logs observed up to this point

		err = mgr.Close(context.Background())
		test.That(t, err, test.ShouldBeNil)

		opts = modmanageroptions.Options{
			UntrustedEnv: false,
		}
		mgr = setupModManager(t, ctx, parentAddr, logger, opts)

		err = mgr.FirstRun(ctx, modCfg)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, logs.FilterMessage("first run already ran").Len(), test.ShouldEqual, 1)
	})

	t.Run("with timeout", func(t *testing.T) {
		ctx := context.Background()
		logger := logging.NewTestLogger(t)
		exePath := rtestutils.BuildTempModuleWithFirstRun(t, "module/testmodule")
		parentAddr := setupSocketWithRobot(t)
		opts := modmanageroptions.Options{
			UntrustedEnv: false,
		}
		mgr := setupModManager(t, ctx, parentAddr, logger, opts)

		// set a timeout that is slow enough to allow a process to start
		// but expires before the process finishes. this should result
		// in the process getting killed.
		modCfg := config.Module{
			Name:            "test-module",
			ExePath:         exePath,
			FirstRunTimeout: utils.Duration(100 * time.Millisecond),
		}
		err := mgr.FirstRun(ctx, modCfg)
		test.That(t, err, test.ShouldNotBeNil)

		var errExit *exec.ExitError
		test.That(t, errors.As(err, &errExit), test.ShouldBeTrue)
		// This error message might be different on a non-unix platform.
		// Feel free to adjust this assertion if it ever fails on a
		// newly-tested platform (e.g. Windows).
		test.That(t, errExit.String(), test.ShouldContainSubstring, "signal: killed")

		// set a timeout that expires before the process can even start.
		// this should result in a [context.DeadlineExceeded] error.
		modCfg.FirstRunTimeout = utils.Duration(1 * time.Nanosecond)
		err = mgr.FirstRun(ctx, modCfg)
		test.That(t, err, test.ShouldResemble, context.DeadlineExceeded)
	})
}
