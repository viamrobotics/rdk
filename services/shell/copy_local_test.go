package shell

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"go.viam.com/test"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	shelltestutils "go.viam.com/rdk/services/shell/testutils"
)

func TestFixPeerPath(t *testing.T) {
	tempDir := t.TempDir()
	cwd, err := os.Getwd()
	test.That(t, err, test.ShouldBeNil)
	t.Cleanup(func() { os.Chdir(cwd) })
	test.That(t, os.Chdir(tempDir), test.ShouldBeNil)
	// macos uses /private for /var temp dirs, getwd will give us that path
	realTempDir, err := os.Getwd()
	test.That(t, err, test.ShouldBeNil)

	fixed, err := fixPeerPath("/one/two/three", false, true)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fixed, test.ShouldEqual, "/one/two/three")

	homeDir, err := os.UserHomeDir()
	test.That(t, err, test.ShouldBeNil)

	fixed, err = fixPeerPath("one/two/three", false, true)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fixed, test.ShouldEqual, filepath.Join(homeDir, "one/two/three"))

	fixed, err = fixPeerPath("~/one/two/three", false, true)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fixed, test.ShouldEqual, filepath.Join(homeDir, "one/two/three"))

	fixed, err = fixPeerPath("~/one/two/~/three", false, true)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fixed, test.ShouldEqual, filepath.Join(homeDir, "one/two/~/three"))

	fixed, err = fixPeerPath("one/two/three", false, false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fixed, test.ShouldEqual, filepath.Join(realTempDir, "one/two/three"))

	_, err = fixPeerPath("", false, true)
	test.That(t, err, test.ShouldEqual, errUnexpectedEmptyPath)

	fixed, err = fixPeerPath("", true, true)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fixed, test.ShouldEqual, homeDir)

	fixed, err = fixPeerPath("", true, false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fixed, test.ShouldEqual, realTempDir)
}

// TestLocalFileCopy contains tests are very similar to cli.TestShellFileCopy but
// it includes some more detailed testing at the unit level that is more annoying to
// test in the CLI. The RPC side of this is covered by the CLI.
func TestLocalFileCopy(t *testing.T) {
	ctx := context.Background()
	tfs := shelltestutils.SetupTestFileSystem(t)

	t.Run("single file", func(t *testing.T) {
		tempDir := t.TempDir()

		factory, err := NewLocalFileCopyFactory(tempDir, false, false)
		test.That(t, err, test.ShouldBeNil)

		readCopier, err := NewLocalFileReadCopier([]string{tfs.SingleFileNested}, false, false, factory)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, readCopier.ReadAll(ctx), test.ShouldBeNil)
		test.That(t, readCopier.Close(ctx), test.ShouldBeNil)

		rd, err := os.ReadFile(filepath.Join(tempDir, filepath.Base(tfs.SingleFileNested)))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, rd, test.ShouldResemble, tfs.SingleFileNestedData)
	})

	t.Run("single file but destination does not exist", func(t *testing.T) {
		tempDir := t.TempDir()
		tempDirInner := filepath.Join(tempDir, "inner")
		test.That(t, os.Mkdir(tempDirInner, 0o750), test.ShouldBeNil)
		test.That(t, os.RemoveAll(tempDirInner), test.ShouldBeNil)

		factory, err := NewLocalFileCopyFactory(tempDirInner, false, false)
		test.That(t, err, test.ShouldBeNil)

		readCopier, err := NewLocalFileReadCopier([]string{tfs.SingleFileNested}, false, false, factory)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, readCopier.ReadAll(ctx), test.ShouldBeNil)
		test.That(t, readCopier.Close(ctx), test.ShouldBeNil)

		rd, err := os.ReadFile(tempDirInner)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, rd, test.ShouldResemble, tfs.SingleFileNestedData)

		t.Log("parent exists but it is a file not a directory")
		factory, err = NewLocalFileCopyFactory(filepath.Join(tempDirInner, "notthere"), false, false)
		test.That(t, err, test.ShouldBeNil)

		readCopier, err = NewLocalFileReadCopier([]string{tfs.SingleFileNested}, false, false, factory)
		test.That(t, err, test.ShouldBeNil)

		err = readCopier.ReadAll(ctx)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, readCopier.Close(ctx), test.ShouldBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "is an existing file")

		t.Log("parent does not exist")
		factory, err = NewLocalFileCopyFactory(filepath.Join(tempDir, "notthere", "alsonotthere"), false, false)
		test.That(t, err, test.ShouldBeNil)

		readCopier, err = NewLocalFileReadCopier([]string{tfs.SingleFileNested}, false, false, factory)
		test.That(t, err, test.ShouldBeNil)

		err = readCopier.ReadAll(ctx)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, readCopier.Close(ctx), test.ShouldBeNil)
		s, ok := status.FromError(err)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, s.Code(), test.ShouldEqual, codes.NotFound)
	})

	t.Run("single file relative", func(t *testing.T) {
		tempDir := t.TempDir()
		cwd, err := os.Getwd()
		test.That(t, err, test.ShouldBeNil)
		t.Cleanup(func() { os.Chdir(cwd) })
		test.That(t, os.Chdir(tempDir), test.ShouldBeNil)

		factory, err := NewLocalFileCopyFactory("foo", false, false)
		test.That(t, err, test.ShouldBeNil)

		readCopier, err := NewLocalFileReadCopier([]string{tfs.SingleFileNested}, false, false, factory)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, readCopier.ReadAll(ctx), test.ShouldBeNil)
		test.That(t, readCopier.Close(ctx), test.ShouldBeNil)

		rd, err := os.ReadFile(filepath.Join(tempDir, "foo"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, rd, test.ShouldResemble, tfs.SingleFileNestedData)
	})

	t.Run("single directory", func(t *testing.T) {
		tempDir := t.TempDir()

		t.Log("without recursion set")
		factory, err := NewLocalFileCopyFactory(tempDir, false, false)
		test.That(t, err, test.ShouldBeNil)

		_, err = NewLocalFileReadCopier([]string{tfs.Root}, false, false, factory)
		test.That(t, err, test.ShouldNotBeNil)
		s, ok := status.FromError(err)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, s.Code(), test.ShouldEqual, codes.InvalidArgument)
		test.That(t, s.Message(), test.ShouldContainSubstring, "recursion")
		_, err = os.ReadFile(filepath.Join(tempDir, "example"))
		test.That(t, errors.Is(err, fs.ErrNotExist), test.ShouldBeTrue)

		t.Log("with recursion set")
		readCopier, err := NewLocalFileReadCopier([]string{tfs.Root}, true, false, factory)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, readCopier.ReadAll(ctx), test.ShouldBeNil)
		test.That(t, readCopier.Close(ctx), test.ShouldBeNil)

		test.That(t, shelltestutils.DirectoryContentsEqual(tfs.Root, filepath.Join(tempDir, filepath.Base(tfs.Root))), test.ShouldBeNil)
	})

	t.Run("single directory but destination does not exist", func(t *testing.T) {
		tempDir := t.TempDir()
		tempDirInner := filepath.Join(tempDir, "inner")
		test.That(t, os.Mkdir(tempDirInner, 0o750), test.ShouldBeNil)
		test.That(t, os.RemoveAll(tempDirInner), test.ShouldBeNil)

		factory, err := NewLocalFileCopyFactory(tempDirInner, false, false)
		test.That(t, err, test.ShouldBeNil)

		readCopier, err := NewLocalFileReadCopier([]string{tfs.Root}, true, false, factory)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, readCopier.ReadAll(ctx), test.ShouldBeNil)
		test.That(t, readCopier.Close(ctx), test.ShouldBeNil)

		test.That(t, shelltestutils.DirectoryContentsEqual(tfs.Root, tempDirInner), test.ShouldBeNil)

		t.Log("parent exists but it is a file not a directory")
		fileNotDirectory := filepath.Join(tempDir, "file")
		test.That(t, os.WriteFile(fileNotDirectory, nil, 0o640), test.ShouldBeNil)
		factory, err = NewLocalFileCopyFactory(filepath.Join(fileNotDirectory, "notthere"), false, false)
		test.That(t, err, test.ShouldBeNil)

		readCopier, err = NewLocalFileReadCopier([]string{tfs.Root}, true, false, factory)
		test.That(t, err, test.ShouldBeNil)

		err = readCopier.ReadAll(ctx)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, readCopier.Close(ctx), test.ShouldBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "is an existing file")
	})

	t.Run("multiple files", func(t *testing.T) {
		tempDir := t.TempDir()

		factory, err := NewLocalFileCopyFactory(tempDir, false, false)
		test.That(t, err, test.ShouldBeNil)

		readCopier, err := NewLocalFileReadCopier([]string{
			tfs.SingleFileNested,
			tfs.InnerDir,
		}, true, false, factory)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, readCopier.ReadAll(ctx), test.ShouldBeNil)
		test.That(t, readCopier.Close(ctx), test.ShouldBeNil)

		rd, err := os.ReadFile(filepath.Join(tempDir, filepath.Base(tfs.SingleFileNested)))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, rd, test.ShouldResemble, tfs.SingleFileNestedData)

		test.That(t, shelltestutils.DirectoryContentsEqual(tfs.InnerDir, filepath.Join(tempDir, filepath.Base(tfs.InnerDir))), test.ShouldBeNil)
	})

	t.Run("multiple files but destination does not exist", func(t *testing.T) {
		tempDir := t.TempDir()
		test.That(t, os.RemoveAll(tempDir), test.ShouldBeNil)

		factory, err := NewLocalFileCopyFactory(tempDir, false, false)
		test.That(t, err, test.ShouldBeNil)

		readCopier, err := NewLocalFileReadCopier([]string{
			tfs.SingleFileNested,
			tfs.InnerDir,
		}, true, false, factory)
		test.That(t, err, test.ShouldBeNil)

		err = readCopier.ReadAll(ctx)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, readCopier.Close(ctx), test.ShouldBeNil)
		s, ok := status.FromError(err)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, s.Code(), test.ShouldEqual, codes.NotFound)
		test.That(t, s.Message(), test.ShouldContainSubstring, "does not exist or is not a directory")
	})

	t.Run("preserve permissions on a nested file", func(t *testing.T) {
		tfs := shelltestutils.SetupTestFileSystem(t)

		beforeInfo, err := os.Stat(tfs.SingleFileNested)
		test.That(t, err, test.ShouldBeNil)
		t.Log("start with mode", beforeInfo.Mode())
		newMode := os.FileMode(0o444)
		test.That(t, beforeInfo.Mode(), test.ShouldNotEqual, newMode)
		test.That(t, os.Chmod(tfs.SingleFileNested, newMode), test.ShouldBeNil)
		modTime := time.Date(1988, 1, 2, 3, 0, 0, 0, time.UTC)
		test.That(t, os.Chtimes(tfs.SingleFileNested, time.Time{}, modTime), test.ShouldBeNil)
		relNestedPath := strings.TrimPrefix(tfs.SingleFileNested, tfs.Root)

		for _, preserve := range []bool{false, true} {
			t.Run(fmt.Sprintf("preserve=%t", preserve), func(t *testing.T) {
				tempDir := t.TempDir()

				factory, err := NewLocalFileCopyFactory(tempDir, preserve, false)
				test.That(t, err, test.ShouldBeNil)

				readCopier, err := NewLocalFileReadCopier([]string{tfs.Root}, true, false, factory)
				test.That(t, err, test.ShouldBeNil)

				test.That(t, readCopier.ReadAll(ctx), test.ShouldBeNil)
				test.That(t, readCopier.Close(ctx), test.ShouldBeNil)

				nestedCopy := filepath.Join(tempDir, filepath.Base(tfs.Root), relNestedPath)
				test.That(t, shelltestutils.DirectoryContentsEqual(tfs.Root, filepath.Join(tempDir, filepath.Base(tfs.Root))), test.ShouldBeNil)
				afterInfo, err := os.Stat(nestedCopy)
				test.That(t, err, test.ShouldBeNil)
				// mode follows the source with or without preserve (umask applies without)
				test.That(t, afterInfo.Mode(), test.ShouldEqual, newMode)
				if preserve {
					test.That(t, afterInfo.ModTime().UTC().String(), test.ShouldEqual, modTime.String())
				} else {
					test.That(t, afterInfo.ModTime().UTC().String(), test.ShouldNotEqual, modTime.String())
				}
			})
		}
	})

	t.Run("defaults applied when source mode unavailable", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("expects unix file modes; Windows fabricates 0666/0444")
		}
		tempDir := t.TempDir()

		factory, err := NewLocalFileCopyFactory(tempDir, false, false)
		test.That(t, err, test.ShouldBeNil)
		copier, err := factory.MakeFileCopier(ctx, CopyFilesSourceTypeSingleFile)
		test.That(t, err, test.ShouldBeNil)

		err = copier.Copy(ctx, File{
			RelativeName: "legacy",
			Data:         &stubFile{Reader: strings.NewReader("data"), info: fileInfoData{name: "legacy", size: 4}},
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, copier.Close(ctx), test.ShouldBeNil)

		info, err := os.Stat(filepath.Join(tempDir, "legacy"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, info.Mode(), test.ShouldEqual, os.FileMode(0o640))
	})

	t.Run("mode 0o000 file gets default permissions", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("chmod 0o000 becomes read-only 0444 on Windows; fallback branch unreachable")
		}
		srcDir := t.TempDir()
		srcPath := filepath.Join(srcDir, "unreadable")
		test.That(t, os.WriteFile(srcPath, []byte("data"), 0o644), test.ShouldBeNil)
		// open before chmod so the read works without root
		src, err := os.Open(srcPath)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, os.Chmod(srcPath, 0o000), test.ShouldBeNil)

		tempDir := t.TempDir()
		factory, err := NewLocalFileCopyFactory(tempDir, false, false)
		test.That(t, err, test.ShouldBeNil)
		copier, err := factory.MakeFileCopier(ctx, CopyFilesSourceTypeSingleFile)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, copier.Copy(ctx, File{RelativeName: "unreadable", Data: src}), test.ShouldBeNil)
		test.That(t, copier.Close(ctx), test.ShouldBeNil)

		info, err := os.Stat(filepath.Join(tempDir, "unreadable"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, info.Mode(), test.ShouldEqual, os.FileMode(0o640))
	})

	t.Run("stale read-only temp from an interrupted transfer", func(t *testing.T) {
		tempDir := t.TempDir()
		stalePath := filepath.Join(tempDir, "readonly.download")
		test.That(t, os.WriteFile(stalePath, []byte("partial"), 0o444), test.ShouldBeNil)

		factory, err := NewLocalFileCopyFactory(tempDir, false, false)
		test.That(t, err, test.ShouldBeNil)
		copier, err := factory.MakeFileCopier(ctx, CopyFilesSourceTypeSingleFile)
		test.That(t, err, test.ShouldBeNil)

		err = copier.Copy(ctx, File{
			RelativeName: "readonly",
			Data:         &stubFile{Reader: strings.NewReader("data"), info: fileInfoData{name: "readonly", size: 4, mode: 0o444}},
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, copier.Close(ctx), test.ShouldBeNil)

		rd, err := os.ReadFile(filepath.Join(tempDir, "readonly"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, rd, test.ShouldResemble, []byte("data"))
		_, err = os.Stat(stalePath)
		test.That(t, errors.Is(err, fs.ErrNotExist), test.ShouldBeTrue)
	})
}

// stubFile is an in-memory fs.File with arbitrary file info, e.g. for mimicking
// an older sender that omits the file mode.
type stubFile struct {
	*strings.Reader
	info fileInfoData
}

func (f *stubFile) Stat() (fs.FileInfo, error) { return f.info, nil }

func (f *stubFile) Close() error { return nil }
