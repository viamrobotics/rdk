// Package ffmpeg provides an implementation for an ffmpeg based camera
package ffmpeg

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"

	ffmpeg "github.com/u2takey/ffmpeg-go"
	viamutils "go.viam.com/utils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage/transform"
)

// Config is the attribute struct for ffmpeg cameras.
type Config struct {
	CameraParameters     *transform.PinholeCameraIntrinsics `json:"intrinsic_parameters,omitempty"`
	DistortionParameters *transform.BrownConrady            `json:"distortion_parameters,omitempty"`
	VideoPath            string                             `json:"video_path"`
	InputKWArgs          map[string]interface{}             `json:"input_kw_args,omitempty"`
	Filters              []FilterConfig                     `json:"filters,omitempty"`
	OutputKWArgs         map[string]interface{}             `json:"output_kw_args,omitempty"`
}

// FilterConfig is a struct to used to configure ffmpeg filters.
type FilterConfig struct {
	Name   string                 `json:"name"`
	Args   []string               `json:"args"`
	KWArgs map[string]interface{} `json:"kw_args"`
}

// Validate ensures all parts of the config are valid.
func (cfg *Config) Validate(path string) ([]string, error) {
	if cfg.CameraParameters != nil {
		if cfg.CameraParameters.Height < 0 || cfg.CameraParameters.Width < 0 {
			return nil, fmt.Errorf(
				"got illegal negative dimensions for width_px and height_px (%d, %d) fields set in intrinsic_parameters for ffmpeg camera",
				cfg.CameraParameters.Width, cfg.CameraParameters.Height)
		}
	}
	return []string{}, nil
}

var model = resource.DefaultModelFamily.WithModel("ffmpeg")

func init() {
	resource.RegisterComponent(camera.API, model, resource.Registration[camera.Camera, *Config]{
		Constructor: func(
			ctx context.Context, _ resource.Dependencies, conf resource.Config, logger logging.Logger,
		) (camera.Camera, error) {
			newConf, err := resource.NativeConfig[*Config](conf)
			if err != nil {
				return nil, err
			}

			src, err := NewFFMPEGCamera(ctx, newConf, logger)
			if err != nil {
				return nil, err
			}
			return camera.FromVideoSource(conf.ResourceName(), src, logger), nil
		},
	})
}

type ffmpegCamera struct {
	resource.Named
	gostream.VideoReader
	cancelFunc              context.CancelFunc
	activeBackgroundWorkers sync.WaitGroup
	inClose                 func() error
	outClose                func() error
	logger                  logging.Logger
}

type stderrWriter struct {
	logger logging.Logger
}

func (writer stderrWriter) Write(p []byte) (n int, err error) {
	writer.logger.Debug(string(p))
	return len(p), nil
}

// NewFFMPEGCamera instantiates a new camera which leverages ffmpeg to handle a variety of potential video types.
func NewFFMPEGCamera(ctx context.Context, conf *Config, logger logging.Logger) (camera.VideoSource, error) {
	// make sure ffmpeg is in the path before doing anything else
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, err
	}
	// parse attributes into ffmpeg keyword maps
	outArgs := make(map[string]interface{}, len(conf.OutputKWArgs))
	for key, value := range conf.OutputKWArgs {
		outArgs[key] = value
	}
	outArgs["update"] = 1        // always interpret the filename as just a filename, not a pattern
	outArgs["format"] = "image2" // select image file muxer, used to write video frames to image files

	// instantiate camera with cancellable context that will be applied to all spawned processes
	cancelableCtx, cancel := context.WithCancel(context.Background())
	ffCam := &ffmpegCamera{cancelFunc: cancel, logger: logger}

	// We configure ffmpeg to output images to stdout. A goroutine will read those images via the
	// `in` end of the pipe.
	in, out := io.Pipe()

	// Note(erd): For some reason, when running with the race detector, we need to close the pipe
	// even if we kill the process in order for the underlying command Wait to complete.
	ffCam.inClose = in.Close
	ffCam.outClose = out.Close

	// We will launch two goroutines:
	// - One to shell out to ffmpeg and wait on it exiting.
	// - Another to read the image output of ffmpeg and write it to a shared pointer.
	//
	// In addition, there are two other actors in this system:
	// - The application servicing GetImage and video streams will execute the callback registered
	//   via `VideoReaderFunc`.
	// - The robot reconfigure goroutine. All reconfigures are processed as a `Close` followed by
	//   `NewFFMPEGCamera`.
	ffCam.activeBackgroundWorkers.Add(1)
	viamutils.ManagedGo(func() {
		for {
			select {
			case <-cancelableCtx.Done():
				return
			default:
			}
			stream := ffmpeg.Input(conf.VideoPath, conf.InputKWArgs)
			for _, filter := range conf.Filters {
				stream = stream.Filter(filter.Name, filter.Args, filter.KWArgs)
			}
			stream = stream.Output("pipe:", outArgs)
			stream.Context = cancelableCtx

			// The `ffmpeg` command can return for two reasons:
			// - This camera object was `Close`d. Which will close the I/O of ffmpeg causing it to
			//   shutdown.
			// - (Dan): I've observed ffmpeg just returning on its own accord, approximately every
			//   30 seconds. Only on a rpi. This is presumably due to some form of resource
			//   exhaustion.
			//
			// We always want to return to the top of the loop to check the `cancelableCtx`. If the
			// camera was explicitly closed, this goroutine will observe `cancelableCtx` as canceled
			// and gracefully shutdown. If the camera was not closed, we restart ffmpeg. We depend
			// on golang's Command object to not close the I/O pipe such that it can be reused
			// across process invocations.
			cmd := stream.WithOutput(out).WithErrorOutput(stderrWriter{
				logger: logger,
			}).Compile()
			logger.Infow("Execing ffmpeg", "cmd", cmd.String())
			err := cmd.Run()
			logger.Debugw("ffmpeg exited", "err", err)
		}
	}, func() {
		ffCam.activeBackgroundWorkers.Done()
	})

	var latestFrame atomic.Pointer[image.Image]
	// Pause the GetImage reader until the producer provides a first item.
	var gotFirstFrameOnce bool
	gotFirstFrame := make(chan struct{})

	ffCam.activeBackgroundWorkers.Add(1)
	viamutils.ManagedGo(func() {
		for {
			if cancelableCtx.Err() != nil {
				return
			}
			img, err := jpeg.Decode(in)
			if err != nil {
				continue
			}
			latestFrame.Store(&img)
			if !gotFirstFrameOnce {
				close(gotFirstFrame)
				gotFirstFrameOnce = true
			}
		}
	}, ffCam.activeBackgroundWorkers.Done)

	// when next image is requested simply load the image from where it is stored in shared memory
	reader := gostream.VideoReaderFunc(func(ctx context.Context) (image.Image, func(), error) {
		select {
		case <-cancelableCtx.Done():
			return nil, nil, cancelableCtx.Err()
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-gotFirstFrame:
		}
		latest := latestFrame.Load()
		if latest == nil {
			return nil, func() {}, errors.New("no frame yet")
		}
		return *latest, func() {}, nil
	})

	ffCam.VideoReader = reader
	return camera.NewVideoSourceFromReader(
		ctx,
		ffCam,
		&transform.PinholeCameraModel{PinholeCameraIntrinsics: conf.CameraParameters},
		camera.ColorStream)
}

func (fc *ffmpegCamera) Close(ctx context.Context) error {
	fc.cancelFunc()
	viamutils.UncheckedError(fc.inClose())
	viamutils.UncheckedError(fc.outClose())
	fc.activeBackgroundWorkers.Wait()
	return nil
}
