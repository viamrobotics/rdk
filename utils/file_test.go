package utils

import (
	"bytes"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"
)

func TestResolveFile(t *testing.T) {
	sentinel := "great"
	_ = sentinel
	resolved := ResolveFile("utils/file_test.go")
	rd, err := os.ReadFile(resolved)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, bytes.Contains(rd, []byte(`sentinel := "great"`)), test.ShouldBeTrue)
}

func TestSafeJoinDir(t *testing.T) {
	validate := func(parent, in, expectedOut string, expectedErr error) {
		t.Helper()

		out, err := SafeJoinDir(parent, in)
		if expectedErr == nil {
			test.That(t, err, test.ShouldBeNil)
			test.That(t, out, test.ShouldEqual, expectedOut)
		} else {
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, expectedErr.Error())
		}
	}

	parentDir := "/some/parent"
	validate(parentDir, "sub/dir", filepath.Join(parentDir, "sub/dir"), nil)
	validate(parentDir, "/other/parent", filepath.Join(parentDir, "other/parent"), nil)
	validate(parentDir, "meta.json", filepath.Join(parentDir, "meta.json"), nil)
	validate(parentDir, "../../../root", "", errors.New("unsafe path join"))
	validate(parentDir, "..", "", errors.New("unsafe path join"))

	// Relative parents (including ".") must be handled. Previously SafeJoinDir(".", "meta.json")
	// returned a spurious error, which broke module meta.json discovery (notably on Windows).
	validate(".", "meta.json", "meta.json", nil)
	validate(".", "sub/dir", filepath.Join("sub", "dir"), nil)
	validate(".", "..", "", errors.New("unsafe path join"))
	validate(".", "../escape", "", errors.New("unsafe path join"))
	validate("relative/parent", "child", filepath.Join("relative/parent", "child"), nil)
	validate("relative/parent", "../../escape", "", errors.New("unsafe path join"))
}

func TestExpandHomeDir(t *testing.T) {
	usr, err := user.Current()
	test.That(t, err, test.ShouldBeNil)

	path, err := ExpandHomeDir("x")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, path, test.ShouldResemble, "x")

	path, err = ExpandHomeDir("/x")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, path, test.ShouldResemble, "/x")

	path, err = ExpandHomeDir("/x/y")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, path, test.ShouldResemble, "/x/y")

	path, err = ExpandHomeDir("~")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, path, test.ShouldResemble, usr.HomeDir)

	path, err = ExpandHomeDir("/~/y")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, path, test.ShouldResemble, "/~/y")

	path, err = ExpandHomeDir("~/y")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, path, test.ShouldResemble, filepath.Join(usr.HomeDir, "y"))

	path, err = ExpandHomeDir("~\\y")
	test.That(t, err, test.ShouldBeNil)
	if runtime.GOOS == "windows" {
		test.That(t, path, test.ShouldResemble, filepath.Join(usr.HomeDir, "y"))
	} else {
		test.That(t, path, test.ShouldResemble, "~\\y")
	}
}
