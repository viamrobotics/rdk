package packages

// DeferredPackageManager wraps a CloudPackageManager and a goroutine that is establishing a connection to app. It starts up
// instantly and behaves as a noop package manager until a connection to app has been established.
//
// Raison d'être: AquireConnection will waste 5 seconds timing out on robots that have no internet connection but have downloaded
// all of their cloud_packages
//
// This puts an optimization on top of that behavior: On the first start of the package manager, if all of the expected packages are
// present on the system, put the app-connection establishment in a goroutine and use a noopManager for the first Sync.
// If there are missing packages, this will still block to prevent a half-started robot.

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	pb "go.viam.com/api/app/packages/v1"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

type deferredPackageManager struct {
	resource.Named
	resource.TriviallyReconfigurable
	ctx                 context.Context
	establishConnection func(ctx context.Context) (pb.PackageServiceClient, error)
	cloudManager        ManagerSyncer
	cloudManagerArgs    cloudManagerConstructorArgs
	cloudManagerLock    sync.Mutex

	lastSyncedManager     ManagerSyncer
	lastSyncedManagerLock sync.Mutex

	logger logging.Logger
}
type cloudManagerConstructorArgs struct {
	cloudConfig *config.Cloud
	packagesDir string
	logger      logging.Logger
}

var (
	_ Manager       = (*deferredPackageManager)(nil)
	_ ManagerSyncer = (*deferredPackageManager)(nil)
)

// DeferredServiceName is used to refer to/depend on this service internally.
var DeferredServiceName = resource.NewName(resource.APINamespaceRDKInternal.WithServiceType(SubtypeName), "deferred-manager")

// NewDeferredPackageManager returns a DeferredPackageManager. See deferred_package_manager.go for more details.
func NewDeferredPackageManager(
	ctx context.Context,
	establishConnection func(ctx context.Context) (pb.PackageServiceClient, error),
	cloudConfig *config.Cloud,
	packagesDir string,
	logger logging.Logger,
) ManagerSyncer {
	noopManager := noopManager{Named: InternalServiceName.AsNamed()}
	return &deferredPackageManager{
		Named:               DeferredServiceName.AsNamed(),
		ctx:                 ctx,
		establishConnection: establishConnection,
		cloudManagerArgs:    cloudManagerConstructorArgs{cloudConfig, packagesDir, logger},
		lastSyncedManager:   &noopManager,
		logger:              logger,
	}
}

// Sync syncs packages and removes any not in the list from the local file system.
// If there are packages missing on the local fs, this will wait while attempting to establish a connection to app.viam.
//
// Sync is the core state-setting operation of the package manager so if we sync with one manager,
// all subsequent operations should use the same manager until the next sync.
func (m *deferredPackageManager) Sync(ctx context.Context, packages []config.PackageConfig, modules []config.Module) error {
	m.lastSyncedManagerLock.Lock()
	defer m.lastSyncedManagerLock.Unlock()
	mgr, err := m.getManagerForSync(ctx, packages)
	if err != nil {
		return err
	}
	m.lastSyncedManager = mgr
	return mgr.Sync(ctx, packages, modules)
}

// Cleanup removes all unknown packages from the working directory.
func (m *deferredPackageManager) Cleanup(ctx context.Context) error {
	m.lastSyncedManagerLock.Lock()
	defer m.lastSyncedManagerLock.Unlock()
	return m.lastSyncedManager.Cleanup(ctx)
}

// PackagePath returns the package if it exists and already downloaded. If it does not exist it returns a ErrPackageMissing error.
func (m *deferredPackageManager) PackagePath(name PackageName) (string, error) {
	m.lastSyncedManagerLock.Lock()
	defer m.lastSyncedManagerLock.Unlock()
	return m.lastSyncedManager.PackagePath(name)
}

// Close manager.
func (m *deferredPackageManager) Close(ctx context.Context) error {
	m.lastSyncedManagerLock.Lock()
	defer m.lastSyncedManagerLock.Unlock()
	return m.lastSyncedManager.Close(ctx)
}

// getManagerForSync returns the cloudManager if there is one cached (or if there are missing packages)
// otherwise return noopManager and async get a cloudManager.
func (m *deferredPackageManager) getManagerForSync(ctx context.Context, packages []config.PackageConfig) (ManagerSyncer, error) {
	m.cloudManagerLock.Lock()
	// the lock is handed to the goroutine so we do not defer an unlock

	// return cached cloud manager if possible
	if m.cloudManager != nil {
		m.cloudManagerLock.Unlock()
		return m.cloudManager, nil
	}

	// if we are missing packages, run createCloudManager synchronously
	if !packagesAreSynced(packages, m.cloudManagerArgs.packagesDir, m.logger) {
		mgr, err := m.createCloudManager(ctx)
		if err == nil {
			// err == nil, not != nil
			m.cloudManager = mgr
			m.logger.Info("cloud package manager created synchronously")
		}
		m.cloudManagerLock.Unlock()
		return mgr, err
	}

	// otherwise, spawn a goroutine to establish the connection and use a noopManager in the meantime
	// hold the cloudManagerLock until this finishes
	goutils.PanicCapturingGo(func() {
		defer m.cloudManagerLock.Unlock()
		mgr, err := m.createCloudManager(ctx)
		if err != nil {
			m.logger.Warnf("failed to create cloud package manager %v", err)
		} else {
			m.cloudManager = mgr
			m.logger.Info("cloud package manager created asyncronously")
		}
	})
	// No unlock here. The goroutine will unlock
	return &noopManager{Named: InternalServiceName.AsNamed()}, nil
}

// createCloudManager uses the passed establishConnection function to instantiate a cloudManager.
func (m *deferredPackageManager) createCloudManager(ctx context.Context) (ManagerSyncer, error) {
	client, err := m.establishConnection(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to a establish connection to app.viam")
	}
	return NewCloudManager(
		m.cloudManagerArgs.cloudConfig,
		client,
		m.cloudManagerArgs.packagesDir,
		m.cloudManagerArgs.logger,
	)
}

// SyncOne is a no-op for this package manager variant.
func (m *deferredPackageManager) SyncOne(ctx context.Context, mod config.Module) error {
	return nil
}
