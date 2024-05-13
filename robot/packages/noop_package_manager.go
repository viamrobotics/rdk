package packages

import (
	"context"
	"io"
	"os"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
)

// localManager manages IO for local modules that require setup.
type localManager struct {
	resource.Named
	resource.TriviallyReconfigurable

	// this is copied because we treat it as immutable (same as cloudManager).
	packagesDir string

	// managedPackages tracks the packages that this manager knows about.
	managedPackages managedPackageMap
	mu              sync.RWMutex

	logger logging.Logger
}

var (
	_ Manager       = (*localManager)(nil)
	_ ManagerSyncer = (*localManager)(nil)
)

// NewNoopManager returns a noop package manager that does nothing. On path requests it returns the name of the package.
func NewNoopManager() ManagerSyncer {
	return &localManager{
		Named: InternalServiceName.AsNamed(),
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

// Sync for the localManager manages the logic of converting modules to fake packages.
func (m *localManager) Sync(ctx context.Context, packages []config.PackageConfig, modules []config.Module) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// overwrite packages with synthetic local packages that we actually care about in localManager
	packages, err := utils.MapOver(utils.FilterSlice(modules, config.Module.NeedsSyntheticPackage),
		func(mod config.Module) (config.PackageConfig, error) { return mod.SyntheticPackage(true) },
	)
	if err != nil {
		errors.Wrap(err, "making synthetic packages")
	}
	// this map will replace m.managedPackages at the end of the function
	newManagedPackages := m.managedPackages.unchangedPackages(packages)

	// Process the packages that are new or changed
	changedPackages := m.managedPackages.validateAndGetChangedPackages(m.logger, packages)
	if len(changedPackages) > 0 {
		m.logger.Info("Local package changes have been detected, starting sync")
	}

	start := time.Now()
	var outErr error
	for idx, p := range changedPackages {
		pkgStart := time.Now()
		if err := ctx.Err(); err != nil {
			return multierr.Append(outErr, err)
		}
		m.logger.Debugf("Starting package sync [%d/%d] %s:%s", idx+1, len(changedPackages), p.Package, p.Version)
		packageURL := p.SourceModule.ExePath
		if err != nil {
			outErr = multierr.Append(outErr, err)
			continue
		}
		nonEmptyPaths := getNonEmptyPaths(ctx, m.logger, m.packagesDir, p, modules)

		// download or copy package
		err = downloadPackage(ctx, m.logger, m.packagesDir, packageURL, p, nonEmptyPaths, m.fileCopyHelper)
		if err != nil {
			m.logger.Errorf("Failed downloading package %s:%s from %s, %s", p.Package, p.Version, sanitizeURLForLogs(packageURL), err)
			outErr = multierr.Append(outErr, errors.Wrapf(err, "failed downloading package %s:%s from %s",
				p.Package, p.Version, sanitizeURLForLogs(packageURL)))
			continue
		}

		// add to managed packages
		newManagedPackages[PackageName(p.Name)] = &managedPackage{thePackage: p, modtime: time.Now()}

		m.logger.Debugf("Package sync complete [%d/%d] %s:%s after %v", idx+1, len(changedPackages), p.Package, p.Version, time.Since(pkgStart))
	}

	if len(changedPackages) > 0 {
		m.logger.Infof("Package sync complete after %v", time.Since(start))
	}

	// swap for new managed packags.
	m.managedPackages = newManagedPackages

	return outErr
}

// Cleanup removes all unknown packages from the working directory.
func (m *localManager) Cleanup(ctx context.Context) error {
	panic("I need to be implemented")
}
