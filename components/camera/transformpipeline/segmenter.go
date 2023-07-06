package transformpipeline

import (
	"context"
	"fmt"
	"image"

	"github.com/viamrobotics/gostream"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// segmenterConfig is the attribute struct for segementers (their name as found in the vision service).
type segmenterConfig struct {
	SegementerName string `json:"segementer_name"`
	CameraName     string `json:"camera_name"`
}

// segmenterSource takes an image from the camera, and idk overlays?
type segmenterSource struct {
	stream        gostream.VideoStream
	cameraName    string
	segmenterName string
	r             robot.Robot
}

func newSegmentationsTransform(
	ctx context.Context,
	source gostream.VideoSource,
	r robot.Robot,
	am utils.AttributeMap,
) (gostream.VideoSource, camera.ImageType, error) {
	conf, err := resource.TransformAttributeMap[*segmenterConfig](am)
	if err != nil {
		return nil, camera.UnspecifiedStream, err
	}

	props, err := propsFromVideoSource(ctx, source)
	if err != nil {
		return nil, camera.UnspecifiedStream, err
	}

	segmenter := &segmenterSource{
		gostream.NewEmbeddedVideoStream(source),
		conf.CameraName,
		conf.SegementerName,
		r,
	}
	src, err := camera.NewVideoSourceFromReader(ctx, segmenter, nil, props.ImageType)
	if err != nil {
		return nil, camera.UnspecifiedStream, err
	}
	return src, camera.ColorStream, err
}

func (ss *segmenterSource) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	ctx, span := trace.StartSpan(ctx, "camera::transformpipeline::segmenter::NextPointCloud")
	defer span.End()
	// get the service
	srv, err := vision.FromRobot(ss.r, ss.segmenterName)
	if err != nil {
		return nil, fmt.Errorf("source_segmenter cant find vision service: %w", err)
	}
	// apply service
	clouds, err := srv.GetObjectPointClouds(ctx, ss.cameraName, map[string]interface{}{})
	if err != nil {
		return nil, fmt.Errorf("could not get point clouds: %w", err)
	}
	// merge pointclouds
	cloudsWithOffset := make([]pointcloud.CloudAndOffsetFunc, 0, len(clouds))
	for _, cloud := range clouds {
		cloudCopy := cloud
		cloudFunc := func(ctx context.Context) (pointcloud.PointCloud, spatialmath.Pose, error) {
			return cloudCopy, nil, nil
		}
		cloudsWithOffset = append(cloudsWithOffset, cloudFunc)
	}
	mergedCloud, err := pointcloud.MergePointClouds(context.Background(), cloudsWithOffset, nil)
	if err != nil {
		return nil, fmt.Errorf("could not get point clouds: %w", err)
	}
	return mergedCloud, nil
}

// Read returns the image idk.
func (ss *segmenterSource) Read(ctx context.Context) (image.Image, func(), error) {
	img, release, err := ss.stream.Next(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get next source image: %w", err)
	}
	return img, release, nil
}

func (ss *segmenterSource) Close(ctx context.Context) error {
	return ss.stream.Close(ctx)
}
