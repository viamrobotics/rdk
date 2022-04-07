package inject

import (
	"context"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/framesystem"
)

// FrameSystemService is an injected FrameSystem service.
type FrameSystemService struct {
	framesystem.Service
	ConfigFunc        func(ctx context.Context) (framesystem.Parts, error)
	FrameSystemFunc   func(ctx context.Context) (referenceframe.FrameSystem, error)
	TransformPoseFunc func(ctx context.Context, pose *referenceframe.PoseInFrame, dst string) (*referenceframe.PoseInFrame, error)
}

// Config calls the injected Config or the real version.
func (fss *FrameSystemService) Config(ctx context.Context) (framesystem.Parts, error) {
	if fss.ConfigFunc == nil {
		return fss.Config(ctx)
	}
	return fss.ConfigFunc(ctx)
}

// FrameSystem calls the injected FrameSystem or the real version.
func (fss *FrameSystemService) FrameSystem(ctx context.Context) (referenceframe.FrameSystem, error) {
	if fss.FrameSystemFunc == nil {
		return fss.FrameSystem(ctx)
	}
	return fss.FrameSystemFunc(ctx)
}

// TransformPose calls the injected TransformPose or the real version.
func (fss *FrameSystemService) TransformPose(
	ctx context.Context,
	pose *referenceframe.PoseInFrame,
	dst string) (*referenceframe.PoseInFrame, error) {
	if fss.TransformPoseFunc == nil {
		return fss.TransformPose(ctx, pose, dst)
	}
	return fss.TransformPoseFunc(ctx, pose, dst)
}
