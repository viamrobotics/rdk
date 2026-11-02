package packages

import (
	"errors"
	"fmt"
	"time"

	robotpb "go.viam.com/api/robot/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

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
	// PackageStateReady means the package is fully installed and available for use.
	PackageStateReady
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
	// BytesDownloaded is the number of bytes of the package tarball downloaded so far.
	// Only meaningful for packages that go through a cloud download; equals TotalBytes
	// once the download completes.
	BytesDownloaded uint64
	// TotalBytes is the total size of the package tarball in bytes. Zero if unknown.
	TotalBytes uint64
}

// PackageStatusToProto converts a package status to its protobuf equivalent. The returned
// message is populated with unspecified enum values when err is non-nil.
func PackageStatusToProto(status PackageStatus) (*robotpb.PackageStatus, error) {
	pkgType, typeErr := config.PackageTypeToProto(status.Type)
	state, stateErr := packageStateToProto(status.State)
	lastUpdated := timestamppb.New(status.LastUpdated)

	return &robotpb.PackageStatus{
		Name:            status.Name,
		Type:            *pkgType,
		State:           state,
		Error:           status.Error,
		LastUpdated:     lastUpdated,
		Version:         status.Version,
		BytesDownloaded: status.BytesDownloaded,
		TotalBytes:      status.TotalBytes,
	}, errors.Join(typeErr, stateErr, lastUpdated.CheckValid())
}

// PackageStatusFromProto converts a protobuf package status to its RDK equivalent. Unsupported
// enum values and invalid timestamps are left at their zero values and returned as errors.
func PackageStatusFromProto(status *robotpb.PackageStatus) (PackageStatus, error) {
	if status == nil {
		return PackageStatus{}, errors.New("package status is nil")
	}

	pkgType, typeErr := config.PackageTypeFromProto(status.GetType())
	state, stateErr := packageStateFromProto(status.GetState())
	var lastUpdated time.Time
	var timestampErr error
	if timestamp := status.GetLastUpdated(); timestamp != nil {
		timestampErr = timestamp.CheckValid()
		if timestampErr == nil {
			lastUpdated = timestamp.AsTime()
		}
	}

	return PackageStatus{
		Name:            status.GetName(),
		Type:            pkgType,
		State:           state,
		Error:           status.GetError(),
		LastUpdated:     lastUpdated,
		Version:         status.GetVersion(),
		BytesDownloaded: status.GetBytesDownloaded(),
		TotalBytes:      status.GetTotalBytes(),
	}, errors.Join(typeErr, stateErr, timestampErr)
}

func packageStateToProto(state PackageState) (robotpb.PackageStatus_State, error) {
	switch state {
	case PackageStateUnknown:
		return robotpb.PackageStatus_STATE_UNSPECIFIED, nil
	case PackageStateDownloading:
		return robotpb.PackageStatus_STATE_DOWNLOADING, nil
	case PackageStateLoading:
		return robotpb.PackageStatus_STATE_LOADING, nil
	case PackageStateFirstRun:
		return robotpb.PackageStatus_STATE_FIRST_RUN, nil
	case PackageStateReady:
		return robotpb.PackageStatus_STATE_READY, nil
	case PackageStateFailed:
		return robotpb.PackageStatus_STATE_FAILED, nil
	default:
		return robotpb.PackageStatus_STATE_UNSPECIFIED, fmt.Errorf("unknown package state %d", state)
	}
}

func packageStateFromProto(state robotpb.PackageStatus_State) (PackageState, error) {
	switch state {
	case robotpb.PackageStatus_STATE_UNSPECIFIED:
		return PackageStateUnknown, nil
	case robotpb.PackageStatus_STATE_DOWNLOADING:
		return PackageStateDownloading, nil
	case robotpb.PackageStatus_STATE_LOADING:
		return PackageStateLoading, nil
	case robotpb.PackageStatus_STATE_FIRST_RUN:
		return PackageStateFirstRun, nil
	case robotpb.PackageStatus_STATE_READY:
		return PackageStateReady, nil
	case robotpb.PackageStatus_STATE_FAILED:
		return PackageStateFailed, nil
	default:
		return PackageStateUnknown, fmt.Errorf("unknown package state %d", state)
	}
}
