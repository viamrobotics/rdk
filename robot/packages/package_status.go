package packages

import (
	"time"

	"go.viam.com/rdk/config"
)

// PackageState represents the lifecycle state of a package.
type PackageState int

const (
	// PackageStateUnknown is the zero/unset value.
	PackageStateUnknown PackageState = iota
	// PackageStateDownloading means the tarball is actively being fetched from the cloud.
	PackageStateDownloading
	// PackageStateLoading means the tarball has been downloaded and is being extracted/verified.
	PackageStateLoading
	// PackageStateFirstRun means first_run.sh is executing (module packages only).
	PackageStateFirstRun
	// PackageStateDownloaded means the package is fully installed and available for use.
	PackageStateDownloaded
	// PackageStateFailed means the package failed at some lifecycle stage.
	PackageStateFailed
)

// PackageStatus holds the current status of a single package.
type PackageStatus struct {
	// Name is the package name as declared in the robot config (PackageConfig.Name).
	Name string
	// Type is the package type (module, ml_model, slam_map).
	Type config.PackageType
	// State is the current lifecycle state.
	State PackageState
	// Error contains details when State == PackageStateFailed.
	Error string
	// LastUpdated is when this status was last changed.
	LastUpdated time.Time
	// Version is the version string from PackageConfig.
	Version string
}
