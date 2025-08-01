package inject

import (
	"context"

	"go.viam.com/rdk/components/posetracker"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
)

// PoseTracker is an injected pose tracker.
type PoseTracker struct {
	posetracker.PoseTracker
	name      resource.Name
	PosesFunc func(ctx context.Context, bodyNames []string, extra map[string]interface{}) (referenceframe.FrameSystemPoses, error)
	DoFunc    func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	CloseFunc func() error
}

// NewPoseTracker returns a new injected pose tracker.
func NewPoseTracker(name string) *PoseTracker {
	return &PoseTracker{name: posetracker.Named(name)}
}

// Name returns the name of the resource.
func (pT *PoseTracker) Name() resource.Name {
	return pT.name
}

// Poses calls the injected Poses or the real version.
func (pT *PoseTracker) Poses(
	ctx context.Context, bodyNames []string, extra map[string]interface{},
) (referenceframe.FrameSystemPoses, error) {
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

// Close calls the injected Close or the real version.
func (pT *PoseTracker) Close(ctx context.Context) error {
	if pT.CloseFunc == nil {
		if pT.PoseTracker == nil {
			return nil
		}
		return pT.PoseTracker.Close(ctx)
	}
	return pT.CloseFunc()
}
