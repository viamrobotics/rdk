package inject

import (
	"context"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/framesystem"
)

// FrameSystemService is an injected FrameSystem service.
type FrameSystemService struct {
	framesystem.Service
	ConfigFunc        func(ctx context.Context) ([]*config.FrameSystemPart, error)
	TransformPoseFunc func(ctx context.Context, pose *referenceframe.PoseInFrame, dst string) (*referenceframe.PoseInFrame, error)
	FrameSystemFunc   func(ctx context.Context, name string) (referenceframe.FrameSystem, error)
	PrintFunc         func(ctx context.Context) (string, error)
}

// Config calls the injected Config or the real version.
func (fss *FrameSystemService) Config(ctx context.Context) ([]*config.FrameSystemPart, error) {
	if fss.ConfigFunc == nil {
		return fss.Config(ctx)
	}
	return fss.ConfigFunc(ctx)
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

// FrameSystem calls the injected FrameSystem or the real version.
func (fss *FrameSystemService) FrameSystem(ctx context.Context, name string) (referenceframe.FrameSystem, error) {
	if fss.FrameSystemFunc == nil {
		return fss.FrameSystem(ctx, name)
	}
	return fss.FrameSystemFunc(ctx, name)
}

// Print calls the injected Print or the real version.
func (fss *FrameSystemService) Print(ctx context.Context) (string, error) {
	if fss.PrintFunc == nil {
		return fss.Print(ctx)
	}
	return fss.PrintFunc(ctx)
}
