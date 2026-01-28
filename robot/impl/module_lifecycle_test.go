//nolint:dupl
package robotimpl

import (
	"context"
	"os"
	"slices"
	"testing"
	"time"

	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	rtestutils "go.viam.com/rdk/testutils"
	rutils "go.viam.com/rdk/utils"
)

// create an in-process testing robot with a basic modules config, return the client.
func setupModuleTest(t *testing.T, ctx context.Context, failOnFirst bool, logger logging.Logger) (robot.LocalRobot, config.Config) {
	t.Helper()

	// Precompile modules to avoid timeout issues when building takes too long.
	testPath := rtestutils.BuildTempModule(t, "module/testmodule")
	test2Path := rtestutils.BuildTempModule(t, "module/testmodule2")

	// Manually define models, as importing them can cause double registration.
	helperModel := resource.NewModel("rdk", "test", "helper")
	helper2Model := resource.NewModel("rdk", "test", "helper2")

	// VIAM_TESTMODULE_FAIL_ON_FIRST will make it so that resource 'h' will always fail on the first
	// construction and succeed on the second.
	env := make(map[string]string)
	if failOnFirst {
		env["VIAM_TESTMODULE_FAIL_ON_FIRST"] = "1"
	}

	// Config has two working modules and one failing module.
	cfg := config.Config{
		Modules: []config.Module{
			{
				Name:        "mod",
				ExePath:     testPath,
				Environment: env,
			},
			{
				Name:    "mod2",
				ExePath: test2Path,
			},
		},
		Components: []resource.Config{
			{
				Name:  "h",
				Model: helperModel,
				API:   generic.API,
			},
			{
				Name:      "h2",
				Model:     helper2Model,
				API:       generic.API,
				DependsOn: []string{"h"},
			},
			{
				Name:      "h3",
				Model:     fakeModel,
				API:       generic.API,
				DependsOn: []string{"h"},
			},
			{
				Name:      "h4",
				Model:     resource.DefaultModelFamily.WithModel("nonexistent"),
				API:       generic.API,
				DependsOn: []string{"h"},
			},
		},
	}

	r := setupLocalRobot(t, ctx, &cfg, logger, WithDisableCompleteConfigWorker())

	// Assert that if failOnFirst is false, resources are all available after the first pass.
	if !failOnFirst {
		h, err := r.ResourceByName(generic.Named("h"))
		test.That(t, err, test.ShouldBeNil)
		_, err = h.DoCommand(ctx, map[string]any{"command": "get_num_reconfigurations"})
		test.That(t, err, test.ShouldBeNil)

		h2, err := r.ResourceByName(generic.Named("h2"))
		test.That(t, err, test.ShouldBeNil)
		resp, err := h2.DoCommand(ctx, map[string]interface{}{"command": "echo_dep"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, map[string]interface{}{"command": "echo"})

		_, err = r.ResourceByName(generic.Named("h3"))
		test.That(t, err, test.ShouldBeNil)
	} else {
		// Assert that if failOnFirst is true, none of the resources are available after the first attempt.
		_, err := r.ResourceByName(generic.Named("h"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = r.ResourceByName(generic.Named("h2"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = r.ResourceByName(generic.Named("h3"))
		test.That(t, err, test.ShouldNotBeNil)

		// Assert that retrying resource construction creates all of the resources.
		anyChanges := r.(*localRobot).updateRemotesAndRetryResourceConfigure()
		test.That(t, anyChanges, test.ShouldBeTrue)

		_, err = r.ResourceByName(generic.Named("h"))
		test.That(t, err, test.ShouldBeNil)

		h2, err := r.ResourceByName(generic.Named("h2"))
		test.That(t, err, test.ShouldBeNil)
		resp, err := h2.DoCommand(ctx, map[string]interface{}{"command": "echo_dep"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, map[string]interface{}{"command": "echo"})

		_, err = r.ResourceByName(generic.Named("h3"))
		test.That(t, err, test.ShouldBeNil)
	}

	logger.Info("module setup finished")
	return r, cfg
}

func failedModules(r robot.LocalRobot) []string {
	modFailures := r.(*localRobot).manager.moduleManager.FailedModules()
	// guarantee order for test assertions
	slices.Sort(modFailures)
	return modFailures
}

func TestRenamedModuleDependentRecovery(t *testing.T) {
	// on module 1 'mod' rename, test that a modular resource ('h2') on module 2 'mod2'
	// and a builtin resource ('h3') that depends on a modular resource ('h') on 'mod'
	// continues to exist and work.
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	r, cfg := setupModuleTest(t, ctx, false, logger)

	// rename 'mod' to 'mod1'.
	cfg.Modules[0].Name = "mod1"
	r.Reconfigure(ctx, &cfg)

	// Assert that after a module rename, 'h', 'h2', and 'h3' continue to exist and work.
	h, err := r.ResourceByName(generic.Named("h"))
	test.That(t, err, test.ShouldBeNil)
	_, err = h.DoCommand(ctx, map[string]any{"command": "get_num_reconfigurations"})
	test.That(t, err, test.ShouldBeNil)

	h2, err := r.ResourceByName(generic.Named("h2"))
	test.That(t, err, test.ShouldBeNil)
	resp, err := h2.DoCommand(ctx, map[string]interface{}{"command": "echo_dep"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, map[string]interface{}{"command": "echo"})

	_, err = r.ResourceByName(generic.Named("h3"))
	test.That(t, err, test.ShouldBeNil)
}

func TestRenamedModuleDependentRecoveryAfterFailedFirstConstruction(t *testing.T) {
	// on module 1 'mod' rename, test that a modular resource ('h2') on module 2 'mod2'
	// and a builtin resource ('h3') that depends on a modular resource ('h') on 'mod'
	// continues to exist and work.
	//
	// 'h' is setup to always fail on the its first construction on the module.
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	r, cfg := setupModuleTest(t, ctx, true, logger)

	// rename 'mod' to 'mod1'.
	cfg.Modules[0].Name = "mod1"
	r.Reconfigure(ctx, &cfg)

	// Assert that 'h', 'h2', and 'h3' are all not available because 'h' failed construction,
	// meaning 'h2' and 'h3' will also get removed.
	_, err := r.ResourceByName(generic.Named("h"))
	test.That(t, err, test.ShouldNotBeNil)

	_, err = r.ResourceByName(generic.Named("h2"))
	test.That(t, err, test.ShouldNotBeNil)

	_, err = r.ResourceByName(generic.Named("h3"))
	test.That(t, err, test.ShouldNotBeNil)

	// Assert that retrying resource construction creates all of the resources.
	anyChanges := r.(*localRobot).updateRemotesAndRetryResourceConfigure()
	test.That(t, anyChanges, test.ShouldBeTrue)

	h, err := r.ResourceByName(generic.Named("h"))
	test.That(t, err, test.ShouldBeNil)
	_, err = h.DoCommand(ctx, map[string]any{"command": "get_num_reconfigurations"})
	test.That(t, err, test.ShouldBeNil)

	h2, err := r.ResourceByName(generic.Named("h2"))
	test.That(t, err, test.ShouldBeNil)
	resp, err := h2.DoCommand(ctx, map[string]interface{}{"command": "echo_dep"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, map[string]interface{}{"command": "echo"})

	_, err = r.ResourceByName(generic.Named("h3"))
	test.That(t, err, test.ShouldBeNil)
}

func TestReconfiguredModuleDependentRecovery(t *testing.T) {
	// on module 1 'mod' reconfigure, test that a modular resource ('h2') on module 2 'mod2'
	// and a builtin resource ('h3') that depends on a modular resource ('h') on 'mod'
	// continues to exist and work.
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	r, cfg := setupModuleTest(t, ctx, false, logger)

	// reconfigure 'mod'
	cfg.Modules[0].LocalVersion = "1"
	r.Reconfigure(ctx, &cfg)

	// Assert that after a module rename, 'h', 'h2', and 'h3' continue to exist and work.
	h, err := r.ResourceByName(generic.Named("h"))
	test.That(t, err, test.ShouldBeNil)
	_, err = h.DoCommand(ctx, map[string]any{"command": "get_num_reconfigurations"})
	test.That(t, err, test.ShouldBeNil)

	h2, err := r.ResourceByName(generic.Named("h2"))
	test.That(t, err, test.ShouldBeNil)
	resp, err := h2.DoCommand(ctx, map[string]interface{}{"command": "echo_dep"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, map[string]interface{}{"command": "echo"})

	_, err = r.ResourceByName(generic.Named("h3"))
	test.That(t, err, test.ShouldBeNil)
}

func TestReconfiguredModuleDependentRecoveryAfterFailedFirstConstruction(t *testing.T) {
	// on module 1 'mod' reconfigure, test that a modular resource ('h2') on module 2 'mod2'
	// and a builtin resource ('h3') that depends on a modular resource ('h') on 'mod'
	// continues to exist and work.
	//
	// 'h' is setup to always fail on the its first construction on the module.
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	r, cfg := setupModuleTest(t, ctx, true, logger)

	// reconfigure 'mod'
	cfg.Modules[0].LocalVersion = "1"
	r.Reconfigure(ctx, &cfg)

	// Assert that 'h', 'h2', and 'h3' are all not available because 'h' failed construction,
	// meaning 'h2' and 'h3' will also get removed.
	_, err := r.ResourceByName(generic.Named("h"))
	test.That(t, err, test.ShouldNotBeNil)

	_, err = r.ResourceByName(generic.Named("h2"))
	test.That(t, err, test.ShouldNotBeNil)

	_, err = r.ResourceByName(generic.Named("h3"))
	test.That(t, err, test.ShouldNotBeNil)

	// Assert that retrying resource construction creates all of the resources.
	anyChanges := r.(*localRobot).updateRemotesAndRetryResourceConfigure()
	test.That(t, anyChanges, test.ShouldBeTrue)

	h, err := r.ResourceByName(generic.Named("h"))
	test.That(t, err, test.ShouldBeNil)
	_, err = h.DoCommand(ctx, map[string]any{"command": "get_num_reconfigurations"})
	test.That(t, err, test.ShouldBeNil)

	h2, err := r.ResourceByName(generic.Named("h2"))
	test.That(t, err, test.ShouldBeNil)
	resp, err := h2.DoCommand(ctx, map[string]interface{}{"command": "echo_dep"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, map[string]interface{}{"command": "echo"})

	_, err = r.ResourceByName(generic.Named("h3"))
	test.That(t, err, test.ShouldBeNil)
}

func TestRestartModuleDependentRecovery(t *testing.T) {
	// on module 1 'mod' restart, test that a modular resource ('h2') on module 2 'mod2'
	// and a builtin resource ('h3') that depends on a modular resource ('h') on 'mod'
	// continues to exist and work.
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	r, _ := setupModuleTest(t, ctx, false, logger)

	r.RestartModule(ctx, robot.RestartModuleRequest{ModuleName: "mod"})

	// Assert that h is not available, but h2 and h3 are.
	// h2 and h3 should fail any requests that depends on h.
	_, err := r.ResourceByName(generic.Named("h"))
	test.That(t, err, test.ShouldNotBeNil)

	h2, err := r.ResourceByName(generic.Named("h2"))
	test.That(t, err, test.ShouldBeNil)
	_, err = h2.DoCommand(ctx, map[string]interface{}{"command": "echo_dep"})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "rpc error")

	_, err = r.ResourceByName(generic.Named("h3"))
	test.That(t, err, test.ShouldBeNil)

	anyChanges := r.(*localRobot).updateRemotesAndRetryResourceConfigure()
	test.That(t, anyChanges, test.ShouldBeTrue)

	h, err := r.ResourceByName(generic.Named("h"))
	test.That(t, err, test.ShouldBeNil)
	_, err = h.DoCommand(ctx, map[string]any{"command": "get_num_reconfigurations"})
	test.That(t, err, test.ShouldBeNil)

	// h2 and h3 should also continue to exist and requests that go to h should not fail.
	h2, err = r.ResourceByName(generic.Named("h2"))
	test.That(t, err, test.ShouldBeNil)
	resp, err := h2.DoCommand(ctx, map[string]interface{}{"command": "echo_dep"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, map[string]interface{}{"command": "echo"})

	_, err = r.ResourceByName(generic.Named("h3"))
	test.That(t, err, test.ShouldBeNil)
}

func TestRestartModuleDependentRecoveryAfterFailedFirstConstruction(t *testing.T) {
	// on module 1 'mod' restart, test that a modular resource ('h2') on module 2 'mod2'
	// and a builtin resource ('h3') that depends on a modular resource ('h') on 'mod'
	// continues to exist and work.
	//
	// 'h' is setup to always fail on the its first construction on the module.
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	r, _ := setupModuleTest(t, ctx, true, logger)

	r.RestartModule(ctx, robot.RestartModuleRequest{ModuleName: "mod"})

	// Assert that 'h' is not available, but 'h2' and 'h3' are. This happens because RestartModule reconfigures
	// the module, removing resources ('h') on 'mod', and marks any dependents as needing updates.
	// However, those updates are not processed until updateRemotesAndRetryResourceConfigure().
	//
	// 'h2' and 'h3' should fail any requests that depends on 'h'.
	_, err := r.ResourceByName(generic.Named("h"))
	test.That(t, err, test.ShouldNotBeNil)

	h2, err := r.ResourceByName(generic.Named("h2"))
	test.That(t, err, test.ShouldBeNil)
	_, err = h2.DoCommand(ctx, map[string]interface{}{"command": "echo_dep"})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "rpc error")

	_, err = r.ResourceByName(generic.Named("h3"))
	test.That(t, err, test.ShouldBeNil)

	// Assert that after the first attempt at configuring resources, 'h' still is uninitialized
	// (because the first construction attempt failed), and 'h2' and 'h3' have also been removed
	// because 'h' is down.
	anyChanges := r.(*localRobot).updateRemotesAndRetryResourceConfigure()
	test.That(t, anyChanges, test.ShouldBeTrue)

	_, err = r.ResourceByName(generic.Named("h"))
	test.That(t, err, test.ShouldNotBeNil)

	_, err = r.ResourceByName(generic.Named("h2"))
	test.That(t, err, test.ShouldNotBeNil)

	_, err = r.ResourceByName(generic.Named("h3"))
	test.That(t, err, test.ShouldNotBeNil)

	// Assert that after the second attempt at configuring resources, 'h' now exists,
	// and 'h2' and 'h3' have also succeeded construction.
	anyChanges = r.(*localRobot).updateRemotesAndRetryResourceConfigure()
	test.That(t, anyChanges, test.ShouldBeTrue)

	h, err := r.ResourceByName(generic.Named("h"))
	test.That(t, err, test.ShouldBeNil)
	_, err = h.DoCommand(ctx, map[string]any{"command": "get_num_reconfigurations"})
	test.That(t, err, test.ShouldBeNil)

	h2, err = r.ResourceByName(generic.Named("h2"))
	test.That(t, err, test.ShouldBeNil)
	resp, err := h2.DoCommand(ctx, map[string]interface{}{"command": "echo_dep"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, map[string]interface{}{"command": "echo"})

	_, err = r.ResourceByName(generic.Named("h3"))
	test.That(t, err, test.ShouldBeNil)
}

func TestCrashedModuleDependentRecovery(t *testing.T) {
	// on module 1 'mod' crash and recovery, test that a modular resource ('h2') on module 2 'mod2'
	// and a builtin resource ('h3') that depends on a modular resource ('h') on 'mod'
	// continues to exist and work.
	ctx := context.Background()
	logger, logs := logging.NewObservedTestLogger(t)
	r, cfg := setupModuleTest(t, ctx, false, logger)

	// Assert that removing testmodule binary and killing testmodule
	// doesn't remove 'h' but commands fail.
	testPath := cfg.Modules[0].ExePath
	err := os.Rename(testPath, testPath+".disabled")
	test.That(t, err, test.ShouldBeNil)

	h, err := r.ResourceByName(generic.Named("h"))
	test.That(t, err, test.ShouldBeNil)
	_, err = h.DoCommand(ctx, map[string]interface{}{"command": "kill_module"})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "rpc error")

	// Wait for crash and check if module is added to failedModules.
	testutils.WaitForAssertionWithSleep(t, time.Second, 20, func(tb testing.TB) {
		tb.Helper()
		test.That(tb, logs.FilterMessage("Module has unexpectedly exited.").Len(),
			test.ShouldBeGreaterThanOrEqualTo, 1)
		test.That(tb, failedModules(r), test.ShouldResemble, []string{"mod"})
	})

	// Wait for restart attempt in logs.
	testutils.WaitForAssertionWithSleep(t, time.Second, 20, func(tb testing.TB) {
		tb.Helper()
		test.That(tb, logs.FilterMessage("Error while restarting crashed module").Len(),
			test.ShouldBeGreaterThanOrEqualTo, 1)
	})

	r.Reconfigure(ctx, &cfg)
	// Verify module is still in failedModules after reconfigure
	testutils.WaitForAssertionWithSleep(t, time.Second, 20, func(tb testing.TB) {
		tb.Helper()
		test.That(tb, failedModules(r), test.ShouldResemble, []string{"mod"})
	})

	// Check that 'h' is still present but commands fail.
	h, err = r.ResourceByName(generic.Named("h"))
	test.That(t, err, test.ShouldBeNil)
	_, err = h.DoCommand(ctx, map[string]any{"command": "get_num_reconfigurations"})
	test.That(t, err, test.ShouldNotBeNil)

	// 'h2' and 'h3' should also continue to exist, but fail any requests that depends on 'h'.
	h2, err := r.ResourceByName(generic.Named("h2"))
	test.That(t, err, test.ShouldBeNil)
	_, err = h2.DoCommand(ctx, map[string]interface{}{"command": "echo_dep"})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "rpc error")

	_, err = r.ResourceByName(generic.Named("h3"))
	test.That(t, err, test.ShouldBeNil)

	// Assert that restoring the testmodule binary makes 'h' start working again
	// after the auto-restart code succeeds.
	err = os.Rename(testPath+".disabled", testPath)
	test.That(t, err, test.ShouldBeNil)
	testutils.WaitForAssertionWithSleep(t, time.Second, 20, func(tb testing.TB) {
		tb.Helper()
		test.That(tb, logs.FilterMessage("Module resources successfully re-added after module restart").Len(),
			test.ShouldEqual, 1)
	})

	h, err = r.ResourceByName(generic.Named("h"))
	test.That(t, err, test.ShouldBeNil)
	_, err = h.DoCommand(ctx, map[string]any{"command": "get_num_reconfigurations"})
	test.That(t, err, test.ShouldBeNil)

	// Test that restored module is removed from failedModules
	test.That(t, failedModules(r), test.ShouldBeEmpty)

	// 'h2' and 'h3' should also continue to exist and requests that go to 'h' should no longer fail.
	h2, err = r.ResourceByName(generic.Named("h2"))
	test.That(t, err, test.ShouldBeNil)
	resp, err := h2.DoCommand(ctx, map[string]interface{}{"command": "echo_dep"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, map[string]interface{}{"command": "echo"})

	_, err = r.ResourceByName(generic.Named("h3"))
	test.That(t, err, test.ShouldBeNil)
}

func TestCrashedModuleDependentRecoveryAfterFailedFirstConstruction(t *testing.T) {
	// on module 1 'mod' crash and recovery, test that a modular resource ('h2') on module 2 'mod2'
	// and a builtin resource ('h3') that depends on a modular resource ('h') on 'mod'
	// continues to exist and work.
	//
	// 'h' is setup to always fail on the its first construction on the module.
	ctx := context.Background()
	logger, logs := logging.NewObservedTestLogger(t)
	r, cfg := setupModuleTest(t, ctx, true, logger)

	// Assert that removing testmodule binary and killing testmodule
	// doesn't remove 'h' but commands fail.
	testPath := cfg.Modules[0].ExePath
	err := os.Rename(testPath, testPath+".disabled")
	test.That(t, err, test.ShouldBeNil)

	h, err := r.ResourceByName(generic.Named("h"))
	test.That(t, err, test.ShouldBeNil)
	_, err = h.DoCommand(ctx, map[string]interface{}{"command": "kill_module"})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "rpc error")

	// Wait for restart attempt in logs.
	testutils.WaitForAssertionWithSleep(t, time.Second, 20, func(tb testing.TB) {
		tb.Helper()
		test.That(tb, logs.FilterMessage("Error while restarting crashed module").Len(),
			test.ShouldBeGreaterThanOrEqualTo, 1)
	})

	// Check that 'h' is still present but commands fail.
	h, err = r.ResourceByName(generic.Named("h"))
	test.That(t, err, test.ShouldBeNil)
	_, err = h.DoCommand(ctx, map[string]any{"command": "get_num_reconfigurations"})
	test.That(t, err, test.ShouldNotBeNil)

	// 'h2' and 'h3' should also continue to exist, but fail any requests that depends on 'h'.
	h2, err := r.ResourceByName(generic.Named("h2"))
	test.That(t, err, test.ShouldBeNil)
	_, err = h2.DoCommand(ctx, map[string]interface{}{"command": "echo_dep"})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "rpc error")

	_, err = r.ResourceByName(generic.Named("h3"))
	test.That(t, err, test.ShouldBeNil)

	// Assert that restoring the testmodule binary restores the module but not 'h'.
	err = os.Rename(testPath+".disabled", testPath)
	test.That(t, err, test.ShouldBeNil)
	testutils.WaitForAssertionWithSleep(t, time.Second, 20, func(tb testing.TB) {
		tb.Helper()
		test.That(tb, logs.FilterMessage("Module resources successfully re-added after module restart").Len(),
			test.ShouldEqual, 1)
	})

	// Assert that 'h' is not available, but 'h2' and 'h3' are.
	// 'h2' and 'h3' should continue to fail any requests that depends on 'h'.
	_, err = r.ResourceByName(generic.Named("h"))
	test.That(t, err, test.ShouldNotBeNil)

	_, err = r.ResourceByName(generic.Named("h2"))
	test.That(t, err, test.ShouldBeNil)
	_, err = h2.DoCommand(ctx, map[string]interface{}{"command": "echo_dep"})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "rpc error")

	_, err = r.ResourceByName(generic.Named("h3"))
	test.That(t, err, test.ShouldBeNil)

	// Assert that after another attempt at configuring resources, 'h' now exists,
	// and commands on 'h2' and 'h3' that depend on 'h' succeed.
	anyChanges := r.(*localRobot).updateRemotesAndRetryResourceConfigure()
	test.That(t, anyChanges, test.ShouldBeTrue)

	h, err = r.ResourceByName(generic.Named("h"))
	test.That(t, err, test.ShouldBeNil)
	_, err = h.DoCommand(ctx, map[string]any{"command": "get_num_reconfigurations"})
	test.That(t, err, test.ShouldBeNil)

	h2, err = r.ResourceByName(generic.Named("h2"))
	test.That(t, err, test.ShouldBeNil)
	resp, err := h2.DoCommand(ctx, map[string]interface{}{"command": "echo_dep"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, map[string]interface{}{"command": "echo"})

	_, err = r.ResourceByName(generic.Named("h3"))
	test.That(t, err, test.ShouldBeNil)
}

func TestFailedModuleTrackingIntegration(t *testing.T) {
	// test that failing modules are properly tracked in failedModules by breaking
	// and fixing modules and making sure failedModules is updated accordingly.
	ctx := context.Background()
	logger, logs := logging.NewObservedTestLogger(t)
	r, cfg := setupModuleTest(t, ctx, false, logger)

	// TEST: user adds module with invalid exec path and it fails to validate
	mod3 := config.Module{
		Name:    "mod3",
		ExePath: "/nonexistent/path/to/module1",
	}
	cfg.Modules = append(cfg.Modules, mod3)
	r.Reconfigure(ctx, &cfg)

	// Assert that "mod3" gets added to failedModules
	test.That(t, failedModules(r), test.ShouldResemble, []string{"mod3"})
	test.That(t, logs.FilterMessage(`resource build error: unknown resource type: `+
		`API rdk:component:generic with model rdk:builtin:nonexistent not registered; `+
		`May be in failing module: [mod3]; There may be no module in config that provides this model`).Len(),
		test.ShouldBeGreaterThanOrEqualTo, 1)

	// TEST: user adds module with valid exec path but exits immediately by injecting a panic
	panicEnv := map[string]string{
		"VIAM_TESTMODULE_PANIC": "1",
	}
	execFailPath := rtestutils.BuildTempModule(t, "module/testmodule")
	mod4 := config.Module{
		Name:        "mod4",
		ExePath:     execFailPath,
		Environment: panicEnv,
	}
	cfg.Modules = append(cfg.Modules, mod4)
	r.Reconfigure(ctx, &cfg)

	// Assert that "mod4" gets added to failedModules.
	test.That(t, failedModules(r), test.ShouldResemble, []string{"mod3", "mod4"})
	test.That(t, logs.FilterMessage(`resource build error: unknown resource type: `+
		`API rdk:component:generic with model rdk:builtin:nonexistent not registered; `+
		`May be in failing module: [mod3 mod4]; There may be no module in config that provides this model`).Len(),
		test.ShouldBeGreaterThanOrEqualTo, 1)

	// TEST: user reconfigures module with invalid exec path and it fails to validate
	cfg.Modules[0].ExePath = "/nonexistent/path/to/invalid"
	r.Reconfigure(ctx, &cfg)

	// Assert that "mod" gets added to failedModules
	test.That(t, failedModules(r), test.ShouldResemble, []string{"mod", "mod3", "mod4"})
	test.That(t, logs.FilterMessage(`resource build error: unknown resource type: `+
		`API rdk:component:generic with model rdk:builtin:nonexistent not registered; `+
		`May be in failing module: [mod mod3 mod4]; There may be no module in config that provides this model`).Len(),
		test.ShouldBeGreaterThanOrEqualTo, 1)

	// TEST: user reconfigures module with valid exec path but exits immediately by injecting a panic
	cfg.Modules[1].ExePath = execFailPath
	cfg.Modules[1].Environment = panicEnv
	r.Reconfigure(ctx, &cfg)

	// Assert that "mod2" gets added to failedModules
	test.That(t, failedModules(r), test.ShouldResemble, []string{"mod", "mod2", "mod3", "mod4"})
	test.That(t, logs.FilterMessage(`resource build error: unknown resource type: `+
		`API rdk:component:generic with model rdk:builtin:nonexistent not registered; `+
		`May be in failing module: [mod mod2 mod3 mod4]; There may be no module in config that provides this model`).Len(),
		test.ShouldBeGreaterThanOrEqualTo, 1)

	// TEST: user fixes broken module's panic by removing VIAM_TESTMODULE_PANIC.
	cfg.Modules[1].Environment = nil
	cfg.Modules[3].Environment = nil
	r.Reconfigure(ctx, &cfg)

	// Assert that "mod2" is removed from failedModules.
	test.That(t, failedModules(r), test.ShouldResemble, []string{"mod", "mod3"})
	test.That(t, logs.FilterMessage(`resource build error: unknown resource type: `+
		`API rdk:component:generic with model rdk:builtin:nonexistent not registered; `+
		`May be in failing module: [mod mod3]; There may be no module in config that provides this model`).Len(),
		test.ShouldBeGreaterThanOrEqualTo, 1)

	// TEST: user renames module and it is added to failedModules
	cfg.Modules[0].Name = "mod5"
	r.Reconfigure(ctx, &cfg)
	test.That(t, failedModules(r), test.ShouldResemble, []string{"mod3", "mod5"})
	test.That(t, logs.FilterMessage(`resource build error: unknown resource type: `+
		`API rdk:component:generic with model rdk:builtin:nonexistent not registered; `+
		`May be in failing module: [mod3 mod5]; There may be no module in config that provides this model`).Len(),
		test.ShouldBeGreaterThanOrEqualTo, 1)

	// TEST: user fixes broken module's broken exec by providing valid exec paths.
	cfg.Modules[0].ExePath = execFailPath
	cfg.Modules[2].ExePath = rtestutils.BuildTempModule(t, "module/testmodule2")
	r.Reconfigure(ctx, &cfg)

	// Assert that "mod3" is removed from failedModules and empty failedModules log is called.
	test.That(t, logs.FilterMessage(`resource build error: unknown resource type: `+
		`API rdk:component:generic with model rdk:builtin:nonexistent not registered; `+
		`There may be no module in config that provides this model`).Len(),
		test.ShouldBeGreaterThanOrEqualTo, 1)
}

func TestImplicitDependencyUpdatesAfterModuleStartupCrash(t *testing.T) {
	// on module 1 'mod' crash and then modifying 'mod' to no longer crash,
	// test that implicit dependencies are added correctly if resource config
	// is unchanged.
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	// Precompile modules to avoid timeout issues when building takes too long.
	testPath := rtestutils.BuildTempModule(t, "module/testmodule")

	// Manually define models, as importing them can cause double registration.
	sensorModel := resource.NewModel("rdk", "test", "sensordep")

	// Config has one failing module.
	cfg := config.Config{
		Modules: []config.Module{
			{
				Name:        "mod",
				ExePath:     testPath,
				Environment: map[string]string{"VIAM_TESTMODULE_PANIC": "1"},
			},
		},
		Components: []resource.Config{
			{
				Name:       "mod-s",
				Model:      sensorModel,
				API:        sensor.API,
				Attributes: rutils.AttributeMap{"sensor": "s"},
			},
			{
				Name:  "s",
				Model: fakeModel,
				API:   sensor.API,
			},
		},
	}
	r := setupLocalRobot(t, ctx, &cfg, logger, WithDisableCompleteConfigWorker())

	// Assert that "mod" is in failedModules and that "mod-s" is not reachable while "s" is.
	test.That(t, r.(*localRobot).manager.moduleManager.FailedModules(), test.ShouldResemble, []string{"mod"})

	_, err := sensor.FromProvider(r, "mod-s")
	test.That(t, err, test.ShouldNotBeNil)

	s, err := sensor.FromProvider(r, "s")
	test.That(t, err, test.ShouldBeNil)
	resp, err := s.Readings(ctx, map[string]any{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, map[string]any{"a": 1, "b": 2, "c": 3})

	// Reconfigure so that the "mod" is no longer panicking
	cfg.Modules[0].Environment = nil
	r.Reconfigure(ctx, &cfg)

	// Assert that "mod-s" is now online, "s" is still reachable and we validated the config twice,
	// once for resolving implicit dependencies and once right before building.
	modS, err := sensor.FromProvider(r, "mod-s")
	test.That(t, err, test.ShouldBeNil)
	resp, err = modS.Readings(ctx, map[string]any{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, map[string]any{"a": 1.0, "b": 2.0, "c": 3.0})

	resp, err = modS.DoCommand(ctx, map[string]any{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, map[string]any{"validate_calls": 2.0})

	s, err = sensor.FromProvider(r, "s")
	test.That(t, err, test.ShouldBeNil)
	resp, err = s.Readings(ctx, map[string]any{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, map[string]any{"a": 1, "b": 2, "c": 3})
}

func TestImplicitDependencyUpdatesAfterModuleStartupCrashAndConfigMod(t *testing.T) {
	// on module 1 'mod' crash and then modifying 'mod' to no longer crash,
	// test that implicit dependencies are added correctly even if the resource config
	// was modified.
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	// Precompile modules to avoid timeout issues when building takes too long.
	testPath := rtestutils.BuildTempModule(t, "module/testmodule")

	// Manually define models, as importing them can cause double registration.
	sensorModel := resource.NewModel("rdk", "test", "sensordep")

	// Config has one failing module.
	cfg := config.Config{
		Modules: []config.Module{
			{
				Name:        "mod",
				ExePath:     testPath,
				Environment: map[string]string{"VIAM_TESTMODULE_PANIC": "1"},
			},
		},
		Components: []resource.Config{
			{
				Name:       "mod-s",
				Model:      sensorModel,
				API:        sensor.API,
				Attributes: rutils.AttributeMap{"sensor": "s"},
			},
			{
				Name:  "s",
				Model: fakeModel,
				API:   sensor.API,
			},
		},
	}
	r := setupLocalRobot(t, ctx, &cfg, logger, WithDisableCompleteConfigWorker())

	// Assert that "mod" is in failedModules and that "mod-s" is not reachable while "s" is.
	test.That(t, r.(*localRobot).manager.moduleManager.FailedModules(), test.ShouldResemble, []string{"mod"})

	_, err := sensor.FromProvider(r, "mod-s")
	test.That(t, err, test.ShouldNotBeNil)

	s, err := sensor.FromProvider(r, "s")
	test.That(t, err, test.ShouldBeNil)
	resp, err := s.Readings(ctx, map[string]any{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, map[string]any{"a": 1, "b": 2, "c": 3})

	// Reconfigure so that the "mod" is no longer panicking and add additional field to "mod-s" attributes
	cfg.Modules[0].Environment = nil
	cfg.Components[0].Attributes = rutils.AttributeMap{"sensor": "s", "hello": "world"}
	r.Reconfigure(ctx, &cfg)

	// Assert that "mod-s" is now online, "s" is still reachable and we validated the config twice,
	// once for resolving implicit dependencies and once right before building.
	modS, err := sensor.FromProvider(r, "mod-s")
	test.That(t, err, test.ShouldBeNil)
	resp, err = modS.Readings(ctx, map[string]any{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, map[string]any{"a": 1.0, "b": 2.0, "c": 3.0})

	resp, err = modS.DoCommand(ctx, map[string]any{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, map[string]any{"validate_calls": 2.0})

	s, err = sensor.FromProvider(r, "s")
	test.That(t, err, test.ShouldBeNil)
	resp, err = s.Readings(ctx, map[string]any{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, map[string]any{"a": 1, "b": 2, "c": 3})
}

func TestImplicitDependencyUpdatesAfterModuleRestart(t *testing.T) {
	// on module 1 'mod' restart with a different underlying binary,
	// test that implicit dependencies are added correctly if the resource config
	// is unchanged.
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	// Precompile modules to avoid timeout issues when building takes too long.
	// testmodule2's sensor does not have implicit deps while testmodule's does
	testPath2 := rtestutils.BuildTempModule(t, "module/testmodule2")
	testPath := rtestutils.BuildTempModule(t, "module/testmodule")

	// create a symlink - the test will later replace the module using this symlink
	newLoc := t.TempDir() + "testmod"

	err := os.Symlink(testPath2, newLoc)
	test.That(t, err, test.ShouldBeNil)

	// Manually define models, as importing them can cause double registration.
	sensorModel := resource.NewModel("rdk", "test", "sensordep")

	// Config has one failing module.
	cfg := config.Config{
		Modules: []config.Module{
			{
				Name:    "mod",
				ExePath: newLoc,
			},
		},
		Components: []resource.Config{
			{
				Name:       "mod-s",
				Model:      sensorModel,
				API:        sensor.API,
				Attributes: rutils.AttributeMap{"sensor": "s"},
			},
			{
				Name:  "s",
				Model: fakeModel,
				API:   sensor.API,
			},
		},
	}
	r := setupLocalRobot(t, ctx, &cfg, logger, WithDisableCompleteConfigWorker())

	// Assert that "mod-s" and "s" are both healthy.
	modS, err := sensor.FromProvider(r, "mod-s")
	test.That(t, err, test.ShouldBeNil)
	resp, err := modS.Readings(ctx, map[string]any{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, map[string]any{"hello": "world"})

	s, err := sensor.FromProvider(r, "s")
	test.That(t, err, test.ShouldBeNil)
	resp, err = s.Readings(ctx, map[string]any{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, map[string]any{"a": 1, "b": 2, "c": 3})

	// Replace symlink point to testmodule instead. Restart "mod" and force a robot reconfiguration
	// (since module restarts do not automatically build resources).
	err = os.Remove(newLoc)
	test.That(t, err, test.ShouldBeNil)
	err = os.Symlink(testPath, newLoc)
	test.That(t, err, test.ShouldBeNil)
	err = r.RestartModule(ctx, robot.RestartModuleRequest{ModuleName: "mod"})
	test.That(t, err, test.ShouldBeNil)
	// Assert that retrying resource construction creates all of the resources.
	anyChanges := r.(*localRobot).updateRemotesAndRetryResourceConfigure()
	test.That(t, anyChanges, test.ShouldBeTrue)

	// Assert that "mod-s" is now online, "s" is still reachable and we validated the config twice,
	// once for resolving implicit dependencies and once right before building.
	modS, err = sensor.FromProvider(r, "mod-s")
	test.That(t, err, test.ShouldBeNil)
	resp, err = modS.Readings(ctx, map[string]any{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, map[string]any{"a": 1.0, "b": 2.0, "c": 3.0})

	resp, err = modS.DoCommand(ctx, map[string]any{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, map[string]any{"validate_calls": 2.0})

	s, err = sensor.FromProvider(r, "s")
	test.That(t, err, test.ShouldBeNil)
	resp, err = s.Readings(ctx, map[string]any{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, map[string]any{"a": 1, "b": 2, "c": 3})
}
