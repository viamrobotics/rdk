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
	resWarnInterval  = 10 * time.Minute
	// wakeTimeout bounds how long Images waits for an idle camera to reopen and deliver a frame.
	wakeTimeout = 15 * time.Second
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
	IdleTimeoutSeconds   float64                            `json:"idle_timeout_seconds,omitempty"`
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
	if c.IdleTimeoutSeconds < 0 {
		return nil, nil, fmt.Errorf(
			"got illegal negative idle timeout (%.2f) field set for webcam camera",
			c.IdleTimeoutSeconds)
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

	// idleTimeout > 0: buffer worker closes the camera after idleTimeout without an Images call,
	// reopens on wakeCh; readyCh is made on entering idle and closed when the wake attempt ends.
	idleTimeout time.Duration
	lastAccess  time.Time
	idle        bool
	wakeCh      chan struct{}
	readyCh     chan struct{}
	// openCamera is findReaderAndDriver; overridable in tests.
	openCamera func(*WebcamConfig, string, logging.Logger) (video.Reader, driverutils.Driver, string, error)

	logger          logging.Logger
	buffer          *webcamBuffer
	lastResWarnTime time.Time
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
	// Start camera observer for hot-plug support (darwin only, no-op on other platforms).
	// SetupObserver and DestroyObserver are called in RDK's entrypoint main.go to satisfy
	// AVFoundation's threading requirements. startCameraObserver is idempotent and safely
	// starts the observer for this component.
	// See web/cmd/server/observer_darwin.go for details on threading.
	startCameraObserver(logger)

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
	c.idleTimeout = time.Duration(c.conf.IdleTimeoutSeconds * float64(time.Second))
	c.lastAccess = time.Now()
	c.wakeCh = make(chan struct{}, 1)
	c.openCamera = findReaderAndDriver

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
				if c.idle {
					c.mu.Unlock()
					continue
				}
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
			case <-c.wakeCh:
				c.wake()
			case <-ticker.C:
				c.mu.Lock()
				if c.idle {
					c.mu.Unlock()
					continue
				}
				if c.idleTimeout > 0 && !c.disconnected && time.Since(c.lastAccess) > c.idleTimeout {
					c.goIdleLocked()
					continue
				}
				c.mu.Unlock()
				c.readFrame()
			}
		}
	})
}

// readFrame reads one frame into the buffer. Must be called without mu held.
func (c *webcam) readFrame() {
	c.mu.Lock()
	if c.disconnected || c.reader == nil {
		c.mu.Unlock()
		return
	}
	reader := c.reader
	// Release and read happen outside the lock to avoid holding it during I/O.
	oldRelease := c.buffer.release
	c.buffer.release = nil
	c.mu.Unlock()

	if oldRelease != nil {
		oldRelease()
	}
	img, release, err := reader.Read()

	c.mu.Lock()
	defer c.mu.Unlock()
	c.buffer.err = err
	if err != nil {
		c.buffer.release = nil
		c.buffer.frame = nil
		c.logger.Errorw("error reading frame", "error", err)
		return
	}
	c.buffer.frame = img
	c.buffer.release = release
}

// goIdleLocked closes the camera until the next Images call. Must be called with mu held; releases mu.
func (c *webcam) goIdleLocked() {
	oldDriver := c.driver
	oldRelease := c.buffer.release
	c.driver = nil
	c.reader = nil
	c.buffer.release = nil
	c.buffer.frame = nil
	c.idle = true
	c.readyCh = make(chan struct{})
	logger := c.logger
	c.mu.Unlock()

	logger.Infow("no frames requested; closing camera until next request", "idle_timeout", c.idleTimeout.String())
	if oldRelease != nil {
		oldRelease()
	}
	if oldDriver != nil {
		if err := oldDriver.Close(); err != nil {
			logger.Errorw("failed to close idle camera", "error", err)
		}
	}
}

// wake reopens an idle camera, reads a first frame, and releases Images callers blocked on readyCh.
func (c *webcam) wake() {
	c.mu.Lock()
	if !c.idle {
		c.mu.Unlock()
		return
	}
	conf := c.conf
	targetPath := c.targetPath
	logger := c.logger
	c.mu.Unlock()

	reader, driver, _, err := c.openCamera(&conf, targetPath, logger)

	c.mu.Lock()
	if err != nil {
		logger.Errorw("failed to reopen idle camera", "error", err)
		c.buffer.err = fmt.Errorf("failed to reopen idle camera: %w", err)
		close(c.readyCh)
		c.readyCh = make(chan struct{})
		c.mu.Unlock()
		return
	}
	c.reader = reader
	c.driver = driver
	c.idle = false
	c.buffer.err = nil
	c.lastAccess = time.Now()
	c.mu.Unlock()

	logger.Info("camera reopened after idle")
	c.readFrame()

	c.mu.Lock()
	close(c.readyCh)
	c.readyCh = nil
	c.mu.Unlock()
}

// waitForWakeLocked signals the buffer worker to reopen an idle camera and waits for the attempt
// to finish. Must be called with mu held; mu is released while waiting and re-acquired on return.
func (c *webcam) waitForWakeLocked(ctx context.Context) error {
	ready := c.readyCh
	if c.idle {
		select {
		case c.wakeCh <- struct{}{}:
		default:
		}
	}
	c.mu.Unlock()
	defer c.mu.Lock()

	timer := time.NewTimer(wakeTimeout)
	defer timer.Stop()
	select {
	case <-ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return errors.New("timed out waiting for idle camera to reopen")
	}
}

func (c *webcam) Images(ctx context.Context, _ []string, _ map[string]interface{}) ([]camera.NamedImage, resource.ResponseMetadata, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.ensureActive(); err != nil {
		return nil, resource.ResponseMetadata{}, err
	}
	c.lastAccess = time.Now()

	if c.buffer.frame == nil && c.readyCh != nil {
		if err := c.waitForWakeLocked(ctx); err != nil {
			return nil, resource.ResponseMetadata{}, err
		}
		if err := c.ensureActive(); err != nil {
			return nil, resource.ResponseMetadata{}, err
		}
	}

	if c.buffer.frame == nil {
		if c.buffer.err != nil {
			return nil, resource.ResponseMetadata{}, c.buffer.err
		}
		return nil, resource.ResponseMetadata{}, errNoFrames
	}

	img := c.buffer.frame
	if c.conf.Width != 0 && c.conf.Height != 0 {
		if img.Bounds().Dx() != c.conf.Width || img.Bounds().Dy() != c.conf.Height {
			if time.Since(c.lastResWarnTime) > resWarnInterval {
				c.lastResWarnTime = time.Now()
				c.logger.Warnf(
					"requested width and height (%dx%d) do not match actual webcam resolution (%dx%d); using actual resolution",
					c.conf.Width, c.conf.Height, img.Bounds().Dx(), img.Bounds().Dy())
			}
		}
	}
	namedImg, err := camera.NamedImageFromImage(img, c.Name().Name, utils.MimeTypeJPEG, data.Annotations{})
	if err != nil {
		return nil, resource.ResponseMetadata{}, fmt.Errorf("failed to create named image: %w", err)
	}

	return []camera.NamedImage{namedImg}, resource.ResponseMetadata{CapturedAt: time.Now()}, nil
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
	if c.readyCh != nil {
		close(c.readyCh)
		c.readyCh = nil
	}

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
