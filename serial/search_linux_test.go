package serial

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"

	"go.viam.com/core/testutils"
	"go.viam.com/core/usb"

	"go.viam.com/test"
)

func TestSearch(t *testing.T) {
	tempDir1 := testutils.TempDirT(t, "", "")
	defer os.RemoveAll(tempDir1)
	tempDir2 := testutils.TempDirT(t, "", "")
	defer os.RemoveAll(tempDir2)

	prevSysPaths := usb.SysPaths
	defer func() {
		usb.SysPaths = prevSysPaths
	}()
	prevDebPath := devPath
	defer func() {
		devPath = prevDebPath
	}()
	devPathDir := testutils.TempDirT(t, "", "")
	defer os.RemoveAll(devPathDir)
	jetsonPath := filepath.Join(devPathDir, "ttyTHS0")
	test.That(t, os.WriteFile(jetsonPath, []byte("a"), 0666), test.ShouldBeNil)

	dev2Root := testutils.TempDirT(t, tempDir1, "")
	dev3Root := testutils.TempDirT(t, tempDir1, "")
	dev1 := testutils.TempDirT(t, tempDir2, "")
	dev2 := testutils.TempDirT(t, dev2Root, "")
	dev3 := testutils.TempDirT(t, dev3Root, "")

	test.That(t, os.WriteFile(filepath.Join(tempDir2, "uevent"), []byte("PRODUCT=2341/0043"), 0666), test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(dev3Root, "uevent"), []byte("PRODUCT=10c5/ea61"), 0666), test.ShouldBeNil)

	test.That(t, os.Mkdir(filepath.Join(dev1, "tty"), 0700), test.ShouldBeNil)
	test.That(t, os.Mkdir(filepath.Join(dev3, "tty"), 0700), test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(dev1, "tty", "one"), []byte("a"), 0666), test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(dev3, "tty", "two"), []byte("b"), 0666), test.ShouldBeNil)

	test.That(t, os.Symlink(filepath.Join("../", filepath.Base(tempDir2), filepath.Base(dev1)), path.Join(tempDir2, filepath.Base(dev1)+"1")), test.ShouldBeNil)
	test.That(t, os.Symlink(dev3, path.Join(tempDir2, filepath.Base(dev2))), test.ShouldBeNil)

	for i, tc := range []struct {
		Filter   SearchFilter
		DevPath  string
		Paths    []string
		Expected []Description
	}{
		{SearchFilter{}, "", nil, nil},
		{SearchFilter{}, "", []string{"/"}, nil},
		{SearchFilter{}, "", []string{tempDir2}, []Description{
			{Type: TypeArduino, Path: "/dev/one"},
		}},
		{SearchFilter{Type: TypeArduino}, "", []string{tempDir2}, []Description{
			{Type: TypeArduino, Path: "/dev/one"}},
		},
		{SearchFilter{Type: TypeJetson}, "", []string{tempDir2}, nil},

		{SearchFilter{}, devPathDir, nil, []Description{
			{Type: TypeJetson, Path: jetsonPath},
		}},
		{SearchFilter{}, devPathDir, []string{"/"}, []Description{
			{Type: TypeJetson, Path: jetsonPath},
		}},
		{SearchFilter{}, devPathDir, []string{tempDir2}, []Description{
			{Type: TypeArduino, Path: "/dev/one"},
			{Type: TypeJetson, Path: jetsonPath},
		}},
		{SearchFilter{Type: TypeArduino}, devPathDir, []string{tempDir2}, []Description{
			{Type: TypeArduino, Path: "/dev/one"},
		}},
		{SearchFilter{Type: TypeJetson}, devPathDir, []string{tempDir2}, []Description{
			{Type: TypeJetson, Path: jetsonPath},
		}},

		{SearchFilter{Type: TypeJetson}, jetsonPath, []string{tempDir2}, nil},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			usb.SysPaths = tc.Paths
			devPath = tc.DevPath

			result := Search(tc.Filter)
			test.That(t, result, test.ShouldHaveLength, len(tc.Expected))
			expectedM := map[Description]struct{}{}
			for _, e := range tc.Expected {
				expectedM[e] = struct{}{}
			}
			for _, desc := range result {
				delete(expectedM, desc)
			}
			test.That(t, expectedM, test.ShouldBeEmpty)
		})
	}
}
