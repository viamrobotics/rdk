package inject

import (
	"context"

	"go.viam.com/rdk/components/posetracker"
)

// PoseTracker is an injected pose tracker.
type PoseTracker struct {
	posetracker.PoseTracker
	PosesFunc func(ctx context.Context, bodyNames []string, extra map[string]interface{}) (posetracker.BodyToPoseInFrame, error)
	DoFunc    func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
}

// Poses calls the injected Poses or the real version.
func (pT *PoseTracker) Poses(
	ctx context.Context, bodyNames []string, extra map[string]interface{},
) (posetracker.BodyToPoseInFrame, error) {
	if pT.PosesFunc == nil {
		return pT.PoseTracker.Poses(ctx, bodyNames, extra)
	}
	return pT.PosesFunc(ctx, bodyNames, extra)
}

// DoCommand calls the injected DoCommand or the real version.
func (pT *PoseTracker) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if pT.DoFunc == nil {
		return pT.PoseTracker.DoCommand(ctx, cmd)
	}
	return pT.DoFunc(ctx, cmd)
}
