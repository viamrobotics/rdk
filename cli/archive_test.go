package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.viam.com/test"
)

func TestArchive(t *testing.T) {
	tempDir := t.TempDir()

	regularFiles := []string{"file1.txt", "file2.txt"}
	allFiles := append([]string{"file1link"}, regularFiles...)

	for _, filename := range regularFiles {
		fullPath := filepath.Join(tempDir, filename)
		err := os.WriteFile(fullPath, []byte("content"), fs.ModePerm)
		test.That(t, err, test.ShouldBeNil)
	}

	// Create file1link as a symlink to file1.txt
	linkName := filepath.Join(tempDir, "file1link")
	err := os.Symlink("file1.txt", linkName)
	test.That(t, err, test.ShouldBeNil)

	t.Run("archive file paths", func(t *testing.T) {
		foundFiles, err := getArchiveFilePaths([]string{tempDir})
		test.That(t, err, test.ShouldBeNil)

		test.That(t, foundFiles, test.ShouldHaveLength, 3)
		for _, f := range foundFiles {
			if _, err := os.Stat(f); os.IsNotExist(err) {
				t.Errorf("file %s was not found", f)
			}
		}
	})

	t.Run("create archive", func(t *testing.T) {
		paths, err := getArchiveFilePaths([]string{tempDir})
		test.That(t, err, test.ShouldBeNil)

		var buf bytes.Buffer
		err = createArchive(paths, &buf, nil)
		test.That(t, err, test.ShouldBeNil)

		gzr, err := gzip.NewReader(&buf)
		test.That(t, err, test.ShouldBeNil)
		defer gzr.Close()

		tr := tar.NewReader(gzr)

		// Map to track which files we've checked
		verifiedFiles := make(map[string]bool)
		for range allFiles {
			header, err := tr.Next()
			test.That(t, err, test.ShouldBeNil)

			expectedFile := false
			for _, fileName := range allFiles {
				if strings.HasSuffix(header.Name, fileName) {
					expectedFile = true
					verifiedFiles[fileName] = true
					break
				}
			}
			test.That(t, expectedFile, test.ShouldBeTrue)

			fileContent := make([]byte, header.Size)
			_, err = tr.Read(fileContent)
			test.That(t, err, test.ShouldEqual, io.EOF)
			// Note that even the symlink should have a value of `content`
			test.That(t, string(fileContent), test.ShouldEqual, "content")
		}

		// Ensure we visited all files
		test.That(t, len(verifiedFiles), test.ShouldEqual, len(allFiles))
		for _, file := range allFiles {
			test.That(t, verifiedFiles[file], test.ShouldBeTrue)
		}
	})
}
