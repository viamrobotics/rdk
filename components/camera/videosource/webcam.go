// Package videosource implements webcam. It should be renamed webcam.
package videosource

import (
	"context"
	"fmt"
	"image"
	"io"
	"sync"
	"time"

	driverutils "github.com/pion/mediadevices/pkg/driver"
	"github.com/pion/mediadevices/pkg/driver/availability"
	"github.com/pion/mediadevices/pkg/io/video"
	"github.com/pkg/errors"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/depthadapter"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// ModelWebcam is the name of the webcam component.
var ModelWebcam = resource.DefaultModelFamily.WithModel("webcam")

var (
	errClosed       = errors.New("camera has been closed")
	errDisconnected = errors.New("camera is disconnected; please try again in a few moments")
)

// minResolutionDimension is set to 2 to ensure proper fitness distance calculation for resolution selection.
// Setting this to 0 would cause mediadevices' IntRanged.Compare() method to treat all values smaller than ideal
// as equally acceptable. See https://github.com/pion/mediadevices/blob/c10fb000dbbb28597e068468f3175dc68a281bfd/pkg/prop/int.go#L104
// Setting it to 1 could theoretically allow 1x1 resolutions. 2 is small enough and even,
// allowing all real camera resolutions while ensuring proper distance calculations.
const (
	minResolutionDimension = 2
	defaultFrameRate       = float32(30.0)
)

func init() {
	resource.RegisterComponent(
		camera.API,
		ModelWebcam,
		resource.Registration[camera.Camera, *WebcamConfig]{
			Constructor: NewWebcam,
		})
}

// WebcamBuffer is a buffer for webcam frames.
// WARNING: This struct is NOT thread safe. It must be protected by the mutex in the webcam struct.
type WebcamBuffer struct {
	frame   image.Image // Holds the frames and their release functions in the buffer
	release func()
	err     error
	worker  *goutils.StoppableWorkers // A separate worker for the webcam buffer that allows stronger concurrency control.
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
func (c WebcamConfig) Validate(path string) ([]string, []string, error) {
	if c.Width < 0 || c.Height < 0 {
		return nil, nil, fmt.Errorf(
			"got illegal negative dimensions for width_px and height_px (%d, %d) fields set for webcam camera",
			c.Height, c.Width)
	}
	if c.FrameRate < 0 {
		return nil, nil, fmt.Errorf(
			"got illegal non-positive dimension for frame rate (%.2f) field set for webcam camera",
			c.FrameRate)
	}

	return []string{}, nil, nil
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

	closed       bool
	disconnected bool
	logger       logging.Logger
	workers      *goutils.StoppableWorkers

	buffer *WebcamBuffer
}

// NewWebcam returns the webcam discovered based on the given config as the Camera interface type.
func NewWebcam(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (camera.Camera, error) {
	cam := &webcam{
		Named:   conf.ResourceName().AsNamed(),
		logger:  logger.WithFields("camera_name", conf.ResourceName().ShortName()),
		workers: goutils.NewBackgroundStoppableWorkers(),
	}
	cam.buffer = NewWebcamBuffer(cam.workers.Context())

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

	// Stop the driver and frame buffer worker
	c.mu.Lock()
	driver := c.driver
	frameBufferWorker := c.buffer.worker
	c.mu.Unlock()

	if driver != nil {
		if err := driver.Close(); err != nil {
			c.logger.Errorw("failed to close current camera before stopping buffer worker", "error", err)
		}
	}

	if frameBufferWorker != nil {
		frameBufferWorker.Stop()
	}

	// Release buffer
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.buffer != nil {
		if c.buffer.release != nil {
			c.buffer.release()
			c.buffer.release = nil
		}
	}
	c.reader = nil
	c.driver = nil

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

	if c.conf.FrameRate == 0.0 {
		c.conf.FrameRate = defaultFrameRate
	}
	c.buffer = NewWebcamBuffer(c.workers.Context())
	c.startFrameBufferWorker()

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
	c.workers.Add(func(ctx context.Context) {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
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
				reconnectLoop:
					for {
						select {
						case <-ctx.Done():
							return
						case <-ticker.C:
							cont := func() bool {
								c.mu.Lock()
								if err := c.reconnectCamera(&c.conf); err != nil {
									c.mu.Unlock()
									c.logger.Debugw("failed to reconnect camera", "error", err)
									return true
								}
								c.logger.Infow("camera reconnected")

								// Stop the buffer worker outside of the mutex to avoid deadlock
								var oldWorker *goutils.StoppableWorkers
								if c.buffer != nil && c.buffer.worker != nil {
									oldWorker = c.buffer.worker
								}

								c.buffer = NewWebcamBuffer(c.workers.Context())
								c.startFrameBufferWorker()
								c.mu.Unlock()

								if oldWorker != nil {
									oldWorker.Stop()
								}

								return false
							}()
							if cont {
								continue
							}
							break reconnectLoop
						}
					}
				}
			}
		}
	})
}

func (c *webcam) Images(
	ctx context.Context,
	filterSourceNames []string,
	extra map[string]interface{},
) ([]camera.NamedImage, resource.ResponseMetadata, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if err := c.ensureActive(); err != nil {
		return nil, resource.ResponseMetadata{}, err
	}

	img, err := c.getLatestFrame()
	if err != nil {
		return nil, resource.ResponseMetadata{}, errors.Wrap(err, "monitoredWebcam: call to get Images failed")
	}

	namedImg, err := camera.NamedImageFromImage(img, c.Name().Name, utils.MimeTypeJPEG, data.Annotations{})
	if err != nil {
		return nil, resource.ResponseMetadata{}, err
	}
	return []camera.NamedImage{namedImg}, resource.ResponseMetadata{CapturedAt: time.Now()}, nil
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
	c.mu.RLock()
	defer c.mu.RUnlock()

	if err := c.ensureActive(); err != nil {
		return nil, camera.ImageMetadata{}, err
	}
	if c.reader == nil {
		return nil, camera.ImageMetadata{}, errors.New("underlying reader is nil")
	}
	img, err := c.getLatestFrame()
	if err != nil {
		return nil, camera.ImageMetadata{}, err
	}

	if mimeType == "" {
		mimeType = utils.MimeTypeJPEG
	}
	imgBytes, err := rimage.EncodeImage(ctx, img, mimeType)
	if err != nil {
		return nil, camera.ImageMetadata{}, err
	}
	return imgBytes, camera.ImageMetadata{MimeType: mimeType}, nil
}

func (c *webcam) NextPointCloud(ctx context.Context, extra map[string]interface{}) (pointcloud.PointCloud, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if err := c.ensureActive(); err != nil {
		return nil, err
	}

	if c.cameraModel.PinholeCameraIntrinsics == nil {
		return nil, transform.NewNoIntrinsicsError("cannot do a projection to a point cloud")
	}

	img, release, err := c.reader.Read()
	if err != nil {
		return nil, err
	}
	defer release()

	dm, err := rimage.ConvertImageToDepthMap(ctx, img)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot project to a point cloud")
	}

	return depthadapter.ToPointCloud(dm, c.cameraModel.PinholeCameraIntrinsics), nil
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
		SupportsPCD:      c.cameraModel.PinholeCameraIntrinsics != nil,
		ImageType:        camera.ColorStream,
		IntrinsicParams:  c.cameraModel.PinholeCameraIntrinsics,
		DistortionParams: c.cameraModel.Distortion,
		MimeTypes:        []string{utils.MimeTypeJPEG, utils.MimeTypePNG, utils.MimeTypeRawRGBA},
		FrameRate:        frameRate,
	}, nil
}

func (c *webcam) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	return make([]spatialmath.Geometry, 0), nil
}

// NewWebcamBuffer creates a new WebcamBuffer struct.
func NewWebcamBuffer(ctx context.Context) *WebcamBuffer {
	return &WebcamBuffer{
		worker: goutils.NewStoppableWorkers(ctx),
	}
}

// Must lock the mutex before calling this function.
func (c *webcam) getLatestFrame() (image.Image, error) {
	if c.buffer.frame == nil {
		if c.buffer.err != nil {
			return nil, c.buffer.err
		}
		return nil, errors.New("no frames available to read")
	}

	return c.buffer.frame, nil
}

func (c *webcam) startFrameBufferWorker() {
	if c.buffer.frame != nil {
		return // frame buffer worker already started
	}

	interFrameDuration := time.Duration(float32(time.Second) / c.conf.FrameRate)
	ticker := time.NewTicker(interFrameDuration)
	c.buffer.worker.Add(func(closedCtx context.Context) {
		defer ticker.Stop()
		for {
			select {
			case <-closedCtx.Done():
				return
			case <-ticker.C:
				// Make a private copy of the previously published frame so consumers can continue to read it,
				// then return the Reader-managed buffer before we kick off the next read. This avoids holding
				// on to driver memory while still serving the last frame.
				var prevRelease func()
				c.mu.Lock()
				if c.buffer.release != nil && c.buffer.frame != nil {
					c.buffer.frame = rimage.CloneImage(c.buffer.frame)
				}
				prevRelease = c.buffer.release
				c.buffer.release = nil
				c.mu.Unlock()

				if prevRelease != nil {
					prevRelease()
				}

				img, release, err := c.reader.Read()

				c.mu.Lock()
				c.buffer.err = err
				if err != nil {
					c.buffer.release = nil
					c.buffer.frame = nil
					c.logger.Errorf("error reading frame: %v", err)
					isEOF := errors.Is(err, io.EOF)
					if isEOF {
						c.logger.Warnf("camera disconnected (EOF), stopping buffer worker. Error: %v", err)
						c.disconnected = true
					}
					c.mu.Unlock()
					if isEOF {
						return
					}
					continue
				}
				c.buffer.frame = img
				c.buffer.release = release
				c.mu.Unlock()
			}
		}
	})
}

func (c *webcam) Close(ctx context.Context) error {
	c.workers.Stop()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return errors.New("webcam already closed")
	}
	c.closed = true

	if c.buffer != nil && c.buffer.worker != nil {
		c.buffer.worker.Stop()
	}

	if c.driver != nil {
		return c.driver.Close()
	}
	return nil
}
