package packages

import (
	"context"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
)

type noopManager struct {
	resource.Named
	resource.TriviallyReconfigurable
}

var (
	_ Manager       = (*noopManager)(nil)
	_ ManagerSyncer = (*noopManager)(nil)
)

// NewNoopManager returns a noop package manager that does nothing. On path requests it returns the name of the package.
func NewNoopManager() ManagerSyncer {
	return &noopManager{
		Named: InternalServiceName.AsNamed(),
	}
}

// PackagePath returns the package if it exists and already download. If it does not exist it returns a ErrPackageMissing error.
func (m *noopManager) PackagePath(name PackageName) (string, error) {
	return string(name), nil
}

// Close manager.
func (m *noopManager) Close(ctx context.Context) error {
	return nil
}

// SyncAll syncs all given packages and removes any not in the list from the local file system.
func (m *noopManager) Sync(ctx context.Context, packages []config.PackageConfig) error {
	return nil
}

// Cleanup removes all unknown packages from the working directory.
func (m *noopManager) Cleanup(ctx context.Context) error {
	return nil
}
