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
	"strings"

	"github.com/pkg/errors"
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

func unpackArchive(fromFile, toDir string) error {
	if err := os.MkdirAll(toDir, 0o700); err != nil {
		return err
	}

	//nolint:gosec // safe
	f, err := os.Open(fromFile)
	if err != nil {
		return err
	}
	defer utils.UncheckedErrorFunc(f.Close)

	archive, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer utils.UncheckedErrorFunc(archive.Close)

	type link struct {
		Name string
		Path string
	}
	links := []link{}
	symlinks := []link{}

	tarReader := tar.NewReader(archive)
	for {
		header, err := tarReader.Next()

		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return errors.Wrap(err, "read tar")
		}

		path := header.Name

		if path == "" || path == "./" {
			continue
		}

		//nolint:gosec
		path = filepath.Join(toDir, path)

		info := header.FileInfo()

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.Mkdir(path, info.Mode()); err != nil {
				return errors.Wrapf(err, "failed to create directory %s", path)
			}

		case tar.TypeReg:
			// This is required because it is possible create tarballs without a directory entry
			// but whose files names start with a new directory prefix
			// Ex: tar -czf package.tar.gz ./bin/module.exe
			parent := filepath.Dir(path)
			if err := os.MkdirAll(parent, info.Mode()); err != nil {
				return errors.Wrapf(err, "failed to create directory %q", parent)
			}
			//nolint:gosec
			outFile, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600|info.Mode().Perm())
			if err != nil {
				return errors.Wrapf(err, "failed to create file %s", path)
			}
			//nolint:gosec
			if _, err := io.Copy(outFile, tarReader); err != nil && !errors.Is(err, io.EOF) {
				return errors.Wrapf(err, "failed to copy file %s", path)
			}
			utils.UncheckedError(outFile.Close())
		case tar.TypeLink:
			//nolint:gosec
			name := filepath.Join(toDir, header.Linkname)
			links = append(links, link{Path: path, Name: name})
		case tar.TypeSymlink:
			//nolint:gosec
			linkTarget := filepath.Join(toDir, header.Linkname)
			symlinks = append(symlinks, link{Path: path, Name: linkTarget})
		}
	}

	// Now we make another pass creating the links
	for i := range links {
		if err := linkFile(links[i].Name, links[i].Path); err != nil {
			return errors.Wrapf(err, "failed to create link %s", links[i].Path)
		}
	}

	for i := range symlinks {
		if err := linkFile(symlinks[i].Name, symlinks[i].Path); err != nil {
			return errors.Wrapf(err, "failed to create link %s", links[i].Path)
		}
	}

	return nil
}

func linkFile(from, to string) error {
	link, err := os.Readlink(to)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if link == from {
		return nil
	}

	// remove any existing link or SymLink will fail.
	if link != "" {
		utils.UncheckedError(os.Remove(from))
	}

	return os.Symlink(from, to)
}

func isTarball(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".tar.gz") ||
		strings.HasSuffix(strings.ToLower(path), ".tgz")
}
