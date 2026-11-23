package modmanager

import (
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"syscall"
	"testing"

	"go.viam.com/test"
	"golang.org/x/sys/unix"
)

func TestGrantModuleAccessLetsUserRunModule(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root to run a process as another user")
	}
	nobody, err := user.Lookup("nobody")
	if err != nil {
		t.Skip("no nobody user")
	}
	runAs, err := lookupModuleUser(nobody.Username)
	test.That(t, err, test.ShouldBeNil)

	base := t.TempDir()
	if err := unix.Setxattr(base, aclAccessXattr, aclFromMode(0o700).encode(), 0); err != nil {
		t.Skipf("filesystem does not support ACLs: %v", err)
	}
	home := filepath.Join(base, "root")
	packagesDir := filepath.Join(home, ".viam", "packages")
	moduleRoot := filepath.Join(packagesDir, "data", "module", "org_mod-1.0.0")
	dataDir := filepath.Join(home, ".viam", "module-data", "robot", "mod")
	otherSecret := filepath.Join(home, ".viam", "secret.json")
	for _, dir := range []string{filepath.Join(moduleRoot, "bin"), dataDir} {
		test.That(t, os.MkdirAll(dir, 0o700), test.ShouldBeNil)
	}
	test.That(t, os.WriteFile(otherSecret, []byte("secret"), 0o600), test.ShouldBeNil)
	exePath := filepath.Join(moduleRoot, "bin", "run.sh")
	test.That(t, os.WriteFile(exePath, []byte("#!/bin/sh\ncat \"$0\" >/dev/null && echo ok > \"$1/out\"\n"), 0o700),
		test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(dataDir, "from_first_run"), []byte("x"), 0o600), test.ShouldBeNil)

	runAsNobody := func(name string, args ...string) error {
		//nolint:gosec
		cmd := exec.Command(name, args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Credential: &syscall.Credential{Uid: uint32(runAs.uid), Gid: uint32(runAs.gid)},
		}
		return cmd.Run()
	}
	test.That(t, runAsNobody(exePath, dataDir), test.ShouldNotBeNil)

	test.That(t, runAs.grantModuleAccess(exePath, moduleRoot, []string{packagesDir}, dataDir), test.ShouldBeNil)
	test.That(t, runAsNobody(exePath, dataDir), test.ShouldBeNil)
	out, err := os.ReadFile(filepath.Join(dataDir, "out"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, string(out), test.ShouldEqual, "ok\n")
	test.That(t, runAsNobody("/bin/cat", filepath.Join(dataDir, "from_first_run")), test.ShouldBeNil)

	test.That(t, runAsNobody("/bin/cat", otherSecret), test.ShouldNotBeNil)
	test.That(t, runAsNobody("/bin/ls", filepath.Join(home, ".viam")), test.ShouldNotBeNil)

	test.That(t, runAs.grantModuleAccess(exePath, moduleRoot, []string{packagesDir}, dataDir), test.ShouldBeNil)
}

func TestGrantModuleAccessLeavesUnmanagedExecutablesAlone(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root")
	}
	current, err := user.Current()
	test.That(t, err, test.ShouldBeNil)
	runAs, err := lookupModuleUser(current.Username)
	test.That(t, err, test.ShouldBeNil)
	runAs.uid = 54321

	base := t.TempDir()
	exePath := filepath.Join(base, "dev", "run.sh")
	test.That(t, os.MkdirAll(filepath.Dir(exePath), 0o700), test.ShouldBeNil)
	test.That(t, os.WriteFile(exePath, nil, 0o700), test.ShouldBeNil)

	test.That(t, runAs.grantModuleAccess(exePath, "", []string{filepath.Join(base, "packages")}, ""), test.ShouldBeNil)
	_, err = unix.Getxattr(filepath.Dir(exePath), aclAccessXattr, make([]byte, 256))
	test.That(t, err, test.ShouldEqual, unix.ENODATA)
}
