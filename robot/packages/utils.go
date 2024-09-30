package packages

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	rutils "go.viam.com/rdk/utils"
)

// installCallback is the function signature that gets passed to installPackage.
type installCallback func(ctx context.Context, url, dstPath string) (checksum, contentType string, err error)

func installPackage(
	ctx context.Context,
	logger logging.Logger,
	packagesDir string,
	url string,
	p config.PackageConfig,
	installFn installCallback,
) error {
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
	checksum, contentType, err := installFn(ctx, url, dstPath)
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

	statusFile := packageSyncFile{
		PackageID:       p.Package,
		Version:         p.Version,
		ModifiedTime:    time.Now(),
		Status:          syncStatusDone,
		TarballChecksum: checksum,
	}

	err = writeStatusFile(p, statusFile, packagesDir)
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

		// There should be no non-dir files in the packages/data dir except .status.json files. Delete any that exist
		if packageTypeDir.Type()&os.ModeDir != os.ModeDir && !strings.HasSuffix(packageTypeDirName, statusFileExt) {
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
			_, expectedStatusFileToExist := expectedPackageDirectories[strings.TrimSuffix(packageDirName, statusFileExt)]
			if !expectedToExist && !expectedStatusFileToExist {
				logger.Debugf("Removing old package file(s) %s", packageDirName)
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

type syncStatus string

const (
	syncStatusDownloading syncStatus = "downloading"
	syncStatusDone        syncStatus = "done"
)

type packageSyncFile struct {
	PackageID       string     `json:"package_id"`
	Version         string     `json:"version"`
	ModifiedTime    time.Time  `json:"modified_time"`
	Status          syncStatus `json:"sync_status"`
	TarballChecksum string     `json:"tarball_checksum"`
}

var statusFileExt = ".status.json"

func packageIsSynced(pkg config.PackageConfig, packagesDir string, logger logging.Logger) bool {
	syncFile, err := readStatusFile(pkg, packagesDir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		logger.Infow("New package to download detected", "name", pkg.Name, "version", pkg.Version, "id", pkg.Package)
		return false
	case err != nil:
		logger.Warnw("Error reading status file for package",
			// We prefix log output with "package" to avoid ambiguity. E.g: `name` could mean
			// filename given the log line context.
			"packageName", pkg.Name, "packageVersion", pkg.Version, "packageId", pkg.Package, "packagesDir", packagesDir, "err", err)
		return false
	case syncFile.PackageID == pkg.Package && syncFile.Version == pkg.Version && syncFile.Status == syncStatusDone:
		logger.Debugf("Package already downloaded at %s, skipping.", pkg.LocalDataDirectory(packagesDir))
		return true
	default:
		logger.Infof("Package is not in sync for %s: status of '%s' (file) != '%s' (expected) and version of '%s' (file) != '%s' (expected)",
			pkg.Package, syncFile.Status, syncStatusDone, syncFile.Version, pkg.Version)
		return false
	}
}

func packagesAreSynced(packages []config.PackageConfig, packagesDir string, logger logging.Logger) bool {
	for _, pkg := range packages {
		if !packageIsSynced(pkg, packagesDir, logger) {
			return false
		}
	}
	return true
}

func getSyncFileName(packageLocalDataDirectory string) string {
	return packageLocalDataDirectory + statusFileExt
}

func readStatusFile(pkg config.PackageConfig, packagesDir string) (packageSyncFile, error) {
	syncFileName := getSyncFileName(pkg.LocalDataDirectory(packagesDir))
	//nolint:gosec // safe
	syncFileBytes, err := os.ReadFile(syncFileName)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return packageSyncFile{}, errors.Wrapf(err, "cannot find %s", syncFileName)
		}
		return packageSyncFile{}, err
	}
	var syncFile packageSyncFile
	if err := json.Unmarshal(syncFileBytes, &syncFile); err != nil {
		return packageSyncFile{}, err
	}
	return syncFile, nil
}

func writeStatusFile(pkg config.PackageConfig, statusFile packageSyncFile, packagesDir string) error {
	syncFileName := getSyncFileName(pkg.LocalDataDirectory(packagesDir))

	statusFileBytes, err := json.MarshalIndent(statusFile, "", "  ")
	if err != nil {
		return err
	}
	//nolint:gosec
	syncFile, err := os.Create(syncFileName)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", syncFileName)
	}
	if _, err := syncFile.Write(statusFileBytes); err != nil {
		return errors.Wrapf(err, "failed to write syncfile to %s", syncFileName)
	}
	if err := syncFile.Sync(); err != nil {
		return errors.Wrapf(err, "failed to sync %s", syncFileName)
	}

	return nil
}
