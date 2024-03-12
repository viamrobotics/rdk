package packages

// DeferredPackageManager wraps a CloudPackageManager and a channel that is establishing a connection to app. It starts up
// instantly and behaves as a noop package manager until a connection to app has been established.
//
// Raison d'etre: AquireConnection will waste 5 seconds timing out on robots that have no internet connection but have downloaded
// all of their cloud_packages
//
// This puts an optimization on top of that behavior: Put the connection establishment in a goroutine that sends the connection over
// a channel to a DeferredPackageManager. If all of the packages in the config are already present on the config, try to use the
// cloud_package_manager, but dont block and just fallback to the noop_package_manager. If there are missing packages on the robot,
// block for the 5 seconds while we establish a connection.

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
	cloudManagerLock sync.Mutex
	logger           logging.Logger

	lastSyncedManager ManagerSyncer
	syncLock          sync.Mutex
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
	noopManager := noopManager{Named: InternalServiceName.AsNamed()}
	return &deferredPackageManager{
		Named:             DeferredServiceName.AsNamed(),
		logger:            logger,
		cloudManagerArgs:  cloudManagerConstructorArgs{cloudConfig, packagesDir, logger},
		connectionChan:    connectionChan,
		noopManager:       noopManager,
		lastSyncedManager: &noopManager,
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
		m.logger.Info("waiting for connection to app")
		res := <-m.connectionChan
		response = &res
		m.logger.Info("connection to app established")
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

// Sync syncs all given packages and removes any not in the list from the local file system.
// If there are packages missing on the local fs, this will wait while attempting to establish a connection to app.viam.
func (m *deferredPackageManager) Sync(ctx context.Context, packages []config.PackageConfig) error {
	shouldWait := m.isMissingPackages(packages)
	var mgrToSyncWith ManagerSyncer
	if mgr := m.getCloudManager(shouldWait); mgr != nil {
		mgrToSyncWith = mgr
	} else if shouldWait {
		return errors.New("failed to sync packages due to a bad connection with app.viam")
	} else {
		mgrToSyncWith = &m.noopManager
	}
	// replace the lastSyncedManager to ensure we call close and cleanup with the same manager
	m.syncLock.Lock()
	defer m.syncLock.Unlock()
	m.lastSyncedManager = mgrToSyncWith
	return mgrToSyncWith.Sync(ctx, packages)
}

// Cleanup removes all unknown packages from the working directory.
func (m *deferredPackageManager) Cleanup(ctx context.Context) error {
	m.syncLock.Lock()
	defer m.syncLock.Unlock()
	return m.lastSyncedManager.Cleanup(ctx)
}

// PackagePath returns the package if it exists and already download. If it does not exist it returns a ErrPackageMissing error.
func (m *deferredPackageManager) PackagePath(name PackageName) (string, error) {
	m.syncLock.Lock()
	defer m.syncLock.Unlock()
	return m.lastSyncedManager.PackagePath(name)
}

// Close manager.
func (m *deferredPackageManager) Close(ctx context.Context) error {
	m.syncLock.Lock()
	defer m.syncLock.Unlock()
	return m.lastSyncedManager.Close(ctx)
}
