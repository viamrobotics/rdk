package motion

import (
	"github.com/google/uuid"
	pb "go.viam.com/api/service/motion/v1"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// pbToPlanWithStatus converts a *pb.PlanWithStatus to a PlanWithStatus.
func pbToPlanWithStatus(pws *pb.PlanWithStatus) (PlanWithStatus, error) {
	plan, err := pbToPlan(pws.Plan)
	if err != nil {
		return PlanWithStatus{}, err
	}

	status, err := pbToPlanStatus(pws.Status)
	if err != nil {
		return PlanWithStatus{}, err
	}

	statusHistory := []PlanStatus{}
	for _, s := range pws.StatusHistory {
		ps, err := pbToPlanStatus(s)
		if err != nil {
			return PlanWithStatus{}, err
		}
		statusHistory = append(statusHistory, ps)
	}

	return PlanWithStatus{
		Plan:          plan,
		Status:        status,
		StatusHistory: statusHistory,
	}, nil
}

// pbToPlanStatus converts a *pb.PlanStatus to a PlanStatus.
func pbToPlanStatus(ps *pb.PlanStatus) (PlanStatus, error) {
	planID, err := uuid.Parse(ps.PlanId)
	if err != nil {
		return PlanStatus{}, err
	}

	opid, err := uuid.Parse(ps.OperationId)
	if err != nil {
		return PlanStatus{}, err
	}

	var reason string
	if ps.Reason != nil {
		reason = *ps.Reason
	}

	return PlanStatus{
		PlanID:      planID,
		OperationID: opid,
		State:       int32(ps.State.Number()),
		Reason:      reason,
		Timestamp:   ps.Timestamp.AsTime(),
	}, nil
}

// pbToPlan converts a *pb.Plan to a Plan.
func pbToPlan(p *pb.Plan) (Plan, error) {
	id, err := uuid.Parse(p.Id)
	if err != nil {
		return Plan{}, err
	}

	steps := []Step{}
	for _, s := range p.Steps {
		step := make(Step)
		for k, v := range s.Step {
			name, err := resource.NewFromString(k)
			if err != nil {
				return Plan{}, err
			}
			step[name] = spatialmath.NewPoseFromProtobuf(v.Pose)
		}
		steps = append(steps, step)
	}
	return Plan{ID: id, Steps: steps}, nil
}

// pbToPlan converts a *pb.GetPlanResponse to a OpIDPlans.
func pbToOpIDPlans(resp *pb.GetPlanResponse) (OpIDPlans, error) {
	current, err := pbToPlanWithStatus(resp.CurrentPlanWithStatus)
	if err != nil {
		return OpIDPlans{}, err
	}

	replanHistory := []PlanWithStatus{}
	for _, pws := range resp.ReplanHistory {
		p, err := pbToPlanWithStatus(pws)
		if err != nil {
			return OpIDPlans{}, err
		}
		replanHistory = append(replanHistory, p)
	}

	return OpIDPlans{
		CurrentPlanWithStatus: current,
		ReplanHistory:         replanHistory,
	}, nil
}
