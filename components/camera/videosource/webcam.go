package videosource

import (
	"context"
	"image"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/driver"
	"github.com/pion/mediadevices/pkg/driver/availability"
	mediadevicescamera "github.com/pion/mediadevices/pkg/driver/camera"
	"github.com/pion/mediadevices/pkg/frame"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pkg/errors"
	"github.com/viamrobotics/gostream"
	"go.uber.org/multierr"
	pb "go.viam.com/api/component/camera/v1"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/components/camera"
	jetsoncamera "go.viam.com/rdk/components/camera/platforms/jetson"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage/transform"
)

// ModelWebcam is the name of the webcam component.
var ModelWebcam = resource.DefaultModelFamily.WithModel("webcam")

func init() {
	resource.RegisterComponent(
		camera.API,
		ModelWebcam,
		resource.Registration[camera.Camera, *WebcamConfig]{
			Constructor: NewWebcam,
			Discover: func(ctx context.Context, logger golog.Logger) (interface{}, error) {
				return Discover(ctx, getVideoDrivers, logger)
			},
		})
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
func Discover(_ context.Context, getDrivers func() []driver.Driver, logger golog.Logger) (*pb.Webcams, error) {
	mediadevicescamera.Initialize()
	var webcams []*pb.Webcam
	drivers := getDrivers()
	for _, d := range drivers {
		driverInfo := d.Info()

		props, err := getProperties(d)
		if len(props) == 0 {
			logger.Debugw("no properties detected for driver, skipping discovery...", "driver", driverInfo.Label)
			continue
		} else if err != nil {
			logger.Debugw("cannot access driver properties, skipping discovery...", "driver", driverInfo.Label, "error", err)
			continue
		}

		if d.Status() == driver.StateRunning {
			logger.Debugw("driver is in use, skipping discovery...", "driver", driverInfo.Label)
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
				FrameRate:   prop.Video.FrameRate,
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

// WebcamConfig is the attribute struct for webcams.
type WebcamConfig struct {
	resource.TriviallyValidateConfig
	CameraParameters     *transform.PinholeCameraIntrinsics `json:"intrinsic_parameters,omitempty"`
	DistortionParameters *transform.BrownConrady            `json:"distortion_parameters,omitempty"`
	Debug                bool                               `json:"debug,omitempty"`
	Format               string                             `json:"format,omitempty"`
	Path                 string                             `json:"video_path,omitempty"`
	Width                int                                `json:"width_px,omitempty"`
	Height               int                                `json:"height_px,omitempty"`
	FrameRate            float32                            `json:"frame_rate,omitempty"`
}

func (c WebcamConfig) needsDriverReinit(other WebcamConfig) bool {
	return !(c.Format == other.Format &&
		c.Path == other.Path &&
		c.Width == other.Width &&
		c.Height == other.Height)
}

func makeConstraints(conf *WebcamConfig, debug bool, logger golog.Logger) mediadevices.MediaStreamConstraints {
	return mediadevices.MediaStreamConstraints{
		Video: func(constraint *mediadevices.MediaTrackConstraints) {
			if conf.Width > 0 {
				constraint.Width = prop.IntExact(conf.Width)
			} else {
				constraint.Width = prop.IntRanged{Min: 0, Ideal: 640, Max: 4096}
			}

			if conf.Height > 0 {
				constraint.Height = prop.IntExact(conf.Height)
			} else {
				constraint.Height = prop.IntRanged{Min: 0, Ideal: 480, Max: 2160}
			}

			if conf.FrameRate > 0.0 {
				constraint.FrameRate = prop.FloatExact(conf.FrameRate)
			} else {
				constraint.FrameRate = prop.FloatRanged{Min: 0.0, Ideal: 30.0, Max: 140.0}
			}

			if conf.Format == "" {
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
				constraint.FrameFormat = prop.FrameFormatExact(conf.Format)
			}

			if debug {
				logger.Debugf("constraints: %v", constraint)
			}
		},
	}
}

// findAndMakeVideoSource finds a video device and returns a video source with that video device as the source.
func findAndMakeVideoSource(
	ctx context.Context,
	conf *WebcamConfig,
	label string,
	logger golog.Logger,
) (gostream.VideoSource, string, error) {
	mediadevicescamera.Initialize()
	debug := conf.Debug
	constraints := makeConstraints(conf, debug, logger)
	if label != "" {
		cam, err := tryWebcamOpen(ctx, conf, label, false, constraints, logger)
		if err != nil {
			return nil, "", errors.Wrap(err, "cannot open webcam")
		}
		return cam, label, nil
	}

	source, err := gostream.GetAnyVideoSource(constraints, logger)
	if err != nil {
		return nil, "", errors.Wrap(err, "found no webcams")
	}

	if label == "" {
		label = getLabelFromVideoSource(source, logger)
	}

	return source, label, nil
}

// getLabelFromVideoSource returns the path from the camera or an empty string if a path is not found.
func getLabelFromVideoSource(src gostream.VideoSource, logger golog.Logger) string {
	labels, err := gostream.LabelsFromMediaSource[image.Image, prop.Video](src)
	if err != nil || len(labels) == 0 {
		logger.Errorw("could not get labels from media source", "error", err)
		return ""
	}

	return labels[0]
}

// NewWebcam returns a new source based on a webcam discovered from the given config.
func NewWebcam(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger golog.Logger,
) (camera.Camera, error) {
	cancelCtx, cancel := context.WithCancel(context.Background())
	cam := &monitoredWebcam{
		Named:          conf.ResourceName().AsNamed(),
		logger:         logger.With("camera_name", conf.ResourceName().ShortName()),
		originalLogger: logger,
		cancelCtx:      cancelCtx,
		cancel:         cancel,
	}
	if err := cam.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	cam.Monitor()
	return cam, nil
}

type noopCloser struct {
	gostream.VideoSource
}

func (n *noopCloser) Close(ctx context.Context) error {
	return nil
}

func (c *monitoredWebcam) Reconfigure(
	ctx context.Context,
	_ resource.Dependencies,
	conf resource.Config,
) error {
	newConf, err := resource.NativeConfig[*WebcamConfig](conf)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	cameraModel := camera.NewPinholeModelWithBrownConradyDistortion(newConf.CameraParameters, newConf.DistortionParameters)
	projector, err := camera.WrapVideoSourceWithProjector(
		ctx,
		&noopCloser{c},
		&cameraModel,
		camera.ColorStream,
	)
	if err != nil {
		return err
	}

	needDriverReinit := c.conf.needsDriverReinit(*newConf)
	if c.exposedProjector != nil {
		goutils.UncheckedError(c.exposedProjector.Close(ctx))
	}
	c.exposedProjector = projector

	if c.underlyingSource != nil && !needDriverReinit {
		c.conf = *newConf
		return nil
	}
	c.logger.Debug("reinitializing driver")

	c.targetPath = newConf.Path
	if err := c.reconnectCamera(newConf); err != nil {
		return err
	}

	// only set once we're good
	c.conf = *newConf
	return nil
}

// tryWebcamOpen uses getNamedVideoSource to try and find a video device (gostream.MediaSource).
// If successful, it will wrap that MediaSource in a camera.
func tryWebcamOpen(
	ctx context.Context,
	conf *WebcamConfig,
	path string,
	fromLabel bool,
	constraints mediadevices.MediaStreamConstraints,
	logger golog.Logger,
) (gostream.VideoSource, error) {
	source, err := getNamedVideoSource(path, fromLabel, constraints, logger)
	if err != nil {
		return nil, err
	}

	if conf.Width != 0 && conf.Height != 0 {
		img, release, err := gostream.ReadMedia(ctx, source)
		if release != nil {
			defer release()
		}
		if err != nil {
			return nil, err
		}
		if img.Bounds().Dx() != conf.Width || img.Bounds().Dy() != conf.Height {
			return nil, errors.Errorf("requested width and height (%dx%d) are not available for this webcam"+
				" (closest driver found by gostream supports resolution %dx%d)",
				conf.Width, conf.Height, img.Bounds().Dx(), img.Bounds().Dy())
		}
	}
	return source, nil
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

// monitoredWebcam tries to ensure its underlying camera stays connected.
type monitoredWebcam struct {
	resource.Named
	mu sync.RWMutex

	underlyingSource gostream.VideoSource
	exposedSwapper   gostream.HotSwappableVideoSource
	exposedProjector camera.VideoSource

	// this is returned to us as a label in mediadevices but our config
	// treats it as a video path.
	targetPath string
	conf       WebcamConfig

	cancelCtx               context.Context
	cancel                  func()
	closed                  bool
	disconnected            bool
	activeBackgroundWorkers sync.WaitGroup
	logger                  golog.Logger
	originalLogger          golog.Logger
}

func (c *monitoredWebcam) MediaProperties(ctx context.Context) (prop.Video, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.underlyingSource == nil {
		return prop.Video{}, errors.New("no configured camera")
	}
	if provider, ok := c.underlyingSource.(gostream.VideoPropertyProvider); ok {
		return provider.MediaProperties(ctx)
	}
	return prop.Video{}, nil
}

func (c *monitoredWebcam) isCameraConnected() (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.underlyingSource == nil {
		return true, errors.New("no configured camera")
	}
	d, err := gostream.DriverFromMediaSource[image.Image, prop.Video](c.underlyingSource)
	if err != nil {
		return true, errors.Wrap(err, "cannot get driver from media source")
	}

	// TODO(RSDK-1959): this only works for linux
	_, err = driver.IsAvailable(d)
	return !errors.Is(err, availability.ErrNoDevice), nil
}

// reconnectCamera assumes a write lock is held.
func (c *monitoredWebcam) reconnectCamera(conf *WebcamConfig) error {
	if c.underlyingSource != nil {
		c.logger.Debug("closing current camera")
		if err := c.underlyingSource.Close(c.cancelCtx); err != nil {
			c.logger.Errorw("failed to close currents camera", "error", err)
		}
		c.underlyingSource = nil
	}

	newSrc, foundLabel, err := findAndMakeVideoSource(c.cancelCtx, conf, c.targetPath, c.logger)
	if err != nil {
		// If we are on a Jetson Orin AGX, we need to validate hardware/software setup.
		// If not, simply pass through the error.
		err = jetsoncamera.ValidateSetup(
			jetsoncamera.OrinAGX,
			jetsoncamera.ECAM,
			jetsoncamera.AR0234,
			err,
		)
		return errors.Wrap(err, "failed to find camera")
	}

	if c.exposedSwapper == nil {
		c.exposedSwapper = gostream.NewHotSwappableVideoSource(newSrc)
	} else {
		c.exposedSwapper.Swap(newSrc)
	}
	c.underlyingSource = newSrc
	c.disconnected = false
	c.closed = false
	if c.targetPath == "" {
		c.targetPath = foundLabel
	}
	c.logger = c.originalLogger.With("camera_label", c.targetPath)

	return nil
}

// Monitor is responsible for monitoring the liveness of a camera. An example
// is connectivity. Since the model itself knows best about how to maintain this state,
// the reconfigurable offers a safe way to notify if a state needs to be reset due
// to some exceptional event (like a reconnect).
// It is expected that the monitoring code is tied to the lifetime of the resource
// and once the resource is closed, so should the monitor. That is, it should
// no longer send any resets once a Close on its associated resource has returned.
func (c *monitoredWebcam) Monitor() {
	const wait = 500 * time.Millisecond
	c.activeBackgroundWorkers.Add(1)

	goutils.ManagedGo(func() {
		for {
			if !goutils.SelectContextOrWait(c.cancelCtx, wait) {
				return
			}

			c.mu.RLock()
			logger := c.logger
			c.mu.RUnlock()

			ok, err := c.isCameraConnected()
			if err != nil {
				logger.Debugw("cannot determine camera status", "error", err)
				continue
			}

			if !ok {
				c.mu.Lock()
				c.disconnected = true
				c.mu.Unlock()

				logger.Error("camera no longer connected; reconnecting")
				for {
					if !goutils.SelectContextOrWait(c.cancelCtx, wait) {
						return
					}
					cont := func() bool {
						c.mu.Lock()
						defer c.mu.Unlock()

						if err := c.reconnectCamera(&c.conf); err != nil {
							c.logger.Errorw("failed to reconnect camera", "error", err)
							return true
						}
						c.logger.Infow("camera reconnected")
						return false
					}()
					if cont {
						continue
					}
					break
				}
			}
		}
	}, c.activeBackgroundWorkers.Done)
}

func (c *monitoredWebcam) Projector(ctx context.Context) (transform.Projector, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if err := c.ensureActive(); err != nil {
		return nil, err
	}
	return c.exposedProjector.Projector(ctx)
}

func (c *monitoredWebcam) Stream(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if err := c.ensureActive(); err != nil {
		return nil, err
	}
	return c.exposedSwapper.Stream(ctx, errHandlers...)
}

func (c *monitoredWebcam) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if err := c.ensureActive(); err != nil {
		return nil, err
	}
	return c.exposedProjector.NextPointCloud(ctx)
}

func (c *monitoredWebcam) Properties(ctx context.Context) (camera.Properties, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if err := c.ensureActive(); err != nil {
		return camera.Properties{}, err
	}
	return c.exposedProjector.Properties(ctx)
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

	var err error
	if c.exposedSwapper != nil {
		err = multierr.Combine(err, c.exposedSwapper.Close(ctx))
	}
	if c.exposedProjector != nil {
		err = multierr.Combine(err, c.exposedProjector.Close(ctx))
	}
	if c.underlyingSource != nil {
		err = multierr.Combine(err, c.underlyingSource.Close(ctx))
	}
	return err
}
