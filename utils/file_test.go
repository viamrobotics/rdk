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
	parentDir := "/some/parent"

	validate := func(in, expectedOut string, expectedErr error) {
		t.Helper()

		out, err := SafeJoinDir(parentDir, in)
		if expectedErr == nil {
			test.That(t, err, test.ShouldBeNil)
			test.That(t, out, test.ShouldEqual, expectedOut)
		} else {
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, expectedErr.Error())
		}
	}

	validate("sub/dir", "/some/parent/sub/dir", nil)
	validate("/other/parent", "/some/parent/other/parent", nil)
	validate("../../../root", "", errors.New("unsafe path join"))
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
