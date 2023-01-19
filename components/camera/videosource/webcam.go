package videosource

import (
	"context"
	"image"
	"path/filepath"
	"strings"
	"time"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/driver"
	mediadevicescamera "github.com/pion/mediadevices/pkg/driver/camera"
	"github.com/pion/mediadevices/pkg/frame"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pkg/errors"
	pb "go.viam.com/api/component/camera/v1"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/discovery"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
)

var model = resource.NewDefaultModel("webcam")

func init() {
	registry.RegisterComponent(
		camera.Subtype,
		model,
		registry.Component{Constructor: func(
			ctx context.Context,
			_ registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*WebcamAttrs)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
			}
			return NewWebcamSource(ctx, attrs, logger)
		}})

	config.RegisterComponentAttributeMapConverter(camera.Subtype, model,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf WebcamAttrs
			attrs, err := config.TransformAttributeMapToStruct(&conf, attributes)
			if err != nil {
				return nil, err
			}
			result, ok := attrs.(*WebcamAttrs)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(result, attrs)
			}
			return result, nil
		}, &WebcamAttrs{})

	registry.RegisterDiscoveryFunction(
		discovery.NewQuery(camera.Subtype, model),
		func(ctx context.Context) (interface{}, error) { return Discover(ctx, getVideoDrivers) },
	)
}

func getVideoDrivers() []driver.Driver {
	return driver.GetManager().Query(driver.FilterVideoRecorder())
}

// CameraConfig is collection of configuration options for a camera.
type CameraConfig struct {
	Label      string
	Status     driver.State
	Properties []prop.Media
}

// Discover webcam attributes.
func Discover(ctx context.Context, getDrivers func() []driver.Driver) (*pb.Webcams, error) {
	var webcams []*pb.Webcam
	drivers := getDrivers()
	for _, d := range drivers {
		driverInfo := d.Info()

		props, err := getProperties(d)
		if len(props) == 0 {
			golog.Global().Debugw("no properties detected for driver, skipping discovery...", "driver", driverInfo.Label)
			continue
		} else if err != nil {
			golog.Global().Debugw("cannot access driver properties, skipping discovery...", "driver", driverInfo.Label, "error", err)
			continue
		}

		if d.Status() == driver.StateRunning {
			golog.Global().Debugw("driver is in use, skipping discovery...", "driver", driverInfo.Label)
			continue
		}

		labelParts := strings.Split(driverInfo.Label, mediadevicescamera.LabelSeparator)
		label := labelParts[len(labelParts)-1]
		wc := &pb.Webcam{
			Label:      label,
			Status:     string(d.Status()),
			Properties: make([]*pb.Property, 0, len(d.Properties())),
		}

		for _, prop := range props {
			pbProp := &pb.Property{
				WidthPx:     int32(prop.Video.Width),
				HeightPx:    int32(prop.Video.Height),
				FrameFormat: string(prop.Video.FrameFormat),
			}
			wc.Properties = append(wc.Properties, pbProp)
		}
		webcams = append(webcams, wc)
	}
	return &pb.Webcams{Webcams: webcams}, nil
}

func getProperties(d driver.Driver) (_ []prop.Media, err error) {
	// Need to open driver to get properties
	if d.Status() == driver.StateClosed {
		errOpen := d.Open()
		if errOpen != nil {
			return nil, errOpen
		}
		defer func() {
			if errClose := d.Close(); errClose != nil {
				err = errClose
			}
		}()
	}
	return d.Properties(), err
}

// WebcamAttrs is the attribute struct for webcams.
type WebcamAttrs struct {
	CameraParameters     *transform.PinholeCameraIntrinsics `json:"intrinsic_parameters,omitempty"`
	DistortionParameters *transform.BrownConrady            `json:"distortion_parameters,omitempty"`
	Debug                bool                               `json:"debug,omitempty"`
	Format               string                             `json:"format,omitempty"`
	Path                 string                             `json:"video_path,omitempty"`
	Width                int                                `json:"width_px,omitempty"`
	Height               int                                `json:"height_px,omitempty"`
}

func makeConstraints(attrs *WebcamAttrs, debug bool, logger golog.Logger) mediadevices.MediaStreamConstraints {
	minWidth := 0
	maxWidth := 4096
	idealWidth := 800
	minHeight := 0
	maxHeight := 2160
	idealHeight := 600

	if attrs.Width > 0 {
		minWidth = 0
		maxWidth = attrs.Width
		idealWidth = attrs.Width
	}

	if attrs.Height > 0 {
		minHeight = 0
		maxHeight = attrs.Height
		idealHeight = attrs.Height
	}

	return mediadevices.MediaStreamConstraints{
		Video: func(constraint *mediadevices.MediaTrackConstraints) {
			constraint.Width = prop.IntRanged{minWidth, maxWidth, idealWidth}
			constraint.Height = prop.IntRanged{minHeight, maxHeight, idealHeight}
			constraint.FrameRate = prop.FloatRanged{0, 200, 60}

			if attrs.Format == "" {
				constraint.FrameFormat = prop.FrameFormatOneOf{
					frame.FormatI420,
					frame.FormatI444,
					frame.FormatYUY2,
					frame.FormatUYVY,
					frame.FormatRGBA,
					frame.FormatMJPEG,
					frame.FormatNV12,
					frame.FormatNV21,
				}
			} else {
				constraint.FrameFormat = prop.FrameFormatExact(attrs.Format)
			}

			if debug {
				logger.Debugf("constraints: %v", constraint)
			}
		},
	}
}

// findCamera finds a video device and returns a reconfigurable camera with that video device as the source.
func findCamera(
	ctx context.Context,
	attrs *WebcamAttrs,
	label string,
	logger golog.Logger,
) (resource.Reconfigurable, error) {
	var cam camera.Camera
	var err error

	debug := attrs.Debug
	constraints := makeConstraints(attrs, debug, logger)
	if label != "" {
		cam, err = tryWebcamOpen(ctx, attrs, label, false, constraints, logger)
		if err != nil {
			return nil, errors.Wrap(err, "cannot open webcam")
		}
	} else {
		source, err := gostream.GetAnyVideoSource(constraints, logger)
		if err != nil {
			return nil, errors.Wrap(err, "found no webcams")
		}
		cam, err = makeCameraFromSource(ctx, source, attrs)
		if err != nil {
			return nil, errors.Wrap(err, "cannot make webcam from source")
		}
	}

	return camera.WrapWithReconfigurable(cam, camera.Named(model.String()))
}

// getLabelFromCameraOrPath returns the path from the camera or an empty string if a path is not found.
func getLabelFromCamera(cam camera.Camera, logger golog.Logger) string {
	src, err := camera.SourceFromCamera(cam)
	if err != nil {
		logger.Errorw("cannot get source from camera", "error", err)
		return ""
	}

	labels, err := gostream.LabelsFromMediaSource[image.Image, prop.Video](src)
	if err != nil || len(labels) == 0 {
		logger.Errorw("could not get labels from media source", "error", err)
		return ""
	}

	return labels[0]
}

// isConnected returns true if the reconfigurable camera is connected, otherwise false.
func isConnected(reconfCam resource.Reconfigurable) (bool, error) {
	actualCam := utils.UnwrapProxy(reconfCam).(camera.Camera)
	src, err := camera.SourceFromCamera(actualCam)
	if err != nil {
		return true, errors.Wrap(err, "cannot get source from camera")
	}
	props, err := gostream.PropertiesFromMediaSource[image.Image, prop.Video](src)
	if err != nil {
		return true, errors.Wrap(err, "cannot get properties from media source")
	}
	// github.com/pion/mediadevices connects to the OS to get the props for a driver. On disconnect props will be empty.
	return len(props) != 0, nil
}

// reconfigureCamera creates a new camera and reconfigures the given reconfigurable camera.
func reconfigureCamera(
	ctx context.Context,
	oldCam resource.Reconfigurable,
	attrs *WebcamAttrs,
	label string,
	logger golog.Logger,
) (err error) {
	goutils.UncheckedError(goutils.TryClose(ctx, oldCam))
	logger.Debugw("camera disconnected", "label", label)

	newCam, err := findCamera(ctx, attrs, label, logger)
	defer func() {
		if err != nil {
			goutils.UncheckedError(goutils.TryClose(ctx, newCam))
		}
	}()
	if err != nil {
		return errors.Wrap(err, "cannot make camera")
	}

	if err = oldCam.Reconfigure(ctx, newCam); err != nil {
		return errors.Wrap(err, "cannot reconfigure camera")
	}
	return
}

// NewWebcamSource returns a new source based on a webcam discovered from the given attributes.
func NewWebcamSource(ctx context.Context, attrs *WebcamAttrs, logger golog.Logger) (camera.Camera, error) {
	cam, err := findCamera(ctx, attrs, attrs.Path, logger)
	if err != nil {
		return nil, errors.Wrap(err, "cannot find video source for camera")
	}

	label := attrs.Path
	if label == "" {
		label = getLabelFromCamera(utils.UnwrapProxy(cam).(camera.Camera), logger)
	}

	const wait = 500 * time.Millisecond
	camWg := CameraWaitGroup{cam: cam.(camera.Camera)}
	camWg.activeBackgroundWorkers.Add(1)
	goutils.ManagedGo(func() {
		defer goutils.UncheckedError(goutils.TryClose(ctx, cam))
		for {
			if !goutils.SelectContextOrWait(ctx, wait) {
				return
			}

			ok, err := isConnected(cam)
			if err != nil {
				logger.Debugw("cannot determine camera status", "error", err)
				continue
			}

			if !ok {
				for {
					if !goutils.SelectContextOrWait(ctx, wait) {
						return
					}
					if err := reconfigureCamera(ctx, cam, attrs, label, logger); err != nil {
						logger.Debugw("cannot reconfigure camera", "label", label, "error", err)
						continue
					}
					logger.Debugw("camera connected", "label", label)
					break
				}
			}
		}
	}, camWg.activeBackgroundWorkers.Done)

	return &camWg, nil
}

// tryWebcamOpen uses getNamedVideoSource to try and find a video device (gostream.MediaSource).
// If successful, it will wrap that MediaSource in a camera.
func tryWebcamOpen(ctx context.Context,
	attrs *WebcamAttrs,
	path string,
	fromLabel bool,
	constraints mediadevices.MediaStreamConstraints,
	logger golog.Logger,
) (camera.Camera, error) {
	source, err := getNamedVideoSource(path, fromLabel, constraints, logger)
	if err != nil {
		return nil, err
	}

	if attrs.Width != 0 && attrs.Height != 0 {
		img, release, err := gostream.ReadMedia(ctx, source)
		if release != nil {
			defer release()
		}
		if err != nil {
			return nil, err
		}
		if img.Bounds().Dx() != attrs.Width || img.Bounds().Dy() != attrs.Height {
			return nil, errors.Errorf("requested width and height (%dx%d) are not available for this webcam"+
				" (closest driver found by gostream supports resolution %dx%d)",
				attrs.Width, attrs.Height, img.Bounds().Dx(), img.Bounds().Dy())
		}
	}
	return makeCameraFromSource(ctx, source, attrs)
}

// makeCameraFromSource takes a gostream.MediaSource and wraps it so that the return
// is an RDK camera object.
func makeCameraFromSource(ctx context.Context,
	source gostream.MediaSource[image.Image],
	attrs *WebcamAttrs,
) (camera.Camera, error) {
	if source == nil {
		return nil, errors.New("media source not found")
	}
	cameraModel := camera.NewPinholeModelWithBrownConradyDistortion(attrs.CameraParameters, attrs.DistortionParameters)
	return camera.NewFromSource(
		ctx,
		source,
		&cameraModel,
		camera.ColorStream,
	)
}

// getNamedVideoSource attempts to find a video device (not a screen) by the given name.
// First it will try to use the path name after evaluating any symbolic links. If
// evaluation fails, it will try to use the path name as provided.
func getNamedVideoSource(
	path string,
	fromLabel bool,
	constraints mediadevices.MediaStreamConstraints,
	logger golog.Logger,
) (gostream.MediaSource[image.Image], error) {
	if !fromLabel {
		resolvedPath, err := filepath.EvalSymlinks(path)
		if err == nil {
			path = resolvedPath
		}
	}
	return gostream.GetNamedVideoSource(filepath.Base(path), constraints, logger)
}
