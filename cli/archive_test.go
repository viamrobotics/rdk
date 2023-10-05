package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"go.viam.com/test"
)

func TestGetArchiveFilePaths(t *testing.T) {
	// Create a temporary directory
	tempDir := t.TempDir()

	// Create dummy files
	files := []string{"file1.txt", "file2.txt"}
	for _, f := range files {
		fullPath := filepath.Join(tempDir, f)
		err := os.WriteFile(fullPath, []byte("content"), fs.ModePerm)
		test.That(t, err, test.ShouldBeNil)
	}

	// Invoke getArchiveFilePaths function
	foundFiles, err := getArchiveFilePaths([]string{tempDir})
	test.That(t, err, test.ShouldBeNil)

	// Validate found files
	test.That(t, foundFiles, test.ShouldHaveLength, len(files))
	for _, f := range foundFiles {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			t.Errorf("File %s was not found", f)
		}
	}
}

func TestCreateArchive(t *testing.T) {
	// Create a temporary directory
	tempDir := t.TempDir()

	// Create dummy files
	files := []string{"file1.txt", "file2.txt"}
	for _, f := range files {
		fullPath := filepath.Join(tempDir, f)
		err := os.WriteFile(fullPath, []byte("content"), os.ModePerm)
		test.That(t, err, test.ShouldBeNil)
	}

	// Obtain the paths of dummy files
	paths, err := getArchiveFilePaths([]string{tempDir})
	test.That(t, err, test.ShouldBeNil)

	// Invoke createArchive function
	var buf bytes.Buffer
	err = createArchive(paths, &buf, nil)
	test.That(t, err, test.ShouldBeNil)

	// Validate created archive
	gzr, err := gzip.NewReader(&buf)
	test.That(t, err, test.ShouldBeNil)
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for _, file := range files {
		header, err := tr.Next()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, header.Name, test.ShouldEndWith, file)
		fileContent := make([]byte, header.Size)
		_, err = tr.Read(fileContent)
		test.That(t, err, test.ShouldEqual, io.EOF)
		test.That(t, string(fileContent), test.ShouldEqual, "content")
	}
}
