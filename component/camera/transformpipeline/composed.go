package transformpipeline

import (
	"context"
	"image"

	"github.com/edaniels/gostream"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
)

// depthToPretty takes a depth image and turns into a colorful image, with blue being
// farther away, and red being closest. Actual depth information is lost in the transform.
type depthToPretty struct {
	source gostream.ImageSource
	proj   rimage.Projector
}

func newDepthToPrettyTransform(ctx context.Context, source gostream.ImageSource, attrs *camera.AttrConfig) (gostream.ImageSource, error) {
	proj, _ := camera.GetProjector(ctx, attrs, nil)
	return &depthToPretty{source, proj}, nil
}

func (dtp *depthToPretty) Next(ctx context.Context) (image.Image, func(), error) {
	i, release, err := dtp.source.Next(ctx)
	if err != nil {
		return nil, nil, err
	}
	dm, err := rimage.ConvertImageToDepthMap(i)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "source camera does not make depth maps")
	}
	return dm.ToPrettyPicture(0, rimage.MaxDepth), release, nil
}

func (dtp *depthToPretty) PointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	if dtp.proj == nil {
		return nil, errors.Wrapf(transform.ErrNoIntrinsics, "depthToPretty transform cannot project to pointcloud")
	}
	// get the original depth map and colorful output
	col, dm := camera.SimultaneousColorDepthNext(ctx, dtp, dtp.source)
	if col == nil || dm == nil {
		return nil, errors.New("requested color or depth image from camera is nil")
	}
	return dtp.proj.RGBDToPointCloud(rimage.ConvertImage(col), dm)
}

// overlaySource overlays the depth and color 2D images in order to debug the alignment of the two images.
type overlaySource struct {
	src  gostream.ImageSource
	proj rimage.Projector
}

func newOverlayTransform(ctx context.Context, src gostream.ImageSource, attrs *camera.AttrConfig) (gostream.ImageSource, error) {
	proj, _ := camera.GetProjector(ctx, attrs, nil)
	return &overlaySource{src, proj}, nil
}

func (os *overlaySource) Next(ctx context.Context) (image.Image, func(), error) {
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
