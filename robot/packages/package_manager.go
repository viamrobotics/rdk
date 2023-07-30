// Package packages contains utilities and manager to sync Viam packages defined in the RDK config from the Viam app to the local robot.
package packages

import (
	"context"

	"github.com/docker/go-units"
	"github.com/pkg/errors"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
)

type (
	// PackageName is the logical name of the package on the local rdk. Can point to ID/Version of an actual package.
	PackageName string
	// PackageID is a gobally unique package id.
	PackageID string
	// PackageVersion is an immutable package version for a given package id.
	PackageVersion string
)

const maxPackageSize = int64(50 * units.GB)

// ErrPackageMissing is an error when a package cannot be found.
var ErrPackageMissing = errors.New("package missing")

// ErrInvalidPackageRef is an error when a invalid package reference syntax.
var ErrInvalidPackageRef = errors.New("invalid package reference")

// Manager provides a managed interface for looking up package paths. This is separated from ManagerSyncer to avoid passing
// the full sync interface to all components.
type Manager interface {
	resource.Resource

	// PackagePath returns the package if it exists and is already downloaded. If it does not exist it returns a ErrPackageMissing error.
	PackagePath(name PackageName) (string, error)
}

// ManagerSyncer provides a managed interface for both reading package paths and syncing packages from the RDK config.
type ManagerSyncer interface {
	Manager

	// Sync will download and create the symbolic logic links to all the given PackageConfig. Sync will not remove any unused
	// data packages. You must call Cleanup() to remove leftovers. Sync will block until all packages are loaded to the file system.
	// Sync should only be used by one goroutine at once.
	// If errors occur during sync the manager should continue trying to download all packages and then return any errors that occurred.
	// If the context is canceled the manager will stop syncing and return an interrupted error.
	Sync(ctx context.Context, packages []config.PackageConfig) error

	// Cleanup removes any unused packages known to the Manager that are no longer used. It removes the packages from the file system.
	// Returns any errors during the cleanup process.
	Cleanup(ctx context.Context) error
}
