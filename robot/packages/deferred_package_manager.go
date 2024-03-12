package packages

import (
	"context"
	"errors"
	"os"
	"sync"

	pb "go.viam.com/api/app/packages/v1"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

type deferredPackageManager struct {
	resource.Named
	resource.TriviallyReconfigurable
	connectionChan   chan DeferredConnectionResponse
	noopManager      noopManager
	cloudManager     ManagerSyncer
	cloudManagerArgs cloudManagerConstructorArgs
	logger           logging.Logger
	cloudManagerLock sync.Mutex
}
type cloudManagerConstructorArgs struct {
	cloudConfig *config.Cloud
	packagesDir string
	logger      logging.Logger
}

// DeferredConnectionResponse is an entry on the connectionChan
// used to pass a connection from local_robot to here.
type DeferredConnectionResponse struct {
	Client pb.PackageServiceClient
	Err    error
}

var (
	_ Manager       = (*deferredPackageManager)(nil)
	_ ManagerSyncer = (*deferredPackageManager)(nil)
)

// DeferredServiceName is used to refer to/depend on this service internally.
var DeferredServiceName = resource.NewName(resource.APINamespaceRDKInternal.WithServiceType(SubtypeName), "deferred-manager")

// NewDeferredPackageManager returns a package manager that wraps a CloudPackageManager and a channel that is establishing
// a connection to app. It starts up instantly and behaves as a noop package manager until a connection to app has been established.
func NewDeferredPackageManager(
	connectionChan chan DeferredConnectionResponse,
	cloudConfig *config.Cloud,
	packagesDir string,
	logger logging.Logger,
) ManagerSyncer {
	return &deferredPackageManager{
		Named:            DeferredServiceName.AsNamed(),
		logger:           logger,
		cloudManagerArgs: cloudManagerConstructorArgs{cloudConfig, packagesDir, logger},
		connectionChan:   connectionChan,
		noopManager:      noopManager{Named: InternalServiceName.AsNamed()},
	}
}

// getCloudManager is the only function allowed to set or read m.cloudManager
// every other function should use getCloudManager().
func (m *deferredPackageManager) getCloudManager(wait bool) ManagerSyncer {
	m.cloudManagerLock.Lock()
	defer m.cloudManagerLock.Unlock()
	// early exit if the cloud manager already exists
	if m.cloudManager != nil {
		return m.cloudManager
	}
	var response *DeferredConnectionResponse
	if wait {
		res := <-m.connectionChan
		response = &res
	} else {
		select {
		case res := <-m.connectionChan:
			response = &res
		default:
		}
	}
	if response == nil {
		return nil
	}
	// because we have pulled this failed result from the connectionChan,
	// it will retry
	if response.Err != nil {
		m.logger.Warnf("failed to establish a connection to app.viam %w", response.Err)
		return nil
	}
	var err error
	m.cloudManager, err = NewCloudManager(
		m.cloudManagerArgs.cloudConfig,
		response.Client,
		m.cloudManagerArgs.packagesDir,
		m.cloudManagerArgs.logger,
	)
	if err != nil {
		m.logger.Warnf("failed to create cloud package manager %w", response.Err)
	}
	return m.cloudManager
}

// isMissingPackages is used pre-sync to determine if we should force-wait for the connection
// to be established.
func (m *deferredPackageManager) isMissingPackages(packages []config.PackageConfig) bool {
	for _, pkg := range packages {
		dir := pkg.LocalDataDirectory(m.cloudManagerArgs.packagesDir)
		if _, err := os.Stat(dir); err != nil {
			return true
		}
	}
	return false
}

// PackagePath returns the package if it exists and already download. If it does not exist it returns a ErrPackageMissing error.
func (m *deferredPackageManager) PackagePath(name PackageName) (string, error) {
	if mgr := m.getCloudManager(false); mgr != nil {
		return mgr.PackagePath(name)
	}
	return m.noopManager.PackagePath(name)
}

// Close manager.
func (m *deferredPackageManager) Close(ctx context.Context) error {
	if mgr := m.getCloudManager(false); mgr != nil {
		return mgr.Close(ctx)
	}
	return m.noopManager.Close(ctx)
}

// Sync syncs all given packages and removes any not in the list from the local file system.
// If there are packages missing on the local fs, this will wait while attempting to establish a connection to app.viam.
func (m *deferredPackageManager) Sync(ctx context.Context, packages []config.PackageConfig) error {
	shouldWait := m.isMissingPackages(packages)
	if mgr := m.getCloudManager(shouldWait); mgr != nil {
		return mgr.Sync(ctx, packages)
	}
	if shouldWait {
		return errors.New("failed to sync packages due to a bad connection with app.viam")
	}
	return m.noopManager.Sync(ctx, packages)
}

// Cleanup removes all unknown packages from the working directory.
func (m *deferredPackageManager) Cleanup(ctx context.Context) error {
	if mgr := m.getCloudManager(false); mgr != nil {
		return mgr.Cleanup(ctx)
	}
	return m.noopManager.Cleanup(ctx)
}
