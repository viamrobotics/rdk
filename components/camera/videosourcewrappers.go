package camera

import (
	"context"
	"time"

	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"go.uber.org/multierr"

	"go.viam.com/rdk/components/camera/rtppassthrough"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/depthadapter"
	"go.viam.com/rdk/rimage/transform"
)

// FromVideoSource creates a Camera resource from a VideoSource.
// Note: this strips away Reconfiguration and DoCommand abilities.
// If needed, implement the Camera another way. For example, a webcam
// implements a Camera manually so that it can atomically reconfigure itself.
func FromVideoSource(name resource.Name, src VideoSource, logger logging.Logger) Camera {
	var rtpPassthroughSource rtppassthrough.Source
	if ps, ok := src.(rtppassthrough.Source); ok {
		rtpPassthroughSource = ps
	}
	return &sourceBasedCamera{
		rtpPassthroughSource: rtpPassthroughSource,
		Named:                name.AsNamed(),
		VideoSource:          src,
		Logger:               logger,
	}
}

// NewVideoSourceFromReader creates a VideoSource either with or without a projector. The stream type
// argument is for detecting whether or not the resulting camera supports return
// of pointcloud data in the absence of an implemented NextPointCloud function.
// If this is unknown or not applicable, a value of camera.Unspecified stream can be supplied.
func NewVideoSourceFromReader(
	ctx context.Context,
	reader gostream.VideoReader,
	syst *transform.PinholeCameraModel, imageType ImageType,
) (VideoSource, error) {
	if reader == nil {
		return nil, errors.New("cannot have a nil reader")
	}
	var rtpPassthroughSource rtppassthrough.Source
	passthrough, isRTPPassthrough := reader.(rtppassthrough.Source)
	if isRTPPassthrough {
		rtpPassthroughSource = passthrough
	}
	vs := gostream.NewVideoSource(reader, prop.Video{})
	actualSystem := syst
	if actualSystem == nil {
		srcCam, ok := reader.(VideoSource)
		if ok {
			props, err := srcCam.Properties(ctx)
			if err != nil {
				return nil, NewPropertiesError("source camera")
			}

			var cameraModel transform.PinholeCameraModel
			cameraModel.PinholeCameraIntrinsics = props.IntrinsicParams

			if props.DistortionParams != nil {
				cameraModel.Distortion = props.DistortionParams
			}
			actualSystem = &cameraModel
		}
	}
	return &videoSource{
		rtpPassthroughSource: rtpPassthroughSource,
		system:               actualSystem,
		videoSource:          vs,
		videoStream:          gostream.NewEmbeddedVideoStream(vs),
		actualSource:         reader,
		imageType:            imageType,
	}, nil
}

// WrapVideoSourceWithProjector creates a Camera either with or without a projector. The stream type
// argument is for detecting whether or not the resulting camera supports return
// of pointcloud data in the absence of an implemented NextPointCloud function.
// If this is unknown or not applicable, a value of camera.Unspecified stream can be supplied.
func WrapVideoSourceWithProjector(
	ctx context.Context,
	source gostream.VideoSource,
	syst *transform.PinholeCameraModel, imageType ImageType,
) (VideoSource, error) {
	if source == nil {
		return nil, errors.New("cannot have a nil source")
	}

	actualSystem := syst
	if actualSystem == nil {
		//nolint:staticcheck
		srcCam, ok := source.(Camera)
		if ok {
			props, err := srcCam.Properties(ctx)
			if err != nil {
				return nil, NewPropertiesError("source camera")
			}
			var cameraModel transform.PinholeCameraModel
			cameraModel.PinholeCameraIntrinsics = props.IntrinsicParams

			if props.DistortionParams != nil {
				cameraModel.Distortion = props.DistortionParams
			}

			actualSystem = &cameraModel
		}
	}
	return &videoSource{
		system:       actualSystem,
		videoSource:  source,
		videoStream:  gostream.NewEmbeddedVideoStream(source),
		actualSource: source,
		imageType:    imageType,
	}, nil
}

// videoSource implements a Camera with a gostream.VideoSource.
type videoSource struct {
	rtpPassthroughSource rtppassthrough.Source
	videoSource          gostream.VideoSource
	videoStream          gostream.VideoStream
	actualSource         interface{}
	system               *transform.PinholeCameraModel
	imageType            ImageType
}

func (vs *videoSource) Stream(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
	return vs.videoSource.Stream(ctx, errHandlers...)
}

// Images is for getting simultaneous images from different sensors
// If the underlying source did not specify an Images function, a default is applied.
// The default returns a list of 1 image from ReadImage, and the current time.
func (vs *videoSource) Images(ctx context.Context) ([]NamedImage, resource.ResponseMetadata, error) {
	ctx, span := trace.StartSpan(ctx, "camera::videoSource::Images")
	defer span.End()
	if c, ok := vs.actualSource.(ImagesSource); ok {
		return c.Images(ctx)
	}
	img, release, err := ReadImage(ctx, vs.videoSource)
	if err != nil {
		return nil, resource.ResponseMetadata{}, errors.Wrap(err, "videoSource: call to get Images failed")
	}
	defer func() {
		if release != nil {
			release()
		}
	}()
	ts := time.Now()
	return []NamedImage{{img, ""}}, resource.ResponseMetadata{CapturedAt: ts}, nil
}

// NextPointCloud returns the next PointCloud from the camera, or will error if not supported.
func (vs *videoSource) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	ctx, span := trace.StartSpan(ctx, "camera::videoSource::NextPointCloud")
	defer span.End()
	if c, ok := vs.actualSource.(PointCloudSource); ok {
		return c.NextPointCloud(ctx)
	}
	if vs.system == nil || vs.system.PinholeCameraIntrinsics == nil {
		return nil, transform.NewNoIntrinsicsError("cannot do a projection to a point cloud")
	}
	img, release, err := vs.videoStream.Next(ctx)
	defer release()
	if err != nil {
		return nil, err
	}
	dm, err := rimage.ConvertImageToDepthMap(ctx, img)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot project to a point cloud")
	}
	return depthadapter.ToPointCloud(dm, vs.system.PinholeCameraIntrinsics), nil
}

func (vs *videoSource) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if res, ok := vs.videoSource.(resource.Resource); ok {
		return res.DoCommand(ctx, cmd)
	}
	return nil, resource.ErrDoUnimplemented
}

func (vs *videoSource) Properties(ctx context.Context) (Properties, error) {
	_, supportsPCD := vs.actualSource.(PointCloudSource)
	result := Properties{
		SupportsPCD: supportsPCD,
	}
	if vs.system == nil {
		return result, nil
	}
	if (vs.system.PinholeCameraIntrinsics != nil) && (vs.imageType == DepthStream) {
		result.SupportsPCD = true
	}
	result.ImageType = vs.imageType
	result.IntrinsicParams = vs.system.PinholeCameraIntrinsics

	if vs.system.Distortion != nil {
		result.DistortionParams = vs.system.Distortion
	}

	return result, nil
}

func (vs *videoSource) Close(ctx context.Context) error {
	return multierr.Combine(vs.videoStream.Close(ctx), vs.videoSource.Close(ctx))
}
