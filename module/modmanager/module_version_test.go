package modmanager

import (
	"context"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	modmanageroptions "go.viam.com/rdk/module/modmanager/options"
	modulestatus "go.viam.com/rdk/module/status"
	rtestutils "go.viam.com/rdk/testutils"
)

func statusFor(t *testing.T, mgr *Manager, name string) modulestatus.Status {
	t.Helper()
	for _, s := range mgr.Status() {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("no status for module %q", name)
	return modulestatus.Status{}
}

func TestModuleStatusVersion(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	logger := logging.NewTestLogger(t)

	modCfg := config.Module{
		Name:           "test-module",
		ExePath:        rtestutils.BuildTempModule(t, "module/testmodule"),
		Type:           config.ModuleTypeLocal,
		PackageVersion: "1.0.0",
	}

	parentAddr := setupSocketWithRobot(t)
	mgr := setupModManager(t, ctx, parentAddr, logger, modmanageroptions.Options{UntrustedEnv: false})

	test.That(t, mgr.Add(ctx, modCfg), test.ShouldBeNil)
	s := statusFor(t, mgr, modCfg.Name)
	test.That(t, s.State, test.ShouldEqual, modulestatus.ModuleStateReady)
	test.That(t, s.Version, test.ShouldEqual, "1.0.0")
	firstReady := s.LastUpdated

	modCfg.PackageVersion = "1.1.0"
	_, err := mgr.Reconfigure(ctx, modCfg)
	test.That(t, err, test.ShouldBeNil)
	s = statusFor(t, mgr, modCfg.Name)
	test.That(t, s.State, test.ShouldEqual, modulestatus.ModuleStateReady)
	test.That(t, s.Version, test.ShouldEqual, "1.1.0")
	test.That(t, s.LastUpdated.After(firstReady), test.ShouldBeTrue)

	// A version that fails to start never reaches ready, so it is never recorded.
	t.Setenv("VIAM_TESTMODULE_PANIC", "1")
	modCfg.PackageVersion = "1.2.0"
	_, err = mgr.Reconfigure(ctx, modCfg)
	test.That(t, err, test.ShouldNotBeNil)
	s = statusFor(t, mgr, modCfg.Name)
	test.That(t, s.State, test.ShouldEqual, modulestatus.ModuleStateUnhealthy)
	test.That(t, s.Version, test.ShouldNotEqual, "1.2.0")
}
