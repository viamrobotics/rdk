package transformpipeline

import (
	"context"
	"image"

	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"go.viam.com/rdk/gostream"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
)

// depthToPretty takes a depth image and turns into a colorful image, with blue being
// farther away, and red being closest. Actual depth information is lost in the transform.
type depthToPretty struct {
	originalStream gostream.VideoStream
	cameraModel    *transform.PinholeCameraModel
}

func propsFromVideoSource(ctx context.Context, source gostream.VideoSource) (camera.Properties, error) {
	var camProps camera.Properties
	//nolint:staticcheck
	if cameraSrc, ok := source.(camera.Camera); ok {
		props, err := cameraSrc.Properties(ctx)
		if err != nil {
			return camProps, err
		}
		camProps = props
	}
	return camProps, nil
}

func newDepthToPrettyTransform(
	ctx context.Context,
	source gostream.VideoSource,
	stream camera.ImageType,
) (gostream.VideoSource, camera.ImageType, error) {
	if stream != camera.DepthStream {
		return nil, camera.UnspecifiedStream,
			errors.Errorf("source has stream type %s, depth_to_pretty only supports depth stream inputs", stream)
	}
	props, err := propsFromVideoSource(ctx, source)
	if err != nil {
		return nil, camera.UnspecifiedStream, err
	}
	var cameraModel transform.PinholeCameraModel
	cameraModel.PinholeCameraIntrinsics = props.IntrinsicParams

	if props.DistortionParams != nil {
		cameraModel.Distortion = props.DistortionParams
	}
	depthStream := gostream.NewEmbeddedVideoStream(source)
	reader := &depthToPretty{
		originalStream: depthStream,
		cameraModel:    &cameraModel,
	}
	src, err := camera.NewVideoSourceFromReader(ctx, reader, &cameraModel, camera.ColorStream)
	if err != nil {
		return nil, camera.UnspecifiedStream, err
	}
	return src, camera.ColorStream, err
}

func (dtp *depthToPretty) Read(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "camera::transformpipeline::depthToPretty::Read")
	defer span.End()
	i, release, err := dtp.originalStream.Next(ctx)
	if err != nil {
		return nil, nil, err
	}
	dm, err := rimage.ConvertImageToDepthMap(ctx, i)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "source camera does not make depth maps")
	}
	return dm.ToPrettyPicture(0, rimage.MaxDepth), release, nil
}

func (dtp *depthToPretty) Close(ctx context.Context) error {
	return dtp.originalStream.Close(ctx)
}

func (dtp *depthToPretty) PointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	ctx, span := trace.StartSpan(ctx, "camera::transformpipeline::depthToPretty::PointCloud")
	defer span.End()
	if dtp.cameraModel == nil || dtp.cameraModel.PinholeCameraIntrinsics == nil {
		return nil, errors.Wrapf(transform.ErrNoIntrinsics, "depthToPretty transform cannot project to pointcloud")
	}
	i, release, err := dtp.originalStream.Next(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	dm, err := rimage.ConvertImageToDepthMap(ctx, i)
	if err != nil {
		return nil, errors.Wrapf(err, "source camera does not make depth maps")
	}
	img := dm.ToPrettyPicture(0, rimage.MaxDepth)
	return dtp.cameraModel.RGBDToPointCloud(img, dm)
}

// overlayConfig are the attributes for an overlay transform.
type overlayConfig struct {
	IntrinsicParams *transform.PinholeCameraIntrinsics `json:"intrinsic_parameters"`
}

// overlaySource overlays the depth and color 2D images in order to debug the alignment of the two images.
type overlaySource struct {
	src         gostream.VideoSource
	cameraModel *transform.PinholeCameraModel
}

func newOverlayTransform(
	ctx context.Context,
	src gostream.VideoSource,
	stream camera.ImageType,
	am utils.AttributeMap,
) (gostream.VideoSource, camera.ImageType, error) {
	conf, err := resource.TransformAttributeMap[*overlayConfig](am)
	if err != nil {
		return nil, camera.UnspecifiedStream, err
	}

	props, err := propsFromVideoSource(ctx, src)
	if err != nil {
		return nil, camera.UnspecifiedStream, err
	}
	var cameraModel transform.PinholeCameraModel
	cameraModel.PinholeCameraIntrinsics = props.IntrinsicParams

	if props.DistortionParams != nil {
		cameraModel.Distortion = props.DistortionParams
	}
	if conf.IntrinsicParams != nil && conf.IntrinsicParams.Height > 0. &&
		conf.IntrinsicParams.Width > 0. && conf.IntrinsicParams.Fx > 0. && conf.IntrinsicParams.Fy > 0. {
		cameraModel.PinholeCameraIntrinsics = conf.IntrinsicParams
	}
	if cameraModel.PinholeCameraIntrinsics == nil {
		return nil, camera.UnspecifiedStream, transform.ErrNoIntrinsics
	}
	reader := &overlaySource{src, &cameraModel}
	src, err = camera.NewVideoSourceFromReader(ctx, reader, &cameraModel, stream)
	if err != nil {
		return nil, camera.UnspecifiedStream, err
	}
	return src, stream, err
}

func (os *overlaySource) Read(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "camera::transformpipeline::overlay::Read")
	defer span.End()
	if os.cameraModel == nil || os.cameraModel.PinholeCameraIntrinsics == nil {
		return nil, nil, transform.ErrNoIntrinsics
	}
	srcPointCloud, ok := os.src.(camera.PointCloudSource)
	if !ok {
		return nil, nil, errors.New("source of overlay transform does not have PointCloud method")
	}
	pc, err := srcPointCloud.NextPointCloud(ctx)
	if err != nil {
		return nil, nil, err
	}
	col, dm, err := os.cameraModel.PointCloudToRGBD(pc)
	if err != nil {
		return nil, nil, err
	}
	return rimage.Overlay(col, dm), func() {}, nil
}

func (os *overlaySource) Close(ctx context.Context) error {
	return nil
}
