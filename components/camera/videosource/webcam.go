// Package videosource implements webcam. It should be renamed webcam.
package videosource

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pion/mediadevices"
	driverutils "github.com/pion/mediadevices/pkg/driver"
	"github.com/pion/mediadevices/pkg/driver/availability"
	mediadevicescamera "github.com/pion/mediadevices/pkg/driver/camera"
	"github.com/pion/mediadevices/pkg/frame"
	"github.com/pion/mediadevices/pkg/io/video"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pkg/errors"
	pb "go.viam.com/api/component/camera/v1"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
)

// ModelWebcam is the name of the webcam component.
var ModelWebcam = resource.DefaultModelFamily.WithModel("webcam")

//go:embed data/intrinsics.json
var intrinsics []byte

var (
	errClosed       = errors.New("camera has been closed")
	errDisconnected = errors.New("camera is disconnected; please try again in a few moments")
	data            map[string]transform.PinholeCameraIntrinsics
)

func init() {
	resource.RegisterComponent(
		camera.API,
		ModelWebcam,
		resource.Registration[camera.Camera, *WebcamConfig]{
			Constructor: NewWebcam,
			Discover: func(ctx context.Context, logger logging.Logger, extra map[string]interface{}) (interface{}, error) {
				// getVideoDrivers is a callback passed to the registered Discover func to get all video drivers.
				getVideoDrivers := func() []driverutils.Driver {
					return driverutils.GetManager().Query(driverutils.FilterVideoRecorder())
				}
				return Discover(ctx, getVideoDrivers, logger)
			},
		})
	if err := json.Unmarshal(intrinsics, &data); err != nil {
		logging.Global().Errorw("cannot parse intrinsics json", "error", err)
	}
}

// getDriverProperties is a helper func for webcam discovery that returns the Media properties of a specific driver.
func getDriverProperties(d driverutils.Driver) (_ []prop.Media, err error) {
	// Need to open driver to get properties
	if d.Status() == driverutils.StateClosed {
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
func Discover(ctx context.Context, getDrivers func() []driverutils.Driver, logger logging.Logger) (*pb.Webcams, error) {
	mediadevicescamera.Initialize()
	var webcams []*pb.Webcam
	drivers := getDrivers()
	for _, d := range drivers {
		driverInfo := d.Info()
		props, err := getDriverProperties(d)
		if len(props) == 0 {
			logger.CDebugw(ctx, "no properties detected for driver, skipping discovery...", "driver", driverInfo.Label)
			continue
		} else if err != nil {
			logger.CDebugw(ctx, "cannot access driver properties, skipping discovery...", "driver", driverInfo.Label, "error", err)
			continue
		}

		if d.Status() == driverutils.StateRunning {
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
	Debug                bool                               `json:"debug,omitempty"`
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

// makeConstraints is a helper that returns constraints to mediadevices in order to find and make a video source.
// Constraints are specifications for the video stream such as frame format, resolution etc.
func makeConstraints(conf *WebcamConfig, debug bool, logger logging.Logger) mediadevices.MediaStreamConstraints {
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

// findReaderAndDriver finds a video device and returns an image reader and the driver instance,
// as well as the path to the driver.
func findReaderAndDriver(
	conf *WebcamConfig,
	path string,
	logger logging.Logger,
) (video.Reader, driverutils.Driver, string, error) {
	mediadevicescamera.Initialize()
	debug := conf.Debug
	constraints := makeConstraints(conf, debug, logger)

	// Handle specific path
	if path != "" {
		resolvedPath, err := filepath.EvalSymlinks(path)
		if err == nil {
			path = resolvedPath
		}
		reader, driver, err := getReaderAndDriver(filepath.Base(path), constraints, logger)
		if err != nil {
			return nil, nil, "", err
		}

		if conf.Width != 0 && conf.Height != 0 {
			img, release, err := reader.Read()
			if release != nil {
				defer release()
			}
			if err != nil {
				return nil, nil, "", err
			}
			if img.Bounds().Dx() != conf.Width || img.Bounds().Dy() != conf.Height {
				return nil, nil, "", errors.Errorf("requested width and height (%dx%d) are not available for this webcam"+
					" (closest driver found supports resolution %dx%d)",
					conf.Width, conf.Height, img.Bounds().Dx(), img.Bounds().Dy())
			}
		}
		return reader, driver, path, nil
	}

	// Handle "any" path
	reader, driver, err := getReaderAndDriver("", constraints, logger)
	if err != nil {
		return nil, nil, "", errors.Wrap(err, "found no webcams")
	}
	labels := strings.Split(driver.Info().Label, mediadevicescamera.LabelSeparator)
	if len(labels) == 0 {
		logger.Error("no labels parsed from driver")
		return nil, nil, "", nil
	}
	path = labels[0] // path is always the first element

	return reader, driver, path, nil
}

// webcam is a video driver wrapper camera that ensures its underlying driver stays connected.
type webcam struct {
	resource.Named
	mu                      sync.RWMutex
	hasLoggedIntrinsicsInfo bool

	cameraModel transform.PinholeCameraModel

	reader video.Reader
	driver driverutils.Driver

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

	c.cameraModel = camera.NewPinholeModelWithBrownConradyDistortion(newConf.CameraParameters, newConf.DistortionParameters)

	driverReinitNotNeeded := c.conf.Format == newConf.Format &&
		c.conf.Path == newConf.Path &&
		c.conf.Width == newConf.Width &&
		c.conf.Height == newConf.Height

	if c.driver != nil && c.reader != nil && driverReinitNotNeeded {
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

// isCameraConnected is a helper for monitoring connectivity to the driver.
func (c *webcam) isCameraConnected() (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.driver == nil {
		return true, errors.New("no configured camera")
	}

	// TODO(RSDK-1959): this only works for linux
	_, err := driverutils.IsAvailable(c.driver)
	return !errors.Is(err, availability.ErrNoDevice), nil
}

// reconnectCamera tries to reconnect the camera to a driver that matches the config.
// Assumes a write lock is held.
func (c *webcam) reconnectCamera(conf *WebcamConfig) error {
	if c.driver != nil {
		c.logger.Debug("closing current camera")
		if err := c.driver.Close(); err != nil {
			c.logger.Errorw("failed to close current camera", "error", err)
		}
		c.driver = nil
		c.reader = nil
	}

	reader, driver, foundLabel, err := findReaderAndDriver(conf, c.targetPath, c.logger)
	if err != nil {
		return errors.Wrap(err, "failed to find camera")
	}

	c.reader = reader
	c.driver = driver
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
	img, release, err := c.reader.Read()
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

func (c *webcam) Image(ctx context.Context, mimeType string, extra map[string]interface{}) ([]byte, camera.ImageMetadata, error) {
	img, release, err := c.reader.Read()
	if err != nil {
		return nil, camera.ImageMetadata{}, err
	}
	defer release()

	if mimeType == "" {
		mimeType = utils.MimeTypeJPEG
	}
	imgBytes, err := rimage.EncodeImage(ctx, img, mimeType)
	if err != nil {
		return nil, camera.ImageMetadata{}, err
	}
	return imgBytes, camera.ImageMetadata{MimeType: mimeType}, nil
}

func (c *webcam) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	return nil, errors.New("NextPointCloud unimplemented")
}

func (c *webcam) Properties(ctx context.Context) (camera.Properties, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if err := c.ensureActive(); err != nil {
		return camera.Properties{}, err
	}

	var frameRate float32
	if c.conf.FrameRate > 0 {
		frameRate = c.conf.FrameRate
	}
	return camera.Properties{
		SupportsPCD:      false,
		ImageType:        camera.ColorStream,
		IntrinsicParams:  c.cameraModel.PinholeCameraIntrinsics,
		DistortionParams: c.cameraModel.Distortion,
		MimeTypes:        []string{utils.MimeTypeJPEG, utils.MimeTypePNG, utils.MimeTypeRawRGBA},
		FrameRate:        frameRate,
	}, nil

	// props, err := c.exposedProjector.Properties(ctx)
	// if err != nil {
	// 	return camera.Properties{}, err
	// }
	// // Looking for intrinsics in map built using viam camera
	// // calibration here https://github.com/viam-labs/camera-calibration/tree/main
	// if props.IntrinsicParams == nil {
	// 	dInfo := c.getDriverInfo()
	// 	cameraIntrinsics, exists := data[dInfo.Name]
	// 	if !exists {
	// 		if !c.hasLoggedIntrinsicsInfo {
	// 			c.logger.CInfo(ctx, "camera model not found in known camera models for: ", dInfo.Name, ". returning "+
	// 				"properties without intrinsics")
	// 			c.hasLoggedIntrinsicsInfo = true
	// 		}
	// 		return props, nil
	// 	}
	// 	if c.conf.Width != 0 {
	// 		if c.conf.Width != cameraIntrinsics.Width {
	// 			if !c.hasLoggedIntrinsicsInfo {
	// 				c.logger.CInfo(ctx, "camera model found in known camera models for: ", dInfo.Name, " but "+
	// 					"intrinsics width doesn't match configured image width")
	// 				c.hasLoggedIntrinsicsInfo = true
	// 			}
	// 			return props, nil
	// 		}
	// 	}
	// 	if c.conf.Height != 0 {
	// 		if c.conf.Height != cameraIntrinsics.Height {
	// 			if !c.hasLoggedIntrinsicsInfo {
	// 				c.logger.CInfo(ctx, "camera model found in known camera models for: ", dInfo.Name, " but "+
	// 					"intrinsics height doesn't match configured image height")
	// 				c.hasLoggedIntrinsicsInfo = true
	// 			}
	// 			return props, nil
	// 		}
	// 	}
	// 	if !c.hasLoggedIntrinsicsInfo {
	// 		c.logger.CInfo(ctx, "Intrinsics are known for camera model: ", dInfo.Name, ". adding intrinsics "+
	// 			"to camera properties")
	// 		c.hasLoggedIntrinsicsInfo = true
	// 	}
	// 	props.IntrinsicParams = &cameraIntrinsics

	// 	if c.conf.FrameRate > 0 {
	// 		props.FrameRate = c.conf.FrameRate
	// 	}
	// }
	// return props, nil
}

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

	return c.driver.Close()
}
