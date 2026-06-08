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
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	errw "github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	rutils "go.viam.com/rdk/utils"
	"go.viam.com/rdk/utils/diskusage"
)

const partialsDirName = "part"

// cleanup partial downloads that were started this long ago
const maxPartialAge = 72 * time.Hour

// enoughFreeSpace reports whether the volume holding path has at least minBytes
// available. It is a package var so tests can inject a low-space result without
// having to actually fill a disk.
var enoughFreeSpace = diskusage.EnoughFreeSpace

// errInsufficientDiskSpace is returned by checkDiskSpace when blocking is on and the volume is
// low. Callers use errors.Is to tell a disk-space refusal from other failures (e.g. a corrupt
// archive) and surface an accurate message.
var errInsufficientDiskSpace = errors.New("not enough free disk space")

// diskSpaceBlockingEnabled reports whether low-space conditions should refuse the operation
// (download, local copy, or unpack). Default (unset) is false: low-space is logged but the
// operation proceeds (log-only). See rutils.ViamEnableDiskSpaceBlockEnvVar.
func diskSpaceBlockingEnabled() bool {
	return rutils.GetenvBool(rutils.ViamEnableDiskSpaceBlockEnvVar, false)
}

// checkDiskSpace checks whether the volume holding path has required bytes free. It returns
// low=true whenever space is low (so callers can warn at most once) and always logs a warning
// then. It returns an error (refusing the op) only when blocking is enabled via
// ViamEnableDiskSpaceBlockEnvVar. A failed check is logged and treated as "proceed" so a broken
// statfs never blocks installs. desc names the op in logs/errors; extraFields extend the warning.
func checkDiskSpace(logger logging.Logger, path, desc string, required uint64, extraFields ...any) (low bool, err error) {
	enough, available, err := enoughFreeSpace(path, required)
	if err != nil {
		logger.Warnw("could not check free disk space; proceeding",
			append([]any{"desc", desc, "path", path, "error", err}, extraFields...)...)
		return false, nil
	}
	if enough {
		return false, nil
	}
	blocking := diskSpaceBlockingEnabled()
	logger.Warnw("not enough free disk space",
		append([]any{
			"desc", desc, "path", path,
			"available", diskusage.FormatBytes(available),
			"required", diskusage.FormatBytes(required),
			"blocking", blocking,
		}, extraFields...)...)
	if !blocking {
		return true, nil
	}
	return true, fmt.Errorf("%w for %s: %s available, %s required",
		errInsufficientDiskSpace, desc, diskusage.FormatBytes(available), diskusage.FormatBytes(required))
}

// create a partials folder for this URL and return a destination path for the file.
func partialDownloadPath(parentDir, rawURL string) (string, error) {
	var filename string
	if parsed, err := url.Parse(rawURL); err != nil {
		filename = "UNPARSED"
	} else {
		filename = rutils.RIndex(strings.Split(parsed.Path, "/"), -1, "UNPARSED")
	}

	partialsDir := filepath.Join(parentDir, partialsDirName, rutils.HashString(rawURL, 7))
	if err := os.MkdirAll(parentDir, 0o750); err != nil {
		return "", err
	}
	return filepath.Join(partialsDir, filename+".part"), nil
}

// installCallback is the function signature that gets passed to installPackage.
type installCallback func(ctx context.Context, url, dstPath string) (checksum, contentType string, err error)

// the common logic that wraps an `installCallback` function across different package managers.
// when `supportsPartial` is true, this goes to `partialDownloadPath` instead of `LocalDownloadPath`.
func installPackage(
	ctx context.Context,
	logger logging.Logger,
	packagesDir string,
	url string,
	p config.PackageConfig,
	supportsPartial bool,
	installFn installCallback,
) error {
	// Disk guarding happens where bytes are written: installFn pre-filters when it knows the
	// artifact size (skip an obviously-too-big download or local copy), and unpackFile checks the
	// floor incrementally as it extracts (the unpacked size isn't known up front).

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

	// The paths here are:
	// LocalDownloadPath: the destination of the download
	// PartialDownloadPath: the download destination for partials, which have different cleanup logic
	// tmpDataPath: a successful download is unpacked into here
	// renameDest: after unpacking, we rename atomically to the final location

	parentDir := p.LocalDataParentDirectory(packagesDir)
	var dstPath string
	if supportsPartial {
		var err error
		if dstPath, err = partialDownloadPath(parentDir, url); err != nil {
			return errw.Wrap(err, "creating partials dir")
		}
	} else {
		dstPath = p.LocalDownloadPath(packagesDir)
	}
	checksum, contentType, err := installFn(ctx, url, dstPath)
	if err != nil {
		return err
	}

	if contentType != allowedContentType {
		utils.UncheckedError(cleanup(packagesDir, p))
		return fmt.Errorf("unknown content-type for package %s", contentType)
	}

	// unpack to temp directory to ensure we do an atomic rename once finished.
	tmpDataPath, err := os.MkdirTemp(parentDir, "*.tmp")
	if err != nil {
		return err
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
	err = unpackFile(ctx, logger, dstPath, tmpDataPath)
	if err != nil {
		// A low-space refusal is transient, not a bad archive: don't write syncStatusFailed
		// (packageIsSynced treats "failed" as synced to avoid retrying forever, which would block
		// re-download until the version changes). Without it the next sync retries once there's
		// space. Surface as-is, not as "try a different version".
		if errors.Is(err, errInsufficientDiskSpace) {
			utils.UncheckedError(cleanup(packagesDir, p))
			return err
		}
		statusFile := packageSyncFile{
			PackageID:       p.Package,
			Version:         p.Version,
			ModifiedTime:    time.Now(),
			Status:          syncStatusFailed,
			TarballChecksum: "",
		}
		utils.UncheckedError(writeStatusFile(p, statusFile, packagesDir))
		utils.UncheckedError(cleanup(packagesDir, p))
		return fmt.Errorf("failed to unzip archive, please try a different version: %w", err)
	}

	renameDest := p.LocalDataDirectory(packagesDir)
	if runtime.GOOS == "windows" {
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
// unpackDiskCheckInterval batches the free-space re-check during unpack so we don't statfs per
// file: a run of small files is checked once per interval of accumulated data, while any file
// larger than the interval is checked on its own (its size is folded into the required floor).
const unpackDiskCheckInterval = 8 * 1024 * 1024

func unpackFile(ctx context.Context, logger logging.Logger, fromFile, toDir string) error {
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

	// Seeded at the interval so the first regular file is checked before we write it. The unpacked
	// size is unknown up front (gzip compression hides it), so this incremental floor check — not
	// an up-front reservation — is what keeps unpack from filling the disk.
	bytesSinceDiskCheck := uint64(unpackDiskCheckInterval)
	// In log-only mode we'd re-warn every interval; free space only drops during unpack and we
	// proceed regardless, so latch after the first warning and stop checking. Blocking mode returns
	// on the first low result below, so the latch is moot there.
	loggedLowSpace := false

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
			if err := os.MkdirAll(path, info.Mode()); err != nil {
				return fmt.Errorf("failed to create directory %q %w", path, err)
			}

		case tar.TypeReg:
			// Re-check headroom before writing this file. Require room for the file itself plus the
			// reserved floor, so even a single large member can't push the volume below MinFreeBytes.
			// Runs of small files are batched (one statfs per ~interval of data) to avoid a syscall
			// per tiny file; a file larger than the interval trips the check on its own. The caller's
			// defer cleans up a partial unpack, so aborting leaves no debris.
			bytesSinceDiskCheck += uint64(header.Size)
			if !loggedLowSpace && bytesSinceDiskCheck >= unpackDiskCheckInterval {
				bytesSinceDiskCheck = 0
				required := diskusage.MinFreeBytes + uint64(header.Size)
				low, err := checkDiskSpace(logger, toDir, "unpacking package", required)
				if err != nil {
					return err
				}
				// log-only mode: warned once; don't re-check for the rest of this unpack.
				loggedLowSpace = low
			}

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
			if _, err := io.Copy(outFile, tarReader); err != nil && !errors.Is(err, io.EOF) { //nolint:gosec
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
	topLevelFiles, err := os.ReadDir(packagesDataDir)
	if err != nil {
		if os.IsNotExist(err) {
			// Directory doesn't exist. Nothing to clean up.
			return nil
		}

		return err
	}

	var allErrors error

	// A packageTypeDir is a directory that contains all of the packages for the specified type. ex: data/ml_model
	for _, packageTypeDir := range topLevelFiles {
		packageTypeDirName, err := rutils.SafeJoinDir(packagesDataDir, packageTypeDir.Name())
		if err != nil {
			allErrors = errors.Join(allErrors, err)
			continue
		}

		// Delete any non-directory files in the packages/data dir except for those with suffixes:
		//
		// `.status.json` - these files contain download status infomration.
		// `.first_run_succeeded` - these mark successful setup phase runs.
		if packageTypeDir.Type()&os.ModeDir != os.ModeDir && !strings.HasSuffix(packageTypeDirName, statusFileExt) {
			allErrors = errors.Join(allErrors, os.Remove(packageTypeDirName))
			continue
		}
		// read all of the packages in the directory and delete those that aren't in expectedPackageEntries
		packageDirs, err := os.ReadDir(packageTypeDirName)
		if err != nil {
			allErrors = errors.Join(allErrors, err)
			continue
		}
		for _, entry := range packageDirs {
			if entry.Name() == partialsDirName {
				continue
			}
			entryPath, err := rutils.SafeJoinDir(packageTypeDirName, entry.Name())
			if err != nil {
				allErrors = errors.Join(allErrors, err)
				continue
			}
			if shouldDeletePackageEntry(expectedPackageEntries, entryPath) {
				logger.Debugf("Removing old package file(s) %s", entryPath)
				allErrors = errors.Join(allErrors, os.RemoveAll(entryPath))
			}
		}

		partialsFolder := filepath.Join(packageTypeDirName, partialsDirName)
		now := time.Now()
		if _, err := os.Stat(partialsFolder); err == nil {
			entries, err := os.ReadDir(partialsFolder)
			if err != nil {
				allErrors = errors.Join(err)
				continue
			}
			for _, entry := range entries {
				info, err := entry.Info()
				if err != nil {
					allErrors = errors.Join(err)
					continue
				}
				age := now.Sub(info.ModTime())
				if age >= maxPartialAge {
					logger.Debugf("deleting partial %q with age %s >= %s", entry.Name(), age, maxPartialAge)
					allErrors = errors.Join(allErrors, os.RemoveAll(filepath.Join(partialsFolder, entry.Name())))
				}
			}
		}

		// re-read the directory, if there is nothing left in it, delete the directory
		packageDirs, err = os.ReadDir(packageTypeDirName)
		if err != nil {
			allErrors = errors.Join(allErrors, err)
			continue
		}
		if len(packageDirs) == 0 {
			allErrors = errors.Join(allErrors, os.RemoveAll(packageTypeDirName))
		}
	}
	return allErrors
}

// shouldDeletePackageEntry checks if a file or directory in the modules data directory should be deleted or not.
func shouldDeletePackageEntry(expectedPackageEntries map[string]bool, entryPath string) bool {
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
	// note: there is no syncStatus for resumable downloads; the stored state for a resumable download is the `.part` file.
	syncStatusDownloading syncStatus = "downloading"
	syncStatusDone        syncStatus = "done"
	syncStatusFailed      syncStatus = "failed"

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
	case syncFile.PackageID == pkg.Package && syncFile.Version == pkg.Version && syncFile.Status == syncStatusFailed:
		// packageIsSynced returns true here because we don't want to infinitely retry. This will fail
		// later in reconfigure + get cleaned up when you upgrade the package. See PR 5260 for more info.
		logger.Debugf("Package failed to unzip at %s.", pkg.LocalDataDirectory(packagesDir))
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
	if runtime.GOOS == "windows" {
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

// starts a goroutine that watches `dest` file size, logs progress until `dest` no longer exists or `done` is closed.
// If onProgress is non-nil it is invoked with the current file size on every tick.
func fileSizeProgress(ctx context.Context, logger logging.Logger, dest string, length int64, onProgress func(curSize int64)) {
	if length <= 0 {
		logger.Info("download has no Content-Length, not logging progress")
	}

	writer := newLogProgressWriter(logger, dest, length)

	ticker := time.NewTicker(writer.logFrequency)
	for {
		select {
		case <-ticker.C:
			stat, err := os.Stat(dest)
			if err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					// we don't warn if the file is missing because that means completion
					logger.Warnw("progress bar stat error", "err", err)
				}
				return
			}
			writer.Update(stat.Size())
			if onProgress != nil {
				onProgress(stat.Size())
			}
		case <-ctx.Done():
			return
		}
	}
}
