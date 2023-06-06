package inject

import (
	"context"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
)

// FrameSystemService represents a fake instance of an framesystem service.
// Due to the nature of the FrameSystem service, there should never be more than one on a robot.
// If you use an injected frame system, do not also create the system's default frame system as well.
type FrameSystemService struct {
	framesystem.Service
	name              resource.Name
	TransformPoseFunc func(
		ctx context.Context,
		pose *referenceframe.PoseInFrame,
		dst string,
		additionalTransforms []*referenceframe.LinkInFrame,
	) (*referenceframe.PoseInFrame, error)
	TransformPointCloudFunc func(
		ctx context.Context,
		srcpc pointcloud.PointCloud,
		srcName, dstName string,
	) (pointcloud.PointCloud, error)
	CurrentInputsFunc func(ctx context.Context) (map[string][]referenceframe.Input, map[string]referenceframe.InputEnabled, error)
	FrameSystemFunc   func(
		ctx context.Context,
		additionalTransforms []*referenceframe.LinkInFrame,
	) (referenceframe.FrameSystem, error)
	DoCommandFunc func(ctx context.Context,
		cmd map[string]interface{}) (map[string]interface{}, error)
	CloseFunc func(ctx context.Context) error
}

// NewFrameSystemService returns a new injected framesystem service.
func NewFrameSystemService() *FrameSystemService {
	return &FrameSystemService{name: resource.NewName(framesystem.API, "builtin")}
}

// Name returns the name of the resource.
func (fs *FrameSystemService) Name() resource.Name {
	return fs.name
}

// TransformPose calls the injected method or the real variant.
func (fs *FrameSystemService) TransformPose(
	ctx context.Context,
	pose *referenceframe.PoseInFrame,
	dst string,
	additionalTransforms []*referenceframe.LinkInFrame,
) (*referenceframe.PoseInFrame, error) {
	if fs.TransformPoseFunc == nil {
		return fs.Service.TransformPose(ctx, pose, dst, additionalTransforms)
	}
	return fs.TransformPoseFunc(ctx, pose, dst, additionalTransforms)
}

// TransformPointCloud calls the injected method or the real variant.
func (fs *FrameSystemService) TransformPointCloud(
	ctx context.Context,
	srcpc pointcloud.PointCloud,
	srcName, dstName string,
) (pointcloud.PointCloud, error) {
	if fs.TransformPointCloudFunc == nil {
		return fs.Service.TransformPointCloud(ctx, srcpc, srcName, dstName)
	}
	return fs.TransformPointCloudFunc(ctx, srcpc, srcName, dstName)
}

// CurrentInputs calls the injected method or the real variant.
func (fs *FrameSystemService) CurrentInputs(ctx context.Context) (map[string][]referenceframe.Input, map[string]referenceframe.InputEnabled, error) {
	if fs.CurrentInputsFunc == nil {
		return fs.Service.CurrentInputs(ctx)
	}
	return fs.CurrentInputsFunc(ctx)
}

func (fs *FrameSystemService) FrameSystem(
	ctx context.Context,
	additionalTransforms []*referenceframe.LinkInFrame,
) (referenceframe.FrameSystem, error) {
	if fs.FrameSystemFunc == nil {
		return fs.Service.FrameSystem(ctx, additionalTransforms)
	}
	return fs.FrameSystemFunc(ctx, additionalTransforms)
}

// DoCommand calls the injected DoCommand or the real variant.
func (fs *FrameSystemService) DoCommand(ctx context.Context,
	cmd map[string]interface{},
) (map[string]interface{}, error) {
	if fs.DoCommandFunc == nil {
		return fs.Service.DoCommand(ctx, cmd)
	}
	return fs.DoCommandFunc(ctx, cmd)
}

// Close calls the injected Close or the real version.
func (fs *FrameSystemService) Close(ctx context.Context) error {
	if fs.CloseFunc == nil {
		if fs.Service == nil {
			return nil
		}
		return fs.Service.Close(ctx)
	}
	return fs.CloseFunc(ctx)
}
