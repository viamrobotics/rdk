// Package videosource implements various camera models including webcam
package videosource

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"image"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/driver"
	"github.com/pion/mediadevices/pkg/driver/availability"
	mediadevicescamera "github.com/pion/mediadevices/pkg/driver/camera"
	"github.com/pion/mediadevices/pkg/frame"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	pb "go.viam.com/api/component/camera/v1"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage/transform"
)

// ModelWebcam is the name of the webcam component.
var ModelWebcam = resource.DefaultModelFamily.WithModel("webcam")

//go:embed data/intrinsics.json
var intrinsics []byte

var data map[string]transform.PinholeCameraIntrinsics

// getVideoDrivers is a helper callback passed to the registered Discover func to get all video drivers.
func getVideoDrivers() []driver.Driver {
	return driver.GetManager().Query(driver.FilterVideoRecorder())
}

func init() {
	resource.RegisterComponent(
		camera.API,
		ModelWebcam,
		resource.Registration[camera.Camera, *WebcamConfig]{
			Constructor: NewWebcam,
			Discover: func(ctx context.Context, logger logging.Logger, extra map[string]interface{}) (interface{}, error) {
				return Discover(ctx, getVideoDrivers, logger)
			},
		})
	if err := json.Unmarshal(intrinsics, &data); err != nil {
		logging.Global().Errorw("cannot parse intrinsics json", "error", err)
	}
}

// getProperties is a helper func for webcam discovery that returns the Media properties of a specific driver.
// It is NOT related to the GetProperties camera proto API.
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

// Discover webcam attributes.
func Discover(ctx context.Context, getDrivers func() []driver.Driver, logger logging.Logger) (*pb.Webcams, error) {
	mediadevicescamera.Initialize()
	var webcams []*pb.Webcam
	drivers := getDrivers()
	for _, d := range drivers {
		driverInfo := d.Info()

		props, err := getProperties(d)
		if len(props) == 0 {
			logger.CDebugw(ctx, "no properties detected for driver, skipping discovery...", "driver", driverInfo.Label)
			continue
		} else if err != nil {
			logger.CDebugw(ctx, "cannot access driver properties, skipping discovery...", "driver", driverInfo.Label, "error", err)
			continue
		}

		if d.Status() == driver.StateRunning {
			logger.CDebugw(ctx, "driver is in use, skipping discovery...", "driver", driverInfo.Label)
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

// WebcamConfig is the native config attribute struct for webcams.
type WebcamConfig struct {
	CameraParameters     *transform.PinholeCameraIntrinsics `json:"intrinsic_parameters,omitempty"`
	DistortionParameters *transform.BrownConrady            `json:"distortion_parameters,omitempty"`
	Format               string                             `json:"format,omitempty"`
	Path                 string                             `json:"video_path"`
	Width                int                                `json:"width_px,omitempty"`
	Height               int                                `json:"height_px,omitempty"`
	FrameRate            float32                            `json:"frame_rate,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (c WebcamConfig) Validate(path string) ([]string, error) {
	if c.Width < 0 || c.Height < 0 {
		return nil, fmt.Errorf(
			"got illegal negative dimensions for width_px and height_px (%d, %d) fields set for webcam camera",
			c.Height, c.Width)
	}
	if c.FrameRate < 0 {
		return nil, fmt.Errorf(
			"got illegal non-positive dimension for frame rate (%.2f) field set for webcam camera",
			c.FrameRate)
	}

	return []string{}, nil
}

// needsDriverReinit is a helper to check for config diffs and returns true iff the driver needs to be reinitialized.
func (c WebcamConfig) needsDriverReinit(other WebcamConfig) bool {
	return !(c.Format == other.Format &&
		c.Path == other.Path &&
		c.Width == other.Width &&
		c.Height == other.Height)
}

// makeConstraints is a helper that returns constraints to mediadevices in order to find and make a video source.
// Constraints are specifications for the video stream such as frame format, resolution etc.
func makeConstraints(conf *WebcamConfig, logger logging.Logger) mediadevices.MediaStreamConstraints {
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

			logger.Debugf("constraints: %v", constraint)
		},
	}
}

// getNamedVideoSource is a helper function for trying to open a webcam that attempts to
// find a video device (not a screen) by the given name.
//
// First it will try to use the path name after evaluating any symbolic links. If
// evaluation fails, it will try to use the path name as provided.
func getNamedVideoSource(
	path string,
	fromLabel bool,
	constraints mediadevices.MediaStreamConstraints,
	logger logging.Logger,
) (gostream.MediaSource[image.Image], error) {
	if !fromLabel {
		resolvedPath, err := filepath.EvalSymlinks(path)
		if err == nil {
			path = resolvedPath
		}
	}
	return gostream.GetNamedVideoSource(filepath.Base(path), constraints, logger)
}

// tryWebcamOpen is a helper for finding and making the video source that uses getNamedVideoSource to try and find
// a video device (gostream.MediaSource). If successful, it will wrap that MediaSource in a VideoSource.
func tryWebcamOpen(
	ctx context.Context,
	conf *WebcamConfig,
	path string,
	fromLabel bool,
	constraints mediadevices.MediaStreamConstraints,
	logger logging.Logger,
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

// getPathFromVideoSource is a helper function for finding and making the video source that
// returns the path derived from the underlying driver via MediaSource or an empty string if a path is not found.
func getPathFromVideoSource(src gostream.VideoSource, logger logging.Logger) string {
	labels, err := gostream.LabelsFromMediaSource[image.Image, prop.Video](src)
	if err != nil || len(labels) == 0 {
		logger.Errorw("could not get labels from media source", "error", err)
		return ""
	}

	return labels[0] // path is always the first element
}

// findAndMakeVideoSource finds a video device and returns a video source with that video device as the source.
func findAndMakeVideoSource(
	ctx context.Context,
	conf *WebcamConfig,
	path string,
	logger logging.Logger,
) (gostream.VideoSource, string, error) {
	mediadevicescamera.Initialize()
	constraints := makeConstraints(conf, logger)
	if path != "" {
		cam, err := tryWebcamOpen(ctx, conf, path, false, constraints, logger)
		if err != nil {
			return nil, "", errors.Wrap(err, "cannot open webcam")
		}
		return cam, path, nil
	}

	source, err := gostream.GetAnyVideoSource(constraints, logger)
	if err != nil {
		return nil, "", errors.Wrap(err, "found no webcams")
	}

	if path == "" {
		path = getPathFromVideoSource(source, logger)
	}

	return source, path, nil
}

// webcam is a video driver wrapper camera that ensures its underlying source stays connected.
type webcam struct {
	resource.Named
	mu                      sync.RWMutex
	hasLoggedIntrinsicsInfo bool

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
	logger                  logging.Logger
}

// NewWebcam returns the webcam discovered based on the given config as the Camera interface type.
func NewWebcam(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (camera.Camera, error) {
	cancelCtx, cancel := context.WithCancel(context.Background())

	cam := &webcam{
		Named:     conf.ResourceName().AsNamed(),
		logger:    logger.WithFields("camera_name", conf.ResourceName().ShortName()),
		cancelCtx: cancelCtx,
		cancel:    cancel,
	}
	if err := cam.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	cam.Monitor()

	return cam, nil
}

// noopCloser overwrites the actual close method so that the real close method isn't called on Reconfigure.
// TODO(hexbabe): https://viam.atlassian.net/browse/RSDK-9264
type noopCloser struct {
	gostream.VideoSource
}

func (n *noopCloser) Close(ctx context.Context) error {
	return nil
}

func (c *webcam) Reconfigure(
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
	c.logger.CDebug(ctx, "reinitializing driver")

	c.targetPath = newConf.Path
	if err := c.reconnectCamera(newConf); err != nil {
		return err
	}

	c.hasLoggedIntrinsicsInfo = false

	// only set once we're good
	c.conf = *newConf
	return nil
}

// MediaProperties returns the mediadevices Video properties of the underlying camera.
// It fulfills the MediaPropertyProvider interface.
func (c *webcam) MediaProperties(ctx context.Context) (prop.Video, error) {
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

// isCameraConnected is a helper for the alive-ness monitoring.
func (c *webcam) isCameraConnected() (bool, error) {
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

// reconnectCamera tries to reconnect the camera to a driver that matches the config.
// Assumes a write lock is held.
func (c *webcam) reconnectCamera(conf *WebcamConfig) error {
	if c.underlyingSource != nil {
		c.logger.Debug("closing current camera")
		if err := c.underlyingSource.Close(c.cancelCtx); err != nil {
			c.logger.Errorw("failed to close currents camera", "error", err)
		}
		c.underlyingSource = nil
	}

	newSrc, foundLabel, err := findAndMakeVideoSource(c.cancelCtx, conf, c.targetPath, c.logger)
	if err != nil {
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

	c.logger = c.logger.WithFields("camera_label", c.targetPath)

	return nil
}

// Monitor is responsible for monitoring the liveness of a camera. An example
// is connectivity. Since the model itself knows best about how to maintain this state,
// the reconfigurable offers a safe way to notify if a state needs to be reset due
// to some exceptional event (like a reconnect).
// It is expected that the monitoring code is tied to the lifetime of the resource
// and once the resource is closed, so should the monitor. That is, it should
// no longer send any resets once a Close on its associated resource has returned.
func (c *webcam) Monitor() {
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
							c.logger.Debugw("failed to reconnect camera", "error", err)
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

func (c *webcam) Images(ctx context.Context) ([]camera.NamedImage, resource.ResponseMetadata, error) {
	if c, ok := c.underlyingSource.(camera.ImagesSource); ok {
		return c.Images(ctx)
	}
	img, release, err := camera.ReadImage(ctx, c.underlyingSource)
	if err != nil {
		return nil, resource.ResponseMetadata{}, errors.Wrap(err, "monitoredWebcam: call to get Images failed")
	}
	defer func() {
		if release != nil {
			release()
		}
	}()
	return []camera.NamedImage{{img, c.Name().Name}}, resource.ResponseMetadata{time.Now()}, nil
}

// ensureActive is a helper that guards logic that requires the camera to be actively connected.
func (c *webcam) ensureActive() error {
	if c.closed {
		return errClosed
	}
	if c.disconnected {
		return errDisconnected
	}
	return nil
}

func (c *webcam) Stream(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if err := c.ensureActive(); err != nil {
		return nil, err
	}
	return c.exposedSwapper.Stream(ctx, errHandlers...)
}

func (c *webcam) Image(ctx context.Context, mimeType string, extra map[string]interface{}) ([]byte, camera.ImageMetadata, error) {
	return camera.ReadImageBytes(ctx, c.underlyingSource, mimeType)
}

func (c *webcam) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if err := c.ensureActive(); err != nil {
		return nil, err
	}
	return c.exposedProjector.NextPointCloud(ctx)
}

// driverInfo gets the mediadevices Info struct containing info such as name and device type of the given driver.
// It is a helper func for serving Properties.
func (c *webcam) driverInfo() (driver.Info, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.underlyingSource == nil {
		return driver.Info{}, errors.New("no underlying source found in camera")
	}
	d, err := gostream.DriverFromMediaSource[image.Image, prop.Video](c.underlyingSource)
	if err != nil {
		return driver.Info{}, errors.Wrap(err, "cannot get driver from media source")
	}
	return d.Info(), nil
}

func (c *webcam) Properties(ctx context.Context) (camera.Properties, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if err := c.ensureActive(); err != nil {
		return camera.Properties{}, err
	}

	props, err := c.exposedProjector.Properties(ctx)
	if err != nil {
		return camera.Properties{}, err
	}
	// Looking for intrinsics in map built using viam camera
	// calibration here https://github.com/viam-labs/camera-calibration/tree/main
	if props.IntrinsicParams == nil {
		dInfo, err := c.driverInfo()
		if err != nil {
			if !c.hasLoggedIntrinsicsInfo {
				c.logger.CErrorw(ctx, "can't find driver info for camera")
				c.hasLoggedIntrinsicsInfo = true
			}
		}

		cameraIntrinsics, exists := data[dInfo.Name]
		if !exists {
			if !c.hasLoggedIntrinsicsInfo {
				c.logger.CInfo(ctx, "camera model not found in known camera models for: ", dInfo.Name, ". returning "+
					"properties without intrinsics")
				c.hasLoggedIntrinsicsInfo = true
			}
			return props, nil
		}
		if c.conf.Width != 0 {
			if c.conf.Width != cameraIntrinsics.Width {
				if !c.hasLoggedIntrinsicsInfo {
					c.logger.CInfo(ctx, "camera model found in known camera models for: ", dInfo.Name, " but "+
						"intrinsics width doesn't match configured image width")
					c.hasLoggedIntrinsicsInfo = true
				}
				return props, nil
			}
		}
		if c.conf.Height != 0 {
			if c.conf.Height != cameraIntrinsics.Height {
				if !c.hasLoggedIntrinsicsInfo {
					c.logger.CInfo(ctx, "camera model found in known camera models for: ", dInfo.Name, " but "+
						"intrinsics height doesn't match configured image height")
					c.hasLoggedIntrinsicsInfo = true
				}
				return props, nil
			}
		}
		if !c.hasLoggedIntrinsicsInfo {
			c.logger.CInfo(ctx, "Intrinsics are known for camera model: ", dInfo.Name, ". adding intrinsics "+
				"to camera properties")
			c.hasLoggedIntrinsicsInfo = true
		}
		props.IntrinsicParams = &cameraIntrinsics

		if c.conf.FrameRate > 0 {
			props.FrameRate = c.conf.FrameRate
		}
	}
	return props, nil
}

var (
	errClosed       = errors.New("camera has been closed")
	errDisconnected = errors.New("camera is disconnected; please try again in a few moments")
)

func (c *webcam) Close(ctx context.Context) error {
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
