package modmanager

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"syscall"
	"testing"

	"go.viam.com/test"
	"golang.org/x/sys/unix"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	modmanageroptions "go.viam.com/rdk/module/modmanager/options"
	rtestutils "go.viam.com/rdk/testutils"
	rutils "go.viam.com/rdk/utils"
)

func TestModManagerRunsModuleAsUser(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root to run a module as another user")
	}
	nobody, err := user.Lookup("nobody")
	if err != nil {
		t.Skip("no nobody user")
	}
	viamHome := filepath.Join(t.TempDir(), ".viam")
	test.That(t, os.MkdirAll(viamHome, 0o700), test.ShouldBeNil)
	if err := unix.Setxattr(viamHome, aclAccessXattr, aclFromMode(0o700).encode(), 0); err != nil {
		t.Skipf("filesystem does not support ACLs: %v", err)
	}

	builtPath := rtestutils.BuildTempModule(t, "examples/customresources/demos/simplemodule")
	packagesDir := filepath.Join(viamHome, "packages")
	moduleRoot := filepath.Join(packagesDir, "data", "module", "acme_simple-1.0.0")
	test.That(t, os.MkdirAll(moduleRoot, 0o700), test.ShouldBeNil)
	binary, err := os.ReadFile(builtPath)
	test.That(t, err, test.ShouldBeNil)
	exePath := filepath.Join(moduleRoot, "simplemodule")
	test.That(t, os.WriteFile(exePath, binary, 0o700), test.ShouldBeNil)

	t.Setenv(rutils.ViamTCPSocketsEnvVar, "true")
	parentAddr := setupSocketWithRobot(t)
	t.Setenv(rutils.ViamTCPSocketsEnvVar, "false")

	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	mgr := setupModManager(t, ctx, parentAddr, logger, modmanageroptions.Options{
		ViamHomeDir: viamHome,
		PackagesDir: packagesDir,
	})
	modCfg := config.Module{
		Name:        "simple-module",
		ExePath:     exePath,
		RunAsUser:   nobody.Username,
		Environment: map[string]string{"VIAM_MODULE_ROOT": moduleRoot},
	}
	test.That(t, mgr.Add(ctx, modCfg), test.ShouldBeNil)

	mod, ok := mgr.modules.Load(modCfg.Name)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, mod.isRunningInTCPMode(), test.ShouldBeTrue)
	pid, err := mod.process.UnixPid()
	test.That(t, err, test.ShouldBeNil)
	status, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, string(status), test.ShouldContainSubstring, "\nUid:\t"+nobody.Uid+"\t")

	info, err := os.Stat(mod.dataDir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fmt.Sprint(info.Sys().(*syscall.Stat_t).Uid), test.ShouldEqual, nobody.Uid)
}
