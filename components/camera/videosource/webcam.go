// Package videosource implements webcam. It should be renamed webcam.
package videosource

import (
	"context"
	"fmt"
	"image"
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
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// ModelWebcam is the name of the webcam component.
var ModelWebcam = resource.DefaultModelFamily.WithModel("webcam")

var (
	errClosed       = errors.New("webcam has been closed")
	errDisconnected = errors.New("webcam is disconnected; please try again in a few moments")
	errNoFrames     = errors.New("no frames available to read")
	errNoDriver     = errors.New("no camera driver set")
)

const (
	defaultFrameRate = float32(30.0)
)

func init() {
	resource.RegisterComponent(
		camera.API,
		ModelWebcam,
		resource.Registration[camera.Camera, *WebcamConfig]{
			Constructor: NewWebcam,
		})
}

// WebcamConfig is the native config attribute struct for webcams.
type WebcamConfig struct {
	CameraParameters     *transform.PinholeCameraIntrinsics `json:"intrinsic_parameters,omitempty"`
	DistortionParameters *transform.BrownConrady            `json:"distortion_parameters,omitempty"`
	Format               string                             `json:"format,omitempty"`
	Path                 string                             `json:"video_path,omitempty"`
	Width                int                                `json:"width_px,omitempty"`
	Height               int                                `json:"height_px,omitempty"`
	FrameRate            float32                            `json:"frame_rate,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (c WebcamConfig) Validate(path string) ([]string, []string, error) {
	if c.Width < 0 || c.Height < 0 {
		return nil, nil, fmt.Errorf(
			"got illegal negative dimensions for width_px and height_px (%d, %d) fields set for webcam camera",
			c.Width, c.Height)
	}
	if c.FrameRate < 0 {
		return nil, nil, fmt.Errorf(
			"got illegal negative frame rate (%.2f) field set for webcam camera",
			c.FrameRate)
	}

	return []string{}, nil, nil
}

// webcam is a video driver wrapper camera that ensures its underlying driver stays connected,
// handling hot unplugs/replugs, and provides a buffer to read frames from.
type webcam struct {
	resource.Named
	resource.AlwaysRebuild

	// workers is not protected by mu. Workers may acquire mu, so holding mu while
	// calling workers.Stop() or workers.Add() causes deadlock.
	workers *goutils.StoppableWorkers

	// mu protects all fields below
	mu sync.Mutex

	cameraModel transform.PinholeCameraModel

	reader video.Reader
	driver driverutils.Driver

	// This is returned to us as a label in mediadevices but our config
	// treats it as a video path.
	targetPath string
	conf       WebcamConfig

	closed       bool // set by Close method
	disconnected bool // set by monitor worker

	logger logging.Logger
	buffer *webcamBuffer
}

// webcamBuffer is a buffer for webcam frames.
// It must be protected by the mutex in the webcam struct.
type webcamBuffer struct {
	frame image.Image
	// release is a function provided by the mediadevices camera driver that must be called
	// to release resources associated with the frame after we're done using it.
	// It is set by the buffer worker after reading a frame from reader.Read().
	//
	// Note: While the above is true in theory, mediadevices decoders currently
	// return empty (no-op) release functions on all platforms (darwin, windows, linux).
	// We still call release to comply with the Reader interface, and in case
	// decoders eventually provide a non-nil release function.
	release func()
	err     error
}

// newWebcamBuffer creates a new WebcamBuffer struct.
func newWebcamBuffer() *webcamBuffer {
	return &webcamBuffer{}
}

// NewWebcam returns the webcam discovered based on the given config as the Camera interface type.
func NewWebcam(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (camera.Camera, error) {
	c := &webcam{
		Named:   conf.ResourceName().AsNamed(),
		logger:  logger.WithFields("camera_name", conf.ResourceName().ShortName()),
		workers: goutils.NewBackgroundStoppableWorkers(),
		buffer:  newWebcamBuffer(),
	}

	nativeConf, err := resource.NativeConfig[*WebcamConfig](conf)
	if err != nil {
		return nil, err
	}

	c.cameraModel = camera.NewPinholeModelWithBrownConradyDistortion(nativeConf.CameraParameters, nativeConf.DistortionParameters)

	c.targetPath = nativeConf.Path
	reader, driver, label, err := findReaderAndDriver(nativeConf, c.targetPath, c.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to find camera: %w", err)
	}

	c.reader = reader
	c.driver = driver
	c.disconnected = false
	if c.targetPath == "" {
		c.targetPath = label
	}
	c.logger = c.logger.WithFields("camera_name", c.Name().ShortName(), "camera_label", c.targetPath)

	// only set once we're good
	c.conf = *nativeConf

	if c.conf.FrameRate == 0.0 {
		c.conf.FrameRate = defaultFrameRate
	}

	// Start both workers after successful configuration
	c.startMonitorWorker()
	c.startBufferWorker()

	return c, nil
}

// ensureActive checks the camera's state and returns the appropriate error if it is not active.
// Must be called with mu held.
func (c *webcam) ensureActive() error {
	if c.closed {
		return errClosed
	}
	if c.disconnected {
		return errDisconnected
	}
	return nil
}

// isCameraConnected is a helper for monitoring connectivity to the driver.
// Performs I/O operations, so must be called without holding mu.
func isCameraConnected(driver driverutils.Driver) (bool, error) {
	if driver == nil {
		return false, fmt.Errorf("cannot determine camera status: %w", errNoDriver)
	}

	// TODO(RSDK-1959): this only works for linux
	_, err := driverutils.IsAvailable(driver)
	return !errors.Is(err, availability.ErrNoDevice), nil
}

// startMonitorWorker starts a worker that monitors camera connectivity and handles reconnection.
// This worker runs continuously until the context is cancelled (via workers.Stop()).
// It checks camera connectivity using isCameraConnected every ticker tick. If disconnected,
// it marks the camera as disconnected and attempts reconnection every tick until successful.
// Upon successful reconnection, it resets the buffer state and flags to resume healthy operation.
func (c *webcam) startMonitorWorker() {
	c.workers.Add(func(ctx context.Context) {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				c.logger.Debug("monitor worker context done")
				return
			case <-ticker.C:
				c.mu.Lock()
				logger := c.logger
				driver := c.driver
				c.mu.Unlock()

				ok, err := isCameraConnected(driver)
				if err != nil {
					logger.Debugw("cannot determine camera status", "error", err)
					continue
				}
				if ok {
					continue
				}

				c.mu.Lock()
				c.disconnected = true
				c.mu.Unlock()

				logger.Error("camera no longer connected; reconnecting")
			reconnectLoop:
				for {
					select {
					case <-ctx.Done():
						c.logger.Debug("reconnect loop context done")
						return
					case <-ticker.C:
						// Get current state and clear driver/reader while holding lock
						c.mu.Lock()
						oldDriver := c.driver
						oldRelease := c.buffer.release
						conf := c.conf
						targetPath := c.targetPath

						c.driver = nil
						c.reader = nil
						c.buffer.release = nil
						c.buffer.frame = nil
						c.mu.Unlock()

						// Close old driver outside lock (I/O operation)
						if oldDriver != nil {
							c.logger.Debug("closing current camera")
							if err := oldDriver.Close(); err != nil {
								c.logger.Errorw("failed to close current camera", "error", err)
							}
						}

						// Release old buffer frame outside lock (I/O operation)
						if oldRelease != nil {
							oldRelease()
						}

						// Try to find and reconnect to camera outside lock (heavy I/O)
						reader, driver, label, err := findReaderAndDriver(&conf, targetPath, c.logger)
						if err != nil {
							c.logger.Debugw("failed to reconnect camera", "error", err)
							continue
						}

						// Successfully reconnected, update state while holding lock
						c.mu.Lock()
						c.reader = reader
						c.driver = driver
						c.disconnected = false
						if c.targetPath == "" {
							c.targetPath = label
						}
						c.logger = c.logger.WithFields("camera_name", c.Name().ShortName(), "camera_label", c.targetPath)

						// Clear any error from before reconnection
						c.buffer.err = nil

						c.logger.Infow("camera reconnected")
						c.mu.Unlock()
						break reconnectLoop
					}
				}
			}
		}
	})
}

// startBufferWorker starts a worker that continuously reads frames from the camera and writes them to the buffer.
// When disconnected, it skips reading but continues running to resume immediately upon reconnection.
func (c *webcam) startBufferWorker() {
	c.mu.Lock()
	frameRate := c.conf.FrameRate
	c.mu.Unlock()

	interFrameDuration := time.Duration(float32(time.Second) / frameRate)

	c.workers.Add(func(ctx context.Context) {
		ticker := time.NewTicker(interFrameDuration)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				c.logger.Debug("buffer worker context done")
				return
			case <-ticker.C:
				c.mu.Lock()

				if c.disconnected {
					c.mu.Unlock()
					continue
				}

				reader := c.reader
				if reader == nil {
					c.mu.Unlock()
					continue
				}

				// Get the release function to call outside the lock to avoid potential deadlocks.
				oldRelease := c.buffer.release
				c.buffer.release = nil
				c.mu.Unlock()

				// Call release and read outside the lock to avoid holding the lock during I/O
				if oldRelease != nil {
					oldRelease()
				}
				img, release, err := reader.Read()

				c.mu.Lock()
				c.buffer.err = err
				if err != nil {
					c.buffer.release = nil
					c.buffer.frame = nil
					c.logger.Errorw("error reading frame", "error", err)
					c.mu.Unlock()
					continue
				}
				c.buffer.frame = img
				c.buffer.release = release
				c.mu.Unlock()
			}
		}
	})
}

func (c *webcam) Images(_ context.Context, _ []string, _ map[string]interface{}) ([]camera.NamedImage, resource.ResponseMetadata, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.ensureActive(); err != nil {
		return nil, resource.ResponseMetadata{}, err
	}

	if c.buffer.frame == nil {
		if c.buffer.err != nil {
			return nil, resource.ResponseMetadata{}, c.buffer.err
		}
		return nil, resource.ResponseMetadata{}, errNoFrames
	}

	img := c.buffer.frame
	namedImg, err := camera.NamedImageFromImage(img, c.Name().Name, utils.MimeTypeJPEG, data.Annotations{})
	if err != nil {
		return nil, resource.ResponseMetadata{}, fmt.Errorf("failed to create named image: %w", err)
	}

	return []camera.NamedImage{namedImg}, resource.ResponseMetadata{CapturedAt: time.Now()}, nil
}

func (c *webcam) Image(ctx context.Context, _ string, extra map[string]interface{}) ([]byte, camera.ImageMetadata, error) {
	imgs, _, err := c.Images(ctx, nil, extra)
	if err != nil {
		return nil, camera.ImageMetadata{}, err
	}
	if len(imgs) == 0 {
		return nil, camera.ImageMetadata{}, errors.New("no images from webcam")
	}
	imgBytes, err := imgs[0].Bytes(ctx)
	if err != nil {
		return nil, camera.ImageMetadata{}, err
	}
	return imgBytes, camera.ImageMetadata{
		MimeType: imgs[0].MimeType(),
	}, nil
}

func (c *webcam) Properties(ctx context.Context) (camera.Properties, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureActive(); err != nil {
		return camera.Properties{}, err
	}

	var frameRate float32
	if c.conf.FrameRate > 0 {
		frameRate = c.conf.FrameRate
	}
	return camera.Properties{
		SupportsPCD:      false, // RGB webcams cannot generate point clouds
		ImageType:        camera.ColorStream,
		IntrinsicParams:  c.cameraModel.PinholeCameraIntrinsics,
		DistortionParams: c.cameraModel.Distortion,
		MimeTypes:        []string{utils.MimeTypeJPEG, utils.MimeTypePNG, utils.MimeTypeRawRGBA},
		FrameRate:        frameRate,
	}, nil
}

func (c *webcam) NextPointCloud(ctx context.Context, extra map[string]interface{}) (pointcloud.PointCloud, error) {
	return nil, errors.New("not supported for webcams")
}

func (c *webcam) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	return make([]spatialmath.Geometry, 0), nil
}

func (c *webcam) Close(ctx context.Context) error {
	// Stop workers before acquiring mu
	c.workers.Stop()

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return fmt.Errorf("webcam already closed: %w", errClosed)
	}
	c.closed = true

	// Extract resources to clean up outside the lock
	oldRelease := c.buffer.release
	oldDriver := c.driver

	// Clear state
	c.buffer.release = nil
	c.buffer.frame = nil
	c.reader = nil
	c.driver = nil
	c.mu.Unlock()

	// Perform I/O operations outside the lock
	if oldRelease != nil {
		oldRelease()
	}

	if oldDriver != nil {
		err := oldDriver.Close()
		if err != nil {
			return fmt.Errorf("webcam failed to close (failed to close camera driver): %w", err)
		}
	}

	return nil
}
