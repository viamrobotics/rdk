package inject

import (
	"context"

	"go.viam.com/rdk/components/posetracker"
)

// PoseTracker is an injected pose tracker.
type PoseTracker struct {
	posetracker.PoseTracker
	GetPosesFunc func(ctx context.Context, bodyNames []string) (posetracker.BodyToPoseInFrame, error)
	DoFunc       func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
}

// GetPoses calls the injected GetPoses or the real version.
func (pT *PoseTracker) GetPoses(
	ctx context.Context, bodyNames []string,
) (posetracker.BodyToPoseInFrame, error) {
	if pT.GetPosesFunc == nil {
		return pT.PoseTracker.GetPoses(ctx, bodyNames)
	}
	return pT.GetPosesFunc(ctx, bodyNames)
}

// DoCommand calls the injected DoCommand or the real version.
func (pT *PoseTracker) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if pT.DoFunc == nil {
		return pT.PoseTracker.DoCommand(ctx, cmd)
	}
	return pT.DoFunc(ctx, cmd)
}
