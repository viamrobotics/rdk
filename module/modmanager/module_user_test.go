package modmanager

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/config"
	rutils "go.viam.com/rdk/utils"
)

func TestRunAsUserForcesTCPMode(t *testing.T) {
	t.Setenv(rutils.ViamTCPSocketsEnvVar, "false")
	m := &module{cfg: config.Module{Name: "m"}}
	test.That(t, m.tcpMode(), test.ShouldBeFalse)
	m.cfg.RunAsUser = "someone"
	test.That(t, m.tcpMode(), test.ShouldBeTrue)
}

func TestLookupModuleUser(t *testing.T) {
	if runtime.GOOS == "windows" {
		_, err := lookupModuleUser("anyone")
		test.That(t, err, test.ShouldNotBeNil)
		return
	}
	_, err := lookupModuleUser("viam-user-that-does-not-exist")
	test.That(t, err, test.ShouldNotBeNil)

	current, err := user.Current()
	test.That(t, err, test.ShouldBeNil)
	u, err := lookupModuleUser(current.Username)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, u.homeDir, test.ShouldEqual, current.HomeDir)

	dir := t.TempDir()
	test.That(t, os.WriteFile(filepath.Join(dir, "f"), []byte("x"), 0o600), test.ShouldBeNil)
	test.That(t, u.chownTree(dir), test.ShouldBeNil)
}
