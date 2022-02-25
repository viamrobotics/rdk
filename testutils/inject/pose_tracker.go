package inject

import (
	"context"

	"go.viam.com/rdk/component/posetracker"
)

// PoseTracker is an injected pose tracker.
type PoseTracker struct {
	posetracker.PoseTracker
	GetPosesFunc func(ctx context.Context, bodyNames []string) (posetracker.BodyToPoseInFrame, error)
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
