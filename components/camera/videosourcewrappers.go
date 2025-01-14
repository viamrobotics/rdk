package camera

import (
	"context"
	"time"

	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/components/camera/rtppassthrough"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/depthadapter"
	"go.viam.com/rdk/rimage/transform"
)

// FromVideoSource is DEPRECATED!!! Please implement cameras according to the camera.Camera interface.
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

type sourceBasedCamera struct {
	resource.Named
	resource.AlwaysRebuild
	VideoSource
	rtpPassthroughSource rtppassthrough.Source
	logging.Logger
}

func (vs *sourceBasedCamera) SubscribeRTP(
	ctx context.Context,
	bufferSize int,
	packetsCB rtppassthrough.PacketCallback,
) (rtppassthrough.Subscription, error) {
	if vs.rtpPassthroughSource != nil {
		return vs.rtpPassthroughSource.SubscribeRTP(ctx, bufferSize, packetsCB)
	}
	return rtppassthrough.NilSubscription, errors.New("SubscribeRTP unimplemented")
}

func (vs *sourceBasedCamera) Unsubscribe(ctx context.Context, id rtppassthrough.SubscriptionID) error {
	if vs.rtpPassthroughSource != nil {
		return vs.rtpPassthroughSource.Unsubscribe(ctx, id)
	}
	return errors.New("Unsubscribe unimplemented")
}

func (vs *videoSource) SubscribeRTP(
	ctx context.Context,
	bufferSize int,
	packetsCB rtppassthrough.PacketCallback,
) (rtppassthrough.Subscription, error) {
	if vs.rtpPassthroughSource != nil {
		return vs.rtpPassthroughSource.SubscribeRTP(ctx, bufferSize, packetsCB)
	}
	return rtppassthrough.NilSubscription, errors.New("SubscribeRTP unimplemented")
}

func (vs *videoSource) Unsubscribe(ctx context.Context, id rtppassthrough.SubscriptionID) error {
	if vs.rtpPassthroughSource != nil {
		return vs.rtpPassthroughSource.Unsubscribe(ctx, id)
	}
	return errors.New("Unsubscribe unimplemented")
}

// NewPinholeModelWithBrownConradyDistortion is DEPRECATED!!! Please implement cameras according to the camera.Camera interface.
// NewPinholeModelWithBrownConradyDistortion creates a transform.PinholeCameraModel from
// a *transform.PinholeCameraIntrinsics and a *transform.BrownConrady.
// If *transform.BrownConrady is `nil`, transform.PinholeCameraModel.Distortion
// is not set & remains nil, to prevent https://go.dev/doc/faq#nil_error.
func NewPinholeModelWithBrownConradyDistortion(pinholeCameraIntrinsics *transform.PinholeCameraIntrinsics,
	distortion *transform.BrownConrady,
) transform.PinholeCameraModel {
	var cameraModel transform.PinholeCameraModel
	cameraModel.PinholeCameraIntrinsics = pinholeCameraIntrinsics

	if distortion != nil {
		cameraModel.Distortion = distortion
	}
	return cameraModel
}

// NewVideoSourceFromReader is DEPRECATED!!! Please implement cameras according to the camera.Camera interface.
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
		actualSource:         reader,
		imageType:            imageType,
	}, nil
}

// WrapVideoSourceWithProjector is DEPRECATED!!! Please implement cameras according to the camera.Camera interface.
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
		actualSource: source,
		imageType:    imageType,
	}, nil
}

// videoSource implements a Camera with a gostream.VideoSource.
type videoSource struct {
	rtpPassthroughSource rtppassthrough.Source
	videoSource          gostream.VideoSource
	actualSource         interface{}
	system               *transform.PinholeCameraModel
	imageType            ImageType
}

func (vs *videoSource) Stream(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
	return vs.videoSource.Stream(ctx, errHandlers...)
}

// ReadImageBytes wraps ReadImage given a mimetype to encode the image as bytes data, returning
// supplementary metadata for downstream processing.
// TODO(hexbabe): make function private or remove altogether once the usages are limited to this file.
func ReadImageBytes(ctx context.Context, src gostream.VideoSource, mimeType string) ([]byte, ImageMetadata, error) {
	img, release, err := ReadImage(ctx, src)
	if err != nil {
		return nil, ImageMetadata{}, err
	}
	defer release()
	imgBytes, err := rimage.EncodeImage(ctx, img, mimeType)
	if err != nil {
		return nil, ImageMetadata{}, err
	}
	return imgBytes, ImageMetadata{MimeType: mimeType}, nil
}

func (vs *videoSource) Image(ctx context.Context, mimeType string, extra map[string]interface{}) ([]byte, ImageMetadata, error) {
	return ReadImageBytes(ctx, vs.videoSource, mimeType)
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
	img, release, err := ReadImage(ctx, vs.videoSource)
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
	if res, ok := vs.actualSource.(resource.Resource); ok {
		return res.Close(ctx)
	}
	return vs.videoSource.Close(ctx)
}
