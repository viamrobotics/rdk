package packages

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
)

var (
	_ Manager       = (*localManager)(nil)
	_ ManagerSyncer = (*localManager)(nil)
)

// localManager manages IO for local modules that require setup.
type localManager struct {
	resource.Named
	resource.TriviallyReconfigurable

	// this is copied because we treat it as immutable (same as cloudManager).
	packagesDir string

	// managedModules tracks the modules this manager knows about.
	managedModules managedModuleMap
	mu             sync.RWMutex

	logger logging.Logger
}

type managedModule struct {
	module config.Module

	// modTime and dirty are used together to trigger a re-copy.
	modTime time.Time
	dirty   bool
}

type managedModuleMap map[string]*managedModule

// NewLocalManager returns a noop package manager that does nothing. On path requests it returns the name of the package.
func NewLocalManager(conf *config.Config, logger logging.Logger) ManagerSyncer {
	return &localManager{
		Named: InternalServiceName.AsNamed(),
		// note: packagesDir needs suffix so cloudManager + this one don't overwrite each other.
		packagesDir: filepath.Dir(conf.PackagePath) + "-local",
		logger:      logger,
	}
}

// PackagePath returns the package if it exists and already download. If it does not exist it returns a ErrPackageMissing error.
func (m *localManager) PackagePath(name PackageName) (string, error) {
	return string(name), nil
}

// Close manager.
func (m *localManager) Close(ctx context.Context) error {
	return nil
}

// fileCopyHelper is the downloadCallback for local tarball modules.
func (m *localManager) fileCopyHelper(ctx context.Context, path, dstPath string) (string, error) {
	src, err := os.Open(path) //nolint:gosec
	if err != nil {
		return "", err
	}
	defer src.Close()              //nolint:errcheck
	dst, err := os.Create(dstPath) //nolint:gosec
	if err != nil {
		return "", err
	}
	defer dst.Close() //nolint:errcheck
	nBytes, err := io.Copy(dst, src)
	if err != nil {
		return "", err
	}
	m.logger.Debugf("copied %d bytes to %s", nBytes, dstPath)
	// note: we can hardcode expected contentType because this is probably a synthetic package which already passed tarballExtensionsRegexp
	return allowedContentType, nil
}

// getAddedAndChanged is a helper for managing maps of things. It returns (map of existing, slice of added).
func getAddedAndChanged[Key comparable, ManagedVal any, Val any](previous map[Key]ManagedVal, incoming []Val, keyFn func(Val) Key,
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
func (m managedModuleMap) getAddedAndChanged(incoming []config.Module) (managedModuleMap, []config.Module) {
	return getAddedAndChanged(m, incoming,
		func(mod config.Module) string { return mod.Name },
		func(old *managedModule, incoming config.Module) bool { return old.module.ExePath == incoming.ExePath },
	)
}

// Sync for the localManager manages the logic of converting modules to fake packages.
func (m *localManager) Sync(ctx context.Context, packages []config.PackageConfig, modules []config.Module) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// overwrite incoming modules with filtered slice; we only manage local tarball modules
	modules = utils.FilterSlice(modules, config.Module.IsLocalTarball)
	existing, changed := m.managedModules.getAddedAndChanged(modules)
	if len(changed) > 0 {
		m.logger.Info("Local package changes have been detected, starting sync")
	}

	start := time.Now()
	var outErr error
	for idx, mod := range changed {
		pkgStart := time.Now()
		if err := ctx.Err(); err != nil {
			return multierr.Append(outErr, err)
		}
		m.logger.Debugf("Starting local package sync [%d/%d] %s", idx+1, len(changed), mod.Name)
		pkg, err := mod.SyntheticPackage()
		if err != nil {
			outErr = multierr.Append(outErr, err)
			continue
		}
		err = downloadPackage(ctx, m.logger, m.packagesDir, mod.ExePath, pkg, []string{}, m.fileCopyHelper)
		if err != nil {
			m.logger.Errorf("Failed downloading package %s from %s, %s", mod.Name, mod.ExePath, err)
			outErr = multierr.Append(outErr, errors.Wrapf(err, "failed downloading package %s from %s",
				mod.Name, mod.ExePath))
			continue
		}

		// add to managed packages
		existing[mod.Name] = &managedModule{module: mod}

		m.logger.Debugf("Local package sync complete [%d/%d] %s after %v", idx+1, len(changed), mod.Name, time.Since(pkgStart))
	}

	if len(changed) > 0 {
		m.logger.Infof("Local package sync complete after %v", time.Since(start))
	}

	// swap for new managed packages.
	m.managedModules = existing

	return outErr
}

// Cleanup removes all unknown packages from the working directory.
func (m *localManager) Cleanup(ctx context.Context) error {
	panic("I need to be implemented")
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

// RecopyIfChanged recopies from the tarball if the tarball is newer than the destination.
// It also adds or overwrites the module in managedModules. Noop except for localManager.
func (m *localManager) RecopyIfChanged(ctx context.Context, mod config.Module) error {
	if !mod.IsLocalTarball() {
		return nil
	}
	pkg, err := mod.SyntheticPackage()
	if err != nil {
		return err
	}
	pkgDir := pkg.LocalDataDirectory(m.packagesDir)

	m.mu.Lock()
	defer m.mu.Unlock()

	dirty, err := newerOrMissing(mod.ExePath, pkgDir)
	if err != nil {
		return err
	}
	if dirty {
		cleanup(m.packagesDir, pkg)
		err = downloadPackage(ctx, m.logger, m.packagesDir, mod.ExePath, pkg, []string{}, m.fileCopyHelper)
		if err != nil {
			m.logger.Errorf("Failed copying package %s:%s from %s, %s", pkg.Package, pkg.Version, mod.ExePath, err)
			return errors.Wrapf(err, "failed downloading package %s:%s from %s", pkg.Package, pkg.Version, mod.ExePath)
		}
		m.managedModules[mod.Name] = &managedModule{module: mod}
	}
	return nil
}
