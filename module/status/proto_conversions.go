package status

import (
	pb "go.viam.com/api/robot/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ToProto converts a module Status to its proto equivalent.
func (s Status) ToProto() *pb.ModuleStatus {
	pbStatus := &pb.ModuleStatus{
		ModuleName:          s.Name,
		LastUpdated:         timestamppb.New(s.LastUpdated),
		ConsecutiveFailures: uint32(s.ConsecutiveFailures),
	}

	// No default so we don't swallow any new cases that get added later
	switch s.State {
	case ModuleStateUnknown:
		pbStatus.State = pb.ModuleStatus_STATE_UNSPECIFIED
	case ModuleStatePending:
		pbStatus.State = pb.ModuleStatus_STATE_PENDING
	case ModuleStateStarting:
		pbStatus.State = pb.ModuleStatus_STATE_STARTING
	case ModuleStateReady:
		pbStatus.State = pb.ModuleStatus_STATE_READY
	case ModuleStateUnhealthy:
		pbStatus.State = pb.ModuleStatus_STATE_UNHEALTHY
	case ModuleStateClosing:
		pbStatus.State = pb.ModuleStatus_STATE_CLOSING
	}

	if s.Error != nil {
		pbStatus.Error = s.Error.Error()
	}
	return pbStatus
}
