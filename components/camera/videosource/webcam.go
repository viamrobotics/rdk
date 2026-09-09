// Package videosource implements webcam. It should be renamed webcam.
package videosource

import (
	"context"
	"fmt"
	"image"
	"image/jpeg"
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
	// defaultWakeTimeout bounds how long Images waits for an idle camera to reopen and deliver a frame.
	defaultWakeTimeout = 15 * time.Second
)

// openCameraFunc opens the camera described by the config; findReaderAndDriver in production.
type openCameraFunc func(*WebcamConfig, string, logging.Logger) (video.Reader, driverutils.Driver, string, error)

// idleState tracks where the camera is in the idle-timeout lifecycle.
type idleState uint8

const (
	stateStreaming idleState = iota // camera open, buffer worker reading frames
	stateIdle                       // camera closed for inactivity; Images asks the buffer worker to wake it
	stateWaking                     // buffer worker reopening the camera; Images callers wait on readyCh
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
	IdleTimeoutMs        int                                `json:"idle_timeout_ms,omitempty"`
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
	if c.IdleTimeoutMs < 0 {
		return nil, nil, fmt.Errorf(
			"got illegal negative idle timeout (%d) field set for webcam camera",
			c.IdleTimeoutMs)
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
	disconnected bool // set by monitor worker, or by a failed reopen after idle

	// idleTimeout > 0: buffer worker closes the camera after idleTimeout without an Images call and
	// reopens it when Images signals wakeCh. readyCh is non-nil whenever idleState != stateStreaming
	// and is closed once the reopen attempt finishes, successfully or not.
	idleTimeout time.Duration
	wakeTimeout time.Duration
	lastAccess  time.Time
	idleState   idleState
	wakeCh      chan struct{}
	readyCh     chan struct{}
	openCamera  openCameraFunc

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

	nativeConf, err := resource.NativeConfig[*WebcamConfig](conf)
	if err != nil {
		return nil, err
	}

	name := conf.ResourceName()
	logger = logger.WithFields("camera_name", name.ShortName())
	targetPath := nativeConf.Path
	reader, driver, label, err := findReaderAndDriver(nativeConf, targetPath, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to find camera: %w", err)
	}
	if targetPath == "" {
		targetPath = label
	}
	logger = logger.WithFields("camera_label", targetPath)

	return newWebcam(name, *nativeConf, targetPath, reader, driver, findReaderAndDriver, logger), nil
}

// newWebcam wraps an already-open camera and starts the monitor and buffer workers.
func newWebcam(
	name resource.Name,
	conf WebcamConfig,
	targetPath string,
	reader video.Reader,
	driver driverutils.Driver,
	openCamera openCameraFunc,
	logger logging.Logger,
) *webcam {
	if conf.FrameRate == 0.0 {
		conf.FrameRate = defaultFrameRate
	}
	c := &webcam{
		Named:       name.AsNamed(),
		workers:     goutils.NewBackgroundStoppableWorkers(),
		cameraModel: camera.NewPinholeModelWithBrownConradyDistortion(conf.CameraParameters, conf.DistortionParameters),
		reader:      reader,
		driver:      driver,
		targetPath:  targetPath,
		conf:        conf,
		idleTimeout: time.Duration(conf.IdleTimeoutMs) * time.Millisecond,
		wakeTimeout: defaultWakeTimeout,
		lastAccess:  time.Now(),
		wakeCh:      make(chan struct{}, 1),
		openCamera:  openCamera,
		logger:      logger,
		buffer:      newWebcamBuffer(),
	}
	c.startMonitorWorker()
	c.startBufferWorker()
	return c
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

// detachLocked clears the open camera and buffered frame, returning what the caller must hand to
// closeCamera outside the lock. Must be called with mu held.
func (c *webcam) detachLocked() (driverutils.Driver, func()) {
	driver, release := c.driver, c.buffer.release
	c.driver = nil
	c.reader = nil
	c.buffer.release = nil
	c.buffer.frame = nil
	return driver, release
}

// attachLocked installs a freshly opened camera and clears stale buffer errors. Must be called with mu held.
func (c *webcam) attachLocked(reader video.Reader, driver driverutils.Driver) {
	c.reader = reader
	c.driver = driver
	c.disconnected = false
	c.buffer.err = nil
}

// closeCamera releases a buffered frame and closes a driver. Performs I/O, so must be called without mu held.
func closeCamera(driver driverutils.Driver, release func()) error {
	if release != nil {
		release()
	}
	if driver != nil {
		return driver.Close()
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
// A failed reopen after idle also marks the camera disconnected, so this worker owns all recovery.
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
				if c.idleState != stateStreaming {
					c.mu.Unlock()
					continue
				}
				logger := c.logger
				driver := c.driver
				disconnected := c.disconnected
				c.mu.Unlock()

				if !disconnected {
					ok, err := isCameraConnected(driver)
					if err != nil {
						logger.Debugw("cannot determine camera status", "error", err)
						continue
					}
					if ok {
						continue
					}

					c.mu.Lock()
					// The camera may have gone idle during the probe; reconnecting then would leak a driver on wake.
					if c.idleState != stateStreaming {
						c.mu.Unlock()
						continue
					}
					c.disconnected = true
					c.mu.Unlock()

					logger.Error("camera no longer connected; reconnecting")
				}
			reconnectLoop:
				for {
					select {
					case <-ctx.Done():
						c.logger.Debug("reconnect loop context done")
						return
					case <-ticker.C:
						c.mu.Lock()
						oldDriver, oldRelease := c.detachLocked()
						conf := c.conf
						targetPath := c.targetPath
						c.mu.Unlock()

						if err := closeCamera(oldDriver, oldRelease); err != nil {
							c.logger.Errorw("failed to close current camera", "error", err)
						}

						// Heavy I/O, so stays outside the lock.
						reader, driver, label, err := c.openCamera(&conf, targetPath, c.logger)
						if err != nil {
							c.logger.Debugw("failed to reconnect camera", "error", err)
							continue
						}

						c.mu.Lock()
						c.attachLocked(reader, driver)
						if c.targetPath == "" {
							c.targetPath = label
						}
						c.logger = c.logger.WithFields("camera_name", c.Name().ShortName(), "camera_label", c.targetPath)
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
				if c.idleExpired() {
					c.goIdle()
					continue
				}
				c.readFrame()
			}
		}
	})
}

// idleExpired reports whether the camera should be closed for inactivity.
func (c *webcam) idleExpired() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.idleTimeout > 0 && c.idleState == stateStreaming && !c.disconnected && time.Since(c.lastAccess) > c.idleTimeout
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

		var jpegErr jpeg.FormatError
		if errors.As(err, &jpegErr) {
			c.logger.Debugw("dropped corrupt frame (usually benign, investigate only if continuous)", "error", err)
		} else {
			c.logger.Errorw("error reading frame", "error", err)
		}
		return
	}
	c.buffer.frame = img
	c.buffer.release = release
}

// goIdle closes the camera until the next Images call. Must be called without mu held.
func (c *webcam) goIdle() {
	c.mu.Lock()
	if c.idleState != stateStreaming || c.disconnected {
		c.mu.Unlock()
		return
	}
	oldDriver, oldRelease := c.detachLocked()
	c.idleState = stateIdle
	c.readyCh = make(chan struct{})
	logger := c.logger
	c.mu.Unlock()

	logger.Debugw("no frames requested; closing camera until next request", "idle_timeout", c.idleTimeout.String())
	if err := closeCamera(oldDriver, oldRelease); err != nil {
		logger.Errorw("failed to close idle camera", "error", err)
	}
}

// wake reopens an idle camera, reads a first frame, and releases Images callers blocked on readyCh.
// On failure the camera is marked disconnected and the monitor worker takes over reconnection.
func (c *webcam) wake() {
	c.mu.Lock()
	if c.idleState != stateIdle {
		c.mu.Unlock()
		return
	}
	c.idleState = stateWaking
	conf := c.conf
	targetPath := c.targetPath
	logger := c.logger
	c.mu.Unlock()

	reader, driver, _, err := c.openCamera(&conf, targetPath, logger)

	c.mu.Lock()
	c.idleState = stateStreaming
	if err != nil {
		logger.Errorw("failed to reopen idle camera; reconnecting", "error", err)
		c.disconnected = true
		close(c.readyCh)
		c.readyCh = nil
		c.mu.Unlock()
		return
	}
	c.attachLocked(reader, driver)
	c.lastAccess = time.Now()
	c.mu.Unlock()

	logger.Debug("camera reopened after idle")
	c.readFrame()

	c.mu.Lock()
	close(c.readyCh)
	c.readyCh = nil
	c.mu.Unlock()
}

// waitForWake blocks until a reopen attempt signalled by ready finishes, the context ends, or timeout elapses.
func waitForWake(ctx context.Context, ready <-chan struct{}, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
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
	if err := c.ensureActive(); err != nil {
		c.mu.Unlock()
		return nil, resource.ResponseMetadata{}, err
	}
	c.lastAccess = time.Now()
	// readyCh is non-nil while idle or waking; ask the buffer worker to wake and wait outside the lock.
	ready, wakeTimeout := c.readyCh, c.wakeTimeout
	if c.idleState == stateIdle {
		select {
		case c.wakeCh <- struct{}{}:
		default:
		}
	}
	c.mu.Unlock()

	if ready != nil {
		if err := waitForWake(ctx, ready, wakeTimeout); err != nil {
			return nil, resource.ResponseMetadata{}, err
		}
	}

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

	oldDriver, oldRelease := c.detachLocked()
	c.mu.Unlock()

	if err := closeCamera(oldDriver, oldRelease); err != nil {
		return fmt.Errorf("webcam failed to close (failed to close camera driver): %w", err)
	}
	return nil
}
