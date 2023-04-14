package videosource

import (
	"context"
	"image"
	"path/filepath"
	"strings"
	"sync"
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
	"go.viam.com/rdk/pointcloud"
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
			return NewWebcamSource(ctx, config.Name, attrs, logger)
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
func Discover(_ context.Context, getDrivers func() []driver.Driver) (*pb.Webcams, error) {
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
		label := labelParts[0]

		name, id := func() (string, string) {
			nameParts := strings.Split(driverInfo.Name, mediadevicescamera.LabelSeparator)
			if len(nameParts) > 1 {
				return nameParts[0], nameParts[1]
			}
			// fallback to the label if the name does not have an any additional parts to use.
			return nameParts[0], label
		}()

		wc := &pb.Webcam{
			Name:       name,
			Id:         id,
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
	return mediadevices.MediaStreamConstraints{
		Video: func(constraint *mediadevices.MediaTrackConstraints) {
			if attrs.Width > 0 {
				constraint.Width = prop.IntExact(attrs.Width)
			} else {
				constraint.Width = prop.IntRanged{Min: 0, Ideal: 640, Max: 4096}
			}

			if attrs.Height > 0 {
				constraint.Height = prop.IntExact(attrs.Height)
			} else {
				constraint.Height = prop.IntRanged{Min: 0, Ideal: 480, Max: 2160}
			}
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
					frame.FormatZ16,
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

// findAndMakeCamera finds a video device and returns a camera with that video device as the source.
func findAndMakeCamera(
	ctx context.Context,
	attrs *WebcamAttrs,
	label string,
	logger golog.Logger,
) (camera.Camera, error) {
	debug := attrs.Debug
	constraints := makeConstraints(attrs, debug, logger)
	if label != "" {
		cam, err := tryWebcamOpen(ctx, attrs, label, false, constraints, logger)
		if err != nil {
			return nil, errors.Wrap(err, "cannot open webcam")
		}
		return cam, nil
	}

	source, err := gostream.GetAnyVideoSource(constraints, logger)
	if err != nil {
		return nil, errors.Wrap(err, "found no webcams")
	}
	cam, err := makeCameraFromSource(ctx, source, attrs)
	if err != nil {
		return nil, errors.Wrap(err, "cannot make webcam from source")
	}
	return cam, nil
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

// NewWebcamSource returns a new source based on a webcam discovered from the given attributes.
func NewWebcamSource(ctx context.Context, name string, attrs *WebcamAttrs, logger golog.Logger) (camera.Camera, error) {
	cam, err := findAndMakeCamera(ctx, attrs, attrs.Path, logger)
	if err != nil {
		return nil, errors.Wrap(err, "cannot find video source for camera")
	}

	logger = logger.With("camera_name", name)
	label := attrs.Path
	if label == "" {
		label = getLabelFromCamera(cam, logger)
		logger = logger.With("camera_label", label)
	}

	cancelCtx, cancel := context.WithCancel(context.Background())
	return &monitoredWebcam{
		cam:       cam,
		label:     label,
		attrs:     attrs,
		cancelCtx: cancelCtx,
		cancel:    cancel,
		logger:    logger,
	}, nil
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

var _ = camera.LivenessMonitor(&monitoredWebcam{})

// monitoredWebcam tries to ensure its underlying camera stays connected.
type monitoredWebcam struct {
	mu    sync.RWMutex
	cam   camera.Camera
	label string
	attrs *WebcamAttrs

	cancelCtx               context.Context
	cancel                  func()
	closed                  bool
	disconnected            bool
	activeBackgroundWorkers sync.WaitGroup
	logger                  golog.Logger
}

func (c *monitoredWebcam) isCameraConnected() (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	src, err := camera.SourceFromCamera(c.cam)
	if err != nil {
		return true, errors.Wrap(err, "cannot get source from camera")
	}
	props, err := gostream.PropertiesFromMediaSource[image.Image, prop.Video](src)
	if err != nil {
		return true, errors.Wrap(err, "cannot get properties from media source")
	}
	// github.com/pion/mediadevices connects to the OS to get the props for a driver. On disconnect props will be empty.
	// TODO(RSDK-1959): this only works for linux
	return len(props) != 0, nil
}

func (c *monitoredWebcam) reconnectCamera(notifyReset func()) error {
	c.logger.Debug("closing disconnected camera")
	if err := c.cam.Close(c.cancelCtx); err != nil {
		c.logger.Errorw("failed to close disconnected camera", "error", err)
	}

	newCam, err := findAndMakeCamera(c.cancelCtx, c.attrs, c.label, c.logger)
	if err != nil {
		return errors.Wrap(err, "failed to find camera")
	}

	c.mu.Lock()
	c.cam = newCam
	c.disconnected = false
	c.closed = false
	c.mu.Unlock()

	notifyReset()
	return nil
}

func (c *monitoredWebcam) Monitor(notifyReset func()) {
	const wait = 500 * time.Millisecond
	c.activeBackgroundWorkers.Add(1)

	goutils.ManagedGo(func() {
		for {
			if !goutils.SelectContextOrWait(c.cancelCtx, wait) {
				return
			}

			ok, err := c.isCameraConnected()
			if err != nil {
				c.logger.Debugw("cannot determine camera status", "error", err)
				continue
			}

			if !ok {
				c.mu.Lock()
				c.disconnected = true
				c.mu.Unlock()

				c.logger.Error("camera no longer connected; reconnecting")
				for {
					if !goutils.SelectContextOrWait(c.cancelCtx, wait) {
						return
					}
					if err := c.reconnectCamera(notifyReset); err != nil {
						c.logger.Errorw("failed to reconnect camera", "error", err)
						continue
					}
					c.logger.Infow("camera reconnected")
					break
				}
			}
		}
	}, c.activeBackgroundWorkers.Done)
}

func (c *monitoredWebcam) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if err := c.ensureActive(); err != nil {
		return nil, err
	}
	return c.cam.DoCommand(ctx, cmd)
}

func (c *monitoredWebcam) Projector(ctx context.Context) (transform.Projector, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if err := c.ensureActive(); err != nil {
		return nil, err
	}
	return c.cam.Projector(ctx)
}

func (c *monitoredWebcam) Stream(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if err := c.ensureActive(); err != nil {
		return nil, err
	}
	return c.cam.Stream(ctx, errHandlers...)
}

func (c *monitoredWebcam) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if err := c.ensureActive(); err != nil {
		return nil, err
	}
	return c.cam.NextPointCloud(ctx)
}

func (c *monitoredWebcam) Properties(ctx context.Context) (camera.Properties, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if err := c.ensureActive(); err != nil {
		return camera.Properties{}, err
	}
	return c.cam.Properties(ctx)
}

var (
	errClosed       = errors.New("camera has been closed")
	errDisconnected = errors.New("camera is disconnected; please try again in a few moments")
)

func (c *monitoredWebcam) ensureActive() error {
	if c.closed {
		return errClosed
	}
	if c.disconnected {
		return errDisconnected
	}
	return nil
}

func (c *monitoredWebcam) Close(ctx context.Context) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return errors.New("webcam already closed")
	}
	c.closed = true
	c.mu.Unlock()
	c.cancel()
	c.activeBackgroundWorkers.Wait()
	return c.cam.Close(ctx)
}

func (c *monitoredWebcam) ProxyFor() interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cam
}
