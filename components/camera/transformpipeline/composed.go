package transformpipeline

import (
	"context"
	"image"

	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"go.uber.org/multierr"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	rdkutils "go.viam.com/rdk/utils"
)

// depthToPretty takes a depth image and turns into a colorful image, with blue being
// farther away, and red being closest. Actual depth information is lost in the transform.
type depthToPretty struct {
	colorStream         gostream.VideoStream
	depthStream         gostream.VideoStream
	source              gostream.VideoSource
	intrinsicParameters *transform.PinholeCameraIntrinsics
}

func newDepthToPrettyTransform(
	ctx context.Context,
	source gostream.VideoSource,
	stream camera.StreamType,
	cameraParams *transform.PinholeCameraIntrinsics,
) (gostream.VideoSource, error) {
	depthStream := gostream.NewEmbeddedVideoStream(source)
	reader := &depthToPretty{
		depthStream:         depthStream,
		source:              source,
		intrinsicParameters: cameraParams,
	}
	reader.colorStream = gostream.NewEmbeddedVideoStreamFromReader(reader)
	return camera.NewFromReader(
		ctx,
		reader,
		&transform.PinholeCameraModel{reader.intrinsicParameters, nil},
		stream,
	)
}

func (dtp *depthToPretty) Read(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "camera::transformpipeline::depthToPretty::Read")
	defer span.End()
	i, release, err := dtp.depthStream.Next(ctx)
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
	return multierr.Combine(dtp.colorStream.Close(ctx), dtp.depthStream.Close(ctx))
}

func (dtp *depthToPretty) PointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	ctx, span := trace.StartSpan(ctx, "camera::transformpipeline::depthToPretty::PointCloud")
	defer span.End()
	if dtp.intrinsicParameters == nil {
		return nil, errors.Wrapf(transform.ErrNoIntrinsics, "depthToPretty transform cannot project to pointcloud")
	}
	// get the original depth map and colorful output
	col, dm := camera.SimultaneousColorDepthNext(ctx, dtp.colorStream, dtp.depthStream)
	if col == nil || dm == nil {
		return nil, errors.New("requested color or depth image from camera is nil")
	}
	return dtp.intrinsicParameters.RGBDToPointCloud(rimage.ConvertImage(col), dm)
}

// overlayAttrs are the attributes for an overlay transform.
type overlayAttrs struct {
	IntrinsicParams *transform.PinholeCameraIntrinsics `json:"intrinsic_parameters"`
}

// overlaySource overlays the depth and color 2D images in order to debug the alignment of the two images.
type overlaySource struct {
	src                 gostream.VideoSource
	intrinsicParameters *transform.PinholeCameraIntrinsics
}

func newOverlayTransform(
	ctx context.Context,
	src gostream.VideoSource,
	stream camera.StreamType,
	am config.AttributeMap,
) (gostream.VideoSource, error) {
	conf, err := config.TransformAttributeMapToStruct(&(overlayAttrs{}), am)
	if err != nil {
		return nil, err
	}
	attrs, ok := conf.(*overlayAttrs)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(attrs, conf)
	}
	if attrs.IntrinsicParams == nil {
		return nil, transform.ErrNoIntrinsics
	}
	if attrs.IntrinsicParams.Height <= 0. || attrs.IntrinsicParams.Width <= 0. {
		return nil, errors.Wrapf(
			transform.ErrNoIntrinsics,
			"cannot do overlay with intrinsics (width,height) = (%v, %v)",
			attrs.IntrinsicParams.Width, attrs.IntrinsicParams.Height,
		)
	}
	if attrs.IntrinsicParams.Fx <= 0. || attrs.IntrinsicParams.Fy <= 0. {
		return nil, errors.Wrapf(
			transform.ErrNoIntrinsics,
			"cannot do overlay with intrinsics (Fx,Fy) = (%v, %v)",
			attrs.IntrinsicParams.Fx, attrs.IntrinsicParams.Fy,
		)
	}
	reader := &overlaySource{src, attrs.IntrinsicParams}
	return camera.NewFromReader(
		ctx,
		reader,
		&transform.PinholeCameraModel{reader.intrinsicParameters, nil},
		stream,
	)
}

func (os *overlaySource) Read(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "camera::transformpipeline::overlay::Read")
	defer span.End()
	if os.intrinsicParameters == nil {
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
	col, dm, err := os.intrinsicParameters.PointCloudToRGBD(pc)
	if err != nil {
		return nil, nil, err
	}
	return rimage.Overlay(col, dm), func() {}, nil
}
