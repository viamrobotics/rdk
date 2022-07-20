// Package ffmpeg provides an implementation for ffmpeg based cameras
package ffmpeg

import (
	"context"
	"image"
	"image/jpeg"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	ffmpeg "github.com/u2takey/ffmpeg-go"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/utils"
	viamutils "go.viam.com/utils"
)

// ffmpegAttrs is the attribute struct for ffmpeg cameras.
type ffmpegAttrs struct {
	*camera.AttrConfig
	Source       string                 `json:"source"`
	InputKWArgs  map[string]interface{} `json:"input_kw_args"`
	OutputKWArgs map[string]interface{} `json:"output_kw_args"`
}

// type inputAttrs struct {
// 	source
// }

// type filterAttrs struct {
// 	filterName string
// 	args       []string
// 	kWArgs     []string
// }

const model = "ffmpeg"

func init() {
	registry.RegisterComponent(camera.Subtype, model, registry.Component{
		Constructor: func(ctx context.Context, _ registry.Dependencies, cfg config.Component, logger golog.Logger) (interface{}, error) {
			attrs, ok := cfg.ConvertedAttributes.(*ffmpegAttrs)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(attrs, cfg.ConvertedAttributes)
			}
			return NewFFmpegCamera(attrs, logger)
		},
	})

	config.RegisterComponentAttributeMapConverter(
		camera.SubtypeName,
		model,
		func(attributes config.AttributeMap) (interface{}, error) {
			cameraAttrs, err := camera.CommonCameraAttributes(attributes)
			if err != nil {
				return nil, err
			}
			var conf ffmpegAttrs
			attrs, err := config.TransformAttributeMapToStruct(&conf, attributes)
			if err != nil {
				return nil, err
			}
			result, ok := attrs.(*ffmpegAttrs)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(result, attrs)
			}
			result.AttrConfig = cameraAttrs
			return result, nil
		},
		&ffmpegAttrs{},
	)
}

type ffmpegCamera struct {
	gostream.ImageSource
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

func NewFFmpegCamera(attrs *ffmpegAttrs, logger golog.Logger) (camera.Camera, error) {
	// make sure ffmpeg is in the path before doing anything else
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, err
	}

	// parse attributes into ffmpeg keyword maps
	outArgs := make(map[string]interface{}, len(attrs.OutputKWArgs))
	for key, value := range attrs.OutputKWArgs {
		outArgs[key] = value
	}
	outArgs["update"] = 1
	outArgs["format"] = "image2"

	// instantiate camera with cancellable context that will be applied to all spawned processes
	cancelableCtx, cancel := context.WithCancel(context.Background())
	ffCam := &ffmpegCamera{cancelFunc: cancel}

	// launch thread to run ffmpeg and pull images from the url and put them into the pipe
	in, out := io.Pipe()
	var ffmpegErr atomic.Value
	ffCam.activeBackgroundWorkers.Add(1)
	viamutils.ManagedGo(func() {
		stream := ffmpeg.Input(attrs.Source, attrs.InputKWArgs)
		// for filter := range attrs.Filters {
		// 	stream.Filter()
		// }
		stream = stream.Output("pipe:", outArgs)
		stream.Context = cancelableCtx
		if err := stream.WithOutput(out).Run(); err != nil {
			ffmpegErr.Store(err)
		}
	}, func() {
		cancel()
		ffCam.activeBackgroundWorkers.Done()
	})

	// launch thread to consume images from the pipe and store the latest in shared memory
	gotFirstFrame := make(chan struct{})
	var latestFrame atomic.Value
	var gotFirstFrameOnce bool
	ffCam.activeBackgroundWorkers.Add(1)
	viamutils.ManagedGo(func() {
		for {
			if cancelableCtx.Err() != nil {
				return
			}
			if ffmpegErr.Load() != nil {
				return
			}
			img, err := jpeg.Decode(in)
			if err != nil {
				continue
			}
			latestFrame.Store(img)
			if !gotFirstFrameOnce {
				close(gotFirstFrame)
				gotFirstFrameOnce = true
			}
		}
	}, ffCam.activeBackgroundWorkers.Done)

	// when next image is requested simply load the image from where it is stored in shared memory
	ffCam.ImageSource = gostream.ImageSourceFunc(func(ctx context.Context) (image.Image, func(), error) {
		if ffmpegErr.Load() != nil {
			return nil, nil, ffmpegErr.Load().(error)
		}
		select {
		case <-cancelableCtx.Done():
			if ffmpegErr.Load() != nil {
				return nil, nil, ffmpegErr.Load().(error)
			}
			return nil, nil, cancelableCtx.Err()
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-gotFirstFrame:
		}
		return latestFrame.Load().(image.Image), func() {}, nil
	})

	return camera.New(ffCam, attrs.AttrConfig, nil)
}

func (fc *ffmpegCamera) Close() {
	fc.cancelFunc()
	fc.activeBackgroundWorkers.Wait()
}
