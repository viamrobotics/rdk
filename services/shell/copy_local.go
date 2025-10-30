package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/multierr"
	"go.viam.com/utils"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// copy_local supports local filesystem copy operations and is agnostic to which
// side of the RPC copying is happening from.

// NewLocalFileCopyFactory returns a FileCopyFactory that is responsible for making
// FileCopiers that copy from the local filesystem. The destination is used later on
// in tandem with a the CopyFilesSourceType passed into MakeFileCopier.
func NewLocalFileCopyFactory(
	destination string,
	preserve bool,
	relativeToHome bool,
) (FileCopyFactory, error) {
	// fixup destination to something we can work with
	destination, err := fixPeerPath(destination, true, relativeToHome)
	if err != nil {
		return nil, err
	}
	return &localFileCopyFactory{destination: destination, preserve: preserve}, nil
}

type localFileCopyFactory struct {
	destination string
	preserve    bool
}

// MakeFileCopier makes a new FileCopier that is ready to copy files into the factory's
// file destination.
func (f *localFileCopyFactory) MakeFileCopier(ctx context.Context, sourceType CopyFilesSourceType) (FileCopier, error) {
	finalDestination := f.destination
	var overrideName string
	switch sourceType {
	case CopyFilesSourceTypeMultipleFiles:
		// for multiple files (a b c machine:~/some/dir), ~/some/dir needs to already exist
		// as a directory
		dstInfo, err := os.Stat(f.destination)
		if err != nil || dstInfo == nil || !dstInfo.IsDir() {
			return nil, fmt.Errorf("%q does not exist or is not a directory", f.destination)
		}
		if err := os.MkdirAll(filepath.Dir(f.destination), 0o750); err != nil {
			return nil, fmt.Errorf("MkdirAll all failed (%s): %w", f.destination, err)
		}
	case CopyFilesSourceTypeSingleFile, CopyFilesSourceTypeSingleDirectory:
		// for single files (a machine:~/some/dir_or_file):
		// if destination exists and
		// 		it is a directory, then put the source file/directory in it.
		//		it is a file and the source is a file, overwrite.
		//      it is a file and the source is a directory, error.
		// or if destination does not exist and
		//		if the parent exists and
		//			it is a directory, then put the source/file directory in it.
		//			it is a file, then error.
		//		the parent does not exist, then error.

		var rename bool
		dstInfo, err := os.Stat(f.destination)
		// if destination exists and
		if err == nil {
			if dstInfo == nil {
				return nil, fmt.Errorf("expected file info for %q", f.destination)
			}
			switch {
			case dstInfo.IsDir():
				// it is a directory, then put the source file/directory in it
				// destination stays the same
			case sourceType == CopyFilesSourceTypeSingleFile:
				// it is a file and the source is a file, overwrite
				// destination becomes parent
				rename = true
			default:
				// it is a file and the source is a directory, error
				return nil, fmt.Errorf("destination %q is an existing file", f.destination)
			}
		} else { // or if destination does not exist and
			parent := filepath.Dir(f.destination)
			parentInfo, err := os.Stat(parent)
			if err != nil {
				// the parent does not exist, then error
				return nil, err
			}
			if parentInfo == nil {
				return nil, fmt.Errorf("expected file info for %q", parent)
			}
			// if the parent exists and
			if parentInfo.IsDir() {
				// it is a directory, then put the source/file directory in it
				// destination becomes parent
				rename = true
			} else {
				// it is a file, then error
				return nil, fmt.Errorf("parent of destination %q is an existing file, not a directory", f.destination)
			}
		}

		if rename {
			overrideName = filepath.Base(f.destination)
			finalDestination = filepath.Dir(f.destination)
		}
	case CopyFilesSourceTypeMultipleUnknown:
		fallthrough
	default:
		return nil, fmt.Errorf("do not know how to process source copy type %q", sourceType)
	}
	return &localFileCopier{
		sourceType:   sourceType,
		dst:          finalDestination,
		overrideName: overrideName,
		preserve:     f.preserve,
	}, nil
}

func (f *localFileCopyFactory) Close(ctx context.Context) error {
	return nil
}

// A localFileCopier takes in files and copies them to a set destination. It should be created
// with a localFileCopyFactory.
type localFileCopier struct {
	sourceType   CopyFilesSourceType
	dst          string
	overrideName string
	preserve     bool
}

func (copier *localFileCopier) Copy(ctx context.Context, file File) error {
	defer func() {
		utils.UncheckedError(file.Data.Close())
	}()

	fileName := file.RelativeName
	if copier.overrideName != "" {
		// only change the first part of the file name for directory
		// renaming purposes. This is based on SCP logic.
		fileSplit := splitPath(file.RelativeName)
		if len(fileSplit) == 0 {
			fileSplit = []string{copier.overrideName}
		} else {
			fileSplit[0] = copier.overrideName
		}
		fileName = filepath.Join(fileSplit...)
	}
	fullPath := filepath.Join(copier.dst, fileName)

	fileInfo, err := file.Data.Stat()
	if err != nil {
		return err
	}
	if fileInfo == nil {
		return fmt.Errorf("expected file info for %q to be non-nil", fileName)
	}

	parentPath := filepath.Dir(fullPath)
	if parentPath == "" {
		return fmt.Errorf("expected non-empty parent path to destination %q", copier.dst)
	}
	if fileInfo, err := os.Stat(parentPath); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		// this will later be updated with chmod for a specific directory. It's safe to make
		// directories here because we assume whoever constructed us has validated the top-level
		// directory as existing or created.
		//nolint:gosec // this is from an authenticated/authorized connection
		if err := os.MkdirAll(parentPath, 0o755); err != nil {
			return err
		}
	} else if fileInfo == nil {
		return fmt.Errorf("expected file info for %q to be non-nil", parentPath)
	} else if !fileInfo.IsDir() {
		return fmt.Errorf("invariant: parent path %q should have been a directory", parentPath)
	}

	var fileMode fs.FileMode
	modTime := time.Now()
	switch {
	case copier.preserve:
		modTime = fileInfo.ModTime()
		fileMode = fileInfo.Mode()
	case fileInfo.IsDir():
		fileMode = 0o750
	default:
		fileMode = 0o640
	}

	if fileInfo.IsDir() {
		if err := os.Mkdir(fullPath, fileMode); err != nil {
			if !errors.Is(err, fs.ErrExist) {
				return err
			}
			if copier.preserve {
				// Update the mode since it maye have been created via mkdirall above
				if err := os.Chmod(fullPath, fileMode); err != nil {
					return err
				}
			}
		}
	} else {
		// Since file may be streamed (see copyFile in copy_rpc.go), write to a temp file (filename.download) and rename after the download
		// has completed. This way a temporary network blip won't leave a corrupted partial file in the expected location.
		// Technically the temp file can be deleted upon any Copy error, but it will also be clobbered next time we retry.
		fullPathTmp := fullPath + ".download"

		//nolint:gosec // this is from an authenticated/authorized connection
		localFile, err := os.OpenFile(fullPathTmp, os.O_CREATE|os.O_WRONLY, fileMode)
		if err != nil {
			return err
		}
		if _, err := io.Copy(localFile, file.Data); err != nil {
			closeErr := localFile.Close()
			// Remove partially downloaded file if possible. Don't error if it does not exist.
			cleanupErr := os.Remove(fullPathTmp)
			if errors.Is(cleanupErr, fs.ErrNotExist) {
				cleanupErr = nil
			}
			return multierr.Combine(err, closeErr, cleanupErr)
		}
		if err := os.Rename(fullPathTmp, fullPath); err != nil {
			return err
		}
	}
	if copier.preserve {
		// Update the mode since it maye have been created via mkdirall above
		// or modified by umask
		if err := os.Chmod(fullPath, fileMode); err != nil {
			return err
		}
		// Note(erd): maybe support access time in the future if needed
		if err := os.Chtimes(fullPath, time.Now(), modTime); err != nil {
			return err
		}
	}
	return nil
}

// Close does nothing.
func (copier *localFileCopier) Close(ctx context.Context) error {
	return nil
}

type localFileReadCopier struct {
	filesToCopy []*os.File
	copyFactory FileCopyFactory
}

// NewLocalFileReadCopier returns a FileReadCopier that will have its ReadAll
// method iteratively copy each file found indicated by paths into a FileCopier
// created by the FileCopyFactory. The Factory is used since we don't yet know
// what type of files we are going to copy until ReadAll is called.
func NewLocalFileReadCopier(
	paths []string,
	allowRecursive bool,
	relativeToHome bool,
	copyFactory FileCopyFactory,
) (FileReadCopier, error) {
	var filesToCopy []*os.File

	for _, p := range paths {
		p, err := fixPeerPath(p, false, relativeToHome)
		if err != nil {
			return nil, err
		}

		//nolint:gosec // this is from an authenticated/authorized connection
		fileToCopy, err := os.Open(p)
		if err != nil {
			return nil, err
		}
		if !allowRecursive {
			fileInfo, err := fileToCopy.Stat()
			if err != nil {
				return nil, err
			}
			if fileInfo.IsDir() {
				details := &errdetails.BadRequest_FieldViolation{
					Field:       "paths",
					Description: fmt.Sprintf("local %q is a directory but copy recursion not used", p),
				}
				s, err := status.New(codes.InvalidArgument, ErrMsgDirectoryCopyRequestNoRecursion).WithDetails(details)
				if err != nil {
					return nil, err
				}
				return nil, s.Err()
			}
		}
		filesToCopy = append(filesToCopy, fileToCopy)
	}
	if len(filesToCopy) == 0 {
		return nil, errors.New("no files provided to copy")
	}
	return &localFileReadCopier{filesToCopy: filesToCopy, copyFactory: copyFactory}, nil
}

// ErrMsgDirectoryCopyRequestNoRecursion should be returned when a file is included in a path for a copy request
// where recursion is not enabled.
var ErrMsgDirectoryCopyRequestNoRecursion = "file is a directory but copy recursion not used"

// ReadAll processes and copies each file one by one into a newly constructed FileCopier until
// complete.
func (reader *localFileReadCopier) ReadAll(ctx context.Context) error {
	if len(reader.filesToCopy) == 0 {
		return nil
	}

	var sourceType CopyFilesSourceType
	if len(reader.filesToCopy) == 1 {
		fileInfo, err := reader.filesToCopy[0].Stat()
		if err != nil {
			return err
		}
		if fileInfo.IsDir() {
			sourceType = CopyFilesSourceTypeSingleDirectory
		} else {
			sourceType = CopyFilesSourceTypeSingleFile
		}
	} else {
		sourceType = CopyFilesSourceTypeMultipleFiles
	}

	copier, err := reader.copyFactory.MakeFileCopier(ctx, sourceType)
	if err != nil {
		return err
	}
	defer func() {
		utils.UncheckedError(copier.Close(ctx))
	}()

	// Note: okay with recursion for now. may want to check depth later...

	makeRelName := func(relDir string, file *os.File) string {
		return filepath.Join(relDir, filepath.Base(file.Name()))
	}
	var copyFiles func(relDir string, files []*os.File) error
	copyFiles = func(relDir string, files []*os.File) error {
		for _, f := range files {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			fileInfo, err := f.Stat()
			if err != nil {
				return err
			}
			if fileInfo.IsDir() {
				filesEntriesInDir, err := f.ReadDir(0)
				if err != nil {
					return err
				}
				filesInDir := make([]*os.File, 0, len(filesEntriesInDir))
				for _, dirEntry := range filesEntriesInDir {
					entryPath := filepath.Join(f.Name(), dirEntry.Name())
					//nolint:gosec // this is from an authenticated/authorized connection
					entryFile, err := os.Open(entryPath)
					if err != nil {
						for _, f := range filesInDir {
							utils.UncheckedError(f.Close())
						}
						return err
					}
					filesInDir = append(filesInDir, entryFile)
				}

				if err := copyFiles(makeRelName(relDir, f), filesInDir); err != nil {
					return err
				}
			}

			if err := copier.Copy(ctx, File{
				RelativeName: makeRelName(relDir, f),
				Data:         f,
			}); err != nil {
				return err
			}
		}

		return nil
	}
	if err := copyFiles("", reader.filesToCopy); err != nil {
		return err
	}

	return nil
}

// Close closes all files that were used for copying at the top-level. They have
// likely already been closed deeper down the stack.
func (reader *localFileReadCopier) Close(ctx context.Context) error {
	var errs error
	for _, f := range reader.filesToCopy {
		if err := f.Close(); err != nil && !errors.Is(err, fs.ErrClosed) {
			errs = multierr.Combine(errs, err)
		}
	}
	return errs
}

var errUnexpectedEmptyPath = errors.New("unexpected empty path")

// fixPeerPath works with the usage of ~ or empty paths and turns
// them into the proper HOME pathings.
// Security Note: this is the only time we end up interpreting a user's path
// string before it's passed to a file related syscall.
func fixPeerPath(path string, allowEmpty, relativeToHome bool) (string, error) {
	if !filepath.IsAbs(path) {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}

		switch {
		case strings.HasPrefix(path, "~/"):
			path = strings.Replace(path, "~", homeDir, 1)
		case path == "":
			if !allowEmpty {
				return "", errUnexpectedEmptyPath
			}
			if relativeToHome {
				path = homeDir
			} else {
				path, err = filepath.Abs("")
				if err != nil {
					return "", err
				}
			}
		case relativeToHome:
			// From path has us use HOME paths
			path = filepath.Join(homeDir, path)
		default:
			// To path has us use CWD paths
			path, err = filepath.Abs(path)
			if err != nil {
				return "", err
			}
		}
	}
	return path, nil
}

// this seems to mostly work as a cross-platform splitter.
func splitPath(path string) []string {
	var split []string
	vol := filepath.VolumeName(path)
	start := len(vol)
	for i := start; i < len(path); i++ {
		if os.IsPathSeparator(path[i]) {
			split = append(split, path[start:i])
			start = i + 1
		} else if i+1 == len(path) {
			split = append(split, path[start:])
		}
	}
	return split
}
