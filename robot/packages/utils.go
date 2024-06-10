package packages

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	rutils "go.viam.com/rdk/utils"
)

// installCallback is the function signature that gets passed to installPackage.
type installCallback func(ctx context.Context, url, dstPath string) (contentType string, err error)

func installPackage(ctx context.Context, logger logging.Logger, packagesDir, url string, p config.PackageConfig,
	nonEmptyPaths []string, installFn installCallback,
) error {
	if dataDir := p.LocalDataDirectory(packagesDir); dirExists(dataDir) {
		if checkNonemptyPaths(p.Name, logger, nonEmptyPaths) {
			logger.Debugf("Package already downloaded at %s, skipping.", dataDir)
			return nil
		}
	}

	// Create the parent directory for the package type if it doesn't exist
	if err := os.MkdirAll(p.LocalDataParentDirectory(packagesDir), 0o700); err != nil {
		return err
	}

	// Force redownload of package archive.
	if err := cleanup(packagesDir, p); err != nil {
		logger.Debug(err)
	}

	if p.Type == config.PackageTypeMlModel {
		symlinkPath, err := rutils.SafeJoinDir(packagesDir, p.Name)
		if err == nil {
			if err := os.Remove(symlinkPath); err != nil {
				utils.UncheckedError(err)
			}
		}
	}

	dstPath := p.LocalDownloadPath(packagesDir)
	contentType, err := installFn(ctx, url, dstPath)
	if err != nil {
		return err
	}

	if contentType != allowedContentType {
		utils.UncheckedError(cleanup(packagesDir, p))
		return fmt.Errorf("unknown content-type for package %s", contentType)
	}

	// unpack to temp directory to ensure we do an atomic rename once finished.
	tmpDataPath, err := os.MkdirTemp(p.LocalDataParentDirectory(packagesDir), "*.tmp")
	if err != nil {
		return errors.Wrap(err, "failed to create temp data dir path")
	}

	defer func() {
		// cleanup archive file.
		if err := os.Remove(dstPath); err != nil {
			logger.Debug(err)
		}
		if err := os.RemoveAll(tmpDataPath); err != nil {
			logger.Debug(err)
		}
	}()

	// unzip archive.
	err = unpackFile(ctx, dstPath, tmpDataPath)
	if err != nil {
		utils.UncheckedError(cleanup(packagesDir, p))
		return err
	}

	err = os.Rename(tmpDataPath, p.LocalDataDirectory(packagesDir))
	if err != nil {
		utils.UncheckedError(cleanup(packagesDir, p))
		return err
	}

	return nil
}

func cleanup(packagesDir string, p config.PackageConfig) error {
	return multierr.Combine(
		os.RemoveAll(p.LocalDataDirectory(packagesDir)),
		os.Remove(p.LocalDownloadPath(packagesDir)),
	)
}

// unpackFile extracts a tgz to a directory.
func unpackFile(ctx context.Context, fromFile, toDir string) error {
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
		if err := ctx.Err(); err != nil {
			return err
		}

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

		if path, err = rutils.SafeJoinDir(toDir, path); err != nil {
			return err
		}

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
			if err := os.MkdirAll(parent, 0o700); err != nil {
				return errors.Wrapf(err, "failed to create directory %q", parent)
			}
			//nolint:gosec // path sanitized with rutils.SafeJoin
			outFile, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600|info.Mode().Perm())
			if err != nil {
				return errors.Wrapf(err, "failed to create file %s", path)
			}
			if _, err := io.CopyN(outFile, tarReader, maxPackageSize); err != nil && !errors.Is(err, io.EOF) {
				return errors.Wrapf(err, "failed to copy file %s", path)
			}
			if err := outFile.Sync(); err != nil {
				return errors.Wrapf(err, "failed to sync %s", path)
			}
			utils.UncheckedError(outFile.Close())
		case tar.TypeLink:
			name := header.Linkname

			if name, err = rutils.SafeJoinDir(toDir, name); err != nil {
				return err
			}
			links = append(links, link{Path: path, Name: name})
		case tar.TypeSymlink:
			linkTarget, err := safeLink(toDir, header.Linkname)
			if err != nil {
				return err
			}
			symlinks = append(symlinks, link{Path: path, Name: linkTarget})
		}
	}

	// Now we make another pass creating the links
	for i := range links {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := linkFile(links[i].Name, links[i].Path); err != nil {
			return errors.Wrapf(err, "failed to create link %s", links[i].Path)
		}
	}

	for i := range symlinks {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := linkFile(symlinks[i].Name, symlinks[i].Path); err != nil {
			return errors.Wrapf(err, "failed to create link %s", links[i].Path)
		}
	}

	return nil
}

// checkNonemptyPaths returns true if all required paths are present and non-empty.
// This exists because we have no way to check the integrity of modules *after* they've been unpacked.
// (We do look at checksums of downloaded tarballs, though). Once we have a better integrity check
// system for unpacked modules, this should be removed.
func checkNonemptyPaths(packageName string, logger logging.Logger, absPaths []string) bool {
	missingOrEmpty := 0
	for _, nePath := range absPaths {
		if !path.IsAbs(nePath) {
			// note: we expect paths to be absolute in most cases.
			// We're okay not having corrupted files coverage for relative paths, and prefer it to
			// risking a re-download loop from an edge case.
			logger.Debugw("ignoring non-abs required path", "path", nePath, "package", packageName)
			continue
		}
		// note: os.Stat treats symlinks as their destination. os.Lstat would stat the link itself.
		info, err := os.Stat(nePath)
		if err != nil {
			if !os.IsNotExist(err) {
				logger.Warnw("ignoring non-NotExist error for required path",
					"path", nePath, "package", packageName, "error", err.Error())
			} else {
				logger.Warnw("a required path is missing, re-downloading", "path", nePath, "package", packageName)
				missingOrEmpty++
			}
		} else if info.Size() == 0 {
			missingOrEmpty++
			logger.Warnw("a required path is empty, re-downloading", "path", nePath, "package", packageName)
		}
	}
	return missingOrEmpty == 0
}

// commonCleanup is a helper for the various ManagerSyncer.Cleanup functions.
func commonCleanup(logger logging.Logger, expectedPackageDirectories map[string]bool, packagesDataDir string) error {
	topLevelFiles, err := os.ReadDir(packagesDataDir)
	if err != nil {
		return err
	}

	var allErrors error

	// A packageTypeDir is a directory that contains all of the packages for the specified type. ex: data/ml_model
	for _, packageTypeDir := range topLevelFiles {
		packageTypeDirName, err := rutils.SafeJoinDir(packagesDataDir, packageTypeDir.Name())
		if err != nil {
			allErrors = multierr.Append(allErrors, err)
			continue
		}

		// There should be no non-dir files in the packages/data dir. Delete any that exist
		if packageTypeDir.Type()&os.ModeDir != os.ModeDir {
			allErrors = multierr.Append(allErrors, os.Remove(packageTypeDirName))
			continue
		}
		// read all of the packages in the directory and delete those that aren't in expectedPackageDirectories
		packageDirs, err := os.ReadDir(packageTypeDirName)
		if err != nil {
			allErrors = multierr.Append(allErrors, err)
			continue
		}
		for _, packageDir := range packageDirs {
			packageDirName, err := rutils.SafeJoinDir(packageTypeDirName, packageDir.Name())
			if err != nil {
				allErrors = multierr.Append(allErrors, err)
				continue
			}
			_, expectedToExist := expectedPackageDirectories[packageDirName]
			if !expectedToExist {
				logger.Debugf("Removing old package %s", packageDirName)
				allErrors = multierr.Append(allErrors, os.RemoveAll(packageDirName))
			}
		}
		// re-read the directory, if there is nothing left in it, delete the directory
		packageDirs, err = os.ReadDir(packageTypeDirName)
		if err != nil {
			allErrors = multierr.Append(allErrors, err)
			continue
		}
		if len(packageDirs) == 0 {
			allErrors = multierr.Append(allErrors, os.RemoveAll(packageTypeDirName))
		}
	}
	return allErrors
}
