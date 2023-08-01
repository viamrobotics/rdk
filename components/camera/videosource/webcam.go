package videosource

import (
	"bufio"
	"context"
	"encoding/json"
	"image"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/google/uuid"
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
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage/transform"
)

// ModelWebcam is the name of the webcam component.
var (
	ModelWebcam       = resource.DefaultModelFamily.WithModel("webcam")
	cameraDebugLogger CameraDebugLogger
)

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
	cameraDebugLogger.AddWebcam(ctx, cam, logger)
	if err := cam.Reconfigure(ctx, deps, conf); err != nil {
		cam.Close(ctx)
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

	localLogger := c.logger
	if cameraDebugLogger.activeCams[c].discoveryLogFilePath != "" {
		cameraDebugLogger.activeCams[c].discoveryLogFileMu.Lock()
		defer cameraDebugLogger.activeCams[c].discoveryLogFileMu.Unlock()

		// Clear the discovery log
		os.Truncate(cameraDebugLogger.activeCams[c].discoveryLogFilePath, 0)

		fileConsoleTeeLoggerConfig := golog.NewDebugLoggerConfig()
		fileConsoleTeeLoggerConfig.OutputPaths = append(fileConsoleTeeLoggerConfig.OutputPaths, cameraDebugLogger.activeCams[c].discoveryLogFilePath)
		fileConsoleTeeLogger, err := fileConsoleTeeLoggerConfig.Build()
		if err == nil {
			localLogger = fileConsoleTeeLogger.Sugar()
		} else {
			c.logger.Errorw("failed to open discovery service log file", err)
		}
	}

	newSrc, foundLabel, err := findAndMakeVideoSource(c.cancelCtx, conf, c.targetPath, localLogger)
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

func (c *monitoredWebcam) Images(ctx context.Context) ([]image.Image, time.Time, error) {
	if c, ok := c.underlyingSource.(camera.ImagesSource); ok {
		return c.Images(ctx)
	}
	img, release, err := camera.ReadImage(ctx, c.underlyingSource)
	if err != nil {
		return nil, time.Time{}, errors.Wrap(err, "monitoredWebcam: call to get Images failed")
	}
	defer func() {
		if release != nil {
			release()
		}
	}()
	return []image.Image{img}, time.Now(), nil
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

	cameraDebugLogger.numActiveCamsMu.Lock()
	cameraDebugLogger.numActiveCams--
	if cameraDebugLogger.numActiveCams == 0 {
		cameraDebugLogger.cancel()
		cameraDebugLogger.activeBackgroundWorker.Wait()
	}
	cameraDebugLogger.numActiveCamsMu.Unlock()

	return err
}

type CameraDebugInfo struct {
	discoveryLogFilePath string
	discoveryLogFileMu   sync.RWMutex
}

type CameraDebugLogger struct {
	gologger               golog.Logger
	activeCams             map[*monitoredWebcam]*CameraDebugInfo
	isRunning              bool
	isRunningMu            sync.Mutex
	activeBackgroundWorker sync.WaitGroup
	logFile                *os.File
	logFilePath            string
	cancelCtx              context.Context
	cancel                 func()
	numActiveCams          int
	numActiveCamsMu        sync.Mutex
}

func (logger *CameraDebugLogger) AddWebcam(ctx context.Context, cam *monitoredWebcam, gologger golog.Logger) {
	// This only supports Linux
	if runtime.GOOS != "linux" {
		return
	}

	logger.isRunningMu.Lock()
	start := !logger.isRunning
	logger.isRunning = true
	logger.isRunningMu.Unlock()

	if logger.activeCams == nil {
		logger.activeCams = make(map[*monitoredWebcam]*CameraDebugInfo)
	}

	if start {
		logger.cancelCtx, logger.cancel = context.WithCancel(context.Background())
		cameraDebugLogger.activeBackgroundWorker.Add(1)
		logger.gologger = gologger
	}

	discoveryLogFilePath := filepath.Join(config.ViamDotDir, "camera_discovery_"+cam.Name().Name+".txt")
	discoveryLogFile, err := os.Create(discoveryLogFilePath)
	if err != nil {
		logger.gologger.Errorw("failed to create discovery log file at "+discoveryLogFilePath, err)
		return
	}
	discoveryLogFile.Close()

	logger.activeCams[cam] = &CameraDebugInfo{discoveryLogFilePath, sync.RWMutex{}}

	logger.numActiveCamsMu.Lock()
	logger.numActiveCams++
	logger.numActiveCamsMu.Unlock()

	if !start {
		return
	}

	// TODO: If last write was >=24hr ago, truncate and then continue
	logger.logFilePath = filepath.Join(config.ViamDotDir, "debug_camera_component_"+uuid.NewString()+".txt")
	logger.logFile, err = os.Create(logger.logFilePath)
	if err != nil {
		logger.gologger.Errorw("failed to create camera debug log file at "+logger.logFilePath, err)
		return
	}

	goutils.ManagedGo(func() { logger.run() }, cameraDebugLogger.activeBackgroundWorker.Done)
}

func (logger *CameraDebugLogger) run() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	defer logger.logFile.Close()

	defer func() {
		logger.isRunningMu.Lock()
		defer logger.isRunningMu.Unlock()
		logger.isRunning = false
	}()

	for {
		select {
		case <-logger.cancelCtx.Done():
			return
		case <-ticker.C:
		}

		for cam := range logger.activeCams {
			select {
			case <-cam.cancelCtx.Done():
				delete(logger.activeCams, cam)
			default:
			}
		}
		if len(logger.activeCams) == 0 {
			return
		}

		writeHeading := func(heading string) {
			logger.logFile.WriteString("===== " + heading + " =====\n")
		}

		writeCommand := func(name string, arg ...string) {
			cmd := exec.Command(name, arg...)
			cmdOutput, err := cmd.CombinedOutput()
			if err != nil && err.Error() != "exit status 1" && !strings.HasPrefix(err.Error(), "signal:") {
				logger.gologger.Errorw("command "+name+" "+strings.Join(arg, " ")+" finished with error", err)
			}

			logger.logFile.WriteString(string(cmdOutput))
		}

		writeHeading("operating system")
		writeCommand("lsb_release", "--description", "--short")
		writeCommand("uname", "--kernel-name", "--kernel-release", "--kernel-version")
		writeCommand("uname", "--machine")
		logger.logFile.WriteString("\n")

		writeLockedCameraName := func(cam *monitoredWebcam) {
			logger.logFile.WriteString(cam.Name().Name + " with path " + cam.targetPath + "\n")
		}

		writeHeading("camera configs")
		for cam := range logger.activeCams {
			cam.mu.RLock()
			select {
			case <-cam.cancelCtx.Done():
				cam.mu.RUnlock()
				continue
			default:
			}

			writeLockedCameraName(cam)
			configJson, err := json.Marshal(cam.conf)
			cam.mu.RUnlock()

			if err != nil {
				logger.gologger.Error("failed to marshal config")
			}

			logger.logFile.Write(configJson)
			logger.logFile.WriteString("\n")
		}
		logger.logFile.WriteString("\n")

		writeHeading("camera devices")
		writeCommand("v4l2-ctl", "--list-devices")
		logger.logFile.WriteString("\n")

		writeHeading("camera compliance")
		for cam := range logger.activeCams {
			cam.mu.RLock()
			select {
			case <-cam.cancelCtx.Done():
				cam.mu.RUnlock()
				continue
			default:
			}

			writeLockedCameraName(cam)
			writeCommand("v4l2-compliance", "--device", "/dev/v4l/by-id/"+cam.targetPath)
			cam.mu.RUnlock()
		}
		logger.logFile.WriteString("\n")

		writeHeading("camera discovery")
		for cam := range logger.activeCams {
			cam.mu.RLock()
			select {
			case <-cam.cancelCtx.Done():
				cam.mu.RUnlock()
				continue
			default:
			}
			writeLockedCameraName(cam)
			cam.mu.RUnlock()

			logger.activeCams[cam].discoveryLogFileMu.RLock()
			discoveryLogFile, err := os.Open(logger.activeCams[cam].discoveryLogFilePath)
			if err != nil {
				logger.gologger.Errorw("failed to open camera discovery log file at "+logger.activeCams[cam].discoveryLogFilePath, err)
				continue
			}

			discoveryLogReader := bufio.NewScanner(discoveryLogFile)
			for discoveryLogReader.Scan() {
				line := discoveryLogReader.Text()
				if err != nil && err != io.EOF {
					logger.gologger.Errorw("failed to read camera discovery log file", err)
					break
				}

				// This regex matches for: Start of line. Non-whitespace
				// (timestamp). Whitespace. Non-whitespace (log level).
				// Whitespace. The rest (to the end of the line) is what we
				// capture.
				regex := regexp.MustCompile(`^\S+\s+\S+\s+(.*)$`)
				matchArr := regex.FindStringSubmatch(line)
				if len(matchArr) > 1 {
					line = matchArr[1]
				}

				logger.logFile.WriteString(line + "\n")
			}
			discoveryLogFile.Close()
			logger.activeCams[cam].discoveryLogFileMu.RUnlock()

			logger.logFile.WriteString("\n\n")
		}

		writeHeading("can each open camera retrieve an image?")
		for cam := range logger.activeCams {
			func() {
				cam.mu.RLock()
				defer cam.mu.RUnlock()
				select {
				case <-cam.cancelCtx.Done():
					return
				default:
				}
				writeLockedCameraName(cam)
				if cam.underlyingSource == nil {
					logger.logFile.WriteString("\tunderlying source is nil\n")
					return
				}
				stream, err := cam.underlyingSource.Stream(context.Background())
				if err != nil {
					logger.logFile.WriteString("\terror creating stream\n")
					return
				}
				img, _, err := stream.Next(context.Background())
				if err != nil {
					logger.logFile.WriteString("\terror getting image from stream\n")
					return
				}
				if img == nil {
					logger.logFile.WriteString("\tgot nil image from stream\n")
					return
				}

				logger.logFile.WriteString("\tyes\n")
			}()
		}
		logger.logFile.WriteString("\n")

		// Reduce file size if necessary
		fileInfo, err := logger.logFile.Stat()
		if err == nil {
			// 512 kB
			if fileInfo.Size() > 512_000 {
				// Discard the top half of the file by writing the bottom half
				// to a temp file and copying it over
				tmpFilePath := filepath.Join(os.TempDir(), uuid.NewString())
				tmpFile, err := os.Create(tmpFilePath)
				if err != nil {
					logger.gologger.Errorw("failed to create temp file", err)
				}
				buf := make([]byte, 1024)
				totalBytes := int64(0)
				for {
					n, err := logger.logFile.ReadAt(buf, totalBytes)
					if err != nil && err != io.EOF {
						logger.gologger.Errorw("failed to read log file", err)
						break
					}
					if n == 0 {
						break
					}

					if totalBytes > fileInfo.Size()/2 {
						tmpFile.Write(buf[:n])
					}
					totalBytes += int64(n)
				}
				tmpFile.Close()
				logger.logFile.Close()

				os.Rename(tmpFilePath, logger.logFilePath)

				logger.logFile, err = os.OpenFile(logger.logFilePath, os.O_RDWR, 0666)
				if err != nil {
					logger.gologger.Errorw("failed to reopen camera debug log file at "+logger.logFilePath, err)
				}
			}
		} else {
			logger.gologger.Error("failed to read log file size")
		}
	}
}
