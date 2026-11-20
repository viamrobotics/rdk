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
	// Error is the most recent failure. It is kept through retries of the same version and
	// cleared once the package is ready.
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
	// ConsecutiveFailures is the number of failed first runs for this version. Reset when
	// the package is ready or its version changes.
	ConsecutiveFailures uint
}

// newPackageStatus builds the status for p. Download progress and first-run failure history
// carry over from prev when the version is unchanged.
func newPackageStatus(p config.PackageConfig, state PackageState, errMsg string, prev *PackageStatus) *PackageStatus {
	status := &PackageStatus{
		Name:    p.Name,
		Type:    p.Type,
		Version: p.Version,
	}
	if prev != nil && prev.Version == p.Version {
		status.BytesDownloaded = prev.BytesDownloaded
		status.TotalBytes = prev.TotalBytes
		status.Error = prev.Error
		status.ConsecutiveFailures = prev.ConsecutiveFailures
	}
	status.setState(state, errMsg)
	return status
}

// setState mirrors module status: the last error survives intermediate states so pollers
// still see it while a failure is retried.
func (s *PackageStatus) setState(state PackageState, errMsg string) {
	s.State = state
	s.LastUpdated = time.Now()
	if state == PackageStateFailed {
		s.Error = errMsg
	} else if state == PackageStateReady {
		s.Error = ""
		s.ConsecutiveFailures = 0
	}
}

// setFirstRunState is setState for the first-run stage, where failures are counted.
func (s *PackageStatus) setFirstRunState(state PackageState, errMsg string) {
	if state == PackageStateFailed {
		s.ConsecutiveFailures++
	}
	s.setState(state, errMsg)
}

// PackageStatusToProto converts a package status to its protobuf equivalent. The returned
// message is populated with unspecified enum values when err is non-nil.
func PackageStatusToProto(status PackageStatus) (*robotpb.PackageStatus, error) {
	pkgType, typeErr := config.PackageTypeToProto(status.Type)
	state, stateErr := packageStateToProto(status.State)
	lastUpdated := timestamppb.New(status.LastUpdated)

	return &robotpb.PackageStatus{
		Name:                status.Name,
		Type:                *pkgType,
		State:               state,
		Error:               status.Error,
		LastUpdated:         lastUpdated,
		Version:             status.Version,
		BytesDownloaded:     status.BytesDownloaded,
		TotalBytes:          status.TotalBytes,
		ConsecutiveFailures: uint32(status.ConsecutiveFailures),
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
		Name:                status.GetName(),
		Type:                pkgType,
		State:               state,
		Error:               status.GetError(),
		LastUpdated:         lastUpdated,
		Version:             status.GetVersion(),
		BytesDownloaded:     status.GetBytesDownloaded(),
		TotalBytes:          status.GetTotalBytes(),
		ConsecutiveFailures: uint(status.GetConsecutiveFailures()),
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
