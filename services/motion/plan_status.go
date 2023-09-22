package motion

import (
	"time"

	pb "go.viam.com/api/service/motion/v1"

	"go.viam.com/rdk/resource"
)

type PlanWithStatus struct {
	plan   Plan
	status []PlanStatus
}

func (pws PlanWithStatus) String() string {
	// TODO
	return ""
}

func planWithStatusToProto(pws PlanWithStatus) *pb.PlanWithStatus {
	// TODO
	return nil
}

func planWithStatusFromProto(pws *pb.PlanWithStatus) PlanWithStatus {
	// TODO
	return PlanWithStatus{}
}

type PlanStatus struct {
	PlanID
	State     PlanState
	Timestamp time.Time
	Reason    string
}

func (ps PlanStatus) String() string {
	// TODO
	return ""
}

func planStatusToProto(ps PlanStatus) *pb.PlanStatus {
	// TODO
	return nil
}

func planStatusFromProto(pws *pb.PlanStatus) PlanStatus {
	// TODO
	return PlanStatus{}
}

type PlanID struct {
	UniqueID      string
	ExecutionID   string
	RootComponent resource.Name
}

type Plan struct {
	PlanID
	// the plan itself
}

type PlanState int8

const (
	PlanStateUnspecified = PlanState(iota)
	PlanStateInProgress
	PlanStateStopped
	PlanStateSucceeded
	PlanStateFailed
)

func (ps PlanState) String() string {
	switch ps {
	case PlanStateInProgress:
		return "in progress"
	case PlanStateStopped:
		return "stopped"
	case PlanStateSucceeded:
		return "succeeded"
	case PlanStateFailed:
		return "failed"
	default:
		return "unspecified"
	}
}

func planStateFromProto(ps pb.PlanState) PlanState {
	switch ps {
	case pb.PlanState_PLAN_STATE_IN_PROGRESS:
		return PlanStateInProgress
	case pb.PlanState_PLAN_STATE_STOPPED:
		return PlanStateStopped
	case pb.PlanState_PLAN_STATE_SUCCEEDED:
		return PlanStateSucceeded
	case pb.PlanState_PLAN_STATE_FAILED:
		return PlanStateFailed
	default:
		return PlanStateUnspecified
	}
}

func planStateToProto(ps PlanState) pb.PlanState {
	switch ps {
	case PlanStateInProgress:
		return pb.PlanState_PLAN_STATE_IN_PROGRESS
	case PlanStateStopped:
		return pb.PlanState_PLAN_STATE_STOPPED
	case PlanStateSucceeded:
		return pb.PlanState_PLAN_STATE_SUCCEEDED
	case PlanStateFailed:
		return pb.PlanState_PLAN_STATE_FAILED
	default:
		return pb.PlanState_PLAN_STATE_UNSPECIFIED
	}
}
