package inject

//  CR erodkin: delete me, replace references to use of inject robot
import (
	"context"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/robot/framesystem"
	framesystemparts "go.viam.com/rdk/robot/framesystem/parts"
)

// FrameSystemService is an injected FrameSystem service.
type FrameSystemService struct {
	framesystem.Service
	ConfigFunc        func(ctx context.Context, additionalTransforms []*commonpb.Transform) (framesystemparts.Parts, error)
	TransformPoseFunc func(
		ctx context.Context, pose *referenceframe.PoseInFrame, dst string,
		additionalTransforms []*commonpb.Transform,
	) (*referenceframe.PoseInFrame, error)
}

// Config calls the injected Config or the real version.
func (fss *FrameSystemService) Config(ctx context.Context, additionalTransforms []*commonpb.Transform) (framesystemparts.Parts, error) {
	if fss.ConfigFunc == nil {
		return fss.Config(ctx, additionalTransforms)
	}
	return fss.ConfigFunc(ctx, additionalTransforms)
}

// TransformPose calls the injected TransformPose or the real version.
func (fss *FrameSystemService) TransformPose(
	ctx context.Context,
	pose *referenceframe.PoseInFrame,
	dst string,
	additionalTransforms []*commonpb.Transform,
) (*referenceframe.PoseInFrame, error) {
	if fss.TransformPoseFunc == nil {
		return fss.TransformPose(ctx, pose, dst, additionalTransforms)
	}
	return fss.TransformPoseFunc(ctx, pose, dst, additionalTransforms)
}
