package inject

import (
	"context"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/posetracker"
	rdkutils "go.viam.com/rdk/utils"
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

// Do calls the injected Do or the real version.
func (pT *PoseTracker) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if pT.DoFunc == nil {
		if doer, ok := pT.PoseTracker.(generic.Generic); ok {
			return doer.Do(ctx, cmd)
		}
		return nil, rdkutils.NewUnimplementedInterfaceError("Generic", pT.PoseTracker)
	}
	return pT.DoFunc(ctx, cmd)
}
