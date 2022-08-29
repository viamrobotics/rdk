package transformpipeline

import (
	"context"
	"image"

	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"go.uber.org/multierr"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
)

// depthToPretty takes a depth image and turns into a colorful image, with blue being
// farther away, and red being closest. Actual depth information is lost in the transform.
type depthToPretty struct {
	colorStream gostream.VideoStream
	depthStream gostream.VideoStream
	source      gostream.VideoSource
	proj        rimage.Projector // keep a local copy for faster use
}

func newDepthToPrettyTransform(ctx context.Context, source gostream.VideoSource, attrs *camera.AttrConfig) (gostream.VideoSource, error) {
	proj, _ := camera.GetProjector(ctx, attrs, nil)
	depthStream := gostream.NewEmbeddedVideoStream(source)
	reader := &depthToPretty{
		depthStream: depthStream,
		source:      source,
		proj:        proj,
	}
	reader.colorStream = gostream.NewEmbeddedVideoStreamFromReader(reader)
	return camera.NewFromReader(reader, proj)
}

func (dtp *depthToPretty) Read(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "camera::transformpipeline::depthToPretty::Read")
	defer span.End()
	i, release, err := dtp.depthStream.Next(ctx)
	if err != nil {
		return nil, nil, err
	}
	dm, err := rimage.ConvertImageToDepthMap(i)
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
	if dtp.proj == nil {
		return nil, errors.Wrapf(transform.ErrNoIntrinsics, "depthToPretty transform cannot project to pointcloud")
	}
	// get the original depth map and colorful output
	col, dm := camera.SimultaneousColorDepthNext(ctx, dtp.colorStream, dtp.depthStream)
	if col == nil || dm == nil {
		return nil, errors.New("requested color or depth image from camera is nil")
	}
	return dtp.proj.RGBDToPointCloud(rimage.ConvertImage(col), dm)
}

// overlaySource overlays the depth and color 2D images in order to debug the alignment of the two images.
type overlaySource struct {
	src  gostream.VideoSource
	proj rimage.Projector
}

func newOverlayTransform(ctx context.Context, src gostream.VideoSource, attrs *camera.AttrConfig) (gostream.VideoSource, error) {
	proj, _ := camera.GetProjector(ctx, attrs, nil)
	reader := &overlaySource{src, proj}
	return camera.NewFromReader(reader, proj)
}

func (os *overlaySource) Read(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "camera::transformpipeline::overlay::Read")
	defer span.End()
	if os.proj == nil {
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
	col, dm, err := os.proj.PointCloudToRGBD(pc)
	if err != nil {
		return nil, nil, err
	}
	return rimage.Overlay(col, dm), func() {}, nil
}
