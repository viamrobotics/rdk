package robotimpl

import (
	"context"
	"os"
	"testing"
	"time"

	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	rtestutils "go.viam.com/rdk/testutils"
)

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
		},
	}

	r := setupLocalRobot(t, ctx, &cfg, logger, withDisableCompleteConfigWorker())

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

	return r, cfg
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
	// continues to be and work.
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	r, cfg := setupModuleTest(t, ctx, false, logger)

	// reconfigure 'mod' by changing the ExePath.
	testPathReconf := rtestutils.BuildTempModule(t, "module/testmodule")
	cfg.Modules[0].ExePath = testPathReconf
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
	// continues to be and work.
	//
	// 'h' is setup to always fail on the its first construction on the module.
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	r, cfg := setupModuleTest(t, ctx, true, logger)

	// reconfigure 'mod' by changing the ExePath.
	testPathReconf := rtestutils.BuildTempModule(t, "module/testmodule")
	cfg.Modules[0].ExePath = testPathReconf
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
	// continues to be and work.
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
	// continues to be and work.
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
	// continues to be and work.
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
	// continues to be and work.
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
