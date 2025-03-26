package packages

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"go.viam.com/utils"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	rutils "go.viam.com/rdk/utils"
)

const (
	windowsOS = "windows" // constant for Windows OS identifier
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
		return fmt.Errorf("failed to create temp data dir path %w", err)
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

	renameDest := p.LocalDataDirectory(packagesDir)
	if runtime.GOOS == windowsOS {
		if _, err := os.Stat(renameDest); err == nil {
			logger.Debug("package rename destination exists, deleting")
			if err := os.RemoveAll(renameDest); err != nil {
				logger.Warnf("ignoring error from removing rename dest %s", err)
			}
		}
	}
	err = os.Rename(tmpDataPath, renameDest)
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
	return errors.Join(
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
			return fmt.Errorf("read tar %w", err)
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
				return fmt.Errorf("failed to create directory %q %w", path, err)
			}

		case tar.TypeReg:
			// This is required because it is possible create tarballs without a directory entry
			// but whose files names start with a new directory prefix
			// Ex: tar -czf package.tar.gz ./bin/module.exe
			parent := filepath.Dir(path)
			if err := os.MkdirAll(parent, 0o700); err != nil {
				return fmt.Errorf("failed to create directory %q %w", parent, err)
			}
			//nolint:gosec // path sanitized with rutils.SafeJoin
			outFile, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600|info.Mode().Perm())
			if err != nil {
				return fmt.Errorf("failed to create file %q %w", path, err)
			}
			if _, err := io.CopyN(outFile, tarReader, maxPackageSize); err != nil && !errors.Is(err, io.EOF) {
				return fmt.Errorf("failed to copy file %q %w", path, err)
			}
			if err := outFile.Sync(); err != nil {
				return fmt.Errorf("failed to sync %q %w", path, err)
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
			return fmt.Errorf("failed to create link %q %w", links[i].Path, err)
		}
	}

	for i := range symlinks {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := linkFile(symlinks[i].Name, symlinks[i].Path); err != nil {
			return fmt.Errorf("failed to create link %q %w", links[i].Path, err)
		}
	}

	return nil
}

// commonCleanup is a helper for the various ManagerSyncer.Cleanup functions.
func commonCleanup(logger logging.Logger, expectedPackageEntries map[string]bool, packagesDataDir string) error {
	logger.Infof("Starting cleanup with %d expected package entries", len(expectedPackageEntries))

	topLevelFiles, err := os.ReadDir(packagesDataDir)
	if err != nil {
		logger.Errorf("Failed to read packages data directory: %v", err)
		return err
	}

	var allErrors error

	// A packageTypeDir is a directory that contains all of the packages for the specified type. ex: data/ml_model
	for _, packageTypeDir := range topLevelFiles {
		packageTypeDirName, err := rutils.SafeJoinDir(packagesDataDir, packageTypeDir.Name())
		if err != nil {
			logger.Debugf("Failed to join directory name %s: %v", packageTypeDir.Name(), err)
			allErrors = errors.Join(allErrors, err)
			continue
		}

		// Handle regular files (non-directories)
		if packageTypeDir.Type()&os.ModeDir != os.ModeDir {
			// Skip status files and first run success files
			if strings.HasSuffix(packageTypeDirName, statusFileExt) ||
				strings.HasSuffix(packageTypeDirName, config.FirstRunSuccessSuffix) {
				continue
			}

			// Remove non-directory files
			if err := os.Remove(packageTypeDirName); err != nil {
				logger.Debugf("Failed to remove file %s: %v", packageTypeDirName, err)
				allErrors = errors.Join(allErrors, err)
			}
			continue
		}

		// This is a package type directory (e.g., "module")
		if runtime.GOOS == windowsOS {
			if err := cleanupPackageTypeDirectoryWindows(logger, packageTypeDirName, expectedPackageEntries); err != nil {
				allErrors = errors.Join(allErrors, err)
			}
		} else {
			if err := cleanupPackageTypeDirectoryGeneric(logger, packageTypeDirName, expectedPackageEntries); err != nil {
				allErrors = errors.Join(allErrors, err)
			}
		}

		// After cleanup, check if package type directory is empty and can be removed
		packageDirs, err := os.ReadDir(packageTypeDirName)
		if err != nil {
			logger.Debugf("Failed to read package type directory %s: %v", packageTypeDirName, err)
			allErrors = errors.Join(allErrors, err)
			continue
		}

		if len(packageDirs) == 0 {
			logger.Debugf("Removing empty package type directory: %s", packageTypeDirName)
			if err := os.RemoveAll(packageTypeDirName); err != nil {
				logger.Errorf("Failed to remove empty package type directory: %v", err)
				allErrors = errors.Join(allErrors, err)
			}
		}
	}

	return allErrors
}

// cleanupPackageTypeDirectoryGeneric handles cleanup for non-Windows platforms.
func cleanupPackageTypeDirectoryGeneric(logger logging.Logger, typeDir string, expectedPackageEntries map[string]bool) error {
	var allErrors error

	// Read all entries in the package type directory
	entries, err := os.ReadDir(typeDir)
	if err != nil {
		logger.Errorf("Failed to read package type directory: %v", err)
		return err
	}

	// Process each entry
	for _, entry := range entries {
		entryPath, err := rutils.SafeJoinDir(typeDir, entry.Name())
		if err != nil {
			logger.Errorf("Failed to join entry path: %v", err)
			allErrors = errors.Join(allErrors, err)
			continue
		}

		// Check if we should delete this entry
		if deletePackageEntry(expectedPackageEntries, entryPath) {
			logger.Infof("Removing old package: %s", entryPath)
			if err := os.RemoveAll(entryPath); err != nil {
				logger.Errorf("Failed to remove entry: %v", err)
				allErrors = errors.Join(allErrors, err)
			}
		}
	}

	return allErrors
}

// cleanupPackageTypeDirectoryWindows handles cleanup for Windows, using a hierarchical approach
// to clean up old versions while properly handling directories that might contain locked files.
func cleanupPackageTypeDirectoryWindows(logger logging.Logger, typeDir string, expectedPackageEntries map[string]bool) error {
	var allErrors error

	expectedPackageEntries[typeDir] = true

	// List the ID directories in this type directory
	idDirs, err := os.ReadDir(typeDir)
	if err != nil {
		logger.Errorf("Failed to read package type directory: %v", err)
		return err
	}

	// Process each ID directory
	for _, idEntry := range idDirs {
		// Skip files
		if !idEntry.IsDir() {
			logger.Debugf("Skipping non-directory entry: %s", idEntry.Name())
			continue
		}

		idPath := filepath.Join(typeDir, idEntry.Name())

		// List version directories in this ID directory
		versionDirs, err := os.ReadDir(idPath)
		if err != nil {
			logger.Errorf("Failed to read package ID directory: %v", err)
			allErrors = errors.Join(allErrors, err)
			continue
		}

		// Scan version directories and remove those not in expected map
		keepIDDir := false
		for _, versionEntry := range versionDirs {
			versionPath := filepath.Join(idPath, versionEntry.Name())
			if !deletePackageEntry(expectedPackageEntries, versionPath) {
				// This is a current version, keep it and its parent
				logger.Debugf("Keeping current version: %s", versionPath)
				keepIDDir = true
			} else {
				// This is an old version, remove it
				logger.Infof("Removing old package version: %s", versionPath)
				if err := os.RemoveAll(versionPath); err != nil {
					logger.Errorf("Failed to remove old version directory: %v", err)
					allErrors = errors.Join(allErrors, err)
				}
			}
		}

		// If this ID directory has no current versions, try to remove it
		if !keepIDDir {
			// Check if the directory is now empty after removing old versions
			remainingEntries, err := os.ReadDir(idPath)
			if err != nil {
				logger.Errorf("Failed to re-read ID directory: %v", err)
				allErrors = errors.Join(allErrors, err)
				continue
			}

			if len(remainingEntries) == 0 {
				if err := os.Remove(idPath); err != nil {
					logger.Errorf("Failed to remove empty ID directory: %v", err)
					allErrors = errors.Join(allErrors, err)
				}
			}
		}
	}

	return allErrors
}

// deletePackageEntry checks if a file or directory in the modules data directory should be deleted or not.
func deletePackageEntry(expectedPackageEntries map[string]bool, entryPath string) bool {
	// check if directory corresponds to a module version that is still managed by the package
	// manager - if so DO NOT delete it.
	if _, ok := expectedPackageEntries[entryPath]; ok {
		return false
	}

	// check if directory corresponds to a module version download status file - if so DO NOT delete it.
	if _, ok := expectedPackageEntries[strings.TrimSuffix(entryPath, statusFileExt)]; ok {
		return false
	}

	// check if directory corresponds to a first run success marker file - if so DO NOT delete it.
	if _, ok := expectedPackageEntries[strings.TrimSuffix(entryPath, config.FirstRunSuccessSuffix)]; ok {
		return false
	}

	// if we reached this point then this directory or file does not correspond to an actively-managed
	// module version, and it can safely be deleted.
	return true
}

type syncStatus string

const (
	syncStatusDownloading syncStatus = "downloading"
	syncStatusDone        syncStatus = "done"

	statusFileExt = ".status.json"
)

type packageSyncFile struct {
	PackageID       string     `json:"package_id"`
	Version         string     `json:"version"`
	ModifiedTime    time.Time  `json:"modified_time"`
	Status          syncStatus `json:"sync_status"`
	TarballChecksum string     `json:"tarball_checksum"`
}

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
			return packageSyncFile{}, fmt.Errorf("cannot find %q %w", syncFileName, err)
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
	if runtime.GOOS == windowsOS {
		if err := os.MkdirAll(pkg.LocalDataDirectory(packagesDir), os.ModeDir); err != nil {
			return err
		}
	}

	statusFileBytes, err := json.MarshalIndent(statusFile, "", "  ")
	if err != nil {
		return err
	}
	//nolint:gosec
	syncFile, err := os.Create(syncFileName)
	if err != nil {
		return fmt.Errorf("failed to create %q %w", syncFileName, err)
	}
	if _, err := syncFile.Write(statusFileBytes); err != nil {
		return fmt.Errorf("failed to write syncfile to %q %w", syncFileName, err)
	}
	if err := syncFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync %q %w", syncFileName, err)
	}

	return nil
}
