package cli

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"

	"go.viam.com/utils"
	"golang.org/x/exp/maps"
)

// getArchiveFilePaths traverses the provided rootpaths recursively,
// collecting the file paths of all regular files and symlinks.
// This list of paths should be passed to createArchive.
func getArchiveFilePaths(rootpaths []string) ([]string, error) {
	files := map[string]bool{}
	for _, pathRoot := range rootpaths {
		err := filepath.WalkDir(filepath.Clean(pathRoot), func(path string, info fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			// If the file is regular (no mode type set) or is a symlink, add it to the files
			// The only files we are excluding are special files:
			// 	 ModeNamedPipe | ModeSocket | ModeDevice | ModeCharDevice | ModeIrregular
			if info.Type()&fs.ModeType&^fs.ModeSymlink == 0 {
				files[path] = true
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return maps.Keys(files), nil
}

// createArchive compresses and archives the provided file paths into a tar.gz format,
// writing the resulting binary data to the supplied "buf" writer.
// If "stdout" is provided, the function outputs compression progress information.
func createArchive(files []string, buf, stdout io.Writer) error {
	// Create new Writers for gzip and tar
	// These writers are chained. Writing to the tar writer will
	// write to the gzip writer which in turn will write to
	// the "buf" writer
	gw := gzip.NewWriter(buf)
	//nolint:errcheck
	defer gw.Close()
	tw := tar.NewWriter(gw)
	//nolint:errcheck
	defer tw.Close()

	// Close the line with the progress reading
	defer func() {
		if stdout != nil {
			printf(stdout, "")
		}
	}()

	if stdout != nil {
		fmt.Fprintf(stdout, "\rCompressing... %d%% (%d/%d files)", 0, 1, len(files)) // no newline
	}
	// Iterate over files and add them to the tar archive
	for i, file := range files {
		err := addToArchive(tw, file)
		if err != nil {
			return err
		}
		if stdout != nil {
			compressPercent := int(math.Ceil(100 * float64(i+1) / float64(len(files))))
			fmt.Fprintf(stdout, "\rCompressing... %d%% (%d/%d files)", compressPercent, i+1, len(files)) // no newline
		}
	}
	return nil
}

func addToArchive(tw *tar.Writer, filename string) error {
	// Open the file which will be written into the archive
	//nolint:gosec
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer utils.UncheckedErrorFunc(file.Close)

	info, err := file.Stat()
	if err != nil {
		return err
	}

	// Create a tar Header from the FileInfo data
	header, err := tar.FileInfoHeader(info, info.Name())
	if err != nil {
		return err
	}
	// See tar.FileInfoHeader:
	//   Since fs.FileInfo's Name method only returns the base name of
	//   the file it describes, it may be necessary to modify Header.Name
	//   to provide the full path name of the file.
	header.Name = filename

	err = tw.WriteHeader(header)
	if err != nil {
		return err
	}

	// Copy file content to tar archive
	_, err = io.Copy(tw, file)
	if err != nil {
		return err
	}

	return nil
}
