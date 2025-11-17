package module

import (
	"context"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/robot/framesystem"
)

type frameSystemClient struct {
	robotClient *client.RobotClient
	resource.TriviallyCloseable
	resource.TriviallyReconfigurable
}

// NewFrameSystemClient provides access to only the framesystem.Service functions contained inside RobotClient.
func NewFrameSystemClient(robotClient *client.RobotClient) framesystem.Service {
	return &frameSystemClient{robotClient: robotClient}
}

func (f *frameSystemClient) Name() resource.Name {
	return framesystem.PublicServiceName
}

func (f *frameSystemClient) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return nil, resource.ErrDoUnimplemented
}

func (f *frameSystemClient) FrameSystemConfig(ctx context.Context) (*framesystem.Config, error) {
	return f.robotClient.FrameSystemConfig(ctx)
}

func (f *frameSystemClient) GetPose(ctx context.Context,
	componentName, destinationFrame string,
	supplementalTransforms []*referenceframe.LinkInFrame,
	extra map[string]interface{},
) (*referenceframe.PoseInFrame, error) {
	return f.robotClient.GetPose(ctx, componentName, destinationFrame, supplementalTransforms, extra)
}

func (f *frameSystemClient) TransformPose(
	ctx context.Context,
	pose *referenceframe.PoseInFrame,
	dst string,
	supplementalTransforms []*referenceframe.LinkInFrame,
) (*referenceframe.PoseInFrame, error) {
	return f.robotClient.TransformPose(ctx, pose, dst, supplementalTransforms)
}

func (f *frameSystemClient) TransformPointCloud(
	ctx context.Context,
	srcpc pointcloud.PointCloud,
	srcName,
	dstName string,
) (pointcloud.PointCloud, error) {
	return f.robotClient.TransformPointCloud(ctx, srcpc, srcName, dstName)
}

func (f *frameSystemClient) CurrentInputs(ctx context.Context) (referenceframe.FrameSystemInputs, error) {
	return f.robotClient.CurrentInputs(ctx)
}
