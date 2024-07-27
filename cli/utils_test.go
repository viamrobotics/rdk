package cli

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"os/exec"
	"testing"

	"go.viam.com/test"
)

func TestSamePath(t *testing.T) {
	equal, _ := samePath("/x", "/x")
	test.That(t, equal, test.ShouldBeTrue)
	equal, _ = samePath("/x", "x")
	test.That(t, equal, test.ShouldBeFalse)
}

func TestGetMapString(t *testing.T) {
	m := map[string]any{
		"x": "x",
		"y": 10,
	}
	test.That(t, getMapString(m, "x"), test.ShouldEqual, "x")
	test.That(t, getMapString(m, "y"), test.ShouldEqual, "")
	test.That(t, getMapString(m, "z"), test.ShouldEqual, "")
}

func chdir(t *testing.T, path string) {
	wd, err := os.Getwd()
	test.That(t, err, test.ShouldBeNil)
	err = os.Chdir(path)
	test.That(t, err, test.ShouldBeNil)
	t.Cleanup(func() { os.Chdir(wd) })
}

// helper; starts and waits for an exec.Cmd.
func runCommand(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	err := errors.Join(cmd.Start(), cmd.Wait())
	test.That(t, err, test.ShouldBeNil)
}

// helper; returns map of {name: length} for tarball contents.
func tarContents(t *testing.T, path string) map[string]int64 {
	r, err := os.Open(path)
	test.That(t, err, test.ShouldBeNil)
	gz, err := gzip.NewReader(r)
	test.That(t, err, test.ShouldBeNil)
	tr := tar.NewReader(gz)
	ret := make(map[string]int64)
	for {
		head, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		test.That(t, err, test.ShouldBeNil)
		ret[head.Name] = head.Size
	}
	return ret
}

func TestReplaceInTarGz(t *testing.T) {
	tmp := t.TempDir()
	chdir(t, tmp)

	// setup
	runCommand(t, exec.Command("touch", "hello.txt"))
	runCommand(t, exec.Command("touch", "replaceme.txt"))
	// add a symlink to make sure this doesn't choke on symlinks
	runCommand(t, exec.Command("ln", "-s", "hello.txt", "linkhello"))
	runCommand(t, exec.Command("tar", "czf", "archive.tar.gz", "hello.txt", "replaceme.txt", "linkhello"))

	err := replaceInTarGz("archive.tar.gz", map[string][]byte{
		// replace an entry
		"replaceme.txt": []byte("hi"),
		// add an entry
		"new.txt": []byte("hey"),
	})
	test.That(t, err, test.ShouldBeNil)

	// verify
	test.That(t, tarContents(t, "archive.tar.gz"), test.ShouldResemble, map[string]int64{
		"hello.txt":     0,
		"linkhello":     0,
		"replaceme.txt": 2,
		"new.txt":       3,
	})
}
