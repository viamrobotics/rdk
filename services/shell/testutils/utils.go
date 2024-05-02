// Package shelltestutils contains test utilities for working with the shell service
// like test file system directories and comparison tools.
package shelltestutils

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils"
)

// DirectoryContentsEqual checks if the directory contents on the left and right are equal,
// excluding metadata (atime, mtime, mode, etc.)
//
//nolint:gosec // for testing
func DirectoryContentsEqual(leftRoot, rightRoot string) error {
	traverseAndGather := func(root string) (map[string]*os.File, error) {
		files := map[string]*os.File{}

		var traverseAndGatherInner func(relDir, currentDir string) error
		traverseAndGatherInner = func(relDir, currentDir string) error {
			entries, err := os.ReadDir(currentDir)
			if err != nil {
				return err
			}

			for _, entry := range entries {
				entryPath := filepath.Join(currentDir, entry.Name())
				relPath := filepath.Join(relDir, entry.Name())
				file, err := os.Open(entryPath)
				if err != nil {
					return err
				}
				info, err := file.Stat()
				if err != nil {
					return err
				}
				if info.IsDir() {
					if err := traverseAndGatherInner(relPath, entryPath); err != nil {
						return err
					}
				}
				files[relPath] = file
			}
			return nil
		}
		if err := traverseAndGatherInner("", root); err != nil {
			return nil, err
		}
		return files, nil
	}
	closeFiles := func(files map[string]*os.File) {
		for _, file := range files {
			utils.UncheckedError(file.Close())
		}
	}

	leftFiles, err := traverseAndGather(leftRoot)
	if err != nil {
		return err
	}
	defer closeFiles(leftFiles)

	rightFiles, err := traverseAndGather(rightRoot)
	if err != nil {
		return err
	}
	defer closeFiles(rightFiles)

	if len(leftFiles) != len(rightFiles) {
		return fmt.Errorf(
			"%q has %d files while %q has %d files",
			leftRoot, len(leftFiles),
			rightRoot, len(rightFiles),
		)
	}
	for leftFilePath, leftFile := range leftFiles {
		rightFile, ok := rightFiles[leftFilePath]
		if !ok {
			return fmt.Errorf("right does not have %q", leftFilePath)
		}
		delete(rightFiles, leftFilePath)

		if filepath.Base(leftFile.Name()) != filepath.Base(rightFile.Name()) {
			panic(fmt.Errorf("unexpected mismatched name %q, %q", leftFile.Name(), rightFile.Name()))
		}
		leftInfo, err := leftFile.Stat()
		if err != nil {
			return err
		}
		rightInfo, err := rightFile.Stat()
		if err != nil {
			return err
		}
		if leftInfo.IsDir() != rightInfo.IsDir() {
			return fmt.Errorf("%q directory/file mismatch", leftFilePath)
		}
		if leftInfo.IsDir() {
			continue
		}
		leftRd, err := io.ReadAll(leftFile)
		if err != nil {
			return err
		}
		rightRd, err := io.ReadAll(rightFile)
		if err != nil {
			return err
		}
		if !bytes.Equal(leftRd, rightRd) {
			return fmt.Errorf("%q contents not equal", leftFilePath)
		}
	}
	if len(rightFiles) != 0 {
		return fmt.Errorf("left does not have the following files %v", rightFiles)
	}
	return nil
}

// TestFileSystemInfo contains information about where the file system is and
// some various file paths/details.
type TestFileSystemInfo struct {
	Root                 string
	SingleFileNested     string
	SingleFileNestedData []byte
	InnerDir             string
}

// SetupTestFileSystem sets up and returns a test file system for playing around with files. It
// contains a nested symlinked directory.
//
//nolint:gosec
func SetupTestFileSystem(t *testing.T, prefixData ...string) TestFileSystemInfo {
	tempStaticDir := t.TempDir()
	helloFile := filepath.Join(tempStaticDir, "hello")
	worldFile := filepath.Join(tempStaticDir, "world")
	emptyDir := filepath.Join(tempStaticDir, "emptydir")
	innerDir := filepath.Join(tempStaticDir, "inner")
	innerDirEmptyFile := filepath.Join(innerDir, "file")
	innerDirExampleFile := filepath.Join(innerDir, "example")
	tempStaticDirForLinking := t.TempDir()
	symlinkFile := filepath.Join(tempStaticDirForLinking, "kibby")

	helloFileData := []byte(strings.Repeat(strings.Join(prefixData, "")+"world", 32))
	test.That(t, os.WriteFile(helloFile, helloFileData, 0o666), test.ShouldBeNil)
	test.That(t, os.WriteFile(worldFile, nil, 0o644), test.ShouldBeNil)
	test.That(t, os.Mkdir(emptyDir, 0o750), test.ShouldBeNil)
	test.That(t, os.Mkdir(innerDir, 0o750), test.ShouldBeNil)
	test.That(t, os.WriteFile(innerDirEmptyFile, nil, 0o644), test.ShouldBeNil)
	innerDirExampleFileData := []byte(strings.Repeat(strings.Join(prefixData, "")+"a", 1<<18))
	test.That(t, os.WriteFile(innerDirExampleFile, innerDirExampleFileData, 0o644), test.ShouldBeNil)
	test.That(t, os.Symlink(tempStaticDirForLinking, filepath.Join(innerDir, "elsewhere")), test.ShouldBeNil)
	test.That(t, os.WriteFile(symlinkFile, []byte(strings.Repeat(strings.Join(prefixData, "")+"a", 1<<10)), 0o644), test.ShouldBeNil)

	return TestFileSystemInfo{
		Root:                 tempStaticDir,
		SingleFileNested:     innerDirExampleFile,
		SingleFileNestedData: innerDirExampleFileData,
		InnerDir:             innerDir,
	}
}
