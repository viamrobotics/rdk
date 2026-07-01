//go:build !no_cgo

// Package inject separates the injected motion service from the rest of the injected packages to isolate an NLopt dependency.
package inject

import (
	"context"

	"braces.dev/errtrace"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
)

// MotionService represents a fake instance of an motion
// service.
type MotionService struct {
	motion.Service
	name     resource.Name
	MoveFunc func(
		ctx context.Context,
		req motion.MoveReq,
	) (bool, error)
	MoveOnMapFunc func(
		ctx context.Context,
		req motion.MoveOnMapReq,
	) (motion.ExecutionID, error)
	MoveOnGlobeFunc func(
		ctx context.Context,
		req motion.MoveOnGlobeReq,
	) (motion.ExecutionID, error)
	GetPoseFunc func(
		ctx context.Context,
		componentName string,
		destinationFrame string,
		supplementalTransforms []*referenceframe.LinkInFrame,
		extra map[string]interface{},
	) (*referenceframe.PoseInFrame, error)
	StopPlanFunc func(
		ctx context.Context,
		req motion.StopPlanReq,
	) error
	ListPlanStatusesFunc func(
		ctx context.Context,
		req motion.ListPlanStatusesReq,
	) ([]motion.PlanStatusWithID, error)
	PlanHistoryFunc func(
		ctx context.Context,
		req motion.PlanHistoryReq,
	) ([]motion.PlanWithStatus, error)
	DoCommandFunc func(
		ctx context.Context,
		cmd map[string]interface{}) (map[string]interface{}, error,
	)
	StatusFunc func(ctx context.Context) (map[string]interface{}, error)
	CloseFunc  func(ctx context.Context) error
}

// NewMotionService returns a new injected motion service.
func NewMotionService(name string) *MotionService {
	return &MotionService{name: motion.Named(name)}
}

// Name returns the name of the resource.
func (mgs *MotionService) Name() resource.Name {
	return mgs.name
}

// Move calls the injected Move or the real variant.
func (mgs *MotionService) Move(ctx context.Context, req motion.MoveReq) (bool, error) {
	if mgs.MoveFunc == nil {
		return errtrace.Wrap2(mgs.Service.Move(ctx, req))
	}
	return errtrace.Wrap2(mgs.MoveFunc(ctx, req))
}

// MoveOnMap calls the injected MoveOnMap or the real variant.
func (mgs *MotionService) MoveOnMap(
	ctx context.Context,
	req motion.MoveOnMapReq,
) (motion.ExecutionID, error) {
	if mgs.MoveOnMapFunc == nil {
		return errtrace.Wrap2(mgs.Service.MoveOnMap(ctx, req))
	}
	return errtrace.Wrap2(mgs.MoveOnMapFunc(ctx, req))
}

// MoveOnGlobe calls the injected MoveOnGlobe or the real variant.
func (mgs *MotionService) MoveOnGlobe(ctx context.Context, req motion.MoveOnGlobeReq) (motion.ExecutionID, error) {
	if mgs.MoveOnGlobeFunc == nil {
		return errtrace.Wrap2(mgs.Service.MoveOnGlobe(ctx, req))
	}
	return errtrace.Wrap2(mgs.MoveOnGlobeFunc(ctx, req))
}

// GetPose calls the injected GetPose or the real variant.
func (mgs *MotionService) GetPose(
	ctx context.Context,
	componentName string,
	destinationFrame string,
	supplementalTransforms []*referenceframe.LinkInFrame,
	extra map[string]interface{},
) (*referenceframe.PoseInFrame, error) {
	if mgs.GetPoseFunc == nil {
		return errtrace.Wrap2(mgs.Service.GetPose(ctx, componentName, destinationFrame, supplementalTransforms, extra))
	}
	return errtrace.Wrap2(mgs.GetPoseFunc(ctx, componentName, destinationFrame, supplementalTransforms, extra))
}

// StopPlan calls the injected StopPlan or the real variant.
func (mgs *MotionService) StopPlan(
	ctx context.Context,
	req motion.StopPlanReq,
) error {
	if mgs.StopPlanFunc == nil {
		return errtrace.Wrap(mgs.Service.StopPlan(ctx, req))
	}
	return errtrace.Wrap(mgs.StopPlanFunc(ctx, req))
}

// ListPlanStatuses calls the injected ListPlanStatuses or the real variant.
func (mgs *MotionService) ListPlanStatuses(
	ctx context.Context,
	req motion.ListPlanStatusesReq,
) ([]motion.PlanStatusWithID, error) {
	if mgs.ListPlanStatusesFunc == nil {
		return errtrace.Wrap2(mgs.Service.ListPlanStatuses(ctx, req))
	}
	return errtrace.Wrap2(mgs.ListPlanStatusesFunc(ctx, req))
}

// PlanHistory calls the injected PlanHistory or the real variant.
func (mgs *MotionService) PlanHistory(
	ctx context.Context,
	req motion.PlanHistoryReq,
) ([]motion.PlanWithStatus, error) {
	if mgs.PlanHistoryFunc == nil {
		return errtrace.Wrap2(mgs.Service.PlanHistory(ctx, req))
	}
	return errtrace.Wrap2(mgs.PlanHistoryFunc(ctx, req))
}

// DoCommand calls the injected DoCommand or the real variant.
func (mgs *MotionService) DoCommand(ctx context.Context,
	cmd map[string]interface{},
) (map[string]interface{}, error) {
	if mgs.DoCommandFunc == nil {
		return errtrace.Wrap2(mgs.Service.DoCommand(ctx, cmd))
	}
	return errtrace.Wrap2(mgs.DoCommandFunc(ctx, cmd))
}

// Status calls the injected Status or the real version.
func (mgs *MotionService) Status(ctx context.Context) (map[string]interface{}, error) {
	if mgs.StatusFunc != nil {
		return errtrace.Wrap2(mgs.StatusFunc(ctx))
	}
	if mgs.Service != nil {
		return errtrace.Wrap2(mgs.Service.Status(ctx))
	}
	return map[string]interface{}{}, nil
}

// Close calls the injected Close or the real version.
func (mgs *MotionService) Close(ctx context.Context) error {
	if mgs.CloseFunc == nil {
		if mgs.Service == nil {
			return nil
		}
		return errtrace.Wrap(mgs.Service.Close(ctx))
	}
	return errtrace.Wrap(mgs.CloseFunc(ctx))
}
