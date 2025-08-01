package inject

import (
	"context"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
)

// FrameSystemService represents a fake instance of a framesystem service.
// Due to the nature of the framesystem service, there should never be more than one on a robot.
// If you use an injected frame system, do not also create the system's default frame system as well.
type FrameSystemService struct {
	framesystem.Service
	name        resource.Name
	GetPoseFunc func(
		ctx context.Context,
		componentName, destinationFrame string,
		supplementalTransforms []*referenceframe.LinkInFrame,
		extra map[string]interface{},
	) (*referenceframe.PoseInFrame, error)
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
	CurrentInputsFunc func(ctx context.Context) (referenceframe.FrameSystemInputs, map[string]framesystem.InputEnabled, error)
	FrameSystemFunc   func(
		ctx context.Context,
		additionalTransforms []*referenceframe.LinkInFrame,
	) (*referenceframe.FrameSystem, error)
	DoCommandFunc func(
		ctx context.Context,
		cmd map[string]interface{},
	) (map[string]interface{}, error)
	CloseFunc func(ctx context.Context) error
}

// NewFrameSystemService returns a new injected framesystem service.
func NewFrameSystemService(name string) *FrameSystemService {
	resourceName := resource.NewName(
		resource.APINamespaceRDKInternal.WithServiceType("framesystem"),
		name,
	)
	return &FrameSystemService{name: resourceName}
}

// Name returns the name of the resource.
func (fs *FrameSystemService) Name() resource.Name {
	return fs.name
}

// GetPose calls the injected GetPose or the real variant.
func (fs *FrameSystemService) GetPose(
	ctx context.Context,
	componentName, destinationFrame string,
	supplementalTransforms []*referenceframe.LinkInFrame,
	extra map[string]interface{},
) (*referenceframe.PoseInFrame, error) {
	if fs.GetPoseFunc == nil {
		return fs.Service.GetPose(ctx, componentName, destinationFrame, supplementalTransforms, extra)
	}
	return fs.GetPoseFunc(ctx, componentName, destinationFrame, supplementalTransforms, extra)
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
