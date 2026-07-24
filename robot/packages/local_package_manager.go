package packages

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	rUtils "go.viam.com/rdk/utils"
	"go.viam.com/rdk/utils/diskusage"
)

var (
	_ Manager       = (*localManager)(nil)
	_ ManagerSyncer = (*localManager)(nil)
)

// localManager manages IO for local modules that require setup.
type localManager struct {
	resource.Named
	resource.TriviallyReconfigurable

	// packagesDir is the parent dir for unpacked package tars.
	packagesDir     string
	packagesDataDir string

	// managedModules tracks the modules this manager knows about.
	managedModules  managedModuleMap
	packageStatuses map[PackageName]*PackageStatus
	mu              sync.RWMutex

	logger logging.Logger
}

type managedModule struct {
	module config.Module
}

type managedModuleMap map[string]*managedModule

// NewLocalManager returns a manager that unpacks local tarball modules into the package directory.
// On path requests it returns the name of the package. packagesParentDir is the parent directory
// packages are stored under (the local manager appends its own suffix).
func NewLocalManager(packagesParentDir string, logger logging.Logger) (ManagerSyncer, error) {
	packagesDir := LocalPackagesDir(packagesParentDir)
	packagesDataDir := filepath.Join(packagesDir, "data")
	// Don't eagerly create the package directories: a robot with no local tarball modules never
	// uses them, and creating them here would litter the package dir (~/.viam/packages-local) for
	// every robot/test that doesn't sync local packages. installPackage creates them on demand.
	return &localManager{
		Named:           InternalServiceName.AsNamed(),
		managedModules:  make(managedModuleMap),
		packageStatuses: make(map[PackageName]*PackageStatus),
		packagesDir:     packagesDir,
		packagesDataDir: packagesDataDir,
		logger:          logger,
	}, nil
}

// LocalPackagesDir transforms a packagesDir string to the suffixed version for localManager.
// local + cloud manager need separate parent dirs so they don't delete each other in Cleanup.
func LocalPackagesDir(packagesDir string) string {
	return filepath.Clean(packagesDir) + config.LocalPackagesSuffix
}

// PackagePath returns the package if it exists and already downloaded. If it does not exist it returns a ErrPackageMissing error.
func (m *localManager) PackagePath(name PackageName) (string, error) {
	return string(name), nil
}

// Close manager.
func (m *localManager) Close(ctx context.Context) error {
	return nil
}

// fileCopyHelper is the downloadCallback for local tarball modules.
func (m *localManager) fileCopyHelper(ctx context.Context, path, dstPath string) (string, string, error) {
	path, err := rUtils.ExpandHomeDir(path)
	if err != nil {
		return "", "", err
	}

	// Cheap pre-filter sized off the source archive (copied to dstPath before unpacking): refuse
	// if the destination can't hold the archive plus the reserved floor. Expanded contents are
	// guarded incrementally in unpackFile.
	required := diskusage.MinFreeBytes
	if info, statErr := os.Stat(path); statErr == nil && info.Mode().IsRegular() {
		required = uint64(info.Size()) + diskusage.MinFreeBytes
	}
	if _, err := checkDiskSpace(m.logger, dstPath, fmt.Sprintf("local package %q", filepath.Base(path)), required); err != nil {
		return "", "", err
	}

	src, err := os.Open(path) //nolint:gosec
	if err != nil {
		return "", "", err
	}
	defer src.Close()              //nolint:errcheck
	dst, err := os.Create(dstPath) //nolint:gosec
	if err != nil {
		return "", "", err
	}

	hash := crc32Hash()
	out := io.MultiWriter(dst, hash)

	defer dst.Close() //nolint:errcheck
	nBytes, err := io.Copy(out, src)
	if err != nil {
		return "", "", err
	}
	m.logger.Debugf("copied %d bytes to %s", nBytes, dstPath)
	checksum := hash.Sum(nil)
	// note: we can hardcode expected contentType because this is probably a synthetic package which already passed tarballExtensionsRegexp
	return string(checksum), allowedContentType, nil
}

// getAddedAndChanged is a helper for managing maps of things. It returns (map of existing, slice of added).
func getAddedAndChanged[Key comparable, ManagedVal, Val any](previous map[Key]ManagedVal, incoming []Val, keyFn func(Val) Key,
	compareFn func(ManagedVal, Val) bool,
) (map[Key]ManagedVal, []Val) {
	existing := make(map[Key]ManagedVal, len(previous))
	changed := make([]Val, 0)
	for _, val := range incoming {
		key := keyFn(val)
		if oldVal, ok := previous[key]; ok {
			if compareFn(oldVal, val) {
				existing[key] = oldVal
				continue
			}
		}
		changed = append(changed, val)
	}
	return existing, changed
}

// getAddedAndChanged specializes the generic function for managedModuleMap.
func (m managedModuleMap) getAddedAndChanged(
	incoming []config.Module,
	packagesDir string,
	logger logging.Logger,
) (managedModuleMap, []config.Module) {
	return getAddedAndChanged(m, incoming,
		func(mod config.Module) string { return mod.Name },
		func(old *managedModule, incoming config.Module) bool {
			pkg, err := old.module.SyntheticPackage()
			if err != nil {
				return false
			}
			return packageIsSynced(pkg, packagesDir, logger)
		},
	)
}

// Sync for the localManager manages copying of local tarballs.
func (m *localManager) Sync(ctx context.Context, packages []config.PackageConfig, modules []config.Module) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// overwrite incoming modules with filtered slice; we only manage local tarball modules
	modules = rUtils.FilterSlice(modules, config.Module.NeedsSyntheticPackage)
	existing, changed := m.managedModules.getAddedAndChanged(modules, m.packagesDir, m.logger)

	if len(changed) > 0 {
		m.logger.Info("Local package changes have been detected, starting sync...")
	}

	start := time.Now()
	var outErr error
	for idx, mod := range changed {
		pkgStart := time.Now()
		if err := ctx.Err(); err != nil {
			m.logger.Errorf("Context canceled. Canceling local package manager sync. Time spent: %v", time.Since(start))
			return multierr.Append(outErr, err)
		}
		m.logger.Debugf("Starting local package sync [%d/%d] %s", idx+1, len(changed), mod.Name)
		pkg, err := mod.SyntheticPackage()
		if err != nil {
			m.logger.Warnf("Local tarball package error. Skipping module. Module: %v Err: %v",
				mod.Name, err)
			m.setPackageStatusLocked(pkg, PackageStateFailed, fmt.Sprintf("local tarball package error: %v", err.Error()))
			outErr = multierr.Append(outErr, err)
			continue
		}

		m.setPackageStatusLocked(pkg, PackageStateDownloading, "")
		// Local tarballs are already on disk, so there is no download to track: report
		// the tarball size as both the total and already-"downloaded" byte counts.
		if exePath, err := rUtils.ExpandHomeDir(mod.ExePath); err == nil {
			if stat, err := os.Stat(exePath); err == nil {
				if s, ok := m.packageStatuses[PackageName(pkg.Name)]; ok {
					s.BytesDownloaded = uint64(stat.Size())
					s.TotalBytes = uint64(stat.Size())
				}
			}
		}
		err = installPackage(ctx, m.logger, m.packagesDir, mod.ExePath, pkg, false,
			func(ctx context.Context, path, dstPath string) (string, string, error) {
				checksum, contentType, err := m.fileCopyHelper(ctx, path, dstPath)
				if err != nil {
					return checksum, contentType, err
				}

				// The tarball is fully copied; installPackage will now verify and
				// extract it.
				m.setPackageStatusLocked(pkg, PackageStateLoading, "")
				return checksum, contentType, nil
			})
		if err != nil {
			m.logger.Warnf("Failed installing tarball package. Skipping module. Module: %s Path: %s Err: %s",
				mod.Name, mod.ExePath, err)
			m.setPackageStatusLocked(pkg, PackageStateFailed, fmt.Sprintf("failed installing tarball package: %v", err.Error()))
			outErr = multierr.Append(outErr, fmt.Errorf("failed copying package %s from %s: %w",
				mod.Name, mod.ExePath, err))
			continue
		}

		// add to managed packages
		m.setPackageStatusLocked(pkg, PackageStateReady, "")
		existing[mod.Name] = &managedModule{module: mod}

		m.logger.Debugf("Local package sync complete [%d/%d] %s after %v", idx+1, len(changed), mod.Name, time.Since(pkgStart))
	}

	if len(changed) > 0 {
		m.logger.Infof("Local package sync complete after %v", time.Since(start))
	}

	// Prune packageStatuses to match the requested config so stale entries from removed
	// modules don't accumulate across reconfigures. Prune against the requested modules
	// rather than the managed set: failed packages are absent from the managed set but
	// must keep their Failed status visible.
	expectedPkgNames := make(map[PackageName]bool, len(modules))
	for _, mod := range modules {
		if pkg, err := mod.SyntheticPackage(); err == nil {
			expectedPkgNames[PackageName(pkg.Name)] = true
		}
	}
	for name := range m.packageStatuses {
		if !expectedPkgNames[name] {
			delete(m.packageStatuses, name)
		}
	}

	// swap for new managed packages.
	m.managedModules = existing

	return outErr
}

// Cleanup removes all unknown packages from the working directory.
func (m *localManager) Cleanup(ctx context.Context) error {
	m.logger.Debug("Starting package cleanup...")

	// Only allow one rdk process to operate on the manager at once. This is generally safe to keep locked for an extended period of time
	// since the config reconfiguration process is handled by a single thread.
	m.mu.Lock()
	defer m.mu.Unlock()

	expectedPackageDirectories := map[string]bool{}
	for _, mod := range m.managedModules {
		pkg, err := mod.module.SyntheticPackage()
		if err != nil {
			m.logger.CWarnf(ctx, "ignoring error in Cleanup for mod %s, %s", mod.module.Name, err)
			continue
		}
		expectedPackageDirectories[pkg.LocalDataDirectory(m.packagesDir)] = true
	}

	return commonCleanup(m.logger, expectedPackageDirectories, m.packagesDataDir)
}

// PackageStatuses returns a snapshot of the current status for all managed local packages.
func (m *localManager) PackageStatuses() []PackageStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	statuses := make([]PackageStatus, 0, len(m.packageStatuses))
	for _, s := range m.packageStatuses {
		statuses = append(statuses, *s)
	}
	return statuses
}

// SetPackageState updates the in-memory state for the named package.
func (m *localManager) SetPackageState(name PackageName, state PackageState, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.packageStatuses[name]; ok {
		s.State = state
		s.Error = errMsg
		s.LastUpdated = time.Now()
	}
}

// setPackageStatusLocked sets the full status entry for a package. Must be called with
// m.mu held (write). Tarball byte counts are preserved when updating an existing entry
// for the same package version.
func (m *localManager) setPackageStatusLocked(p config.PackageConfig, state PackageState, errMsg string) {
	name := PackageName(p.Name)
	status := &PackageStatus{
		Name:        p.Name,
		Type:        p.Type,
		State:       state,
		Error:       errMsg,
		LastUpdated: time.Now(),
		Version:     p.Version,
	}
	if prev, ok := m.packageStatuses[name]; ok && prev.Version == p.Version {
		status.BytesDownloaded = prev.BytesDownloaded
		status.TotalBytes = prev.TotalBytes
	}
	m.packageStatuses[name] = status
}

// newerOrMissing takes two file paths. It returns true if src path is newer than dest, or if dest is missing.
func newerOrMissing(src, dest string) (bool, error) {
	srcStat, err := os.Stat(src)
	if err != nil {
		return false, err
	}
	destStat, err := os.Stat(dest)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return srcStat.ModTime().After(destStat.ModTime()), nil
}

// SyncOne recopies from the tarball if the tarball is newer than the destination.
// It also adds or overwrites the module in managedModules.
func (m *localManager) SyncOne(ctx context.Context, mod config.Module) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !mod.NeedsSyntheticPackage() {
		return nil
	}

	pkg, err := mod.SyntheticPackage()
	if err != nil {
		return err
	}

	pkgDir := pkg.LocalDataDirectory(m.packagesDir)
	exePath, err := rUtils.ExpandHomeDir(mod.ExePath)
	if err != nil {
		return err
	}

	dirty, err := newerOrMissing(exePath, pkgDir)
	if err != nil {
		return err
	}

	if dirty {
		m.logger.CDebugf(ctx, "%s is newer, recopying", mod.ExePath)
		utils.UncheckedError(cleanup(m.packagesDir, pkg))
		err = installPackage(ctx, m.logger, m.packagesDir, mod.ExePath, pkg, false, m.fileCopyHelper)
		if err != nil {
			return fmt.Errorf("failed installing package %s:%s installPath: %q err: %w", pkg.Package, pkg.Version, mod.ExePath, err)
		}
		m.managedModules[mod.Name] = &managedModule{module: mod}
	}

	return nil
}
